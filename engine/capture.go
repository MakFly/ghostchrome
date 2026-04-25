package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// CaptureSpec configures a passive network capture session.
// Generic DevTools-Network-tab style recorder: no request modification,
// no auth bypass — just observation of what the page already requested.
type CaptureSpec struct {
	// URLMatch is a regex applied to the full request URL. Empty = match all.
	URLMatch string
	// MimeMatch is a regex applied to the response MIME type. Empty = match all.
	MimeMatch string
	// Max is the number of MATCHING entries to collect before ReachedMax fires.
	// 0 = unlimited.
	Max int
	// IncludeBody, when true, fetches the response body via
	// Network.getResponseBody for every matching entry.
	IncludeBody bool
	// OutputPath, if set, streams each entry as it is captured (NDJSON).
	OutputPath string
}

// CapturedEntry is one fully-hydrated request/response pair.
type CapturedEntry struct {
	RequestID    string            `json:"request_id"`
	Method       string            `json:"method"`
	URL          string            `json:"url"`
	ResourceType string            `json:"resource_type"`
	Status       int               `json:"status"`
	StatusText   string            `json:"status_text,omitempty"`
	MimeType     string            `json:"mime_type,omitempty"`
	ReqHeaders   map[string]string `json:"request_headers,omitempty"`
	ResHeaders   map[string]string `json:"response_headers,omitempty"`
	PostData     string            `json:"post_data,omitempty"`
	Body         string            `json:"body,omitempty"`
	BodyBase64   bool              `json:"body_base64,omitempty"`
	BodyError    string            `json:"body_error,omitempty"`
	StartedAt    string            `json:"started_at"`
}

// CaptureSession owns the goroutine that listens to Network events.
type CaptureSession struct {
	spec       CaptureSpec
	urlRe      *regexp.Regexp
	mimeRe     *regexp.Regexp
	page       *rod.Page
	stop       func()
	outFile    *os.File
	outMu      sync.Mutex
	mu         sync.Mutex
	pending    map[proto.NetworkRequestID]*CapturedEntry
	matched    []*CapturedEntry
	reachedMax chan struct{}
	maxFired   bool
}

// StartCapture enables the Network domain and begins listening.
func StartCapture(page *rod.Page, spec CaptureSpec) (*CaptureSession, error) {
	enable := proto.NetworkEnable{}
	if err := enable.Call(page); err != nil {
		return nil, fmt.Errorf("network enable: %w", err)
	}

	s := &CaptureSession{
		spec:       spec,
		page:       page,
		pending:    map[proto.NetworkRequestID]*CapturedEntry{},
		reachedMax: make(chan struct{}),
	}
	if spec.URLMatch != "" {
		re, err := regexp.Compile(spec.URLMatch)
		if err != nil {
			return nil, fmt.Errorf("url-match regex: %w", err)
		}
		s.urlRe = re
	}
	if spec.MimeMatch != "" {
		re, err := regexp.Compile(spec.MimeMatch)
		if err != nil {
			return nil, fmt.Errorf("mime-match regex: %w", err)
		}
		s.mimeRe = re
	}
	if spec.OutputPath != "" {
		f, err := os.OpenFile(spec.OutputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, fmt.Errorf("open output: %w", err)
		}
		s.outFile = f
	}

	s.stop = page.EachEvent(
		func(e *proto.NetworkRequestWillBeSent) {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.pending[e.RequestID] = &CapturedEntry{
				RequestID:    string(e.RequestID),
				Method:       e.Request.Method,
				URL:          e.Request.URL,
				ResourceType: string(e.Type),
				ReqHeaders:   flattenHeaders(e.Request.Headers),
				PostData:     e.Request.PostData,
				StartedAt:    time.Now().UTC().Format(time.RFC3339Nano),
			}
		},
		func(e *proto.NetworkResponseReceived) {
			s.mu.Lock()
			entry, ok := s.pending[e.RequestID]
			s.mu.Unlock()
			if !ok {
				return
			}
			entry.Status = e.Response.Status
			entry.StatusText = e.Response.StatusText
			entry.MimeType = e.Response.MIMEType
			entry.ResHeaders = flattenHeaders(e.Response.Headers)
		},
		func(e *proto.NetworkLoadingFinished) {
			s.mu.Lock()
			entry, ok := s.pending[e.RequestID]
			if ok {
				delete(s.pending, e.RequestID)
			}
			s.mu.Unlock()
			if !ok {
				return
			}
			if !s.matches(entry) {
				return
			}
			if spec.IncludeBody {
				body, err := proto.NetworkGetResponseBody{RequestID: e.RequestID}.Call(page)
				if err != nil {
					entry.BodyError = err.Error()
				} else if body != nil {
					entry.Body = body.Body
					entry.BodyBase64 = body.Base64Encoded
				}
			}
			s.record(entry)
		},
		func(e *proto.NetworkLoadingFailed) {
			s.mu.Lock()
			delete(s.pending, e.RequestID)
			s.mu.Unlock()
		},
	)

	return s, nil
}

func (s *CaptureSession) matches(e *CapturedEntry) bool {
	if s.urlRe != nil && !s.urlRe.MatchString(e.URL) {
		return false
	}
	if s.mimeRe != nil && !s.mimeRe.MatchString(e.MimeType) {
		return false
	}
	return true
}

func (s *CaptureSession) record(e *CapturedEntry) {
	s.mu.Lock()
	s.matched = append(s.matched, e)
	n := len(s.matched)
	s.mu.Unlock()

	if s.outFile != nil {
		s.outMu.Lock()
		if data, err := json.Marshal(e); err == nil {
			s.outFile.Write(data)
			s.outFile.Write([]byte("\n"))
		}
		s.outMu.Unlock()
	}

	if s.spec.Max > 0 && n >= s.spec.Max {
		s.mu.Lock()
		if !s.maxFired {
			s.maxFired = true
			close(s.reachedMax)
		}
		s.mu.Unlock()
	}
}

// ReachedMax fires once Max matches have been collected.
func (s *CaptureSession) ReachedMax() <-chan struct{} { return s.reachedMax }

// Stop detaches listeners and closes the output file.
func (s *CaptureSession) Stop() []*CapturedEntry {
	if s.stop != nil {
		s.stop()
	}
	if s.outFile != nil {
		s.outFile.Close()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.matched
}

// Entries returns a snapshot of collected entries.
func (s *CaptureSession) Entries() []*CapturedEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*CapturedEntry, len(s.matched))
	copy(out, s.matched)
	return out
}

func flattenHeaders(h proto.NetworkHeaders) map[string]string {
	if h == nil {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		switch vv := v.Val().(type) {
		case string:
			out[k] = vv
		default:
			out[k] = strings.TrimSpace(fmt.Sprintf("%v", vv))
		}
	}
	return out
}
