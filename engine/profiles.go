package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ProfileInfo describes a persistent Chrome profile on disk
// (~/.ghostchrome/profiles/<name>).
type ProfileInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

func profilesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ghostchrome", "profiles"), nil
}

// dirSize returns the total byte size of a directory tree (best-effort:
// unreadable entries are skipped).
func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, ierr := d.Info(); ierr == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// ListProfiles returns every persistent profile with its on-disk size.
func ListProfiles() ([]ProfileInfo, error) {
	dir, err := profilesDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]ProfileInfo, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		out = append(out, ProfileInfo{Name: e.Name(), Path: p, Bytes: dirSize(p)})
	}
	return out, nil
}

// RemoveProfile deletes a profile directory. The name is validated and the
// resolved path is confirmed to live under the profiles dir before any
// removal, so it can never escape via traversal.
func RemoveProfile(name string) error {
	if err := validateSessionName(name); err != nil {
		return err
	}
	dir, err := profilesDir()
	if err != nil {
		return err
	}
	target := filepath.Join(dir, name)
	// Defense in depth: the cleaned target must be a direct child of dir.
	if filepath.Dir(target) != filepath.Clean(dir) {
		return fmt.Errorf("refusing to remove %q: outside the profiles directory", target)
	}
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return fmt.Errorf("profile %q not found", name)
	}
	return os.RemoveAll(target)
}

// GhostchromeDataDirs returns the data directories a full uninstall removes:
// ~/.ghostchrome (profiles, contexts, sessions registry) and the
// ~/.cache/ghostchrome session-snapshot cache. Missing dirs are omitted.
func GhostchromeDataDirs() []string {
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".ghostchrome"))
	}
	if cache, err := os.UserCacheDir(); err == nil {
		dirs = append(dirs, filepath.Join(cache, "ghostchrome"))
	}
	existing := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if _, err := os.Stat(d); err == nil {
			existing = append(existing, d)
		}
	}
	return existing
}
