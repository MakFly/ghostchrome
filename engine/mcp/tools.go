// Package mcp tool registrations.
//
// MCP v1.0 surface: 11 essential tools for an LLM-agent browser loop.
// Everything that isn't on the hot path (sniff/trace/cookies/storage/tabs/
// viewport/dialog/blocker_stats) lives in the CLI only — adding tools here
// has a real token cost in every `tools/list` the model receives, so we
// stay deliberately small.
package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/go-rod/rod"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"
)

// registerTools wires every MCP tool. The set is intentionally small —
// snapshot covers preview/extract/errors, type covers fill_form, eval is
// the escape hatch for everything else.
func registerTools(srv *mcpsrv.MCPServer, s *Server) {
	srv.AddTool(mcpgo.NewTool("snapshot",
		mcpgo.WithDescription("All-in-one page report: status, console+network errors, compact DOM with refs (@1, @2, ...). If `url` is given, navigate first; otherwise snapshot the current page. This is the canonical first call when an agent visits or revisits a page. Refs from this snapshot stay valid until the next snapshot or navigate."),
		mcpgo.WithString("url", mcpgo.Description("Absolute URL to navigate to before snapshotting (optional — omit to snapshot the current page)")),
		mcpgo.WithString("wait", mcpgo.Description("Wait strategy when navigating: domcontentloaded (default), load, stable, idle, none"), mcpgo.Enum("domcontentloaded", "load", "stable", "idle", "none"), mcpgo.DefaultString("domcontentloaded")),
		mcpgo.WithString("level", mcpgo.Description("DOM extraction depth: skeleton (interactive only, smallest), content (default — adds text), full (everything named)"), mcpgo.Enum("skeleton", "content", "full"), mcpgo.DefaultString("content")),
		mcpgo.WithString("selector", mcpgo.Description("Optional CSS selector to scope the DOM extraction to a subtree")),
	), s.handleSnapshot)

	srv.AddTool(mcpgo.NewTool("navigate",
		mcpgo.WithDescription("Navigate to a URL without a full snapshot. Returns only status + title + load time. Use when you don't need the DOM yet (e.g. chaining click-through pages)."),
		mcpgo.WithString("url", mcpgo.Required(), mcpgo.Description("Absolute URL to navigate to (https://...)")),
		mcpgo.WithString("wait", mcpgo.Description("domcontentloaded (default), load, stable, idle, none"), mcpgo.Enum("domcontentloaded", "load", "stable", "idle", "none"), mcpgo.DefaultString("domcontentloaded")),
	), s.handleNavigate)

	srv.AddTool(mcpgo.NewTool("click",
		mcpgo.WithDescription("Click an element by its ref (@1, @2, ...). Ref comes from the last snapshot. Auto-waits for the element to be attached + visible + stable + enabled."),
		mcpgo.WithString("ref", mcpgo.Required(), mcpgo.Description("Element ref from the last snapshot (e.g. @3 or 3)")),
	), s.handleClick)

	srv.AddTool(mcpgo.NewTool("type",
		mcpgo.WithDescription("Type text into an input/textarea by ref. Set `submit: true` to press Enter after typing (covers the common fill-and-submit pattern in one call)."),
		mcpgo.WithString("ref", mcpgo.Required(), mcpgo.Description("Element ref from the last snapshot")),
		mcpgo.WithString("text", mcpgo.Required(), mcpgo.Description("Text to type (the field is cleared first)")),
		mcpgo.WithBoolean("submit", mcpgo.Description("If true, press Enter after typing"), mcpgo.DefaultBool(false)),
	), s.handleType)

	srv.AddTool(mcpgo.NewTool("select",
		mcpgo.WithDescription("Select one or more options in a <select> element by ref. Pass a single string or an array of strings."),
		mcpgo.WithString("ref", mcpgo.Required(), mcpgo.Description("Element ref of the <select>")),
		mcpgo.WithString("value", mcpgo.Required(), mcpgo.Description("Option value or visible label to select. For multi-select, pass JSON array as string (e.g. \"[\\\"a\\\",\\\"b\\\"]\").")),
	), s.handleSelect)

	srv.AddTool(mcpgo.NewTool("press",
		mcpgo.WithDescription("Press a keyboard key. If `ref` is provided, focus that element first; otherwise the key fires on the document."),
		mcpgo.WithString("key", mcpgo.Required(), mcpgo.Description("Key name: Enter, Tab, Escape, ArrowDown, ArrowUp, PageDown, Backspace, ...")),
		mcpgo.WithString("ref", mcpgo.Description("Optional element ref to focus before pressing")),
	), s.handlePress)

	srv.AddTool(mcpgo.NewTool("wait_for",
		mcpgo.WithDescription("Wait for a condition: selector to appear, text to appear, or just a timeout. At least one of selector/text/timeout_ms must be given."),
		mcpgo.WithString("selector", mcpgo.Description("CSS selector to wait for")),
		mcpgo.WithString("text", mcpgo.Description("Visible text substring to wait for")),
		mcpgo.WithNumber("timeout_ms", mcpgo.Description("Maximum wait time in milliseconds (default 5000, max 30000)"), mcpgo.DefaultNumber(5000)),
	), s.handleWaitFor)

	srv.AddTool(mcpgo.NewTool("eval",
		mcpgo.WithDescription("Evaluate a JavaScript expression in the page and return its serialized value. Async expressions are awaited. Use this as an escape hatch for anything the other tools can't do (read localStorage, dispatch a custom event, scroll, etc.)."),
		mcpgo.WithString("expression", mcpgo.Required(), mcpgo.Description("JS expression. Top-level await OK.")),
		mcpgo.WithString("ref", mcpgo.Description("Optional @ref to scope `this` to an element")),
		mcpgo.WithNumber("timeout_ms", mcpgo.Description("Per-call deadline in milliseconds (default 8000)"), mcpgo.DefaultNumber(8000)),
	), s.handleEval)

	srv.AddTool(mcpgo.NewTool("screenshot",
		mcpgo.WithDescription("Capture the current page (or one element by ref) as an image embedded in the MCP result. Defaults to WebP quality 60 — typically 30-70% lighter than JPEG/PNG for UI captures. Use annotate=true to overlay numbered borders on interactive elements."),
		mcpgo.WithString("ref", mcpgo.Description("Capture only this element by @ref")),
		mcpgo.WithBoolean("full_page", mcpgo.Description("Capture the full scrollable page instead of the viewport"), mcpgo.DefaultBool(false)),
		mcpgo.WithString("format", mcpgo.Description("Image format: webp (default), jpeg, png"), mcpgo.Enum("webp", "jpeg", "png"), mcpgo.DefaultString("webp")),
		mcpgo.WithNumber("quality", mcpgo.Description("Quality 1-100 for webp/jpeg (default 60). Ignored for png."), mcpgo.DefaultNumber(60)),
		mcpgo.WithBoolean("annotate", mcpgo.Description("Overlay numbered borders on interactive elements (forces PNG output)"), mcpgo.DefaultBool(false)),
	), s.handleScreenshot)

	srv.AddTool(mcpgo.NewTool("hover",
		mcpgo.WithDescription("Hover over an element by ref. Triggers CSS :hover states and any hover-bound JS listeners."),
		mcpgo.WithString("ref", mcpgo.Required(), mcpgo.Description("Element ref from the last snapshot")),
	), s.handleHover)

	srv.AddTool(mcpgo.NewTool("drag",
		mcpgo.WithDescription("Drag an element from one ref to another. Simulates a full mouse drag-and-drop sequence."),
		mcpgo.WithString("from", mcpgo.Required(), mcpgo.Description("Source element ref")),
		mcpgo.WithString("to", mcpgo.Required(), mcpgo.Description("Target element ref")),
		mcpgo.WithNumber("steps", mcpgo.Description("Intermediate mouse move steps (default 10)"), mcpgo.DefaultNumber(10)),
	), s.handleDrag)

	srv.AddTool(mcpgo.NewTool("fill_form",
		mcpgo.WithDescription("Fill multiple form fields in one call. Pass a JSON object mapping refs to values: {\"@1\": \"John\", \"@2\": \"john@example.com\"}"),
		mcpgo.WithString("fields", mcpgo.Required(), mcpgo.Description("JSON object mapping @ref to text value")),
	), s.handleFillForm)

	srv.AddTool(mcpgo.NewTool("upload",
		mcpgo.WithDescription("Upload file(s) to a file input element by ref."),
		mcpgo.WithString("ref", mcpgo.Required(), mcpgo.Description("File input element ref")),
		mcpgo.WithString("paths", mcpgo.Required(), mcpgo.Description("File path or JSON array of file paths")),
	), s.handleUpload)

	srv.AddTool(mcpgo.NewTool("tabs",
		mcpgo.WithDescription("List open browser tabs with their URLs, titles, and indices. Use the index with 'switch' action to change the active tab."),
		mcpgo.WithString("action", mcpgo.Description("list (default), switch, close"), mcpgo.Enum("list", "switch", "close"), mcpgo.DefaultString("list")),
		mcpgo.WithNumber("index", mcpgo.Description("Tab index for switch/close actions")),
	), s.handleTabs)

	srv.AddTool(mcpgo.NewTool("back",
		mcpgo.WithDescription("Navigate back in browser history. Returns the new URL."),
	), s.handleBack)

	srv.AddTool(mcpgo.NewTool("forward",
		mcpgo.WithDescription("Navigate forward in browser history. Returns the new URL."),
	), s.handleForward)
}

