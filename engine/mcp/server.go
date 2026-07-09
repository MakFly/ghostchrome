// Package mcp exposes ghostchrome as a Model Context Protocol server so
// that LLM agents (Claude Code, Codex, Cursor, ...) can drive the browser
// via stdio JSON-RPC instead of forking the CLI per call.
//
// The server holds a single long-lived *engine.Browser + *rod.Page across
// tool calls so refs (@1, @2) extracted by one tool stay valid for the
// next click/type.
package mcp

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/MakFly/ghostchrome/engine/policy"
	"github.com/go-rod/rod"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"
)

// Options configure the shared browser the MCP server attaches to. Mirrors
// the global flags exposed by the ghostchrome CLI.
type Options struct {
	Connect        string // ws:// URL or "auto" to discover, empty = auto-launch
	Headless       bool
	Invisible      bool
	UserProfile    string
	Stealth        bool
	DismissCookies bool
	Proxy          string
	TimeoutSec     int
	// BlockTrackers enables the curated anti-bot script blocker
	// (engine.AntiBotPatterns). Auto-enabled when Stealth is true unless
	// GHOSTCHROME_MCP_NO_BLOCKER=1 is set.
	BlockTrackers bool
	// Policy restricts which domains can be navigated to, and which actions
	// (eval, upload, clipboard) are allowed. Nil means no restrictions.
	Policy *policy.Policy
}

// Server holds the shared browser and exposes tool handlers backed by the
// engine package. Methods are concurrency-safe via mu.
type Server struct {
	opts Options

	mu         sync.Mutex
	browser    *engine.Browser
	page       *rod.Page
	snapshot   *engine.PageSnapshot     // last in-memory snapshot (for ref resolution)
	observer   *engine.Observer         // long-lived; feeds error/network info into snapshot
	observerFn context.CancelFunc       // cancel attached to the observer's context
	blocker    *engine.InterceptSession // non-nil when anti-bot blocker is active
}

// New returns a Server that lazy-initializes the browser on the first tool
// call. Build registers all tools on a fresh MCP server and returns it.
func New(opts Options) *Server {
	if opts.TimeoutSec <= 0 {
		opts.TimeoutSec = 30
	}
	if opts.Policy != nil {
		engine.ActivePolicy = opts.Policy
	}
	return &Server{opts: opts}
}

// PrewarmAsync spawns Chrome in the background so the first user-facing tool
// call doesn't pay the ~1.3s cold-start. Safe to call from main(): if the
// browser is already up it's a no-op; if init fails, the next withPage()
// will surface the error normally.
func (s *Server) PrewarmAsync() {
	go func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		_, _, _ = s.ensurePageLocked()
	}()
}

// ExtraToolRegistrars lets optional, build-tagged recipes (compiled with
// `-tags recipes`) attach extra MCP tools without touching the core tool set.
// A recipe appends to it from an init(); Build() applies them after the core
// tools. Empty in the default binary — zero token cost when no recipe is on.
var ExtraToolRegistrars []func(srv *mcpsrv.MCPServer)

// Build returns an MCP server with every ghostchrome tool registered.
// Caller is expected to call mcpsrv.ServeStdio on it.
func (s *Server) Build(name, version string) *mcpsrv.MCPServer {
	srv := mcpsrv.NewMCPServer(name, version, mcpsrv.WithToolCapabilities(true))
	registerTools(srv, s)
	for _, reg := range ExtraToolRegistrars {
		reg(srv)
	}
	return srv
}

// Close releases the underlying browser. Safe to call multiple times.
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.observer != nil {
		_ = s.observer.Stop()
		s.observer = nil
	}
	if s.observerFn != nil {
		s.observerFn()
		s.observerFn = nil
	}
	if s.blocker != nil {
		_ = s.blocker.Stop()
		s.blocker = nil
	}
	if s.browser != nil {
		s.browser.Close()
		s.browser = nil
		s.page = nil
		s.snapshot = nil
	}
}

