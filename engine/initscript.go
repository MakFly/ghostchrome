package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-rod/rod"
)

// InitScriptsDir returns the directory for user init scripts.
func InitScriptsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".ghostchrome", "init-scripts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// ListInitScripts returns the names of installed init scripts.
func ListInitScripts() ([]string, error) {
	dir, err := InitScriptsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".js") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// AddInitScript copies a JS file into the init-scripts directory.
func AddInitScript(srcPath string) (string, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("read script: %w", err)
	}
	dir, err := InitScriptsDir()
	if err != nil {
		return "", err
	}
	name := filepath.Base(srcPath)
	if !strings.HasSuffix(name, ".js") {
		name += ".js"
	}
	dst := filepath.Join(dir, name)
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		return "", err
	}
	return name, nil
}

// RemoveInitScript removes a script by name from the init-scripts directory.
func RemoveInitScript(name string) error {
	dir, err := InitScriptsDir()
	if err != nil {
		return err
	}
	if !strings.HasSuffix(name, ".js") {
		name += ".js"
	}
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("init script %q not found", name)
	}
	return os.Remove(path)
}

// ApplyInitScripts loads and evaluates all init scripts on the page.
func ApplyInitScripts(page *rod.Page) error {
	dir, err := InitScriptsDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		_, err = page.Eval(string(data))
		if err != nil {
			return fmt.Errorf("eval %s: %w", e.Name(), err)
		}
	}
	return nil
}

// ApplyInitScriptFiles registers JavaScript files to run before future page
// scripts. This mirrors Playwright's addInitScript-style behavior for config
// supplied init scripts.
func ApplyInitScriptFiles(page *rod.Page, paths []string) error {
	if page == nil {
		return fmt.Errorf("page is nil")
	}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read init script %s: %w", path, err)
		}
		if _, err := page.EvalOnNewDocument(string(data)); err != nil {
			return fmt.Errorf("register init script %s: %w", path, err)
		}
	}
	return nil
}