// ============================================================================
// handlers
// ============================================================================

func (s *Server) handleSnapshot(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	url := mcpgo.ParseString(req, "url", "")
	wait := mcpgo.ParseString(req, "wait", "domcontentloaded")
	levelStr := mcpgo.ParseString(req, "level", "content")
	selector := mcpgo.ParseString(req, "selector", "")

	level := engine.ExtractLevel(levelStr)
	if err := engine.ValidateExtractLevel(level); err != nil {
		return errResult(err)
	}

	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		// With URL: full Preview (navigate + errors + network + extract).
		if url != "" {
			pv, err := engine.Preview(page, url, wait, level, func(p *rod.Page) error {
				if s.opts.DismissCookies {
					if engine.DismissCookieBanner(p) {
						_ = engine.WaitForPage(p, "stable")
					}
				}
				return nil
			}, s.opts.Stealth)
			if err != nil {
				return errResult(fmt.Errorf("snapshot: %w", err))
			}
			if pv.DOM != nil {
				s.rememberSnapshot(page, pv.DOM)
			}
			return previewResult(pv)
		}

		// Without URL: snapshot the current page. Errors/network come from
		// the long-lived observer (started in ensurePageLocked).
		info, err := page.Info()
		if err != nil {
			return errResult(fmt.Errorf("page info: %w", err))
		}
		extracted, err := engine.Extract(page, level, selector, false)
		if err != nil {
			return errResult(fmt.Errorf("extract: %w", err))
		}
		if selector == "" {
			s.rememberSnapshot(page, extracted)
		}

		// Build a PreviewResult equivalent from observer + extract so the
		// output shape stays consistent with the URL-provided path.
		errs := s.observerErrors()
		net := s.observerNetwork()
		pv := &engine.PreviewResult{
			PageInfo: &engine.PageInfo{URL: info.URL, Title: info.Title, Status: 200},
			Errors:   errs,
			Network:  net,
			DOM:      extracted,
			Summary: engine.PreviewSummary{
				TotalRequests:    len(net),
				FailedRequests:   countFailed(net),
				ErrorCount:       countByLevel(errs, "error"),
				WarningCount:     countByLevel(errs, "warning"),
				InteractiveCount: countInteractive(extracted),
			},
		}
		return previewResult(pv)
	})
}

