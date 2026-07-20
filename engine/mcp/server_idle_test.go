package mcp

import (
	"testing"
	"time"
)

// TestReapIfIdleNoBrowserIsNoop guards the cheap path: with no browser held
// (or reaping disabled) the reaper must never panic and must leave state alone.
// Runs without Chrome.
func TestReapIfIdleNoBrowserIsNoop(t *testing.T) {
	s := New(Options{IdleTimeout: time.Minute})
	s.reapIfIdle() // browser == nil, lastActivity zero
	if s.browser != nil {
		t.Fatal("reapIfIdle materialized a browser out of nothing")
	}

	// Disabled reaper never touches anything, even with a stale lastActivity.
	s2 := New(Options{IdleTimeout: 0})
	s2.lastActivity = time.Now().Add(-time.Hour)
	s2.reapIfIdle()
}

// TestReapIfIdleReleasesThenRelaunches proves the fix: after IdleTimeout of no
// activity the held Chrome is released, but the server stays usable and the
// next ensurePageLocked spins up a fresh browser on demand.
func TestReapIfIdleReleasesThenRelaunches(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Chrome")
	}
	s := New(Options{Headless: true, TimeoutSec: 15, IdleTimeout: time.Second})
	defer s.Close()

	s.mu.Lock()
	b1, _, err := s.ensurePageLocked()
	s.mu.Unlock()
	if err != nil {
		t.Fatalf("first launch: %v", err)
	}

	// Recent activity: the reaper must leave the browser alone.
	s.reapIfIdle()
	s.mu.Lock()
	stillHeld := s.browser
	s.mu.Unlock()
	if stillHeld == nil {
		t.Fatal("reaped a browser that was just active")
	}

	// Simulate the timeout having elapsed, then reap.
	s.mu.Lock()
	s.lastActivity = time.Now().Add(-2 * time.Second)
	s.mu.Unlock()
	s.reapIfIdle()
	s.mu.Lock()
	reaped := s.browser == nil && s.snapshot == nil
	s.mu.Unlock()
	if !reaped {
		t.Fatal("idle browser was not released")
	}

	// Server is still alive: the next call relaunches Chrome on demand.
	s.mu.Lock()
	b2, page2, err := s.ensurePageLocked()
	s.mu.Unlock()
	if err != nil {
		t.Fatalf("relaunch after idle reap: %v", err)
	}
	if b2 == b1 {
		t.Fatal("expected a fresh browser after reap, got the released handle")
	}
	if _, err := page2.Info(); err != nil {
		t.Fatalf("relaunched page unusable: %v", err)
	}
}
