package engine

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// ErrorEntry represents a single console or network error.
type ErrorEntry struct {
	Type    string `json:"type"`               // "console" or "network"
	Level   string `json:"level"`              // "error", "warning", "4xx", "5xx"
	Message string `json:"message"`            // error message or URL
	Source  string `json:"source"`             // file:line for console, URL for network
	Status  int    `json:"status,omitempty"`   // HTTP status for network errors
	Method  string `json:"method,omitempty"`   // HTTP method for network
	TimeMs  int64  `json:"time_ms"`            // timestamp relative to collector start
}

// ErrorCollector collects console and network errors from a page via CDP events.
type ErrorCollector struct {
	mu      sync.Mutex
	errors  []ErrorEntry
	startAt time.Time
}

// NewErrorCollector creates a collector and starts listening on the page.
// It hooks into RuntimeConsoleAPICalled, RuntimeExceptionThrown, and NetworkResponseReceived.
func NewErrorCollector(page *rod.Page) *ErrorCollector {
	c := &ErrorCollector{
		startAt: time.Now(),
	}

	go page.EachEvent(
		func(e *proto.RuntimeConsoleAPICalled) {
			typ := string(e.Type)
			if typ != "error" && typ != "warning" {
				return
			}

			// Build message from args
			var parts []string
			for _, arg := range e.Args {
				if !arg.Value.Nil() {
					parts = append(parts, arg.Value.String())
				} else if arg.Description != "" {
					parts = append(parts, arg.Description)
				} else if arg.UnserializableValue != "" {
					parts = append(parts, string(arg.UnserializableValue))
				}
			}
			msg := strings.Join(parts, " ")
			if msg == "" {
				msg = "(empty)"
			}

			// Build source from stack trace
			source := ""
			if e.StackTrace != nil && len(e.StackTrace.CallFrames) > 0 {
				f := e.StackTrace.CallFrames[0]
				source = fmt.Sprintf("%s:%d", f.URL, f.LineNumber)
			}

			c.mu.Lock()
			c.errors = append(c.errors, ErrorEntry{
				Type:    "console",
				Level:   typ,
				Message: msg,
				Source:  source,
				TimeMs:  time.Since(c.startAt).Milliseconds(),
			})
			c.mu.Unlock()
		},
		func(e *proto.RuntimeExceptionThrown) {
			msg := ""
			source := ""
			if e.ExceptionDetails.Exception != nil {
				if e.ExceptionDetails.Exception.Description != "" {
					msg = e.ExceptionDetails.Exception.Description
				} else if !e.ExceptionDetails.Exception.Value.Nil() {
					msg = e.ExceptionDetails.Exception.Value.String()
				}
			}
			if msg == "" && e.ExceptionDetails.Text != "" {
				msg = e.ExceptionDetails.Text
			}
			if e.ExceptionDetails.URL != "" {
				source = fmt.Sprintf("%s:%d", e.ExceptionDetails.URL, e.ExceptionDetails.LineNumber)
			} else if e.ExceptionDetails.StackTrace != nil && len(e.ExceptionDetails.StackTrace.CallFrames) > 0 {
				f := e.ExceptionDetails.StackTrace.CallFrames[0]
				source = fmt.Sprintf("%s:%d", f.URL, f.LineNumber)
			}

			c.mu.Lock()
			c.errors = append(c.errors, ErrorEntry{
				Type:    "console",
				Level:   "error",
				Message: msg,
				Source:  source,
				TimeMs:  time.Since(c.startAt).Milliseconds(),
			})
			c.mu.Unlock()
		},
		func(e *proto.NetworkResponseReceived) {
			status := e.Response.Status
			if status < 400 {
				return
			}

			level := "4xx"
			if status >= 500 {
				level = "5xx"
			}

			// Try to get the HTTP method from the request
			method := ""
			if e.Response.RequestHeaders != nil {
				// Method is on the request, not the response headers.
				// We approximate via the resource type or leave empty.
			}

			c.mu.Lock()
			c.errors = append(c.errors, ErrorEntry{
				Type:    "network",
				Level:   level,
				Message: e.Response.URL,
				Source:  e.Response.URL,
				Status:  status,
				Method:  method,
				TimeMs:  time.Since(c.startAt).Milliseconds(),
			})
			c.mu.Unlock()
		},
	)()

	return c
}

// Errors returns all collected errors (snapshot).
func (c *ErrorCollector) Errors() []ErrorEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]ErrorEntry, len(c.errors))
	copy(result, c.errors)
	return result
}

// FormatErrors formats errors as compact text lines.
func FormatErrors(errors []ErrorEntry) string {
	if len(errors) == 0 {
		return "No errors found"
	}
	var lines []string
	for _, e := range errors {
		switch e.Type {
		case "console":
			src := ""
			if e.Source != "" {
				src = fmt.Sprintf(" (%s)", e.Source)
			}
			lines = append(lines, fmt.Sprintf("[console:%s] %s%s", e.Level, e.Message, src))
		case "network":
			method := e.Method
			if method == "" {
				method = "GET"
			}
			lines = append(lines, fmt.Sprintf("[network:%d] %s %s (%dms)", e.Status, method, e.Message, e.TimeMs))
		}
	}
	return strings.Join(lines, "\n")
}
