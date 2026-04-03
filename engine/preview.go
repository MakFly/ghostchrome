package engine

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// NetworkEntry represents a captured network request.
type NetworkEntry struct {
	Method   string `json:"method"`
	URL      string `json:"url"`
	Status   int    `json:"status"`
	Size     int    `json:"size_bytes"`
	TimeMs   int64  `json:"time_ms"`
	MimeType string `json:"mime_type,omitempty"`
}

// PreviewResult is the all-in-one dev report for a page.
type PreviewResult struct {
	PageInfo  *PageInfo          `json:"page"`
	Errors    []ErrorEntry       `json:"errors"`
	Network   []NetworkEntry     `json:"network"`
	DOM       *ExtractionResult  `json:"dom"`
	Summary   PreviewSummary     `json:"summary"`
}

// PreviewSummary provides quick stats.
type PreviewSummary struct {
	TotalRequests  int `json:"total_requests"`
	FailedRequests int `json:"failed_requests"`
	ErrorCount     int `json:"error_count"`
	WarningCount   int `json:"warning_count"`
	InteractiveCount int `json:"interactive_count"`
}

// networkCollector captures all network requests (not just errors).
type networkCollector struct {
	mu       sync.Mutex
	entries  []NetworkEntry
	startAt  time.Time
}

func newNetworkCollector() *networkCollector {
	return &networkCollector{startAt: time.Now()}
}

func (nc *networkCollector) listen(page *rod.Page) {
	go page.EachEvent(
		func(e *proto.NetworkResponseReceived) {
			entry := NetworkEntry{
				URL:      e.Response.URL,
				Status:   e.Response.Status,
				MimeType: e.Response.MIMEType,
				TimeMs:   time.Since(nc.startAt).Milliseconds(),
			}
			// Try to extract size from headers
			if e.Response.EncodedDataLength > 0 {
				entry.Size = int(e.Response.EncodedDataLength)
			}

			nc.mu.Lock()
			nc.entries = append(nc.entries, entry)
			nc.mu.Unlock()
		},
	)()
}

func (nc *networkCollector) Entries() []NetworkEntry {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	result := make([]NetworkEntry, len(nc.entries))
	copy(result, nc.entries)
	return result
}

// Preview performs a full page analysis: navigate + collect errors + collect network + extract DOM.
func Preview(page *rod.Page, url string, waitStrategy string, extractLevel ExtractLevel) (*PreviewResult, error) {
	// Start collectors before navigation
	errorCollector := NewErrorCollector(page)
	netCollector := newNetworkCollector()
	netCollector.listen(page)

	// Navigate
	info, err := Navigate(page, url, waitStrategy)
	if err != nil {
		return nil, err
	}

	// Small delay to let async errors/requests settle
	time.Sleep(500 * time.Millisecond)

	// Extract DOM
	dom, err := Extract(page, extractLevel, "")
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}

	// Collect results
	errors := errorCollector.Errors()
	network := netCollector.Entries()

	// Build summary
	failedReqs := 0
	for _, n := range network {
		if n.Status >= 400 {
			failedReqs++
		}
	}
	errorCount := 0
	warningCount := 0
	for _, e := range errors {
		if e.Level == "error" || e.Level == "5xx" {
			errorCount++
		} else {
			warningCount++
		}
	}

	return &PreviewResult{
		PageInfo: info,
		Errors:   errors,
		Network:  network,
		DOM:      dom,
		Summary: PreviewSummary{
			TotalRequests:    len(network),
			FailedRequests:   failedReqs,
			ErrorCount:       errorCount,
			WarningCount:     warningCount,
			InteractiveCount: dom.Stats.InteractiveCount,
		},
	}, nil
}

// FormatPreview renders a compact text report.
func FormatPreview(r *PreviewResult) string {
	var sb strings.Builder

	// Status line
	sb.WriteString(fmt.Sprintf("[%d] %s — %s (%dms)\n", r.PageInfo.Status, r.PageInfo.Title, r.PageInfo.URL, r.PageInfo.TimeMs))

	// Errors summary
	if len(r.Errors) == 0 {
		sb.WriteString("[errors] none\n")
	} else {
		sb.WriteString(fmt.Sprintf("[errors] %d error(s), %d warning(s)\n", r.Summary.ErrorCount, r.Summary.WarningCount))
		for _, e := range r.Errors {
			switch e.Type {
			case "console":
				src := ""
				if e.Source != "" {
					src = fmt.Sprintf(" (%s)", truncateURL(e.Source))
				}
				sb.WriteString(fmt.Sprintf("  [%s] %s%s\n", e.Level, truncate(e.Message, 120), src))
			case "network":
				sb.WriteString(fmt.Sprintf("  [%d] %s %s\n", e.Status, e.Method, truncateURL(e.Message)))
			}
		}
	}

	// Network summary (compact — only show failed + top 5 by size)
	if len(r.Network) == 0 {
		sb.WriteString("[network] no requests\n")
	} else {
		sb.WriteString(fmt.Sprintf("[network] %d reqs, %d failed\n", r.Summary.TotalRequests, r.Summary.FailedRequests))
		// Show failed requests
		count := 0
		for _, n := range r.Network {
			if n.Status >= 400 {
				sb.WriteString(fmt.Sprintf("  [%d] %s (%dms)\n", n.Status, truncateURL(n.URL), n.TimeMs))
				count++
				if count >= 5 {
					break
				}
			}
		}
	}

	// DOM
	sb.WriteString("[dom]\n")
	domText := FormatText(r.DOM)
	// Indent DOM lines
	for _, line := range strings.Split(domText, "\n") {
		if line != "" {
			sb.WriteString("  " + line + "\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// truncate shortens a string to maxLen.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// truncateURL shortens URLs for display.
func truncateURL(u string) string {
	// Remove common prefixes
	u = strings.TrimPrefix(u, "http://localhost")
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return truncate(u, 80)
}
