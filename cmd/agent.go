package cmd

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dev-toolings/ghostchrome/engine"
	"github.com/go-rod/rod"
	"github.com/spf13/cobra"
)

// agentRequest is one line on stdin.
type agentRequest struct {
	ID   string          `json:"id"`
	Op   string          `json:"op"`
	Args json.RawMessage `json:"args,omitempty"`
}

// agentResponse is one line on stdout.
type agentResponse struct {
	ID          string                 `json:"id"`
	OK          bool                   `json:"ok"`
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Events      []engine.ObserverEvent `json:"events,omitempty"`
	Observation *engine.Observation    `json:"observation,omitempty"`
}

// agentSession holds long-lived state across requests.
type agentSession struct {
	browser         *engine.Browser
	page            *rod.Page
	snapshot        *engine.PageSnapshot // last in-memory snapshot (auto-launch mode)
	enc             *json.Encoder
	observer        *engine.Observer      // non-nil when --observe is active
	obsFile         *os.File              // non-nil when --observe-out is set
	lastObservation *engine.Observation   // observation from the previous op
	recoveryHooks   []engine.RecoveryHook // pluggable recovery hooks

	// challengeRecovered records whether the last "navigate" op detected AND
	// cleared a bot challenge (DataDome/Cloudflare interstitial). "extract"
	// reads it to opt into the SSR fallback (includeSSR) on the recovery
	// path — the primary reason an automation client drives this JSONL loop
	// against a DataDome-protected, SSR-rendered (Next.js) site in the first
	// place. Always overwritten by "navigate" (never just set-if-true), so a
	// later clean navigate correctly resets it.
	//
	// One-shot: "extract" consumes it via consumeChallengeRecovered, which
	// resets it to false immediately after reading. Any op that navigates or
	// changes the current page (see resetsChallengeRecovered) also resets it
	// preemptively, so only the "extract" immediately following a recovered
	// "navigate" opts into SSR — not every extract until the next navigate.
	challengeRecovered bool
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run an interactive JSONL loop on stdin/stdout for LLM agents",
	Long: `Reads one JSON request per line on stdin and writes one JSON response per
line on stdout. The browser stays alive across requests so refs from a prior
extract are valid for the next click/type.

Request:  {"id":"r1","op":"navigate","args":{"url":"https://example.com"}}
Response: {"id":"r1","ok":true,"result":{"status":200,"title":"...","url":"..."}}

Supported ops:
  init                                       (open browser, no-op if already open)
  navigate    {url, wait?}                   (wait: load|stable|networkidle)
  back / forward
  extract     {level?, selector?}            level: skeleton|content|full
  click       {ref}
  type        {ref, text}
  press       {key, ref?}
  hover       {ref}
  select      {ref, values[]}
  fill        {fields: {ref: value}}
  scroll_by   {dy}
  scroll_to   {y?, bottom?}
  eval        {expr, ref?}
  screenshot  {full_page?, ref?, quality?}   returns base64 PNG/JPEG
  wait        {selector?, ms?}
  errors                                     console + network errors
  url
  close

Stale refs auto-trigger one re-extraction + retry transparently.

With --stealth: after "navigate" detects+clears a DataDome/Cloudflare
challenge, the very next "extract" auto-includes SSR payloads
(__NEXT_DATA__ / RSC self.__next_f) until the next "navigate".`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runAgentLoop()
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
}

