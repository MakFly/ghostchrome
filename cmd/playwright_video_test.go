package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dev-toolings/ghostchrome/engine"
)

func TestVideoManifestPath(t *testing.T) {
	restore := snapshotConfigGlobals()
	defer restore()
	got := videoManifestPath(".playwright-cli/demo.webm")
	want := filepath.Join(".playwright-cli", "demo.video.json")
	if got != want {
		t.Fatalf("videoManifestPath() = %q, want %q", got, want)
	}
}

func TestVideoManifestPathUsesConfiguredOutputDir(t *testing.T) {
	restore := snapshotConfigGlobals()
	defer restore()
	flagConfigOutputDir = filepath.Join("tmp", "pw-output")

	got := videoManifestPath(".playwright-cli/demo.webm")
	want := filepath.Join("tmp", "pw-output", "demo.video.json")
	if got != want {
		t.Fatalf("videoManifestPath() = %q, want %q", got, want)
	}
}

func TestElapsedVideoMs(t *testing.T) {
	start := "2026-06-13T12:00:00Z"
	now := time.Date(2026, 6, 13, 12, 0, 2, 500*int(time.Millisecond), time.UTC)
	if got := elapsedVideoMs(start, now); got != 2500 {
		t.Fatalf("elapsedVideoMs() = %d, want 2500", got)
	}
}

func TestWriteVideoManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "video.json")
	state := engine.VideoState{
		Filename:  ".playwright-cli/demo.webm",
		StartedAt: "2026-06-13T12:00:00Z",
		Size:      "800x600",
		Auto:      true,
		Source:    "config.saveVideo",
		Chapters:  []engine.VideoChapter{{Title: "Intro", AtMs: 1200}},
	}
	if err := writeVideoManifest(path, state, time.Date(2026, 6, 13, 12, 0, 3, 0, time.UTC), 42, "/tmp/frames"); err != nil {
		t.Fatalf("writeVideoManifest: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if payload["frames_captured"] != float64(42) {
		t.Fatalf("expected frames_captured=42, got %#v", payload["frames_captured"])
	}
	if payload["playwright_compatible"] != false {
		t.Fatalf("expected playwright_compatible=false, got %#v", payload["playwright_compatible"])
	}
	if payload["auto_started"] != true {
		t.Fatalf("expected auto_started=true, got %#v", payload["auto_started"])
	}
	if payload["source"] != "config.saveVideo" {
		t.Fatalf("source = %#v", payload["source"])
	}
}
