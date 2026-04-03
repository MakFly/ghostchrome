package engine

import (
	"net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// PageInfo holds the result of a navigation.
type PageInfo struct {
	URL    string `json:"url"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	TimeMs int64  `json:"time_ms"`
}

// Navigate goes to the given URL and returns page info.
// waitStrategy: "load" (default), "stable", "idle", "none".
// HTTP status is captured via CDP Network events on the main frame document request.
func Navigate(page *rod.Page, rawURL string, waitStrategy string) (*PageInfo, error) {
	start := time.Now()

	// Capture HTTP status from the main document response
	status := 0
	parsedURL, _ := url.Parse(rawURL)
	targetHost := ""
	if parsedURL != nil {
		targetHost = parsedURL.Host
	}

	// Set up event listener for network response
	done := make(chan struct{}, 1)
	go page.EachEvent(func(e *proto.NetworkResponseReceived) bool {
		if e.Type == proto.NetworkResourceTypeDocument {
			reqURL, _ := url.Parse(e.Response.URL)
			if reqURL != nil && reqURL.Host == targetHost {
				status = e.Response.Status
				select {
				case done <- struct{}{}:
				default:
				}
				return true // stop listening
			}
		}
		return false
	})()

	// Navigate
	err := page.Navigate(rawURL)
	if err != nil {
		return nil, err
	}

	// Wait for status to be captured (with a short timeout)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		// Timeout waiting for status — continue with 0
	}

	// Apply wait strategy
	switch waitStrategy {
	case "stable":
		err = page.WaitStable(500 * time.Millisecond)
	case "idle":
		page.WaitRequestIdle(500*time.Millisecond, nil, nil, nil)()
	case "none":
		// no wait
	default: // "load"
		err = page.WaitLoad()
	}
	if err != nil {
		return nil, err
	}

	info, err := page.Info()
	if err != nil {
		return nil, err
	}

	elapsed := time.Since(start).Milliseconds()

	return &PageInfo{
		URL:    page.MustInfo().URL,
		Title:  info.Title,
		Status: status,
		TimeMs: elapsed,
	}, nil
}