// ensurePage opens the browser+page on first call. Subsequent calls reuse
// the held instances. Caller MUST hold s.mu (or be inside a tool handler
// that does so via withPage).
func (s *Server) ensurePageLocked() (*engine.Browser, *rod.Page, error) {
	if s.page != nil && s.browser != nil {
		return s.browser, s.page, nil
	}
	bopts := engine.BrowserOpts{
		ConnectURL: s.opts.Connect,
		Headless:   s.opts.Headless,
		Invisible:  s.opts.Invisible,
		TimeoutSec: s.opts.TimeoutSec,
		Proxy:      s.opts.Proxy,
	}
	if s.opts.Connect == "auto" {
		ws, err := engine.DiscoverCDP(nil, 800*time.Millisecond)
		if err != nil {
			return nil, nil, fmt.Errorf("connect=auto: %w (start Chrome with --remote-debugging-port=9222)", err)
		}
		bopts.ConnectURL = ws
		bopts.AttachFresh = true
	}
	if s.opts.UserProfile != "" && s.opts.Connect == "" {
		dir, err := engine.ResolveProfileDir(s.opts.UserProfile)
		if err != nil {
			return nil, nil, fmt.Errorf("user-profile: %w", err)
		}
		bopts.UserDataDir = dir
	}

	b, err := engine.NewBrowserWith(bopts)
	if err != nil {
		return nil, nil, fmt.Errorf("browser: %w", err)
	}
	page, err := b.Page()
	if err != nil {
		b.Close()
		return nil, nil, fmt.Errorf("page: %w", err)
	}
	if s.opts.Stealth {
		_ = engine.ApplyStealth(page)
	}

	// Anti-bot script blocker. Default-on when Stealth is set, opt-out via
	// GHOSTCHROME_MCP_NO_BLOCKER=1. Blocking heavy fingerprinting JS up
	// front is what prevents the autoscout24-style 30s eval freezes:
	// DataDome's main-thread bursts can't run if the script never loads.
	wantBlocker := s.opts.BlockTrackers || (s.opts.Stealth && os.Getenv("GHOSTCHROME_MCP_NO_BLOCKER") != "1")
	if wantBlocker {
		if sess, err := engine.StartAntiBotBlocker(b.RodBrowser()); err == nil {
			s.blocker = sess
		} else {
			fmt.Fprintf(os.Stderr, "[ghostchrome mcp] anti-bot blocker disabled: %v\n", err)
		}
	}

	// Long-lived Observer so the `errors` tool can return events accumulated
	// since the session started, not only those captured during the call.
	// Skipped in stealth mode: it subscribes to Runtime.consoleAPICalled /
	// Runtime.exceptionThrown, which makes rod auto-enable the Runtime CDP
	// domain on every session — a persistent, detectable signal in stealth
	// mode. The `errors` tool degrades to empty instead of leaking it.
	if s.opts.Stealth {
		fmt.Fprintln(os.Stderr, "[ghostchrome mcp] observer disabled in stealth (avoids Runtime.enable); errors tool will return empty")
	} else {
		obs := engine.NewObserver(page, engine.ObserverOpts{BufferSize: 512})
		obsCtx, cancel := context.WithCancel(context.Background())
		if err := obs.Start(obsCtx); err != nil {
			cancel()
			// Non-fatal: page still works, errors tool will just be empty.
		} else {
			s.observer = obs
			s.observerFn = cancel
		}
	}

	// Pre-enable CDP domains the first extract/snapshot will need. Each is
	// idempotent on the page; doing them now means the first user-facing
	// tool call doesn't pay one round-trip per domain (~5-20ms saved).
	engine.PrewarmDomains(page)

	s.browser = b
	s.page = page
	return b, page, nil
}

// withPage runs fn under the server lock with a guaranteed-live page.
// Returns an MCP error result on initialization failure.
func (s *Server) withPage(fn func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error)) (*mcpgo.CallToolResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, page, err := s.ensurePageLocked()
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return fn(b, page)
}

// rememberSnapshot stores the snapshot built from an extract so subsequent
// ref-based tools can resolve @N. Caller MUST hold s.mu.
func (s *Server) rememberSnapshot(page *rod.Page, result *engine.ExtractionResult) {
	if result == nil || page == nil {
		return
	}
	snap, err := engine.BuildSnapshot(page, result)
	if err != nil {
		return
	}
	s.snapshot = snap
	if s.browser != nil {
		_ = s.browser.SaveSnapshot(page, result)
	}
}

// snapshotForResolve returns the best snapshot to use for ref resolution.
// Prefers the connected browser's snapshot, falls back to in-memory.
func (s *Server) snapshotForResolve(page *rod.Page) *engine.PageSnapshot {
	if s.browser != nil {
		if snap := s.browser.Snapshot(page); snap != nil {
			return snap
		}
	}
	return s.snapshot
}

// errResult turns a Go error into an MCP error tool result.
func errResult(err error) (*mcpgo.CallToolResult, error) {
	return mcpgo.NewToolResultError(err.Error()), nil
}