func runAgentLoop() {
	sess := &agentSession{
		enc:           json.NewEncoder(os.Stdout),
		recoveryHooks: engine.DefaultRecoveryHooks(),
	}
	defer sess.shutdown()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1<<16), 8<<20) // up to 8 MiB lines for big eval/fill payloads
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req agentRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sess.write(agentResponse{ID: "", OK: false, Error: "parse: " + err.Error()})
			continue
		}

		// Capture observer window start time for this op.
		tStart := time.Now().UnixMilli()

		// Snapshot before op (for a11y diff in observation).
		snapshotBefore := sess.currentSnapshotIfAvailable()

		result, err := sess.dispatch(req)

		// Attach observer events captured during this op.
		var obsEvents []engine.ObserverEvent
		if sess.observer != nil {
			obsEvents = sess.observer.Drain(tStart)
			// Also append to file if configured.
			if sess.obsFile != nil {
				enc := json.NewEncoder(sess.obsFile)
				for _, evt := range obsEvents {
					_ = enc.Encode(evt)
				}
			}
		}

		// Build observation (non-nil when a page is active).
		var obs *engine.Observation
		if sess.page != nil {
			snapshotAfter := sess.currentSnapshotIfAvailable()
			built := engine.BuildObservation(sess.page, snapshotBefore, snapshotAfter, obsEvents)
			obs = &built
			sess.lastObservation = obs
		}

		if err != nil {
			resp := agentResponse{ID: req.ID, OK: false, Error: err.Error(), Observation: obs}
			if len(obsEvents) > 0 {
				resp.Events = obsEvents
			}
			sess.write(resp)
			continue
		}
		resp := agentResponse{ID: req.ID, OK: true, Result: result, Observation: obs}
		if len(obsEvents) > 0 {
			resp.Events = obsEvents
		}
		sess.write(resp)
		if req.Op == "close" {
			return
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(os.Stderr, "agent: scanner: %v\n", err)
	}
}

func (s *agentSession) write(resp agentResponse) {
	if err := s.enc.Encode(resp); err != nil {
		fmt.Fprintf(os.Stderr, "agent: write response %s: %v\n", resp.ID, err)
	}
}

func (s *agentSession) shutdown() {
	if s.observer != nil {
		_ = s.observer.Stop()
	}
	if s.obsFile != nil {
		_ = s.obsFile.Close()
	}
	if s.browser != nil {
		s.browser.Close()
	}
}

func (s *agentSession) ensurePage() (*engine.Browser, *rod.Page, error) {
	if s.page != nil {
		return s.browser, s.page, nil
	}
	opts := buildBrowserOpts()
	b, err := engine.NewBrowserWith(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("browser: %w", err)
	}
	page, err := b.Page()
	if err != nil {
		b.Close()
		return nil, nil, fmt.Errorf("page: %w", err)
	}
	if flagStealth {
		if err := engine.ApplyStealth(page); err != nil {
			fmt.Fprintf(os.Stderr, "warning: stealth not fully applied: %v\n", err)
		}
	}
	s.browser = b
	s.page = page

	// Start observer sidecar if --observe is active.
	if flagObserve {
		if flagStealth {
			fmt.Fprintln(os.Stderr, "ghostchrome: --observe enables the Runtime CDP domain, weakening --stealth")
		}
		obsOpts := engine.ObserverOpts{}
		obs := engine.NewObserver(page, obsOpts)
		if err := obs.Start(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "agent: observer start failed: %v\n", err)
		} else {
			s.observer = obs
			// Drain the channel asynchronously so it never blocks.
			go func() {
				for range obs.Events() {
					// Events are consumed by Drain() per-op; just keep channel drained.
				}
			}()
		}
		// Open file if --observe-out is set.
		if flagObserveOut != "" {
			safe, err := validateOutputPath(flagObserveOut)
			if err != nil {
				fmt.Fprintf(os.Stderr, "agent: observe-out: %v\n", err)
			} else {
				f, err := os.OpenFile(safe, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
				if err != nil {
					fmt.Fprintf(os.Stderr, "agent: observe-out open: %v\n", err)
				} else {
					s.obsFile = f
				}
			}
		}
	}

	return b, page, nil
}

