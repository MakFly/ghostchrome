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
	"errors"
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
	// IdleTimeout, when > 0, releases the held Chrome after this long with no
	// tool activity. Unlike the serve daemon (which exits), the MCP server
	// stays up and relaunches Chrome on the next tool call — bounding the RAM a
	// forgotten-but-still-open session holds. The zero value disables reaping;
	// cmd/mcp.go supplies a 15m default (GHOSTCHROME_IDLE_TIMEOUT overrides,
	// =0 disables).
	IdleTimeout time.Duration
	// Policy restricts which domains can be navigated to, and which actions
	// (eval, upload, clipboard) are allowed. Nil means no restrictions.
	Policy *policy.Policy
}

// Server holds the shared browser and exposes tool handlers backed by the
// engine package. Methods are concurrency-safe via mu.
type Server struct {
	opts Options

	mu           sync.Mutex
	browser      *engine.Browser
	page         *rod.Page
	snapshot     *engine.PageSnapshot     // last in-memory snapshot (for ref resolution)
	observer     *engine.Observer         // long-lived; feeds error/network info into snapshot
	observerFn   context.CancelFunc       // cancel attached to the observer's context
	blocker      *engine.InterceptSession // non-nil when anti-bot blocker is active
	lastActivity time.Time                // updated on every ensurePageLocked; drives the idle reaper
	closed       bool                     // terminal state set only by Server.Close
	done         chan struct{}            // stops background reapers after terminal close
	closeDone    chan struct{}            // closes after terminal teardown and prewarm drain
	prewarmWG    sync.WaitGroup           // drains any prewarm admitted before terminal close
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
	return &Server{opts: opts, done: make(chan struct{}), closeDone: make(chan struct{})}
}

var errServerClosed = errors.New("mcp server is closed")

// PrewarmAsync spawns Chrome in the background so the first user-facing tool
// call doesn't pay the ~1.3s cold-start. Safe to call from main(): if the
// browser is already up it's a no-op; if init fails, the next withPage()
// will surface the error normally.
func (s *Server) PrewarmAsync() {
	s.mu.Lock()
	if s.closed {
		done := s.closeDone
		s.mu.Unlock()
		<-done
		return
	}
	s.prewarmWG.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.prewarmWG.Done()
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.closed {
			return
		}
		_, _, _ = s.ensurePageLocked()
	}()
}

// StartIdleReaper launches a background loop that releases the held browser
// after opts.IdleTimeout of no tool activity. The MCP server itself stays
// alive: the next tool call relaunches Chrome via ensurePageLocked (the same
// path as crash-recovery), so a forgotten-but-open session stops squatting
// ~600MB of idle Chrome. No-op when IdleTimeout <= 0. The goroutine lives for
// the process lifetime; once the browser is released, ticks are near-free
// (browser == nil short-circuits) until the next call relaunches it.
func (s *Server) StartIdleReaper() {
	if s.opts.IdleTimeout <= 0 {
		return
	}
	go func() {
		// Poll at a quarter of the timeout, clamped to [5s, 1m], so the reap
		// fires within ~timeout+interval without a hot spin loop.
		interval := s.opts.IdleTimeout / 4
		if interval < 5*time.Second {
			interval = 5 * time.Second
		}
		if interval > time.Minute {
			interval = time.Minute
		}
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				s.reapIfIdle()
			case <-s.done:
				return
			}
		}
	}()
}

// reapIfIdle releases the held browser when IdleTimeout has elapsed with no
// tool activity. Split out of the ticker loop so it can be unit-tested without
// waiting on the (>=5s) poll interval. No-op when idle reaping is disabled, no
// browser is held, or activity is recent.
func (s *Server) reapIfIdle() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.opts.IdleTimeout <= 0 || s.browser == nil || s.lastActivity.IsZero() {
		return
	}
	if time.Since(s.lastActivity) >= s.opts.IdleTimeout {
		fmt.Fprintf(os.Stderr, "[ghostchrome mcp] idle for %s — releasing chrome (relaunches on next call)\n", s.opts.IdleTimeout)
		s.closeLocked()
	}
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
	if s.closed {
		done := s.closeDone
		s.mu.Unlock()
		<-done
		return
	}
	s.closed = true
	close(s.done)
	s.closeLocked()
	s.mu.Unlock()
	s.prewarmWG.Wait()
	close(s.closeDone)
}

// closeLocked tears down the browser and every piece of per-browser state
// (observer, blocker, snapshot). Caller MUST hold s.mu.
func (s *Server) closeLocked() {
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

// alivenessProbeTimeout bounds the per-call liveness ping on the held
// browser. Deliberately much shorter than the 30s CDP default: on a live
// Chrome the ping is ~1ms, on a dead one it fails instantly.
const alivenessProbeTimeout = 2 * time.Second

// ensurePage opens the browser+page on first call. Subsequent calls reuse
// the held instances after a cheap liveness probe: if Chrome died under us
// (crash, OOM kill, ...) the stale handles are torn down and a fresh
// browser is launched instead of failing every call forever. Caller MUST
// hold s.mu (or be inside a tool handler that does so via withPage).
func (s *Server) ensurePageLocked() (*engine.Browser, *rod.Page, error) {
	if s.closed {
		return nil, nil, errServerClosed
	}
	// Refresh the idle clock on every call (cached-page reuse, cold launch, and
	// crash-relaunch alike) so the reaper only fires after real inactivity.
	s.lastActivity = time.Now()
	defer func() { s.lastActivity = time.Now() }()
	if s.page != nil && s.browser != nil {
		if s.browser.Alive(alivenessProbeTimeout) {
			return s.browser, s.page, nil
		}
		// Relaunch invalidates refs (@1, @2) from earlier snapshots;
		// closeLocked drops the snapshot so ref-based tools tell the agent
		// to re-snapshot instead of clicking into the void.
		fmt.Fprintln(os.Stderr, "[ghostchrome mcp] chrome is unresponsive, relaunching a fresh browser")
		s.closeLocked()
		b, page, err := s.launchPageLocked()
		if err != nil {
			return nil, nil, fmt.Errorf("chrome process died and could not be relaunched: %w", err)
		}
		return b, page, nil
	}
	return s.launchPageLocked()
}

// launchPageLocked launches (or connects to) Chrome and initializes all
// per-browser state. Caller MUST hold s.mu.
func (s *Server) launchPageLocked() (*engine.Browser, *rod.Page, error) {
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
