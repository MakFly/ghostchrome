package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/go-rod/rod/lib/launcher"
)

// TestSnapshotPageBypassesCacheWhenSSRRequested is a regression test (real
// headless Chrome, local HTTP server only — no live network) for the
// SSR/cache leak: a plain (non-SSR) snapshotPage call must be served from
// cache on repeat, but an includeSSR=true call must bypass that cache and
// recompute rather than silently return the cached, necessarily SSR-less,
// result.
func TestSnapshotPageBypassesCacheWhenSSRRequested(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Chrome")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><html><body><button>hi</button></body></html>`))
	}))
	defer server.Close()

	l := launcher.New().Headless(true).Leakless(false).NoSandbox(true)
	controlURL, err := l.Launch()
	if err != nil {
		t.Fatalf("launch browser: %v", err)
	}
	defer l.Kill()

	b, err := engine.NewBrowser(controlURL, true, 10)
	if err != nil {
		t.Fatalf("new browser: %v", err)
	}
	defer b.Close()

	page, err := b.Page()
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	if _, err := engine.Navigate(page, server.URL, "load"); err != nil {
		t.Fatalf("navigate: %v", err)
	}

	first := snapshotPage(b, page, engine.LevelSkeleton)
	if first == nil {
		t.Fatal("expected a non-nil extraction result")
	}

	second := snapshotPage(b, page, engine.LevelSkeleton)
	if second != first {
		t.Fatal("expected a repeat non-SSR call to be served from cache (same result pointer)")
	}

	third := snapshotPage(b, page, engine.LevelSkeleton, true)
	if third == second {
		t.Fatal("expected includeSSR=true to bypass the cache and recompute a fresh result")
	}
}
