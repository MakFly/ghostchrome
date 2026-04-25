package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/go-rod/rod"
)

func openPage() (*engine.Browser, *rod.Page) {
	b, err := engine.NewBrowser(flagConnect, flagHeadless, flagTimeout)
	if err != nil {
		exitErr("browser", err)
	}

	page, err := b.Page()
	if err != nil {
		b.Close()
		exitErr("page", err)
	}

	return b, page
}

func applyStealthIfNeeded(page *rod.Page) {
	if flagStealth {
		if err := engine.ApplyStealth(page); err != nil {
			exitErr("stealth", err)
		}
	}
}

func dismissCookiesIfNeeded(page *rod.Page) {
	if flagDismissCookies && engine.DismissCookieBanner(page) {
		_ = engine.WaitForPage(page, "stable")
	}
}

func navigateIfRequested(page *rod.Page, targetURL string, waitStrategy string) *engine.PageInfo {
	if targetURL == "" {
		return nil
	}

	applyStealthIfNeeded(page)
	info, err := engine.Navigate(page, targetURL, waitStrategy)
	if err != nil {
		exitErr("navigate", err)
	}
	waitForChallengeIfStealth(page, info)
	dismissCookiesIfNeeded(page)
	waitForSelectorOrSleep(page)
	return info
}

func waitForChallengeIfStealth(page *rod.Page, info *engine.PageInfo) {
	if !flagStealth || info == nil {
		return
	}
	// 403 is the classic DataDome signal; 503 is Cloudflare's challenge code.
	if info.Status == 403 || info.Status == 503 {
		engine.WaitForBotChallenge(page, 30*time.Second)
	}
}

// waitForSelectorOrSleep applies --wait-selector then --wait-ms, in that
// order. Both are no-ops when their flag is empty/zero. The selector wait
// is bounded by --timeout (default 30s).
func waitForSelectorOrSleep(page *rod.Page) {
	if flagWaitSelector != "" {
		scoped := page.Timeout(time.Duration(flagTimeout) * time.Second)
		el, err := scoped.Element(flagWaitSelector)
		if err != nil || el == nil {
			fmt.Fprintf(os.Stderr, "wait-selector %q not found: %v\n", flagWaitSelector, err)
		} else if err := el.WaitVisible(); err != nil {
			fmt.Fprintf(os.Stderr, "wait-selector %q never became visible: %v\n", flagWaitSelector, err)
		}
	}
	if flagWaitMs > 0 {
		time.Sleep(time.Duration(flagWaitMs) * time.Millisecond)
	}
}

func snapshotPage(b *engine.Browser, page *rod.Page, level engine.ExtractLevel) *engine.ExtractionResult {
	result, err := engine.Extract(page, level, "")
	if err != nil {
		exitErr("extract", err)
	}
	if err := b.SaveSnapshot(page, result); err != nil {
		exitErr("snapshot", err)
	}
	return result
}

func ensureSnapshot(b *engine.Browser, page *rod.Page, targetURL string, waitStrategy string, level engine.ExtractLevel) *engine.PageSnapshot {
	if targetURL != "" {
		navigateIfRequested(page, targetURL, waitStrategy)
		result := snapshotPage(b, page, level)
		if !b.Connected() {
			snapshot, err := engine.BuildSnapshot(page, result)
			if err != nil {
				exitErr("snapshot", err)
			}
			return snapshot
		}
		snapshot := b.Snapshot(page)
		if snapshot == nil {
			exitErr("snapshot", errors.New("failed to persist page snapshot"))
		}
		return snapshot
	}

	snapshot := b.Snapshot(page)
	if snapshot == nil {
		exitErr("snapshot", errors.New("no snapshot for current page: run preview, extract, or navigate --extract first"))
	}
	return snapshot
}

func exitIfStaleRef(err error, action string) {
	if err == nil {
		return
	}
	if errors.Is(err, engine.ErrStaleRef) {
		exitErr(action, err)
	}
}

func errInvalidArg(name, got, allowed string) error {
	return fmt.Errorf("invalid %s %q: use %s", name, got, allowed)
}

func errNeedRefOrLocator() error {
	return fmt.Errorf("need a @ref or one of --by-role / --by-name / --by-label / --by-text")
}

// validateOutputPath ensures that a user-supplied path is safe to write to.
// It rejects paths containing ".." or paths that resolve outside the current
// working directory (unless they are under os.UserCacheDir()/ghostchrome/).
func validateOutputPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("output path cannot be empty")
	}

	// Reject paths with ".." to prevent directory traversal
	cleaned := filepath.Clean(p)
	if strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("output path %q contains parent directory references", p)
	}

	// If the path is absolute, check it's within allowed directories
	if filepath.IsAbs(cleaned) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot determine working directory: %w", err)
		}

		// Allow paths under the current working directory
		if strings.HasPrefix(cleaned, cwd+string(os.PathSeparator)) {
			return cleaned, nil
		}

		// Allow paths under the ghostchrome cache directory
		cacheDir, err := os.UserCacheDir()
		if err == nil {
			ghostchromeCache := filepath.Join(cacheDir, "ghostchrome")
			if strings.HasPrefix(cleaned, ghostchromeCache+string(os.PathSeparator)) {
				return cleaned, nil
			}
		}

		return "", fmt.Errorf("output path %q is outside current directory and cache directory", p)
	}

	// For relative paths, resolve to absolute and check against CWD
	abs := filepath.Join(".", cleaned)
	if !filepath.IsAbs(abs) {
		// Double-check: filepath.Join with "." should give absolute path in most cases
		var err error
		abs, err = filepath.Abs(abs)
		if err != nil {
			return "", fmt.Errorf("cannot resolve relative path: %w", err)
		}
	}
	return abs, nil
}