func (s *Server) handleNavigate(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	url := mcpgo.ParseString(req, "url", "")
	if url == "" {
		return errResult(fmt.Errorf("url is required"))
	}
	wait := mcpgo.ParseString(req, "wait", "domcontentloaded")

	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		info, err := engine.Navigate(page, url, wait)
		if err != nil {
			recovered, recErr := tryNavigateRecovery(page, err)
			if !recovered {
				if recErr != nil {
					return errResult(fmt.Errorf("navigate failed and recovery failed: %w (orig: %v)", recErr, err))
				}
				return errResult(fmt.Errorf("navigate: %w", err))
			}
			if pi, infoErr := page.Info(); infoErr == nil && pi != nil {
				info = &engine.PageInfo{URL: pi.URL, Title: pi.Title, Status: 200}
			}
		}
		if s.opts.DismissCookies {
			if engine.DismissCookieBanner(page) {
				_ = engine.WaitForPage(page, "stable")
			}
		}
		summary := fmt.Sprintf("[%d] %s — %s (%dms)", info.Status, info.Title, info.URL, info.TimeMs)
		return jsonResult(info, summary)
	})
}

func (s *Server) handleClick(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	ref := normalizeRef(mcpgo.ParseString(req, "ref", ""))
	if ref == "" {
		return errResult(fmt.Errorf("ref is required"))
	}
	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		snap := s.snapshotForResolve(page)
		if err := engine.ClickRef(page, ref, snap); err != nil {
			return errResult(fmt.Errorf("click %s: %w", ref, err))
		}
		return mcpgo.NewToolResultText(fmt.Sprintf("clicked %s", ref)), nil
	})
}

