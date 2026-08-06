package engine

import (
	"time"

	"github.com/dev-toolings/ghostchrome/engine/policy"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// ActivePolicy is the session-wide policy enforced on navigation and actions.
// Set by the CLI or MCP server at startup. Nil means no restrictions.
var ActivePolicy *policy.Policy

// PageInfo holds the result of a navigation.
type PageInfo struct {
	URL    string `json:"url"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	TimeMs int64  `json:"time_ms"`
}

// Navigate goes to the given URL and returns page info.
//
// Lifecycle waits ("domcontentloaded" / "load") are registered BEFORE the
// navigation starts so the listener cannot miss the event — registering
// after page.Navigate is a race that timed out on fast-loading pages.
func Navigate(page *rod.Page, rawURL string, waitStrategy string) (*PageInfo, error) {
	if err := ActivePolicy.AllowURL(rawURL); err != nil {
		return nil, err
	}
	start := time.Now()
	requestTracker := newRequestTracker(page)
	requestTracker.listen(page)
	defer requestTracker.close()

	// Pre-arm the lifecycle wait so the event can't fire before we're listening.
	var lifecycleWait func()
	switch waitStrategy {
	case "", "domcontentloaded", "dcl":
		_ = proto.PageSetLifecycleEventsEnabled{Enabled: true}.Call(page)
		lifecycleWait = page.EachEvent(func(e *proto.PageLifecycleEvent) bool {
			return e.Name == proto.PageLifecycleEventNameDOMContentLoaded
		})
	case "load":
		_ = proto.PageSetLifecycleEventsEnabled{Enabled: true}.Call(page)
		lifecycleWait = page.EachEvent(func(e *proto.PageLifecycleEvent) bool {
			return e.Name == proto.PageLifecycleEventNameLoad
		})
	}

	if err := page.Navigate(rawURL); err != nil {
		return nil, err
	}

	if lifecycleWait != nil {
		lifecycleWait()
	} else {
		// "stable", "idle", "none" — handled by the legacy path.
		if err := WaitForPage(page, waitStrategy); err != nil {
			return nil, err
		}
	}

	info, err := page.Info()
	if err != nil {
		return nil, err
	}

	elapsed := time.Since(start).Milliseconds()

	return &PageInfo{
		URL:    info.URL,
		Title:  info.Title,
		Status: requestTracker.MainDocumentStatus(),
		TimeMs: elapsed,
	}, nil
}
