package engine

import (
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Browser wraps a Rod browser with connect/launch logic.
type Browser struct {
	browser   *rod.Browser
	page      *rod.Page
	timeout   time.Duration
	connected bool // true if connected to external Chrome (don't close it)
}

// NewBrowser creates a browser instance.
// If connectURL is set, connects to an existing Chrome via CDP.
// Otherwise, auto-launches a new Chrome process.
func NewBrowser(connectURL string, headless bool, timeoutSec int) (*Browser, error) {
	timeout := time.Duration(timeoutSec) * time.Second

	var b *rod.Browser
	if connectURL != "" {
		b = rod.New().ControlURL(connectURL).Timeout(timeout)
		if err := b.Connect(); err != nil {
			return nil, err
		}
	} else {
		u, err := launcher.New().Headless(headless).Launch()
		if err != nil {
			return nil, err
		}
		b = rod.New().ControlURL(u).Timeout(timeout)
		if err := b.Connect(); err != nil {
			return nil, err
		}
	}

	return &Browser{
		browser:   b,
		timeout:   timeout,
		connected: connectURL != "",
	}, nil
}

// Page returns the active page or creates a new one.
// When connected to an existing Chrome, it gets the first existing page.
func (b *Browser) Page() (*rod.Page, error) {
	if b.page != nil {
		return b.page, nil
	}

	if b.connected {
		// Get existing pages first
		pages, err := b.browser.Pages()
		if err == nil && len(pages) > 0 {
			b.page = pages[0]
			return b.page, nil
		}
	}

	p, err := b.browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, err
	}
	b.page = p
	return p, nil
}

// Connected returns true if connected to external Chrome (not launched by us).
func (b *Browser) Connected() bool {
	return b.connected
}

// Close cleans up the browser resources.
// When connected to an external Chrome, only disconnects (doesn't kill it).
func (b *Browser) Close() {
	if b.browser != nil {
		if b.connected {
			// Don't close external Chrome, just disconnect
			return
		}
		b.browser.Close()
	}
}