// dispatch routes the op to its handler. Handlers may return any JSON-encodable
// value; nil means "ok with no payload".
func (s *agentSession) dispatch(req agentRequest) (interface{}, error) {
	if resetsChallengeRecovered(req.Op) {
		s.challengeRecovered = false
	}
	switch req.Op {
	case "init":
		_, _, err := s.ensurePage()
		return nil, err
	case "navigate":
		return s.opNavigate(req.Args)
	case "back":
		return s.opHistory(-1)
	case "forward":
		return s.opHistory(+1)
	case "reload":
		return s.opReload()
	case "extract":
		return s.opExtract(req.Args)
	case "click":
		return s.opRef(req.Args, "click")
	case "dblclick":
		return s.opRef(req.Args, "dblclick")
	case "check":
		return s.opCheck(req.Args, true)
	case "uncheck":
		return s.opCheck(req.Args, false)
	case "type":
		return s.opType(req.Args)
	case "press":
		return s.opPress(req.Args)
	case "hover":
		return s.opRef(req.Args, "hover")
	case "select":
		return s.opSelect(req.Args)
	case "fill":
		return s.opFill(req.Args)
	case "scroll_by":
		return s.opScrollBy(req.Args)
	case "scroll_to":
		return s.opScrollTo(req.Args)
	case "eval":
		return s.opEval(req.Args)
	case "screenshot":
		return s.opScreenshot(req.Args)
	case "wait":
		return s.opWait(req.Args)
	case "errors":
		return s.opErrors()
	case "url":
		_, page, err := s.ensurePage()
		if err != nil {
			return nil, err
		}
		info, _ := page.Info()
		if info == nil {
			return map[string]string{"url": "", "title": ""}, nil
		}
		return map[string]string{"url": info.URL, "title": info.Title}, nil
	case "close":
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown op %q", req.Op)
	}
}

// resetsChallengeRecovered reports whether the given JSONL op navigates or
// otherwise changes the current page. Such ops must reset challengeRecovered
// preemptively: the one-shot SSR opt-in is only meant for the "extract" call
// immediately following a recovered "navigate", not for an unrelated page
// reached via an intervening click/press/back/forward/reload/type/select.
func resetsChallengeRecovered(op string) bool {
	switch op {
	case "back", "forward", "reload", "click", "dblclick", "type", "press", "select":
		return true
	default:
		return false
	}
}

// consumeChallengeRecovered returns the current SSR opt-in flag and resets it
// to false. One-shot: only the first "extract" after a recovered "navigate"
// sees it as true.
func (s *agentSession) consumeChallengeRecovered() bool {
	v := s.challengeRecovered
	s.challengeRecovered = false
	return v
}

// ----- handlers ------------------------------------------------------------

func (s *agentSession) opNavigate(raw json.RawMessage) (interface{}, error) {
	var a struct {
		URL  string `json:"url"`
		Wait string `json:"wait"`
	}
	if err := unmarshalArgs(raw, &a); err != nil {
		return nil, err
	}
	if a.URL == "" {
		return nil, errors.New("navigate: url required")
	}
	if a.Wait == "" {
		a.Wait = "load"
	}
	_, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	info, err := engine.Navigate(page, a.URL, a.Wait)
	if err != nil {
		return nil, err
	}
	if flagStealth {
		s.challengeRecovered = engine.WaitForBotChallenge(page, 45*time.Second)
	}
	return info, nil
}

func (s *agentSession) opHistory(delta int) (interface{}, error) {
	_, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	if delta < 0 {
		err = page.NavigateBack()
	} else {
		err = page.NavigateForward()
	}
	if err != nil {
		return nil, err
	}
	_ = engine.WaitForPage(page, "load")
	info, _ := page.Info()
	out := map[string]string{}
	if info != nil {
		out["url"] = info.URL
		out["title"] = info.Title
	}
	return out, nil
}

func (s *agentSession) opExtract(raw json.RawMessage) (interface{}, error) {
	var a struct {
		Level    string `json:"level"`
		Selector string `json:"selector"`
	}
	if err := unmarshalArgs(raw, &a); err != nil {
		return nil, err
	}
	if a.Level == "" {
		a.Level = "content"
	}
	level := engine.ExtractLevel(a.Level)
	if err := engine.ValidateExtractLevel(level); err != nil {
		return nil, err
	}
	b, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	// includeSSR opts in on the recovery path (see challengeRecovered doc).
	// One-shot: consumeChallengeRecovered resets it immediately after reading.
	result, err := engine.Extract(page, level, a.Selector, s.consumeChallengeRecovered())
	if err != nil {
		return nil, err
	}
	if a.Selector == "" {
		_ = b.SaveSnapshot(page, result)
		if snap, serr := engine.BuildSnapshot(page, result); serr == nil {
			s.snapshot = snap
		}
	}
	return result, nil
}

