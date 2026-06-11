package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/go-rod/rod"
)

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
	budget := time.Duration(flagTimeout) * time.Second * 4 / 5
	if budget > 110*time.Second {
		budget = 110 * time.Second
	}
	if budget < 45*time.Second {
		budget = 45 * time.Second
	}
	engine.WaitForBotChallenge(page, budget)
}

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