func (s *Server) handleType(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	ref := normalizeRef(mcpgo.ParseString(req, "ref", ""))
	text := mcpgo.ParseString(req, "text", "")
	submit := mcpgo.ParseBoolean(req, "submit", false)
	if ref == "" {
		return errResult(fmt.Errorf("ref is required"))
	}
	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		snap := s.snapshotForResolve(page)
		if err := engine.TypeRef(page, ref, text, snap); err != nil {
			return errResult(fmt.Errorf("type %s: %w", ref, err))
		}
		summary := fmt.Sprintf("typed into %s (%d chars)", ref, len(text))
		if submit {
			if err := engine.PressKey(page, "Enter", ref, snap); err != nil {
				return errResult(fmt.Errorf("submit %s: %w", ref, err))
			}
			summary += " + Enter"
		}
		return mcpgo.NewToolResultText(summary), nil
	})
}

func (s *Server) handleSelect(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	ref := normalizeRef(mcpgo.ParseString(req, "ref", ""))
	value := mcpgo.ParseString(req, "value", "")
	if ref == "" || value == "" {
		return errResult(fmt.Errorf("ref and value are required"))
	}
	// Allow JSON array string for multi-select.
	values := []string{value}
	if strings.HasPrefix(value, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(value), &arr); err == nil && len(arr) > 0 {
			values = arr
		}
	}
	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		snap := s.snapshotForResolve(page)
		if err := engine.SelectOption(page, ref, values, snap); err != nil {
			return errResult(fmt.Errorf("select %s: %w", ref, err))
		}
		return mcpgo.NewToolResultText(fmt.Sprintf("selected %v in %s", values, ref)), nil
	})
}

func (s *Server) handlePress(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	key := mcpgo.ParseString(req, "key", "")
	if key == "" {
		return errResult(fmt.Errorf("key is required"))
	}
	ref := normalizeRef(mcpgo.ParseString(req, "ref", ""))
	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		snap := s.snapshotForResolve(page)
		if err := engine.PressKey(page, key, ref, snap); err != nil {
			return errResult(fmt.Errorf("press %s: %w", key, err))
		}
		if ref != "" {
			return mcpgo.NewToolResultText(fmt.Sprintf("pressed %s on %s", key, ref)), nil
		}
		return mcpgo.NewToolResultText(fmt.Sprintf("pressed %s", key)), nil
	})
}

func (s *Server) handleWaitFor(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	selector := mcpgo.ParseString(req, "selector", "")
	text := mcpgo.ParseString(req, "text", "")
	timeoutMs := int(mcpgo.ParseFloat64(req, "timeout_ms", 5000))
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	if timeoutMs > 30000 {
		timeoutMs = 30000
	}
	if selector == "" && text == "" {
		// Plain timeout-only wait. Useful for "give the SPA a beat to settle".
		select {
		case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		case <-ctx.Done():
			return errResult(ctx.Err())
		}
		return mcpgo.NewToolResultText(fmt.Sprintf("waited %dms (no condition)", timeoutMs)), nil
	}
	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		start := time.Now()
		if selector != "" {
			if err := engine.WaitForSelector(page, selector, (timeoutMs+999)/1000); err != nil {
				return errResult(fmt.Errorf("wait_for selector %q: %w", selector, err))
			}
		}
		if text != "" {
			// Poll page text every 100ms.
			deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
			for time.Now().Before(deadline) {
				body, err := page.Element("body")
				if err == nil && body != nil {
					if pageText, err := body.Text(); err == nil && strings.Contains(pageText, text) {
						return mcpgo.NewToolResultText(fmt.Sprintf("waited %dms for text %q", time.Since(start).Milliseconds(), text)), nil
					}
				}
				select {
				case <-time.After(100 * time.Millisecond):
				case <-ctx.Done():
					return errResult(ctx.Err())
				}
			}
			return errResult(fmt.Errorf("wait_for text %q: timeout after %dms", text, timeoutMs))
		}
		return mcpgo.NewToolResultText(fmt.Sprintf("waited %dms for selector %q", time.Since(start).Milliseconds(), selector)), nil
	})
}

