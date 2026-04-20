package engine

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

type sessionState struct {
	CurrentTargetID string                  `json:"current_target_id,omitempty"`
	Snapshots       map[string]PageSnapshot `json:"snapshots,omitempty"`
}

// PageSnapshot stores the last known interactive refs for a page target.
type PageSnapshot struct {
	TargetID string                 `json:"target_id"`
	URL      string                 `json:"url,omitempty"`
	Title    string                 `json:"title,omitempty"`
	Refs     map[string]RefSnapshot `json:"refs,omitempty"`
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
	return os.WriteFile(path, data, 0o600)
}

func snapshotFromResult(page *rod.Page, result *ExtractionResult) (*PageSnapshot, error) {
	info, err := page.Info()
	if err != nil {
		return nil, err
	}

	snapshot := &PageSnapshot{
		TargetID: string(page.TargetID),
		Refs:     map[string]RefSnapshot{},
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
