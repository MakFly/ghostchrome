package engine

import (
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// WaitForPage applies a supported page wait strategy.
//
// Strategies (ordered fastest → slowest typically):
//   - "domcontentloaded": fires when HTML parsed, before sub-resources. ~200-500ms
//     faster than "load" on real-world pages with images/iframes/3P scripts.
//   - "load":             "load" event — DOM + sub-resources fetched.
//   - "stable":           DOM mutations have settled for 500ms.
//   - "idle":             network has been idle for 500ms.
//   - "none":             return immediately.
//
// Defaults to "domcontentloaded" for empty input — that's the right trade-off
// for an LLM agent: parsing is done, refs are extractable, but we don't pay
// for late-loading ads/analytics.
func WaitForPage(page *rod.Page, waitStrategy string) error {
	switch waitStrategy {
	case "stable":
		return page.WaitStable(500 * time.Millisecond)
	case "idle", "networkidle":
		page.WaitRequestIdle(500*time.Millisecond, nil, nil, nil)()
		return nil
	case "none":
		return nil
	case "load":
		return page.WaitLoad()
	case "", "domcontentloaded", "dcl":
		// PageSetLifecycleEventsEnabled is idempotent (safe to call repeatedly).
		// EachEvent registers a one-shot listener that fires on the named event.
		_ = proto.PageSetLifecycleEventsEnabled{Enabled: true}.Call(page)
		wait := page.EachEvent(func(e *proto.PageLifecycleEvent) bool {
			return e.Name == proto.PageLifecycleEventNameDOMContentLoaded
		})
		wait()
		return nil
	default:
		return fmt.Errorf("invalid wait strategy %q: use domcontentloaded, load, stable, idle, or none", waitStrategy)
	}
}
