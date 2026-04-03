package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// interactiveSelector matches the same elements that get @ref assignments in the AX tree.
const interactiveSelector = `a[href], button, input, select, textarea, [role="button"], [role="link"], [role="textbox"], [role="checkbox"], [role="radio"], [role="combobox"], [role="menuitem"], [role="tab"], [contenteditable="true"]`

// ResolveRef finds an element by its ref (@1, @2, etc.).
// Refs are assigned in DOM order to interactive elements, so @N means the Nth
// element matching the interactive selector.
func ResolveRef(page *rod.Page, ref string) (*rod.Element, error) {
	ref = strings.TrimPrefix(ref, "@")
	idx, err := strconv.Atoi(ref)
	if err != nil || idx < 1 {
		return nil, fmt.Errorf("invalid ref %q: must be @N where N >= 1", ref)
	}

	elements, err := page.Elements(interactiveSelector)
	if err != nil {
		return nil, fmt.Errorf("query interactive elements: %w", err)
	}

	if idx > len(elements) {
		return nil, fmt.Errorf("ref @%d out of range: only %d interactive elements found", idx, len(elements))
	}

	return elements[idx-1], nil
}

// ClickRef clicks the element at the given ref.
func ClickRef(page *rod.Page, ref string) error {
	el, err := ResolveRef(page, ref)
	if err != nil {
		return err
	}

	// Scroll into view and click.
	err = el.ScrollIntoView()
	if err != nil {
		return fmt.Errorf("scroll into view: %w", err)
	}

	err = el.Click(proto.InputMouseButtonLeft, 1)
	if err != nil {
		return fmt.Errorf("click: %w", err)
	}

	// Wait for page to stabilize after click.
	_ = page.WaitStable(500 * time.Millisecond)

	return nil
}

// TypeRef types text into the element at the given ref.
// Uses focus + select all + keyboard typing to work with React/Vue/Angular.
func TypeRef(page *rod.Page, ref string, text string) error {
	el, err := ResolveRef(page, ref)
	if err != nil {
		return err
	}

	// Focus the element first
	err = el.Focus()
	if err != nil {
		return fmt.Errorf("focus: %w", err)
	}

	// Triple-click to select all existing text
	err = el.Click(proto.InputMouseButtonLeft, 3)
	if err != nil {
		// Ignore — field may be empty
		_ = err
	}

	// Small delay for React to process the click
	time.Sleep(50 * time.Millisecond)

	// InsertText dispatches InputEvent which React/Vue/Angular listen to
	err = el.SelectAllText()
	if err != nil {
		_ = err
	}

	err = page.InsertText(text)
	if err != nil {
		return fmt.Errorf("type text: %w", err)
	}

	// Trigger blur to finalize (some forms validate on blur)
	_ = el.Blur()

	return nil
}

// TakeScreenshot captures the page or a specific element.
// If elementRef is non-empty, captures only that element.
// If fullPage is true, captures the full scrollable page.
// quality controls JPEG quality (1-100); PNG is used if quality <= 0.
func TakeScreenshot(page *rod.Page, fullPage bool, elementRef string, quality int) ([]byte, error) {
	if elementRef != "" {
		el, err := ResolveRef(page, elementRef)
		if err != nil {
			return nil, err
		}
		return el.Screenshot(proto.PageCaptureScreenshotFormatPng, 0)
	}

	if fullPage {
		req := &proto.PageCaptureScreenshot{}
		if quality > 0 {
			req.Format = proto.PageCaptureScreenshotFormatJpeg
			req.Quality = intPtr(quality)
		} else {
			req.Format = proto.PageCaptureScreenshotFormatPng
		}
		// Use full page metrics.
		metrics, err := proto.PageGetLayoutMetrics{}.Call(page)
		if err != nil {
			return nil, fmt.Errorf("get layout metrics: %w", err)
		}
		oldClip := req.Clip
		_ = oldClip
		req.Clip = &proto.PageViewport{
			X:      0,
			Y:      0,
			Width:  metrics.ContentSize.Width,
			Height: metrics.ContentSize.Height,
			Scale:  1,
		}
		req.CaptureBeyondViewport = true
		data, err := req.Call(page)
		if err != nil {
			return nil, fmt.Errorf("full page screenshot: %w", err)
		}
		return data.Data, nil
	}

	// Viewport screenshot.
	req := &proto.PageCaptureScreenshot{}
	if quality > 0 {
		req.Format = proto.PageCaptureScreenshotFormatJpeg
		req.Quality = intPtr(quality)
	} else {
		req.Format = proto.PageCaptureScreenshotFormatPng
	}
	data, err := req.Call(page)
	if err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}
	return data.Data, nil
}

// EvalJS evaluates JavaScript on the page or in an element context.
// If elementRef is non-empty, the JS runs with `this` bound to that element.
func EvalJS(page *rod.Page, expr string, elementRef string) (string, error) {
	if elementRef != "" {
		el, err := ResolveRef(page, elementRef)
		if err != nil {
			return "", err
		}
		wrapped := fmt.Sprintf("function(){ return %s }", expr)
		res, err := el.Eval(wrapped)
		if err != nil {
			return "", fmt.Errorf("eval on element: %w", err)
		}
		return formatEvalResult(res), nil
	}

	// Wrap as async arrow function to support await
	wrapped := fmt.Sprintf("async () => { return %s }", expr)
	res, err := page.Eval(wrapped)
	if err != nil {
		return "", fmt.Errorf("eval: %w", err)
	}
	return formatEvalResult(res), nil
}

