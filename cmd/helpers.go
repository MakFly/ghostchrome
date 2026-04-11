package cmd

import (
	"errors"
	"fmt"
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
	return info
}

func waitForChallengeIfStealth(page *rod.Page, info *engine.PageInfo) {
	if !flagStealth || info == nil {
		return
	}
	// Only attempt challenge wait on 403 responses (common for DataDome)
	if info.Status == 403 {
		engine.WaitForBotChallenge(page, 10*time.Second)
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
