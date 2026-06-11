package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/go-rod/rod"
)

func buildBrowserOpts() engine.BrowserOpts {
	// Named session (`-s` / $GHOSTCHROME_SESSION): resolve to a managed,
	// persistent Chrome. Unlike --connect=auto we do NOT mark the tab fresh —
	// the active tab is reused across calls so state (and @refs) persist.
	if flagSession == "" {
		flagSession = os.Getenv("GHOSTCHROME_SESSION")
	}
	if flagSession != "" && flagConnect == "" {
		ws, err := engine.AcquireSession(flagSession, engine.SessionSpawnOpts{
			Headless: flagHeadless,
			Stealth:  flagStealth,
			Proxy:    flagProxy,
		})
		if err != nil {
			exitErr("session", err)
		}
		fmt.Fprintf(os.Stderr, "[session %s] %s\n", flagSession, ws)
		flagConnect = ws
	}

	connectURL := flagConnect
	attachFresh := false
	if connectURL == "auto" {
		ws, err := engine.DiscoverCDP(nil, 800*time.Millisecond)
		if err != nil {
			exitErr("connect=auto", fmt.Errorf("%w (start Chrome with --remote-debugging-port=9222)", err))
		}
		fmt.Fprintf(os.Stderr, "[connect=auto] attached to %s\n", ws)
		connectURL = ws
		attachFresh = true
	}
	opts := engine.BrowserOpts{
		ConnectURL:  connectURL,
		Headless:    flagHeadless,
		Invisible:   flagInvisible,
		TimeoutSec:  flagTimeout,
		Proxy:       flagProxy,
		AttachFresh: attachFresh,
		ContextName: flagContext,
	}
	if flagTab >= 0 {
		if connectURL == "" {
			fmt.Fprintln(os.Stderr, "warning: --tab is ignored without --connect (auto-launch always uses a fresh tab)")
		} else {
			opts.TargetTabIndex = &flagTab
		}
	}
	if connectURL == "" && flagContext != "" {
		fmt.Fprintln(os.Stderr, "warning: --context ignored without --connect (auto-launch already has its own isolated profile)")
		opts.ContextName = ""
	}
	if connectURL != "" {
		if flagUserProfile != "" {
			fmt.Fprintln(os.Stderr, "warning: --user-profile ignored when --connect is set (cannot change Chrome user_data_dir of a running instance)")
		}
		if flagProxy != "" {
			fmt.Fprintln(os.Stderr, "warning: --proxy ignored when --connect is set (proxy is fixed at Chrome launch time)")
			opts.Proxy = ""
		}
		if flagDefaultExtensions || flagExtensions != "" {
			fmt.Fprintln(os.Stderr, "warning: --extensions / --default-extensions ignored when --connect is set (extensions are loaded at Chrome launch time)")
		}
		return opts
	}
	if flagUserProfile != "" {
		dir, err := engine.ResolveProfileDir(flagUserProfile)
		if err != nil {
			exitErr("user-profile", err)
		}
		opts.UserDataDir = dir
	}
	if flagDefaultExtensions {
		paths, err := engine.DefaultExtensionPaths()
		if err != nil {
			exitErr("default-extensions", err)
		}
		opts.Extensions = append(opts.Extensions, paths...)
	}
	if flagExtensions != "" {
		for _, p := range strings.Split(flagExtensions, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if !filepath.IsAbs(p) {
				exitErr("extensions", fmt.Errorf("extension path must be absolute: %q", p))
			}
			manifest := filepath.Join(p, "manifest.json")
			if _, err := os.Stat(manifest); err != nil {
				fmt.Fprintf(os.Stderr, "warning: extension %q skipped (missing manifest.json)\n", p)
				continue
			}
			opts.Extensions = append(opts.Extensions, p)
		}
	}
	return opts
}

func openPage() (*engine.Browser, *rod.Page) {
	opts := buildBrowserOpts()
	b, err := engine.NewBrowserWith(opts)
	if err != nil {
		exitErr("browser", err)
	}

	page, err := b.Page()
	if err != nil {
		b.Close()
		exitErr("page", err)
	}

	if opts.UserDataDir != "" && opts.ConnectURL == "" {
		if cookies, cerr := engine.LoadCookiesJSON(opts.UserDataDir); cerr == nil && len(cookies) > 0 {
			if injected, ierr := engine.InjectCookies(page, cookies); ierr != nil {
				fmt.Fprintf(os.Stderr, "warning: cookie injection failed: %v\n", ierr)
			} else {
				fmt.Fprintf(os.Stderr, "[cookies] injected %d/%d from imported profile\n", injected, len(cookies))
			}
		}
	}

	return b, page
}

func applyStealthIfNeeded(page *rod.Page) {
	if flagStealth {
		if err := engine.ApplyStealth(page); err != nil {
			exitErr("stealth", err)
		}
	}
	if flagProxy != "" && flagConnect == "" {
		if err := engine.ApplyProxyAuth(page, flagProxy); err != nil {
			exitErr("proxy-auth", err)
		}
	}
}

func dismissCookiesIfNeeded(page *rod.Page) {
	if flagDismissCookies && engine.DismissCookieBanner(page) {
		_ = engine.WaitForPage(page, "stable")
	}
}
