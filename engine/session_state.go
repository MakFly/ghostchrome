package engine

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

type sessionState struct {
	CurrentTargetID string                  `json:"current_target_id,omitempty"`
	Snapshots       map[string]PageSnapshot `json:"snapshots,omitempty"`
	PlaywrightLog   PlaywrightLogState      `json:"playwright_log,omitempty"`
	BrowserTrace    BrowserTraceState       `json:"browser_trace,omitempty"`
	Video           VideoState              `json:"video,omitempty"`
	Emulation       EmulationState          `json:"emulation,omitempty"`
}

// EmulationState is the emulation profile of a managed session.
//
// CDP emulation overrides (device metrics, touch, UA, color-scheme, timezone)
// live in the DevTools session, not in the page: Chrome drops them the moment
// the client detaches. With the transparent daemon, every CLI invocation is a
// new short-lived session against a long-lived Chrome, so a viewport set by one
// command was gone by the next one and the page snapped back to the real window
// size. Persisting the profile here lets the next command replay it on attach.
type EmulationState struct {
	Device      string  `json:"device,omitempty"`
	Width       int     `json:"width,omitempty"`
	Height      int     `json:"height,omitempty"`
	DPR         float64 `json:"dpr,omitempty"`
	Mobile      bool    `json:"mobile,omitempty"`
	Touch       bool    `json:"touch,omitempty"`
	UserAgent   string  `json:"user_agent,omitempty"`
	ColorScheme string  `json:"color_scheme,omitempty"`
	Timezone    string  `json:"timezone,omitempty"`
}

// Empty reports whether the profile carries nothing to replay.
func (s EmulationState) Empty() bool {
	return s.Width <= 0 && s.Height <= 0 && !s.Touch && s.UserAgent == "" && s.ColorScheme == "" && s.Timezone == ""
}

// Summary renders the profile as a single compact line for CLI feedback.
func (s EmulationState) Summary() string {
	parts := make([]string, 0, 4)
	if s.Device != "" {
		parts = append(parts, s.Device)
	}
	if s.Width > 0 && s.Height > 0 {
		dpr := s.DPR
		if dpr <= 0 {
			dpr = 1
		}
		parts = append(parts, fmt.Sprintf("%dx%d@%.4gx", s.Width, s.Height, dpr))
	}
	if s.Mobile {
		parts = append(parts, "mobile")
	}
	if s.Touch {
		parts = append(parts, "touch")
	}
	if s.ColorScheme != "" {
		parts = append(parts, s.ColorScheme)
	}
	if s.Timezone != "" {
		parts = append(parts, s.Timezone)
	}
	if s.UserAgent != "" {
		parts = append(parts, "custom-ua")
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " ")
}

// PlaywrightLogState stores bounded command-observation buffers for
// Playwright CLI-compatible console/network inspection.
type PlaywrightLogState struct {
	Console []ObserverEvent `json:"console,omitempty"`
	Network []CapturedEntry `json:"network,omitempty"`
}

// BrowserTraceState tracks whether CDP browser tracing is currently active for
// a persistent session.
type BrowserTraceState struct {
	Active    bool   `json:"active,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	Output    string `json:"output,omitempty"`
}

// VideoState tracks a durable CDP screencast recording. The artifact is a
// sequence of JPEG frames, never a WebM file.
type VideoState struct {
	Active        bool           `json:"active,omitempty"`
	StartedAt     string         `json:"started_at,omitempty"`
	Filename      string         `json:"filename,omitempty"`
	FramesDir     string         `json:"frames_dir,omitempty"`
	RuntimeStatus string         `json:"runtime_status,omitempty"`
	Size          string         `json:"size,omitempty"`
	Auto          bool           `json:"auto,omitempty"`
	Source        string         `json:"source,omitempty"`
	Chapters      []VideoChapter `json:"chapters,omitempty"`
}

// VideoChapter is one requested video chapter marker.
type VideoChapter struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	DurationMs  int    `json:"duration_ms,omitempty"`
	AtMs        int64  `json:"at_ms"`
	CreatedAt   string `json:"created_at"`
}

// PageSnapshot stores the last known interactive refs for a page target.
type PageSnapshot struct {
	TargetID string                 `json:"target_id"`
	URL      string                 `json:"url,omitempty"`
	Title    string                 `json:"title,omitempty"`
	Refs     map[string]RefSnapshot `json:"refs,omitempty"`
	// CachedExtraction is the last ExtractionResult for this page, persisted so
	// subsequent commands can skip the expensive CDP AccessibilityGetFullAXTree
	// call when the URL has not changed.
	CachedExtraction *ExtractionResult `json:"cached_extraction,omitempty"`
}

// RefSnapshot stores a stable backend node mapping for a single ref.
type RefSnapshot struct {
	BackendNodeID proto.DOMBackendNodeID `json:"backend_node_id"`
	Role          string                 `json:"role,omitempty"`
	Name          string                 `json:"name,omitempty"`
}

func sessionStatePath(connectURL string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		cacheDir = os.TempDir()
	}

	hash := sha1.Sum([]byte(connectURL))
	dir := filepath.Join(cacheDir, "ghostchrome", "sessions")
	// 0o700: snapshots may reference URLs with tokens — keep owner-only.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	return filepath.Join(dir, fmt.Sprintf("%x.json", hash)), nil
}

func loadSessionState(path string) (*sessionState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &sessionState{Snapshots: map[string]PageSnapshot{}}, nil
		}
		return nil, err
	}

	state := &sessionState{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, err
	}
	if state.Snapshots == nil {
		state.Snapshots = map[string]PageSnapshot{}
	}
	return state, nil
}

func saveSessionState(path string, state *sessionState) error {
	if state == nil {
		return nil
	}
	if state.Snapshots == nil {
		state.Snapshots = map[string]PageSnapshot{}
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	// Write to a temp file then rename: a crash mid-write must never leave
	// a truncated JSON behind (loadSessionState would fail on every
	// subsequent command for this connect URL).
	tmp, err := os.CreateTemp(filepath.Dir(path), ".session-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func snapshotFromResult(page *rod.Page, result *ExtractionResult) (*PageSnapshot, error) {
	info, err := page.Info()
	if err != nil {
		return nil, err
	}

	snapshot := &PageSnapshot{
		TargetID:         string(page.TargetID),
		Refs:             map[string]RefSnapshot{},
		CachedExtraction: result,
	}
	if info != nil {
		snapshot.URL = info.URL
		snapshot.Title = info.Title
	}

	for ref, node := range result.Refs {
		if node.BackendNodeID == 0 {
			continue
		}
		snapshot.Refs[ref] = RefSnapshot{
			BackendNodeID: node.BackendNodeID,
			Role:          node.Role,
			Name:          node.Name,
		}
	}

	return snapshot, nil
}

// BuildSnapshot creates an in-memory ref snapshot from an extraction result.
func BuildSnapshot(page *rod.Page, result *ExtractionResult) (*PageSnapshot, error) {
	return snapshotFromResult(page, result)
}