func (s *agentSession) opRef(raw json.RawMessage, op string) (interface{}, error) {
	var a struct {
		Ref string `json:"ref"`
	}
	if err := unmarshalArgs(raw, &a); err != nil {
		return nil, err
	}
	if a.Ref == "" {
		return nil, fmt.Errorf("%s: ref required", op)
	}
	b, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	return nil, s.withRefRetry(b, page, func(snap *engine.PageSnapshot) error {
		switch op {
		case "click":
			return engine.ClickRef(page, a.Ref, snap)
		case "dblclick":
			return engine.DblClickRef(page, a.Ref, snap)
		case "hover":
			return engine.HoverRef(page, a.Ref, snap)
		}
		return fmt.Errorf("opRef: unsupported op %q", op)
	})
}

// opCheck ticks (checked=true) or unticks (checked=false) a checkbox/radio.
func (s *agentSession) opCheck(raw json.RawMessage, checked bool) (interface{}, error) {
	var a struct {
		Ref string `json:"ref"`
	}
	if err := unmarshalArgs(raw, &a); err != nil {
		return nil, err
	}
	verb := "check"
	if !checked {
		verb = "uncheck"
	}
	if a.Ref == "" {
		return nil, fmt.Errorf("%s: ref required", verb)
	}
	b, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	return nil, s.withRefRetry(b, page, func(snap *engine.PageSnapshot) error {
		return engine.SetCheckedRef(page, a.Ref, checked, snap)
	})
}

// opReload refreshes the current page.
func (s *agentSession) opReload() (interface{}, error) {
	_, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	if err := page.Reload(); err != nil {
		return nil, err
	}
	_ = engine.WaitForPage(page, "load")
	info, _ := page.Info()
	out := map[string]string{}
	if info != nil {
		out["url"] = info.URL
		out["title"] = info.Title
	}
	return out, nil
}