// formatEvalResult converts a rod eval result to a string.
func formatEvalResult(res *proto.RuntimeRemoteObject) string {
	if res == nil {
		return "undefined"
	}
	val := res.Value.Val()
	if val == nil {
		return "undefined"
	}
	return fmt.Sprintf("%v", val)
}

// intPtr converts an int to *int for proto fields.
func intPtr(v int) *int {
	return &v
}

// ---------------------------------------------------------------------------
// Additional interaction functions
// ---------------------------------------------------------------------------

// SelectOption selects option(s) in a <select> element by visible text.
func SelectOption(page *rod.Page, ref string, values []string) error {
	el, err := ResolveRef(page, ref)
	if err != nil {
		return err
	}
	err = el.Select(values, true, rod.SelectorTypeText)
	if err != nil {
		return fmt.Errorf("select: %w", err)
	}
	_ = page.WaitStable(300 * time.Millisecond)
	return nil
}

// HoverRef hovers over the element at the given ref.
func HoverRef(page *rod.Page, ref string) error {
	el, err := ResolveRef(page, ref)
	if err != nil {
		return err
	}
	err = el.ScrollIntoView()
	if err != nil {
		return fmt.Errorf("scroll: %w", err)
	}
	err = el.Hover()
	if err != nil {
		return fmt.Errorf("hover: %w", err)
	}
	_ = page.WaitStable(300 * time.Millisecond)
	return nil
}

// keyMap maps human-friendly key names to Rod input keys.
var keyMap = map[string]input.Key{
	"enter": input.Enter, "tab": input.Tab, "escape": input.Escape,
	"backspace": input.Backspace, "delete": input.Delete,
	"arrowup": input.ArrowUp, "arrowdown": input.ArrowDown,
	"arrowleft": input.ArrowLeft, "arrowright": input.ArrowRight,
	"space": input.Space, "home": input.Home, "end": input.End,
	"pageup": input.PageUp, "pagedown": input.PageDown,
	"f1": input.F1, "f2": input.F2, "f3": input.F3, "f4": input.F4,
	"f5": input.F5, "f6": input.F6, "f7": input.F7, "f8": input.F8,
	"f9": input.F9, "f10": input.F10, "f11": input.F11, "f12": input.F12,
}

// PressKey sends a keyboard key press. If ref is non-empty, focuses the element first.
func PressKey(page *rod.Page, key string, ref string) error {
	if ref != "" {
		el, err := ResolveRef(page, ref)
		if err != nil {
			return err
		}
		err = el.Focus()
		if err != nil {
			return fmt.Errorf("focus: %w", err)
		}
	}
	k, ok := keyMap[strings.ToLower(key)]
	if !ok {
		return fmt.Errorf("unknown key %q — supported: enter, tab, escape, backspace, delete, arrowup, arrowdown, arrowleft, arrowright, space, home, end, pageup, pagedown", key)
	}
	err := page.Keyboard.Type(k)
	if err != nil {
		return fmt.Errorf("press: %w", err)
	}
	_ = page.WaitStable(300 * time.Millisecond)
	return nil
}

// WaitForSelector waits for a CSS selector to appear in the DOM.
func WaitForSelector(page *rod.Page, selector string, timeoutSec int) error {
	p := page.Timeout(time.Duration(timeoutSec) * time.Second)
	_, err := p.Element(selector)
	if err != nil {
		return fmt.Errorf("timeout waiting for %q after %ds", selector, timeoutSec)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Tab management
// ---------------------------------------------------------------------------

// TabInfo holds metadata about a browser tab.
type TabInfo struct {
	Index int    `json:"index"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

// ListTabs returns info for every open tab in the browser.
func ListTabs(browser *rod.Browser) ([]TabInfo, error) {
	pages, err := browser.Pages()
	if err != nil {
		return nil, err
	}
	var tabs []TabInfo
	for i, p := range pages {
		info, _ := p.Info()
		title := ""
		url := ""
		if info != nil {
			title = info.Title
			url = info.URL
		}
		tabs = append(tabs, TabInfo{Index: i, URL: url, Title: title})
	}
	return tabs, nil
}

// SwitchTab activates the tab at the given index and returns its page.
func SwitchTab(browser *rod.Browser, index int) (*rod.Page, error) {
	pages, err := browser.Pages()
	if err != nil {
		return nil, err
	}
	if index < 0 || index >= len(pages) {
		return nil, fmt.Errorf("tab index %d out of range (0-%d)", index, len(pages)-1)
	}
	_, err = pages[index].Activate()
	if err != nil {
		return nil, err
	}
	return pages[index], nil
}

// CloseTab closes the tab at the given index.
func CloseTab(browser *rod.Browser, index int) error {
	pages, err := browser.Pages()
	if err != nil {
		return err
	}
	if index < 0 || index >= len(pages) {
		return fmt.Errorf("tab index %d out of range (0-%d)", index, len(pages)-1)
	}
	return pages[index].Close()
}

// ---------------------------------------------------------------------------
// Viewport & dialogs
// ---------------------------------------------------------------------------

// SetViewport overrides the page viewport dimensions.
func SetViewport(page *rod.Page, width, height int) error {
	return proto.EmulationSetDeviceMetricsOverride{
		Width:            width,
		Height:           height,
		DeviceScaleFactor: 1,
		Mobile:           width < 768,
	}.Call(page)
}

// HandleNextDialog sets up a one-shot handler for the next JavaScript dialog
// (alert, confirm, prompt). It returns immediately; the handler fires when
// the dialog appears.
func HandleNextDialog(page *rod.Page, accept bool, promptText string) {
	wait, handle := page.HandleDialog()
	go func() {
		wait()
		_ = handle(&proto.PageHandleJavaScriptDialog{
			Accept:     accept,
			PromptText: promptText,
		})
	}()
}
