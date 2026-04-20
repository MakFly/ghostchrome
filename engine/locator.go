package engine

import (
	"fmt"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Locator describes a semantic element match. At least one field must be set.
// When multiple fields are set, the match is conjunctive (all must hold).
type Locator struct {
	// Role matches the ARIA role (see engine.interactiveRoles, engine.skeletonRoles).
	// Accepts canonical role strings or their one-letter agent abbreviation
	// ("b"=button, "a"=link, "t"=textbox, etc.).
	Role string

	// Name matches the accessible name. Comparison is case-insensitive and uses
	// substring matching, so "Sign in" matches "Sign in now".
	Name string

	// Label matches the accessible label derived from <label for=...> or
	// aria-labelledby. In Chromium's a11y tree, that's exposed as the name of
	// a textbox / combobox — so this is equivalent to Name for inputs.
	// Included as a separate field for ergonomic CLI flags (--by-label).
	Label string

	// Text matches via page-wide text search — like Playwright's getByText.
	// When set, Role is ignored.
	Text string
}

// IsEmpty reports whether the locator has no criteria set.
func (l Locator) IsEmpty() bool {
	return l.Role == "" && l.Name == "" && l.Label == "" && l.Text == ""
}

// ResolveByLocator returns the first element matching the locator. Matching
// strategy:
//   - If Text is set: XPath text contains (case-insensitive). Matches <button>,
//     <a>, <label>, generic text containers.
//   - Else: extract the a11y tree at skeleton level, filter by
//     (role, name|label) and return the first hit.
//
// Snapshot is updated as a side-effect only for the Text branch when a new
// Extract is triggered.
func ResolveByLocator(page *rod.Page, loc Locator) (*rod.Element, error) {
	if loc.IsEmpty() {
		return nil, fmt.Errorf("locator: at least one of --by-role, --by-text, --by-name, --by-label required")
	}

	if loc.Text != "" {
		return resolveByText(page, loc.Text)
	}

	role := normaliseRole(loc.Role)
	name := pickName(loc)

	result, err := proto.AccessibilityGetFullAXTree{}.Call(page)
	if err != nil {
		return nil, fmt.Errorf("a11y tree: %w", err)
	}

	for _, n := range result.Nodes {
		if !roleMatches(n, role) {
			continue
		}
		if !nameMatches(n, name) {
			continue
		}
		if n.BackendDOMNodeID == 0 {
			continue
		}
		el, err := page.ElementFromNode(&proto.DOMNode{BackendNodeID: n.BackendDOMNodeID})
		if err != nil {
			continue
		}
		return el, nil
	}

	return nil, fmt.Errorf("locator: no element matches role=%q name=%q", role, name)
}

func pickName(loc Locator) string {
	if loc.Name != "" {
		return loc.Name
	}
	return loc.Label
}

func roleMatches(n *proto.AccessibilityAXNode, want string) bool {
	if want == "" {
		return true
	}
	got := strings.ToLower(axValueStr(n.Role))
	want = strings.ToLower(want)
	return got == want
}

func nameMatches(n *proto.AccessibilityAXNode, want string) bool {
	if want == "" {
		return true
	}
	got := strings.ToLower(axValueStr(n.Name))
	return strings.Contains(got, strings.ToLower(want))
}

// normaliseRole expands one-letter role codes to full role names. Unknown
// inputs are returned as-is so users can pass "dialog", "tooltip", etc.
func normaliseRole(r string) string {
	switch strings.ToLower(strings.TrimSpace(r)) {
	case "":
		return ""
	case "b", "button":
		return "button"
	case "a", "link":
		return "link"
	case "t", "textbox", "input":
		return "textbox"
	case "c", "checkbox":
		return "checkbox"
	case "r", "radio":
		return "radio"
	case "s", "combobox", "select":
		return "combobox"
	case "m", "menuitem":
		return "menuitem"
	case "x", "tab":
		return "tab"
	case "h", "heading":
		return "heading"
	case "img", "image":
		return "image"
	}
	return strings.ToLower(r)
}

// resolveByText uses an XPath text() match to locate the first visible node
// whose text content contains the target (case-insensitive, trim-aware).
func resolveByText(page *rod.Page, text string) (*rod.Element, error) {
	lower := strings.ReplaceAll(text, "'", "\\'")
	xpath := fmt.Sprintf(
		`//*[self::button or self::a or self::label or self::span or self::div or self::li or self::p or self::h1 or self::h2 or self::h3 or self::h4 or self::h5 or self::h6][contains(translate(normalize-space(.), 'ABCDEFGHIJKLMNOPQRSTUVWXYZÀÂÄÉÈÊËÏÎÔÙÛÜÇ', 'abcdefghijklmnopqrstuvwxyzàâäéèêëïîôùûüç'), '%s')]`,
		strings.ToLower(lower),
	)
	els, err := page.ElementsX(xpath)
	if err != nil {
		return nil, fmt.Errorf("text search: %w", err)
	}
	for _, el := range els {
		visible, _ := el.Visible()
		if visible {
			return el, nil
		}
	}
	if len(els) > 0 {
		return els[0], nil
	}
	return nil, fmt.Errorf("locator: no element contains text %q", text)
}
