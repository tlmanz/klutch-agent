// Package agent is the headless core of the Klutch print agent: it holds the WSS
// connection to the backend, enumerates and dispatches to local printers,
// persists outcomes to the local store, and self-updates. It is UI-agnostic: the
// Fyne UI (internal/ui) observes it through Snapshot + Subscribe and drives it
// through its exported methods, and a headless mode runs it with no UI at all.
package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/tlmanz/klutch-agent/internal/store"
	"github.com/tlmanz/klutch-agent/wire"
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
	Printers         []wire.Printer
	JobsOK           int
	JobsFailed       int
	AutoUpdate       bool
}

// Agent owns the connection lifecycle and mutable state.
type Agent struct {
	cfg   Config
	store *store.Store
	log   *log.Logger

	mu    sync.Mutex
	state State
	token string

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
		wp := make([]wire.Printer, len(ps))
		for i, p := range ps {
			wp[i] = wire.Printer{Name: p.Name, Description: p.Description}
		}
		a.state.Printers = wp
	}
	return a
}

// SetRelaunch overrides how the process is replaced after a self-update.
func (a *Agent) SetRelaunch(fn func()) { a.relaunch = fn }

// Snapshot returns a copy of the current state for the UI to render.
func (a *Agent) Snapshot() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.state
	s.Printers = append([]wire.Printer(nil), a.state.Printers...)
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

		err := a.connect(ctx, server, tok)
		if ctx.Err() != nil {
			return
		}
		a.mutate(func(s *State) {
			s.Connected = false
			if err != nil {
				s.LastError = err.Error()
			}
		})
		if err != nil {
			a.log.Printf("connection ended: %v (retrying in %s)", err, backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		case <-a.enrolled: // server changed; reconnect immediately
		}
		if backoff < 30*time.Second {
			backoff *= 2
		} else {
			backoff = time.Second
		}
	}
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
	a.wake()
	return nil
}

// SetServer changes the backend URL, persists it, and forces a reconnect.
func (a *Agent) SetServer(server string) error {
	if err := a.store.SetSetting(store.KeyServer, server); err != nil {
		return err
	}
	a.mutate(func(s *State) { s.Server = server })
	a.mu.Lock()
	a.cfg.Server = server
	a.mu.Unlock()
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

// wake signals the run loop (non-blocking) that enrollment or the server changed.
func (a *Agent) wake() {
	select {
	case a.enrolled <- struct{}{}:
	default:
	}
}
