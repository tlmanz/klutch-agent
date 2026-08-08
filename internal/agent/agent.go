// Package agent is the headless core of the Klutch print agent: it holds the WSS
// connection to the backend, enumerates and dispatches to local printers,
// persists outcomes to the local store, and self-updates. It is UI-agnostic: the
// Fyne UI (internal/ui) observes it through Snapshot + Subscribe and drives it
// through its exported methods, and a headless mode runs it with no UI at all.
package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/tlmanz/klutch-agent/internal/store"
)

// Config is the agent's runtime configuration. Values are seeded from flags/env
// and then overlaid with anything persisted in the store (the store wins, since
// the UI edits it).
type Config struct {
	Server          string
	FallbackPrinter string
	SpoolDir        string
	Refresh         time.Duration
	UpdateURL       string
	UpdateInterval  time.Duration
	AutoUpdate      bool
	Version         string // this build's version, for update comparisons
}

// State is an immutable snapshot of what the UI renders. Snapshot returns a copy
// so the caller can read it without holding the lock.
type State struct {
	Server           string
	Enrolled         bool
	Connected        bool
	Version          string
	AvailableVersion string // newer version the manifest advertises, "" if current
	LastCheck        time.Time
	LastError        string
	Printers         []PrinterInfo
	ActiveJobs       []JobInfo // in-flight / recently-finished live jobs
	JobsOK           int
	JobsFailed       int
	AutoUpdate       bool

	// Redesign settings, persisted in the store.
	DefaultPrinter string
	Theme          string // "dark" | "light"
	NotifyDone     bool
	NotifyFailed   bool
	NotifyWeekly   bool
}

// Agent owns the connection lifecycle and mutable state.
type Agent struct {
	cfg   Config
	store *store.Store
	log   *log.Logger

	mu    sync.Mutex
	state State
	token string

	// jobs tracks in-flight/recently-finished live jobs by ID, guarded by mu.
	// It feeds State.ActiveJobs; completed jobs still roll into the store history.
	jobs map[string]*JobInfo

	// osJobs is the last scan of the OS print queues (jobs the agent did not
	// necessarily dispatch), merged with jobs in Snapshot so the Jobs screen
	// reflects the real queue. Guarded by mu.
	osJobs []JobInfo

	// outcomeHook, if set by the UI, is called with each terminal job so the UI
	// can raise a desktop notification (respecting the user's prefs). The agent
	// core stays UI-agnostic; it just fires the event.
	outcomeHook func(store.JobRecord)

	// runCtx is the process-lifetime context captured by Run. Job handling uses it
	// rather than the per-connection context, so dropping the socket (a manual
	// reconnect, a server change) never kills a print already on its way to the
	// spooler. Guarded by mu.
	runCtx context.Context

	// connCancel tears down the live connection so the run loop redials at once;
	// nil when nothing is connected. manualDrop records that the teardown was ours,
	// so the resulting "context canceled" is not reported to the user as a fault.
	// Both guarded by mu.
	connCancel context.CancelFunc
	manualDrop bool

	// previews caches the last decoded local file so dragging a slider in the
	// print screen does not re-decode a 12-megapixel photo on every frame.
	previews previewCache

	enrolled chan struct{} // signalled when a token becomes available or server changes

	subsMu sync.Mutex
	subs   map[chan struct{}]struct{}

	// relaunch replaces the process after a self-update; set by the caller
	// (the UI restarts the app; headless re-execs). Defaults to Relaunch.
	relaunch func()
}

