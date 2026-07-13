package mcp

import (
	"testing"
	"time"

	"github.com/MakFly/ghostchrome/engine"
)

func TestAliveNilBrowser(t *testing.T) {
	var b *engine.Browser
	if b.Alive(time.Second) {
		t.Error("nil Browser reported alive")
	}
}

// TestEnsurePageRelaunchesAfterBrowserDeath reproduces the wedged-singleton
// bug: Chrome dies under the MCP server, and before the fix every tool call
// failed forever with "context deadline exceeded" because ensurePageLocked
// trusted any non-nil handle. The fix probes liveness and relaunches.
func TestEnsurePageRelaunchesAfterBrowserDeath(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Chrome")
	}
	s := New(Options{Headless: true, TimeoutSec: 15})
	defer s.Close()

	s.mu.Lock()
	b1, _, err := s.ensurePageLocked()
	s.mu.Unlock()
	if err != nil {
		t.Fatalf("first launch: %v", err)
	}
	if !b1.Alive(2 * time.Second) {
		t.Fatal("freshly launched browser reported dead")
	}

	// Kill Chrome out from under the server, leaving the stale handles in
	// place — exactly the state a crash leaves behind.
	_ = b1.RodBrowser().Close()
	deadline := time.Now().Add(5 * time.Second)
	for b1.Alive(500*time.Millisecond) && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if b1.Alive(500 * time.Millisecond) {
		t.Fatal("browser still alive after kill; cannot exercise recovery")
	}

	s.mu.Lock()
	b2, page2, err := s.ensurePageLocked()
	s.mu.Unlock()
	if err != nil {
		t.Fatalf("relaunch after browser death: %v", err)
	}
	if b2 == b1 {
		t.Fatal("expected a fresh browser after death, got the stale handle")
	}
	if _, err := page2.Info(); err != nil {
		t.Fatalf("relaunched page unusable: %v", err)
	}
	if s.snapshot != nil {
		t.Error("stale snapshot survived relaunch; refs would resolve into the dead browser")
	}
}
