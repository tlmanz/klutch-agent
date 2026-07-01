package singleinstance

import (
	"testing"
	"time"
)

// TestSecondInstanceSignalsPrimary verifies that a second Acquire in the same
// directory does not become primary and instead triggers the primary's activate
// callback.
func TestSecondInstanceSignalsPrimary(t *testing.T) {
	dir := t.TempDir()

	first, err := Acquire(dir)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer first.Close()
	if !first.Primary() {
		t.Fatal("first instance should be primary")
	}

	activated := make(chan struct{}, 1)
	first.SetActivate(func() { activated <- struct{}{} })

	second, err := Acquire(dir)
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}
	defer second.Close()
	if second.Primary() {
		t.Fatal("second instance should not be primary while the first is alive")
	}

	select {
	case <-activated:
	case <-time.After(2 * time.Second):
		t.Fatal("primary was not activated by the second instance")
	}
}

// TestStaleLockIsReclaimed verifies that once the primary is gone, a new Acquire
// reclaims primacy rather than deferring to the dead lock file.
func TestStaleLockIsReclaimed(t *testing.T) {
	dir := t.TempDir()

	first, err := Acquire(dir)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	// Simulate a crash without cleanup: close the listener but leave the lock
	// file on disk (Close removes it, so close the listener directly).
	if first.ln != nil {
		_ = first.ln.Close()
	}

	second, err := Acquire(dir)
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}
	defer second.Close()
	if !second.Primary() {
		t.Fatal("second instance should reclaim primacy from a stale lock")
	}
}