func (s *Server) handleEval(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if err := s.opts.Policy.AllowAction("eval"); err != nil {
		return errResult(err)
	}
	expr := mcpgo.ParseString(req, "expression", "")
	if expr == "" {
		return errResult(fmt.Errorf("expression is required"))
	}
	ref := normalizeRef(mcpgo.ParseString(req, "ref", ""))
	timeoutMs := int(mcpgo.ParseFloat64(req, "timeout_ms", 8000))
	if timeoutMs <= 0 {
		timeoutMs = 8000
	}
	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		snap := s.snapshotForResolve(page)
		value, err := engine.EvalJSTimeout(page, expr, ref, snap, time.Duration(timeoutMs)*time.Millisecond)
		if err != nil {
			return errResult(fmt.Errorf("eval: %w", err))
		}
		return mcpgo.NewToolResultText(value), nil
	})
}

func (s *Server) handleScreenshot(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	ref := normalizeRef(mcpgo.ParseString(req, "ref", ""))
	fullPage := mcpgo.ParseBoolean(req, "full_page", false)
	format := mcpgo.ParseString(req, "format", "webp")
	quality := int(mcpgo.ParseFloat64(req, "quality", 60))
	annotate := mcpgo.ParseBoolean(req, "annotate", false)
	if quality < 1 || quality > 100 {
		quality = 60
	}
	if annotate {
		format = "png"
		quality = 0
	}
	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		snap := s.snapshotForResolve(page)
		data, err := engine.TakeScreenshotFormat(page, fullPage, ref, format, quality, snap)
		if err != nil {
			return errResult(fmt.Errorf("screenshot: %w", err))
		}
		if annotate && snap != nil {
			data, err = engine.AnnotateScreenshot(page, snap, data)
			if err != nil {
				return errResult(fmt.Errorf("annotate: %w", err))
			}
		}
		mime := "image/" + format
		summary := fmt.Sprintf("captured %s (%d bytes)", format, len(data))
		if annotate {
			summary += " (annotated)"
		}
		return mcpgo.NewToolResultImage(summary, base64.StdEncoding.EncodeToString(data), mime), nil
	})
}

func (s *Server) handleHover(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	ref := normalizeRef(mcpgo.ParseString(req, "ref", ""))
	if ref == "" {
		return errResult(fmt.Errorf("ref is required"))
	}
	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		snap := s.snapshotForResolve(page)
		if err := engine.HoverRef(page, ref, snap); err != nil {
			return errResult(fmt.Errorf("hover %s: %w", ref, err))
		}
		return mcpgo.NewToolResultText(fmt.Sprintf("hovered %s", ref)), nil
	})
}

func (s *Server) handleDrag(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	from := normalizeRef(mcpgo.ParseString(req, "from", ""))
	to := normalizeRef(mcpgo.ParseString(req, "to", ""))
	steps := int(mcpgo.ParseFloat64(req, "steps", 10))
	if from == "" || to == "" {
		return errResult(fmt.Errorf("from and to refs are required"))
	}
	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		snap := s.snapshotForResolve(page)
		if err := engine.DragDrop(page, from, to, snap, steps); err != nil {
			return errResult(fmt.Errorf("drag %s→%s: %w", from, to, err))
		}
		return mcpgo.NewToolResultText(fmt.Sprintf("dragged %s → %s", from, to)), nil
	})
}