func (s *agentSession) opType(raw json.RawMessage) (interface{}, error) {
	var a struct {
		Ref    string `json:"ref"`
		Text   string `json:"text"`
		Submit bool   `json:"submit"`
	}
	if err := unmarshalArgs(raw, &a); err != nil {
		return nil, err
	}
	if a.Ref == "" {
		return nil, errors.New("type: ref required")
	}
	b, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	if err := s.withRefRetry(b, page, func(snap *engine.PageSnapshot) error {
		if err := engine.TypeRef(page, a.Ref, a.Text, snap); err != nil {
			return err
		}
		if a.Submit {
			el, err := engine.ResolveRef(page, a.Ref, snap)
			if err != nil {
				return err
			}
			return engine.SubmitOnElement(page, el)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *agentSession) opPress(raw json.RawMessage) (interface{}, error) {
	var a struct {
		Key string `json:"key"`
		Ref string `json:"ref"`
	}
	if err := unmarshalArgs(raw, &a); err != nil {
		return nil, err
	}
	if a.Key == "" {
		return nil, errors.New("press: key required")
	}
	b, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	return nil, s.withRefRetry(b, page, func(snap *engine.PageSnapshot) error {
		return engine.PressKey(page, a.Key, a.Ref, snap)
	})
}

func (s *agentSession) opSelect(raw json.RawMessage) (interface{}, error) {
	var a struct {
		Ref    string   `json:"ref"`
		Values []string `json:"values"`
	}
	if err := unmarshalArgs(raw, &a); err != nil {
		return nil, err
	}
	if a.Ref == "" || len(a.Values) == 0 {
		return nil, errors.New("select: ref + values required")
	}
	b, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	return nil, s.withRefRetry(b, page, func(snap *engine.PageSnapshot) error {
		return engine.SelectOption(page, a.Ref, a.Values, snap)
	})
}

func (s *agentSession) opFill(raw json.RawMessage) (interface{}, error) {
	var a struct {
		Fields map[string]string `json:"fields"`
	}
	if err := unmarshalArgs(raw, &a); err != nil {
		return nil, err
	}
	if len(a.Fields) == 0 {
		return nil, errors.New("fill: fields required")
	}
	b, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	// Fill in extraction order (@1, @2, @10, …) — Go map iteration is
	// random, and dependent fields (e.g. country → city) need a stable order.
	refs := make([]string, 0, len(a.Fields))
	for ref := range a.Fields {
		refs = append(refs, ref)
	}
	sortRefs(refs)
	for _, ref := range refs {
		val := a.Fields[ref]
		err := s.withRefRetry(b, page, func(snap *engine.PageSnapshot) error {
			return engine.TypeRef(page, ref, val, snap)
		})
		if err != nil {
			return nil, fmt.Errorf("fill %s: %w", ref, err)
		}
	}
	return map[string]int{"filled": len(a.Fields)}, nil
}

// sortRefs orders refs numerically when they follow the @N form, falling
// back to lexicographic order for anything else.
func sortRefs(refs []string) {
	sort.Slice(refs, func(i, j int) bool {
		ni, iok := refNum(refs[i])
		nj, jok := refNum(refs[j])
		if iok && jok {
			return ni < nj
		}
		if iok != jok {
			return iok
		}
		return refs[i] < refs[j]
	})
}

func refNum(ref string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimPrefix(ref, "@"))
	return n, err == nil
}

func (s *agentSession) opScrollBy(raw json.RawMessage) (interface{}, error) {
	var a struct {
		DY int `json:"dy"`
	}
	if err := unmarshalArgs(raw, &a); err != nil {
		return nil, err
	}
	_, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	y, err := engine.ScrollBy(page, a.DY)
	if err != nil {
		return nil, err
	}
	return map[string]int{"y": y}, nil
}

func (s *agentSession) opScrollTo(raw json.RawMessage) (interface{}, error) {
	var a struct {
		Y      int  `json:"y"`
		Bottom bool `json:"bottom"`
	}
	_ = unmarshalArgs(raw, &a)
	_, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	y, err := engine.ScrollToY(page, a.Y, a.Bottom)
	if err != nil {
		return nil, err
	}
	return map[string]int{"y": y}, nil
}

func (s *agentSession) opEval(raw json.RawMessage) (interface{}, error) {
	var a struct {
		Expr string `json:"expr"`
		Ref  string `json:"ref"`
	}
	if err := unmarshalArgs(raw, &a); err != nil {
		return nil, err
	}
	if a.Expr == "" {
		return nil, errors.New("eval: expr required")
	}
	b, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	var out string
	err = s.withRefRetry(b, page, func(snap *engine.PageSnapshot) error {
		v, err := engine.EvalJS(page, a.Expr, a.Ref, snap)
		out = v
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]string{"value": out}, nil
}

func (s *agentSession) opScreenshot(raw json.RawMessage) (interface{}, error) {
	var a struct {
		FullPage bool    `json:"full_page"`
		Ref      string  `json:"ref"`
		Quality  int     `json:"quality"`
		Scale    float64 `json:"scale"`
	}
	_ = unmarshalArgs(raw, &a)
	b, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	var data []byte
	err = s.withRefRetry(b, page, func(snap *engine.PageSnapshot) error {
		d, err := engine.TakeScreenshotScaled(page, a.FullPage, a.Ref, a.Quality, a.Scale, snap)
		data = d
		return err
	})
	if err != nil {
		return nil, err
	}
	mime := "image/png"
	if a.Quality > 0 {
		mime = "image/jpeg"
	}
	return map[string]string{
		"mime":   mime,
		"base64": base64.StdEncoding.EncodeToString(data),
	}, nil
}

func (s *agentSession) opWait(raw json.RawMessage) (interface{}, error) {
	var a struct {
		Selector string `json:"selector"`
		MS       int    `json:"ms"`
	}
	_ = unmarshalArgs(raw, &a)
	_, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	if a.Selector != "" {
		timeout := time.Duration(flagTimeout) * time.Second
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		scoped := page.Timeout(timeout)
		el, err := scoped.Element(a.Selector)
		if err != nil {
			return nil, fmt.Errorf("wait selector %q: %w", a.Selector, err)
		}
		if err := el.WaitVisible(); err != nil {
			return nil, fmt.Errorf("wait visible %q: %w", a.Selector, err)
		}
	}
	if a.MS > 0 {
		time.Sleep(time.Duration(a.MS) * time.Millisecond)
	}
	return nil, nil
}

func (s *agentSession) opErrors() (interface{}, error) {
	_, page, err := s.ensurePage()
	if err != nil {
		return nil, err
	}
	entries, err := engine.CollectErrors(page, "", "", nil)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// withRecovery calls fn with the current snapshot; if it fails, it runs the
// recovery hook chain. If a hook signals retry=true the op is retried once.
//
// RecoverStaleRef (built-in) explicitly refuses to silently remap refs — it
// returns retry=false with an informative error requiring the agent to run an
// explicit re-extract op. This is intentional (see Vague D notes).
func (s *agentSession) withRecovery(b *engine.Browser, page *rod.Page, opName string, fn func(*engine.PageSnapshot) error) error {
	snap := s.currentSnapshot(b, page)
	err := fn(snap)
	if err == nil {
		return nil
	}
	if len(s.recoveryHooks) == 0 {
		return err
	}
	ctx := engine.RecoveryContext{Page: page, Err: err, OpName: opName}
	retry, hookErr := engine.RecoveryChain(ctx, s.recoveryHooks)
	if hookErr != nil {
		return hookErr
	}
	if !retry {
		return err
	}
	// One retry after recovery.
	return fn(s.currentSnapshot(b, page))
}

// withRefRetry is a legacy alias kept for call sites that pre-date withRecovery.
// New code should call withRecovery directly.
func (s *agentSession) withRefRetry(b *engine.Browser, page *rod.Page, fn func(*engine.PageSnapshot) error) error {
	return s.withRecovery(b, page, "ref", fn)
}

// currentSnapshot returns the most relevant snapshot: the connected-mode
// session snapshot if available, else the in-memory one tracked by the
// agent session.
func (s *agentSession) currentSnapshot(b *engine.Browser, page *rod.Page) *engine.PageSnapshot {
	if snap := b.Snapshot(page); snap != nil {
		return snap
	}
	return s.snapshot
}

// currentSnapshotIfAvailable returns a snapshot without requiring a browser/page
// pair — used before the page is initialised (pre-op snapshot for diffs).
func (s *agentSession) currentSnapshotIfAvailable() *engine.PageSnapshot {
	if s.browser != nil && s.page != nil {
		if snap := s.browser.Snapshot(s.page); snap != nil {
			return snap
		}
	}
	return s.snapshot
}

// unmarshalArgs handles the common case of optional / empty args.
func unmarshalArgs(raw json.RawMessage, dst interface{}) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

// newAgentSession constructs a session ready to be driven in-process (e.g.
// from cmd/ai.go). It does NOT bind stdout — JSON encoding is the caller's
// responsibility. Page is opened lazily on the first ensurePage call.
func newAgentSession() *agentSession {
	return &agentSession{
		enc:           json.NewEncoder(io.Discard),
		recoveryHooks: engine.DefaultRecoveryHooks(),
	}
}

// runOp executes one op against the session, returning the result plus the
// per-op observer events and the freshly built Observation. It mirrors the
// per-iteration logic of runAgentLoop so callers don't need to duplicate
// snapshot bookkeeping.
func (s *agentSession) runOp(op string, rawArgs json.RawMessage) (interface{}, *engine.Observation, []engine.ObserverEvent, error) {
	tStart := time.Now().UnixMilli()
	snapshotBefore := s.currentSnapshotIfAvailable()
	result, err := s.dispatch(agentRequest{Op: op, Args: rawArgs})
	var events []engine.ObserverEvent
	if s.observer != nil {
		events = s.observer.Drain(tStart)
	}
	var obs *engine.Observation
	if s.page != nil {
		snapshotAfter := s.currentSnapshotIfAvailable()
		built := engine.BuildObservation(s.page, snapshotBefore, snapshotAfter, events)
		obs = &built
		s.lastObservation = obs
	}
	return result, obs, events, err
}