// New builds an agent, loading persisted settings + counters from the store.
func New(cfg Config, st *store.Store, logger *log.Logger) *Agent {
	a := &Agent{
		cfg:      cfg,
		store:    st,
		log:      logger,
		enrolled: make(chan struct{}, 1),
		subs:     make(map[chan struct{}]struct{}),
		relaunch: Relaunch,
	}
	a.state.Server = cfg.Server
	a.state.Version = cfg.Version
	a.state.AutoUpdate = cfg.AutoUpdate

	// Overlay persisted settings.
	if v, _ := st.Setting(store.KeyServer); v != "" {
		a.cfg.Server = v
		a.state.Server = v
	}
	if v, _ := st.Setting(store.KeyToken); v != "" {
		a.token = v
		a.state.Enrolled = true
	}
	if v, _ := st.Setting(store.KeyUpdateURL); v != "" {
		a.cfg.UpdateURL = v
	}
	if v, _ := st.Setting(store.KeyAutoUpdate); v != "" {
		a.cfg.AutoUpdate = v == "1"
		a.state.AutoUpdate = a.cfg.AutoUpdate
	}
	if v, _ := st.Setting(store.KeyAvailableVersion); v != "" && versionLess(cfg.Version, v) {
		a.state.AvailableVersion = v
	}
	ok, failed, _ := st.JobCounts()
	a.state.JobsOK, a.state.JobsFailed = ok, failed
	if ps, _ := st.Printers(); len(ps) > 0 {
		wp := make([]PrinterInfo, len(ps))
		for i, p := range ps {
			// Seed from persistence with unknown live status; the first connect
			// re-enumerates and fills status/connection/queue.
			wp[i] = PrinterInfo{Name: p.Name, Description: p.Description, Status: "idle", Connection: "Wi-Fi"}
		}
		a.state.Printers = wp
	}

	// Redesign settings.
	a.state.DefaultPrinter, _ = st.Setting(store.KeyDefaultPrinter)
	a.state.Theme = "dark"
	if v, _ := st.Setting(store.KeyTheme); v == "light" || v == "dark" {
		a.state.Theme = v
	}
	a.state.NotifyDone = settingBool(st, store.KeyNotifyDone, true)
	a.state.NotifyFailed = settingBool(st, store.KeyNotifyFailed, true)
	a.state.NotifyWeekly = settingBool(st, store.KeyNotifyWeekly, false)
	a.jobs = map[string]*JobInfo{}
	return a
}

// settingBool reads a "1"/"0" setting, falling back to def when unset.
func settingBool(st *store.Store, key string, def bool) bool {
	v, _ := st.Setting(key)
	switch v {
	case "1":
		return true
	case "0":
		return false
	default:
		return def
	}
}

// SetRelaunch overrides how the process is replaced after a self-update.
func (a *Agent) SetRelaunch(fn func()) { a.relaunch = fn }

// Snapshot returns a copy of the current state for the UI to render.
func (a *Agent) Snapshot() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.state
	s.Printers = append([]PrinterInfo(nil), a.state.Printers...)
	// Merge agent-tracked jobs with the OS queue scan. Agent-tracked jobs win
	// (they carry the real document name + progress + controls); foreign OS jobs
	// are appended, deduplicated by CUPS request id.
	merged := append([]JobInfo(nil), a.state.ActiveJobs...)
	seen := make(map[string]bool, len(merged))
	for _, j := range merged {
		if j.ReqID != "" {
			seen[j.ReqID] = true
		}
	}
	for _, j := range a.osJobs {
		if j.ReqID != "" && seen[j.ReqID] {
			continue
		}
		merged = append(merged, j)
	}
	s.ActiveJobs = merged
	return s
}

// RecentJobs proxies the store so the UI has a single dependency (the agent).
func (a *Agent) RecentJobs(limit int) ([]store.JobRecord, error) {
	return a.store.RecentJobs(limit)
}

// Subscribe returns a channel that receives a (coalesced) signal whenever the
// state changes, plus a cancel func to unsubscribe. The UI redraws on each tick.
func (a *Agent) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	a.subsMu.Lock()
	a.subs[ch] = struct{}{}
	a.subsMu.Unlock()
	return ch, func() {
		a.subsMu.Lock()
		delete(a.subs, ch)
		a.subsMu.Unlock()
	}
}