func (s *Server) handleFillForm(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	fieldsRaw := mcpgo.ParseString(req, "fields", "")
	if fieldsRaw == "" {
		return errResult(fmt.Errorf("fields is required"))
	}
	var fields map[string]string
	if err := json.Unmarshal([]byte(fieldsRaw), &fields); err != nil {
		return errResult(fmt.Errorf("fields must be a JSON object: %w", err))
	}
	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		snap := s.snapshotForResolve(page)
		filled := 0
		for ref, value := range fields {
			ref = normalizeRef(ref)
			if err := engine.TypeRef(page, ref, value, snap); err != nil {
				return errResult(fmt.Errorf("fill %s: %w", ref, err))
			}
			filled++
		}
		return mcpgo.NewToolResultText(fmt.Sprintf("filled %d fields", filled)), nil
	})
}

func (s *Server) handleUpload(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if err := s.opts.Policy.AllowAction("upload"); err != nil {
		return errResult(err)
	}
	ref := normalizeRef(mcpgo.ParseString(req, "ref", ""))
	pathsRaw := mcpgo.ParseString(req, "paths", "")
	if ref == "" || pathsRaw == "" {
		return errResult(fmt.Errorf("ref and paths are required"))
	}
	var paths []string
	if strings.HasPrefix(pathsRaw, "[") {
		if err := json.Unmarshal([]byte(pathsRaw), &paths); err != nil {
			paths = []string{pathsRaw}
		}
	} else {
		paths = []string{pathsRaw}
	}
	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		snap := s.snapshotForResolve(page)
		el, err := engine.ResolveRef(page, ref, snap)
		if err != nil {
			return errResult(fmt.Errorf("upload %s: %w", ref, err))
		}
		if err := el.SetFiles(paths); err != nil {
			return errResult(fmt.Errorf("upload: %w", err))
		}
		return mcpgo.NewToolResultText(fmt.Sprintf("uploaded %d file(s) to %s", len(paths), ref)), nil
	})
}

func (s *Server) handleTabs(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	action := mcpgo.ParseString(req, "action", "list")
	index := int(mcpgo.ParseFloat64(req, "index", -1))

	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		currentID := string(page.TargetID)
		switch action {
		case "switch":
			if index < 0 {
				return errResult(fmt.Errorf("index is required for switch"))
			}
			newPage, err := engine.SwitchTab(b.RodBrowser(), index)
			if err != nil {
				return errResult(fmt.Errorf("switch tab: %w", err))
			}
			info, _ := newPage.Info()
			url := ""
			if info != nil {
				url = info.URL
			}
			return mcpgo.NewToolResultText(fmt.Sprintf("switched to tab %d: %s", index, url)), nil
		case "close":
			if index < 0 {
				return errResult(fmt.Errorf("index is required for close"))
			}
			_, err := engine.CloseTab(b.RodBrowser(), index)
			if err != nil {
				return errResult(fmt.Errorf("close tab: %w", err))
			}
			return mcpgo.NewToolResultText(fmt.Sprintf("closed tab %d", index)), nil
		default:
			tabs, err := engine.ListTabs(b.RodBrowser(), currentID)
			if err != nil {
				return errResult(fmt.Errorf("list tabs: %w", err))
			}
			return jsonResult(tabs, fmt.Sprintf("%d tabs open", len(tabs)))
		}
	})
}

func (s *Server) handleBack(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		return historyStep(page, "back", (*rod.Page).NavigateBack)
	})
}

func (s *Server) handleForward(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return s.withPage(func(b *engine.Browser, page *rod.Page) (*mcpgo.CallToolResult, error) {
		return historyStep(page, "forward", (*rod.Page).NavigateForward)
	})
}

// historyStep mirrors cmd/back.go: trigger the history step, wait until the
// page is stable, then report the resulting URL. WaitForPage("stable") is
// what the CLI uses — pre-registering the lifecycle listener inside
// engine.Navigate isn't an option here because the navigation kick is one
// method call, not a separate page.Navigate(url).
func historyStep(page *rod.Page, action string, step func(*rod.Page) error) (*mcpgo.CallToolResult, error) {
	if err := step(page); err != nil {
		return errResult(fmt.Errorf("%s: %w", action, err))
	}
	if err := engine.WaitForPage(page, "stable"); err != nil {
		return errResult(fmt.Errorf("%s: wait: %w", action, err))
	}
	info, err := page.Info()
	if err != nil || info == nil {
		return mcpgo.NewToolResultText("navigated " + action), nil
	}
	return mcpgo.NewToolResultText(fmt.Sprintf("navigated %s to %s", action, info.URL)), nil
}

