package engine

import (
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// LauncherOpts configures a stealth-flavored Chrome launcher.
type LauncherOpts struct {
	Headless   bool
	RemotePort int // 0 = random
}

// NewLauncher returns a configured launcher with the shared anti-detection
// flags used by both auto-launch (NewBrowser) and the `serve` command.
// --no-sandbox is auto-enabled when running inside a CI runner (env
// GITHUB_ACTIONS / CI) or as root, because those environments disable the
// Chrome sandbox.
func NewLauncher(opts LauncherOpts) *launcher.Launcher {
	l := launcher.New().
		Headless(false).
		HeadlessNew(opts.Headless).
		Set("disable-blink-features", "AutomationControlled").
		Set("window-size", "1920,1080").
		Delete("enable-automation")
	if needsNoSandbox() {
		l = l.NoSandbox(true)
	}
	if opts.RemotePort > 0 {
		l = l.RemoteDebuggingPort(opts.RemotePort)
	}
	return l
}

// needsNoSandbox reports whether Chrome should be launched with --no-sandbox.
// We check common CI environment markers and root UID (common in containers).
func needsNoSandbox() bool {
	if os.Geteuid() == 0 {
		return true
	}
	for _, key := range []string{"GITHUB_ACTIONS", "CI", "GHOSTCHROME_NO_SANDBOX"} {
		if v := os.Getenv(key); v != "" && v != "0" && v != "false" {
			return true
		}
	}
	return false
}

// Browser wraps a Rod browser with connect/launch logic.
type Browser struct {
	browser    *rod.Browser
	page       *rod.Page
	timeout    time.Duration
	connected  bool // true if connected to external Chrome (don't close it)
	connectURL string
	statePath  string
	state      *sessionState
}

// NewBrowser creates a browser instance.
// If connectURL is set, connects to an existing Chrome via CDP.
// Otherwise, auto-launches a new Chrome process.
func NewBrowser(connectURL string, headless bool, timeoutSec int) (*Browser, error) {
	timeout := time.Duration(timeoutSec) * time.Second

	var b *rod.Browser
	var state *sessionState
	var statePath string
	if connectURL != "" {
		var err error
		statePath, err = sessionStatePath(connectURL)
		if err != nil {
			return nil, err
		}
		state, err = loadSessionState(statePath)
		if err != nil {
			return nil, err
		}
		b = rod.New().ControlURL(connectURL).Timeout(timeout)
		if err := b.Connect(); err != nil {
			return nil, err
		}
	} else {
		u, err := NewLauncher(LauncherOpts{Headless: headless}).Launch()
		if err != nil {
			return nil, err
		}
		b = rod.New().ControlURL(u).Timeout(timeout)
		if err := b.Connect(); err != nil {
			return nil, err
		}
	}

	return &Browser{
		browser:    b,
		timeout:    timeout,
		connected:  connectURL != "",
		connectURL: connectURL,
		statePath:  statePath,
		state:      state,
	}, nil
}

// Page returns the active page or creates a new one.
// When connected to an existing Chrome, it prefers the persisted active tab.
func (b *Browser) Page() (*rod.Page, error) {
	if b.page != nil {
		return b.page, nil
	}

	if b.connected {
		if b.state != nil && b.state.CurrentTargetID != "" {
			p, err := b.browser.PageFromTarget(proto.TargetTargetID(b.state.CurrentTargetID))
			if err == nil {
				b.page = p
				return b.page, nil
			}
		}

		pages, err := b.browser.Pages()
		if err == nil && len(pages) > 0 {
			b.page = pages[0]
			_ = b.setCurrentTargetID(b.page.TargetID)
			return b.page, nil
		}
	}

	p, err := b.browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, err
	}
	b.page = p
	_ = b.setCurrentTargetID(p.TargetID)
	return p, nil
}

// Connected returns true if connected to external Chrome (not launched by us).
func (b *Browser) Connected() bool {
	return b.connected
}

// RodBrowser returns the underlying rod.Browser for advanced operations.
func (b *Browser) RodBrowser() *rod.Browser {
	return b.browser
}

// SetCurrentPage marks the provided page as the current tab for the session.
func (b *Browser) SetCurrentPage(page *rod.Page) error {
	if page == nil {
		return nil
	}
	b.page = page
	return b.setCurrentTargetID(page.TargetID)
}

// SaveSnapshot persists the latest ref snapshot for the page.
func (b *Browser) SaveSnapshot(page *rod.Page, result *ExtractionResult) error {
	if !b.connected || b.state == nil || page == nil || result == nil {
		return nil
	}
	snapshot, err := snapshotFromResult(page, result)
	if err != nil {
		return err
	}
	b.state.Snapshots[snapshot.TargetID] = *snapshot
	b.state.CurrentTargetID = snapshot.TargetID
	b.page = page
	return saveSessionState(b.statePath, b.state)
}

// Snapshot returns the last persisted snapshot for the current page.
func (b *Browser) Snapshot(page *rod.Page) *PageSnapshot {
	if !b.connected || b.state == nil || page == nil {
		return nil
	}
	return b.snapshotByTarget(page.TargetID)
}

// CurrentTargetID returns the persisted current tab target, if any.
func (b *Browser) CurrentTargetID() string {
	if b.connected && b.state != nil {
		return b.state.CurrentTargetID
	}
	if b.page != nil {
		return string(b.page.TargetID)
	}
	return ""
}

func (b *Browser) snapshotByTarget(targetID proto.TargetTargetID) *PageSnapshot {
	if b.state == nil {
		return nil
	}
	snapshot, ok := b.state.Snapshots[string(targetID)]
	if !ok {
		return nil
	}
	copy := snapshot
	return &copy
}

func (b *Browser) setCurrentTargetID(targetID proto.TargetTargetID) error {
	if !b.connected || b.state == nil {
		return nil
	}
	b.state.CurrentTargetID = string(targetID)
	return saveSessionState(b.statePath, b.state)
}

func (b *Browser) deleteSnapshot(targetID proto.TargetTargetID) error {
	if !b.connected || b.state == nil {
		return nil
	}
	delete(b.state.Snapshots, string(targetID))
	if b.state.CurrentTargetID == string(targetID) {
		b.state.CurrentTargetID = ""
	}
	return saveSessionState(b.statePath, b.state)
}

// DeleteSnapshot removes stored ref state for a closed page target.
func (b *Browser) DeleteSnapshot(targetID proto.TargetTargetID) error {
	return b.deleteSnapshot(targetID)
}

// Close cleans up the browser resources.
// External Chrome keeps running; the CLI process owns the websocket lifetime.
func (b *Browser) Close() {
	if b.browser != nil {
		if b.connected {
			return
		}
		_ = b.browser.Close()
	}
}