// notify wakes every subscriber without blocking (coalesced: a subscriber that
// has not drained its last signal simply keeps the one it has).
func (a *Agent) notify() {
	a.subsMu.Lock()
	defer a.subsMu.Unlock()
	for ch := range a.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// mutate runs fn under the state lock and notifies subscribers.
func (a *Agent) mutate(fn func(*State)) {
	a.mu.Lock()
	fn(&a.state)
	a.mu.Unlock()
	a.notify()
}

// Run drives the agent until ctx is cancelled: it starts the background updater
// and then holds the connection open, redialing with backoff. While unenrolled
// it waits for Enroll.
func (a *Agent) Run(ctx context.Context) {
	if a.cfg.UpdateURL != "" {
		go a.updateLoop(ctx)
	}
	a.mu.Lock()
	a.runCtx = ctx
	a.mu.Unlock()

	backoff := time.Second
	for ctx.Err() == nil {
		a.mu.Lock()
		tok, server := a.token, a.cfg.Server
		a.mu.Unlock()

		if tok == "" {
			select {
			case <-ctx.Done():
				return
			case <-a.enrolled:
				continue
			}
		}

		// Each attempt gets its own context so Reconnect/Disconnect/SetServer can
		// drop the socket without waiting for the backend to notice.
		connCtx, cancel := context.WithCancel(ctx)
		a.mu.Lock()
		a.connCancel = cancel
		a.mu.Unlock()

		err := a.connect(connCtx, server, tok)
		cancel()

		a.mu.Lock()
		a.connCancel = nil
		manual := a.manualDrop
		a.manualDrop = false
		a.mu.Unlock()

		if ctx.Err() != nil {
			return
		}
		a.mutate(func(s *State) {
			s.Connected = false
			if err != nil && !manual {
				s.LastError = err.Error()
			}
		})
		if manual {
			// We closed it on purpose: redial straight away, and do not blame the
			// connection for the "context canceled" that resulted.
			backoff = time.Second
			continue
		}
		if err != nil {
			a.log.Printf("connection ended: %v (retrying in %s)", err, backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		case <-a.enrolled: // enrolled, or the server changed; reconnect immediately
		}
		if backoff < 30*time.Second {
			backoff *= 2
		} else {
			backoff = time.Second
		}
	}
}

// dropConnection closes the live connection, if there is one, so the run loop
// redials immediately. Callers that also changed the token or server should call
// wake afterwards, which covers the case where nothing was connected.
func (a *Agent) dropConnection() {
	a.mu.Lock()
	cancel := a.connCancel
	a.connCancel = nil
	a.manualDrop = cancel != nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// jobCtx is the context job handling runs under: the process lifetime, not the
// current connection, so a reconnect cannot cancel a print mid-dispatch.
func (a *Agent) jobCtx() context.Context {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.runCtx != nil {
		return a.runCtx
	}
	return context.Background()
}

// Reconnect drops the current connection and dials the backend again with the
// token this device already has. It is the "it went quiet, try again now" button;
// re-pairing is only needed when the token itself is gone.
func (a *Agent) Reconnect() error {
	a.mu.Lock()
	tok := a.token
	a.mu.Unlock()
	if tok == "" {
		return fmt.Errorf("this device is not paired yet")
	}
	a.mutate(func(s *State) { s.LastError = "" })
	a.dropConnection()
	a.wake()
	return nil
}

// Disconnect unpairs the device: it forgets the token, closes the connection and
// stops redialing. The UI falls back to onboarding, where the (still persisted)
// server URL can be re-paired or pointed somewhere else.
func (a *Agent) Disconnect() error {
	if err := a.store.SetSetting(store.KeyToken, ""); err != nil {
		return err
	}
	a.mu.Lock()
	a.token = ""
	a.mu.Unlock()
	a.mutate(func(s *State) {
		s.Enrolled = false
		s.Connected = false
		s.LastError = ""
	})
	a.dropConnection()
	a.log.Printf("disconnected from backend; device token cleared")
	return nil
}

// Enroll redeems a one-time pairing code for a device token, persists it, and
// kicks the connection loop.
func (a *Agent) Enroll(ctx context.Context, pairingCode string) error {
	a.mu.Lock()
	server := a.cfg.Server
	a.mu.Unlock()

	token, err := redeem(ctx, server, pairingCode)
	if err != nil {
		return err
	}
	if err := a.store.SetSetting(store.KeyToken, token); err != nil {
		a.log.Printf("warning: persist token: %v", err)
	}
	a.mu.Lock()
	a.token = token
	a.state.Enrolled = true
	a.state.LastError = ""
	a.mu.Unlock()
	a.notify()
	a.dropConnection() // an existing socket still carries the old token
	a.wake()
	return nil
}

// SetToken installs a pre-issued device token directly (no pairing), persists
// it, and kicks the connection loop. Used for provisioning (KLUTCH_AGENT_TOKEN)
// where a token is baked into the install instead of redeeming a pairing code.
func (a *Agent) SetToken(token string) error {
	if err := a.store.SetSetting(store.KeyToken, token); err != nil {
		return err
	}
	a.mu.Lock()
	a.token = token
	a.state.Enrolled = true
	a.state.LastError = ""
	a.mu.Unlock()
	a.notify()
	a.dropConnection()
	a.wake()
	return nil
}

// SetServer changes the backend URL, persists it, and forces a reconnect: the
// live socket is closed so the next dial goes to the new address instead of
// waiting for the old one to fail on its own.
func (a *Agent) SetServer(server string) error {
	if err := a.store.SetSetting(store.KeyServer, server); err != nil {
		return err
	}
	a.mutate(func(s *State) {
		s.Server = server
		s.LastError = ""
	})
	a.mu.Lock()
	a.cfg.Server = server
	a.mu.Unlock()
	a.dropConnection()
	a.wake()
	return nil
}

// SetAutoUpdate toggles automatic install of available updates.
func (a *Agent) SetAutoUpdate(on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	if err := a.store.SetSetting(store.KeyAutoUpdate, v); err != nil {
		return err
	}
	a.mu.Lock()
	a.cfg.AutoUpdate = on
	a.mu.Unlock()
	a.mutate(func(s *State) { s.AutoUpdate = on })
	return nil
}

// SetDefaultPrinter pins the OS default queue, persists the choice, and reflects
// it in state immediately (the next enumeration confirms it).
func (a *Agent) SetDefaultPrinter(name string) error {
	if a.isPlaceholder(name) {
		return fmt.Errorf("%q is not a printer on this computer - it is a placeholder shown because none is set up", name)
	}
	if err := setDefaultPrinter(context.Background(), name); err != nil {
		return err
	}
	if err := a.store.SetSetting(store.KeyDefaultPrinter, name); err != nil {
		return err
	}
	a.mutate(func(s *State) {
		s.DefaultPrinter = name
		for i := range s.Printers {
			s.Printers[i].Default = s.Printers[i].Name == name
		}
	})
	return nil
}

// SetTheme persists the UI theme preference ("dark"/"light").
func (a *Agent) SetTheme(theme string) error {
	if theme != "light" {
		theme = "dark"
	}
	if err := a.store.SetSetting(store.KeyTheme, theme); err != nil {
		return err
	}
	a.mutate(func(s *State) { s.Theme = theme })
	return nil
}

// setBoolSetting is the shared path for the notification toggles.
func (a *Agent) setBoolSetting(key string, on bool, apply func(*State)) error {
	v := "0"
	if on {
		v = "1"
	}
	if err := a.store.SetSetting(key, v); err != nil {
		return err
	}
	a.mutate(apply)
	return nil
}

// SetNotifyDone toggles the "job completed" desktop notification.
func (a *Agent) SetNotifyDone(on bool) error {
	return a.setBoolSetting(store.KeyNotifyDone, on, func(s *State) { s.NotifyDone = on })
}

// SetNotifyFailed toggles the "job failed" desktop notification.
func (a *Agent) SetNotifyFailed(on bool) error {
	return a.setBoolSetting(store.KeyNotifyFailed, on, func(s *State) { s.NotifyFailed = on })
}

// SetNotifyWeekly toggles the weekly-summary preference.
func (a *Agent) SetNotifyWeekly(on bool) error {
	return a.setBoolSetting(store.KeyNotifyWeekly, on, func(s *State) { s.NotifyWeekly = on })
}

// wake signals the run loop (non-blocking) that enrollment or the server changed.
func (a *Agent) wake() {
	select {
	case a.enrolled <- struct{}{}:
	default:
	}
}
