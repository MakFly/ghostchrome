package engine

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// InterceptSpec configures a request interception router.
type InterceptSpec struct {
	// BlockPatterns list glob URL patterns to block with
	// NetworkErrorReasonBlockedByClient.
	BlockPatterns []string

	// FulfillPattern optionally matches requests to be answered with the
	// FulfillBody payload and FulfillStatus response code. Only set one pattern.
	FulfillPattern     string
	FulfillBody        []byte
	FulfillStatus      int
	FulfillContentType string
}

// InterceptStats are cumulative counters updated by the router goroutine.
type InterceptStats struct {
	mu        sync.Mutex
	Blocked   int
	Fulfilled int
	Passed    int
}

// Snapshot returns a concurrent-safe copy of the counters.
func (s *InterceptStats) Snapshot() (blocked, fulfilled, passed int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Blocked, s.Fulfilled, s.Passed
}

// InterceptSession owns the router lifetime. Stop() must be called to release
// resources.
type InterceptSession struct {
	router *rod.HijackRouter
	stats  *InterceptStats
	done   chan struct{}
}

// Stats returns the live counters.
func (s *InterceptSession) Stats() *InterceptStats { return s.stats }

// Stop disables interception and waits for the background goroutine.
func (s *InterceptSession) Stop() error {
	err := s.router.Stop()
	<-s.done
	return err
}

// StartIntercept enables Fetch interception on the browser and returns an
// InterceptSession. The caller is responsible for Stop().
func StartIntercept(browser *rod.Browser, spec InterceptSpec) (*InterceptSession, error) {
	if len(spec.BlockPatterns) == 0 && spec.FulfillPattern == "" {
		return nil, fmt.Errorf("intercept: need at least one --block pattern or --fulfill pattern")
	}

	router := browser.HijackRequests()
	stats := &InterceptStats{}

	for _, pattern := range spec.BlockPatterns {
		p := pattern
		if err := router.Add(p, "", func(h *rod.Hijack) {
			h.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
			stats.mu.Lock()
			stats.Blocked++
			stats.mu.Unlock()
		}); err != nil {
			return nil, fmt.Errorf("add block pattern %q: %w", p, err)
		}
	}

	if spec.FulfillPattern != "" {
		body := spec.FulfillBody
		status := spec.FulfillStatus
		if status == 0 {
			status = 200
		}
		contentType := spec.FulfillContentType
		if contentType == "" {
			contentType = detectContentType(body, spec.FulfillPattern)
		}
		if err := router.Add(spec.FulfillPattern, "", func(h *rod.Hijack) {
			h.Response.Payload().ResponseCode = status
			h.Response.Payload().Body = body
			if contentType != "" {
				h.Response.SetHeader("Content-Type", contentType)
			}
			stats.mu.Lock()
			stats.Fulfilled++
			stats.mu.Unlock()
		}); err != nil {
			return nil, fmt.Errorf("add fulfill pattern %q: %w", spec.FulfillPattern, err)
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		router.Run()
	}()

	return &InterceptSession{router: router, stats: stats, done: done}, nil
}

// ParseBlockList splits a comma-separated glob list, trimming spaces and
// dropping empty entries.
func ParseBlockList(s string) []string {
	if s == "" {
		return nil
	}
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// LoadFulfillBody returns raw bytes from a @path literal or the string
// otherwise. Useful so CLI flags can take `"@mock.json"` or an inline payload.
func LoadFulfillBody(value string) ([]byte, error) {
	if strings.HasPrefix(value, "@") {
		return os.ReadFile(value[1:])
	}
	return []byte(value), nil
}

func detectContentType(body []byte, pattern string) string {
	if len(body) > 0 && (body[0] == '{' || body[0] == '[') {
		return "application/json"
	}
	switch strings.ToLower(filepath.Ext(pattern)) {
	case ".json":
		return "application/json"
	case ".html":
		return "text/html"
	case ".js":
		return "application/javascript"
	case ".css":
		return "text/css"
	}
	return http.DetectContentType(body)
}

// drainBody protects against nil readers when callers pass a http.Response.
func drainBody(r io.Reader) []byte {
	if r == nil {
		return nil
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil
	}
	return data
}
