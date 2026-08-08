package agent

import (
	"context"
	"io"
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/tlmanz/klutch-agent/internal/store"
)

// newTestAgent builds an agent pointed at a port nothing listens on, so every
// dial fails fast and the run loop's retry path is what gets exercised.
func newTestAgent(t *testing.T) *Agent {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return New(Config{
		Server:   "http://127.0.0.1:1",
		SpoolDir: t.TempDir(),
		Refresh:  time.Hour,
	}, st, log.New(io.Discard, "", 0))
}

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestDisconnectUnpairsAndStopsDialing(t *testing.T) {
	a := newTestAgent(t)
	if err := a.SetToken("device-token"); err != nil {
		t.Fatalf("set token: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)

	waitFor(t, "the first dial to fail", func() bool { return a.Snapshot().LastError != "" })

	if err := a.Disconnect(); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	s := a.Snapshot()
	if s.Enrolled || s.Connected {
		t.Fatalf("after disconnect: enrolled=%v connected=%v, want both false", s.Enrolled, s.Connected)
	}
	if s.LastError != "" {
		t.Errorf("a deliberate disconnect must not leave an error: %q", s.LastError)
	}
	if tok, _ := a.store.Setting(store.KeyToken); tok != "" {
		t.Errorf("device token = %q, want it forgotten so the next launch starts unpaired", tok)
	}

	// The loop must park instead of retrying: a new error here would mean it kept
	// dialing a backend the operator disconnected from.
	time.Sleep(1500 * time.Millisecond)
	if e := a.Snapshot().LastError; e != "" {
		t.Errorf("disconnected agent kept dialing: %q", e)
	}
}

func TestReconnectRequiresPairing(t *testing.T) {
	a := newTestAgent(t)
	if err := a.Reconnect(); err == nil {
		t.Fatal("Reconnect on an unpaired device must report that it needs pairing")
	}
}

func TestReconnectRedialsWithoutRepairing(t *testing.T) {
	a := newTestAgent(t)
	if err := a.SetToken("device-token"); err != nil {
		t.Fatalf("set token: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)

	waitFor(t, "the first dial to fail", func() bool { return a.Snapshot().LastError != "" })

	if err := a.Reconnect(); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	if s := a.Snapshot(); !s.Enrolled {
		t.Error("Reconnect must keep the device paired")
	}
	// It redials rather than sitting out the backoff, so the failure comes back.
	waitFor(t, "the redial to be attempted", func() bool { return a.Snapshot().LastError != "" })
}

func TestSetServerPersistsAndClearsError(t *testing.T) {
	a := newTestAgent(t)
	a.mutate(func(s *State) { s.LastError = "boom" })
	if err := a.SetServer("https://api.example.com"); err != nil {
		t.Fatalf("set server: %v", err)
	}
	s := a.Snapshot()
	if s.Server != "https://api.example.com" {
		t.Errorf("state server = %q, want the new address", s.Server)
	}
	if s.LastError != "" {
		t.Error("switching backends must clear the previous backend's error")
	}
	if v, _ := a.store.Setting(store.KeyServer); v != "https://api.example.com" {
		t.Errorf("persisted server = %q, want the new address", v)
	}
}