// ============================================================================
// helpers (private)
// ============================================================================

// previewResult shapes a PreviewResult into an MCP text content with a
// one-line human header + structured JSON. Matches the existing CLI output
// style so refs look identical whether the agent uses CLI or MCP.
func previewResult(pv *engine.PreviewResult) (*mcpgo.CallToolResult, error) {
	if pv == nil || pv.PageInfo == nil {
		return mcpgo.NewToolResultError("preview: empty result"), nil
	}
	header := fmt.Sprintf("[%d] %s — %s | %d errors, %d failed reqs, %d interactive",
		pv.PageInfo.Status, pv.PageInfo.Title, pv.PageInfo.URL,
		pv.Summary.ErrorCount, pv.Summary.FailedRequests, pv.Summary.InteractiveCount)
	data, err := json.Marshal(pv)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("marshal: %v", err)), nil
	}
	return mcpgo.NewToolResultText(header + "\n" + string(data)), nil
}

// jsonResult marshals payload as JSON and emits an MCP text result that
// holds both a human summary and the raw JSON.
func jsonResult(payload any, summary string) (*mcpgo.CallToolResult, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("marshal: %v", err)), nil
	}
	if summary == "" {
		return mcpgo.NewToolResultText(string(data)), nil
	}
	return mcpgo.NewToolResultText(summary + "\n" + string(data)), nil
}

// normalizeRef accepts "@3", "3", "ref:3" and returns "@3" (or empty).
func normalizeRef(in string) string {
	r := strings.TrimSpace(in)
	if r == "" {
		return ""
	}
	r = strings.TrimPrefix(r, "ref:")
	r = strings.TrimPrefix(r, "ref-")
	if !strings.HasPrefix(r, "@") {
		r = "@" + r
	}
	return r
}

// observerErrors returns errors accumulated by the long-lived observer.
// Maps ObserverEvent → ErrorEntry so handleSnapshot can build a result
// that matches engine.PreviewResult's shape exactly.
func (s *Server) observerErrors() []engine.ErrorEntry {
	if s.observer == nil {
		return nil
	}
	events := s.observer.Drain(0)
	out := make([]engine.ErrorEntry, 0, len(events))
	for _, e := range events {
		switch e.Kind {
		case engine.KindError, engine.KindConsole:
			if e.Level == "error" || e.Level == "warning" {
				out = append(out, engine.ErrorEntry{
					Type:    "console",
					Level:   e.Level,
					Message: e.Text,
					Source:  e.Source,
					TimeMs:  e.TS,
				})
			}
		}
	}
	return out
}

// observerNetwork returns network entries from the long-lived observer.
func (s *Server) observerNetwork() []engine.NetworkEntry {
	if s.observer == nil {
		return nil
	}
	events := s.observer.Drain(0)
	out := make([]engine.NetworkEntry, 0)
	for _, e := range events {
		if e.Kind != engine.KindNet {
			continue
		}
		out = append(out, engine.NetworkEntry{
			Method:   e.Method,
			URL:      e.URL,
			Status:   e.Status,
			Size:     int(e.Size),
			TimeMs:   e.DurationMs,
			MimeType: e.MimeType,
			Error:    e.Failed,
		})
	}
	return out
}

func countFailed(net []engine.NetworkEntry) int {
	n := 0
	for _, e := range net {
		if e.Status >= 400 || e.Error != "" {
			n++
		}
	}
	return n
}

func countByLevel(errs []engine.ErrorEntry, level string) int {
	n := 0
	for _, e := range errs {
		if e.Level == level {
			n++
		}
	}
	return n
}

func countInteractive(result *engine.ExtractionResult) int {
	if result == nil {
		return 0
	}
	return result.Stats.InteractiveCount
}

// tryNavigateRecovery — minimal version of the old retry hook. If the
// navigate failed with a deadline exceeded mid-anti-bot challenge, give
// the page 5s to settle and probe page.Info() once before giving up.
func tryNavigateRecovery(page *rod.Page, err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	msg := err.Error()
	if !strings.Contains(msg, "deadline") && !strings.Contains(msg, "timeout") {
		return false, nil
	}
	time.Sleep(5 * time.Second)
	if info, err := page.Info(); err == nil && info != nil && info.URL != "" {
		return true, nil
	}
	return false, nil
}
