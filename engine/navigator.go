package engine

import (
	"time"

	"github.com/go-rod/rod"
)

// PageInfo holds the result of a navigation.
type PageInfo struct {
	URL    string `json:"url"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	TimeMs int64  `json:"time_ms"`
}

// Navigate goes to the given URL and returns page info.
func Navigate(page *rod.Page, rawURL string, waitStrategy string) (*PageInfo, error) {
	start := time.Now()
	requestTracker := newRequestTracker(page)
	requestTracker.listen(page)
	defer requestTracker.close()

	err := page.Navigate(rawURL)
	if err != nil {
		return nil, err
	}

	err = WaitForPage(page, waitStrategy)
	if err != nil {
		return nil, err
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
