package engine

import (
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

// WaitForPage applies a supported page wait strategy.
func WaitForPage(page *rod.Page, waitStrategy string) error {
	switch waitStrategy {
	case "stable":
		return page.WaitStable(500 * time.Millisecond)
	case "idle":
		page.WaitRequestIdle(500*time.Millisecond, nil, nil, nil)()
		return nil
	case "none":
		return nil
	case "load":
		return page.WaitLoad()
	default:
		return fmt.Errorf("invalid wait strategy %q: use load, stable, idle, or none", waitStrategy)
	}
}
