package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/tlmanz/klutch-agent/internal/store"
	"github.com/tlmanz/klutch-agent/wire"
)

// wsClient serializes writes to the connection: the read loop (ACKs) and the
// re-enumeration ticker (printer refreshes) both write, and a WebSocket Conn
// permits only one writer at a time.
type wsClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *wsClient) send(ctx context.Context, env wire.Envelope) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return wsjson.Write(ctx, c.conn, env)
}

// connect dials the WS hub, advertises printers, and serves jobs until the
// connection closes or ctx is cancelled.
func (a *Agent) connect(ctx context.Context, server, token string) error {
	if err := os.MkdirAll(a.cfg.SpoolDir, 0o755); err != nil {
		return fmt.Errorf("spool dir: %w", err)
	}
	wsURL := toWS(server) + "/api/printing/agent/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}},
	})
	if err != nil {
		return err
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	conn.SetReadLimit(8 << 20) // jobs carry rendered payloads
	cl := &wsClient{conn: conn}

	// Advertise every printer connected to this PC.
	printers := enumeratePrinters(ctx, a.cfg.FallbackPrinter)
	a.recordPrinters(printers)
	hello, err := wire.Encode(wire.TypeHello, wire.Hello{Printers: printers})
	if err != nil {
		return err
	}
	if err := cl.send(ctx, hello); err != nil {
		return err
	}
	a.log.Printf("connected to %s, advertised %d printer(s)", wsURL, len(printers))
	a.mutate(func(s *State) {
		s.Connected = true
		s.LastError = ""
	})

	// Re-enumerate periodically and re-advertise ONLY when the set changed.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go a.watchPrinters(runCtx, cl, fingerprint(printers))

	for {
		var env wire.Envelope
		if err := wsjson.Read(ctx, conn, &env); err != nil {
			return err
		}
		if env.Type != wire.TypeJob {
			continue
		}
		var job wire.Job
		if err := json.Unmarshal(env.Data, &job); err != nil {
			a.log.Printf("bad job: %v", err)
			continue
		}
		result := a.handleJob(ctx, job)
		ack, _ := wire.Encode(wire.TypeJobResult, result)
		if err := cl.send(ctx, ack); err != nil {
			return err
		}
	}
}

// watchPrinters re-enumerates on a ticker and re-advertises only when the printer
// set's fingerprint changes, so an unchanged set produces no traffic.
func (a *Agent) watchPrinters(ctx context.Context, cl *wsClient, lastFP string) {
	t := time.NewTicker(a.cfg.Refresh)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			cur := enumeratePrinters(ctx, a.cfg.FallbackPrinter)
			fp := fingerprint(cur)
			if fp == lastFP {
				continue
			}
			a.recordPrinters(cur)
			env, err := wire.Encode(wire.TypePrinters, wire.Hello{Printers: cur})
			if err != nil {
				continue
			}
			if err := cl.send(ctx, env); err != nil {
				return
			}
			lastFP = fp
			a.log.Printf("printer set changed, re-advertised %d printer(s)", len(cur))
		}
	}
}

// recordPrinters updates the in-memory state and persists the set for the UI.
func (a *Agent) recordPrinters(printers []wire.Printer) {
	a.mutate(func(s *State) { s.Printers = append([]wire.Printer(nil), printers...) })
	recs := make([]store.PrinterRecord, len(printers))
	now := time.Now()
	for i, p := range printers {
		recs[i] = store.PrinterRecord{Name: p.Name, Description: p.Description, LastSeen: now}
	}
	if err := a.store.UpsertPrinters(recs); err != nil {
		a.log.Printf("warning: persist printers: %v", err)
	}
}

// handleJob spools a job's bytes to disk, hands the file to the OS print system,
// records the outcome in the store, and returns the ACK. The agent never renders
// (the backend does): it only transports the bytes to the target queue.
func (a *Agent) handleJob(ctx context.Context, job wire.Job) wire.JobResult {
	received := time.Now()
	rec := store.JobRecord{
		ID: job.ID, Printer: job.PrinterName, Kind: job.Kind,
		DocumentRef: job.DocumentRef, ReceivedAt: received,
	}
	finish := func(res wire.JobResult) wire.JobResult {
		rec.FinishedAt = time.Now()
		if res.OK {
			rec.Status = "ok"
		} else {
			rec.Status = "failed"
			rec.Error = res.Error
		}
		if err := a.store.RecordJob(rec); err != nil {
			a.log.Printf("warning: record job: %v", err)
		}
		a.refreshCounts()
		return res
	}

	payload, err := base64.StdEncoding.DecodeString(job.PayloadB64)
	if err != nil {
		return finish(wire.JobResult{JobID: job.ID, OK: false, Error: "bad payload encoding"})
	}
	rec.Bytes = len(payload)

	ext := "bin"
	switch job.Kind {
	case "pdf":
		ext = "pdf"
	case "escpos_raster":
		ext = "escpos"
	}
	path := filepath.Join(a.cfg.SpoolDir, job.ID+"."+ext)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return finish(wire.JobResult{JobID: job.ID, OK: false, Error: err.Error()})
	}
	if err := dispatch(ctx, job.PrinterName, path); err != nil {
		a.log.Printf("job %s dispatch to %q failed: %v", job.ID, job.PrinterName, err)
		return finish(wire.JobResult{JobID: job.ID, OK: false, Error: err.Error()})
	}
	a.log.Printf("printed job %s (%s, %d bytes) → printer %q (spooled %s)", job.ID, job.Kind, len(payload), job.PrinterName, path)
	return finish(wire.JobResult{JobID: job.ID, OK: true})
}

// refreshCounts reloads the ok/failed totals from the store into state.
func (a *Agent) refreshCounts() {
	ok, failed, err := a.store.JobCounts()
	if err != nil {
		return
	}
	a.mutate(func(s *State) { s.JobsOK, s.JobsFailed = ok, failed })
}

// redeem exchanges a pairing code for a device token over HTTP.
func redeem(ctx context.Context, server, code string) (string, error) {
	body, _ := json.Marshal(map[string]string{"code": code})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server+"/api/printing/agents/redeem", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("redeem failed: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", fmt.Errorf("redeem returned no token")
	}
	return out.Token, nil
}

// fingerprint is a stable digest of a printer set (order-independent).
func fingerprint(ps []wire.Printer) string {
	items := make([]string, len(ps))
	for i, p := range ps {
		items[i] = p.Name + "\x00" + p.Description
	}
	sort.Strings(items)
	return strings.Join(items, "\n")
}

func toWS(httpURL string) string {
	if s, ok := strings.CutPrefix(httpURL, "https://"); ok {
		return "wss://" + s
	}
	if s, ok := strings.CutPrefix(httpURL, "http://"); ok {
		return "ws://" + s
	}
	return httpURL
}
