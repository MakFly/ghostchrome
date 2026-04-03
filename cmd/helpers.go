package cmd

import (
	"github.com/go-rod/rod"
	"github.com/MakFly/ghostchrome/engine"
)

// applyPageOptions applies stealth and cookie dismissal to a page if flags are set.
// Should be called after getting a page but before navigation.
func applyStealthIfNeeded(page *rod.Page) {
	if flagStealth {
		if err := engine.ApplyStealth(page); err != nil {
			exitErr("stealth", err)
		}
	}
}

// dismissCookiesIfNeeded attempts to dismiss cookie banners after navigation.
func dismissCookiesIfNeeded(page *rod.Page) {
	if flagDismissCookies {
		engine.DismissCookieBanner(page)
	}
}
