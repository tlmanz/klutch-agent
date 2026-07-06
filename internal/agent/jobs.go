package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tlmanz/klutch-agent/internal/store"
)

// JobInfo is a live (in-flight or just-finished) print job for the UI. Terminal
// outcomes still roll into the store history; this type only carries jobs the
// agent is actively tracking so the redesigned Jobs screen can show progress.
type JobInfo struct {
	ID      string
	Printer string
	Doc     string
	Kind    string
	State   string  // queued | printing | paused | failed | done
	Percent float64 // -1 = unknown / indeterminate
	ReqID   string  // CUPS request id (for pause/cancel); "" on Windows
	Started time.Time
}

// --- live-job registry (guarded by a.mu) ------------------------------------

func (a *Agent) putJob(j JobInfo) {
	a.mu.Lock()
	if a.jobs == nil {
		a.jobs = map[string]*JobInfo{}
	}
	cp := j
	a.jobs[j.ID] = &cp
	a.syncActiveLocked()
	a.mu.Unlock()
	a.notify()
}

func (a *Agent) updateJob(id string, fn func(*JobInfo)) {
	a.mu.Lock()
	if j, ok := a.jobs[id]; ok {
		fn(j)
		a.syncActiveLocked()
	}
	a.mu.Unlock()
	a.notify()
}

func (a *Agent) removeJob(id string) {
	a.mu.Lock()
	delete(a.jobs, id)
	a.syncActiveLocked()
	a.mu.Unlock()
	a.notify()
}

// syncActiveLocked rebuilds state.ActiveJobs (newest first). Caller holds a.mu.
func (a *Agent) syncActiveLocked() {
	list := make([]JobInfo, 0, len(a.jobs))
	for _, j := range a.jobs {
		list = append(list, *j)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Started.After(list[j].Started) })
	a.state.ActiveJobs = list
}

func (a *Agent) reqIDOf(id string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if j, ok := a.jobs[id]; ok {
		return j.ReqID
	}
	return ""
}

// --- outcome recording ------------------------------------------------------

// recordOutcome writes a terminal job to the store history and refreshes counts.
func (a *Agent) recordOutcome(rec store.JobRecord) {
	if err := a.store.RecordJob(rec); err != nil {
		a.log.Printf("warning: record job: %v", err)
	}
	a.refreshCounts()
	a.mu.Lock()
	hook := a.outcomeHook
	a.mu.Unlock()
	if hook != nil {
		hook(rec)
	}
}

// SetOutcomeHook registers a callback fired for each terminal job (for desktop
// notifications). Safe to call before Run.
func (a *Agent) SetOutcomeHook(fn func(store.JobRecord)) {
	a.mu.Lock()
	a.outcomeHook = fn
	a.mu.Unlock()
}

// --- CUPS live tracking -----------------------------------------------------

// trackCUPSJob polls the CUPS queue for reqID until the job leaves it, then
// records the outcome and drops it from the live set. Progress percentage is
// left indeterminate (CUPS rarely exposes reliable sheet counts on raw queues).
func (a *Agent) trackCUPSJob(ctx context.Context, rec store.JobRecord, reqID string) {
	deadline := time.Now().Add(15 * time.Minute)
	t := time.NewTicker(1500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if cupsJobPresent(ctx, reqID) {
			a.updateJob(rec.ID, func(j *JobInfo) {
				if j.State != "paused" {
					j.State = "printing"
				}
			})
			if time.Now().After(deadline) {
				break
			}
			continue
		}
		// Job left the queue: treat as completed.
		rec.FinishedAt = time.Now()
		rec.Status = "ok"
		a.recordOutcome(rec)
		a.removeJob(rec.ID)
		return
	}
	// Timed out watching: record best-effort and drop.
	rec.FinishedAt = time.Now()
	rec.Status = "ok"
	a.recordOutcome(rec)
	a.removeJob(rec.ID)
}

// cupsJobPresent reports whether reqID is still an active (not-completed) job.
func cupsJobPresent(ctx context.Context, reqID string) bool {
	out, err := exec.CommandContext(ctx, "lpstat", "-o").Output()
	if err != nil {
		return false
	}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) > 0 && f[0] == reqID {
			return true
		}
	}
	return false
}

// --- job controls (exported for the UI) -------------------------------------

// PauseJob holds a queued/printing job.
func (a *Agent) PauseJob(id string) error {
	req := a.reqIDOf(id)
	if req == "" {
		return fmt.Errorf("job is not active")
	}
	if err := cupsHold(context.Background(), req); err != nil {
		return err
	}
	a.updateJob(id, func(j *JobInfo) { j.State = "paused" })
	return nil
}

// ResumeJob releases a held job back to the queue.
func (a *Agent) ResumeJob(id string) error {
	req := a.reqIDOf(id)
	if req == "" {
		return fmt.Errorf("job is not active")
	}
	if err := cupsRelease(context.Background(), req); err != nil {
		return err
	}
	a.updateJob(id, func(j *JobInfo) { j.State = "printing" })
	return nil
}

// CancelJob removes a job from the queue and records it as failed (cancelled).
func (a *Agent) CancelJob(id string) error {
	req := a.reqIDOf(id)
	if req == "" {
		return fmt.Errorf("job is not active")
	}
	if err := cupsCancel(context.Background(), req); err != nil {
		return err
	}
	a.removeJob(id)
	return nil
}

// PauseAll holds every currently-active job.
func (a *Agent) PauseAll() error {
	a.mu.Lock()
	ids := make([]string, 0, len(a.jobs))
	for id, j := range a.jobs {
		if j.State != "paused" {
			ids = append(ids, id)
		}
	}
	a.mu.Unlock()
	var firstErr error
	for _, id := range ids {
		if err := a.PauseJob(id); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ReprintJob re-dispatches a finished job's spooled file, if still present.
func (a *Agent) ReprintJob(id string) error {
	jobs, err := a.store.RecentJobs(500)
	if err != nil {
		return err
	}
	var rec *store.JobRecord
	for i := range jobs {
		if jobs[i].ID == id {
			rec = &jobs[i]
			break
		}
	}
	if rec == nil {
		return fmt.Errorf("job not found")
	}
	path := filepath.Join(a.cfg.SpoolDir, id+"."+extFor(rec.Kind))
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("the spooled file is no longer available to reprint")
	}
	reqID, err := dispatch(context.Background(), rec.Printer, path)
	if err != nil {
		return err
	}
	newID := fmt.Sprintf("%s-reprint-%d", id, time.Now().UnixNano())
	a.putJob(JobInfo{
		ID: newID, Printer: rec.Printer, Doc: docName(rec.DocumentRef), Kind: rec.Kind,
		State: "printing", Percent: -1, ReqID: reqID, Started: time.Now(),
	})
	newRec := store.JobRecord{ID: newID, Printer: rec.Printer, Kind: rec.Kind, DocumentRef: rec.DocumentRef, ReceivedAt: time.Now()}
	if reqID == "" {
		newRec.FinishedAt = time.Now()
		newRec.Status = "ok"
		a.recordOutcome(newRec)
		a.removeJob(newID)
	} else {
		go a.trackCUPSJob(context.Background(), newRec, reqID)
	}
	return nil
}

// --- helpers ----------------------------------------------------------------

// extFor maps a job kind to its spool-file extension.
func extFor(kind string) string {
	switch kind {
	case "pdf":
		return "pdf"
	case "escpos_raster":
		return "escpos"
	default:
		return "bin"
	}
}

// docName is a friendly document label from a document ref/path.
func docName(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "Print job"
	}
	return filepath.Base(ref)
}
