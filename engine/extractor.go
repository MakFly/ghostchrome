package engine

import (
	"fmt"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// ExtractLevel controls how much of the accessibility tree is returned.
type ExtractLevel string

const (
	LevelSkeleton ExtractLevel = "skeleton"
	LevelContent  ExtractLevel = "content"
	LevelFull     ExtractLevel = "full"
)

// ExtractedNode represents a filtered accessibility node.
type ExtractedNode struct {
	Ref           string                 `json:"ref,omitempty"`
	Role          string                 `json:"role"`
	Name          string                 `json:"name,omitempty"`
	Value         string                 `json:"value,omitempty"`
	Level         int                    `json:"level,omitempty"`
	Href          string                 `json:"href,omitempty"`
	Type          string                 `json:"type,omitempty"`
	Checked       *bool                  `json:"checked,omitempty"`
	Disabled      bool                   `json:"disabled,omitempty"`
	BackendNodeID proto.DOMBackendNodeID `json:"-"`
	Children      []ExtractedNode        `json:"children,omitempty"`
}

// ExtractionResult holds the extraction output.
type ExtractionResult struct {
	Nodes []ExtractedNode          `json:"nodes"`
	Refs  map[string]ExtractedNode `json:"refs"`
	Stats ExtractionStats          `json:"stats"`
}

// ExtractionStats provides extraction metrics.
type ExtractionStats struct {
	TotalNodes       int `json:"total_nodes"`
	FilteredNodes    int `json:"filtered_nodes"`
	InteractiveCount int `json:"interactive_count"`
}

// Roles considered interactive — get @ref assignments.
var interactiveRoles = map[string]bool{
	"button":   true,
	"link":     true,
	"textbox":  true,
	"checkbox": true,
	"radio":    true,
	"combobox": true,
	"menuitem": true,
	"tab":      true,
}

// Skeleton-level roles.
var skeletonRoles = map[string]bool{
	"heading":       true,
	"button":        true,
	"link":          true,
	"textbox":       true,
	"checkbox":      true,
	"radio":         true,
	"combobox":      true,
	"menuitem":      true,
	"tab":           true,
	"navigation":    true,
	"form":          true,
	"search":        true,
	"banner":        true,
	"main":          true,
	"complementary": true,
	"contentinfo":   true,
}

// Content-level additions on top of skeleton.
var contentRoles = map[string]bool{
	"StaticText":   true,
	"text":         true,
	"paragraph":    true,
	"listitem":     true,
	"img":          true,
	"image":        true,
	"table":        true,
	"rowheader":    true,
	"columnheader": true,
}

// Extract retrieves the accessibility tree from the page and filters it.
func Extract(page *rod.Page, level ExtractLevel, selector string) (*ExtractionResult, error) {
	if err := ValidateExtractLevel(level); err != nil {
		return nil, err
	}

	// Get the full accessibility tree via CDP.
	result, err := proto.AccessibilityGetFullAXTree{}.Call(page)
	if err != nil {
		return nil, fmt.Errorf("get accessibility tree: %w", err)
	}

	// Build a lookup map by node ID and find the root(s).
	nodeMap := make(map[proto.AccessibilityAXNodeID]*proto.AccessibilityAXNode, len(result.Nodes))
	for _, n := range result.Nodes {
		nodeMap[n.NodeID] = n
	}

	// If selector is provided, scope to that subtree via DOM + AX query.
	var scopeNodeIDs map[proto.AccessibilityAXNodeID]bool
	if selector != "" {
		scopeNodeIDs, err = resolveScope(page, selector)
		if err != nil {
			return nil, fmt.Errorf("resolve selector %q: %w", selector, err)
		}
	}

	// Build tree from flat nodes.
	refCounter := 0
	stats := ExtractionStats{TotalNodes: len(result.Nodes)}
	refs := make(map[string]ExtractedNode)

	// Find root nodes (no parent or parent not in map).
	var rootIDs []proto.AccessibilityAXNodeID
	for _, n := range result.Nodes {
		if n.ParentID == "" {
			rootIDs = append(rootIDs, n.NodeID)
		}
	}

	var extractedNodes []ExtractedNode
	for _, rootID := range rootIDs {
		children := buildTree(nodeMap, rootID, level, scopeNodeIDs, &refCounter, refs, &stats)
		extractedNodes = append(extractedNodes, children...)
	}

	return &ExtractionResult{
		Nodes: extractedNodes,
		Refs:  refs,
		Stats: stats,
	}, nil
}

// resolveScope finds all AX node IDs that are descendants of the given CSS selector.
func resolveScope(page *rod.Page, selector string) (map[proto.AccessibilityAXNodeID]bool, error) {
	// Get the DOM node for the selector, then query AX tree for its subtree.
	el, err := page.Element(selector)
	if err != nil {
		return nil, fmt.Errorf("element %q not found: %w", selector, err)
	}

	// Get the backend node ID.
	desc, err := el.Describe(0, false)
	if err != nil {
		return nil, fmt.Errorf("describe element: %w", err)
	}

	// Use AccessibilityGetPartialAXTree to get the subtree.
	partial, err := proto.AccessibilityGetPartialAXTree{
		BackendNodeID:  desc.BackendNodeID,
		FetchRelatives: true,
	}.Call(page)
	if err != nil {
		return nil, fmt.Errorf("get partial AX tree: %w", err)
	}

	ids := make(map[proto.AccessibilityAXNodeID]bool, len(partial.Nodes))
	for _, n := range partial.Nodes {
		ids[n.NodeID] = true
	}
	return ids, nil
}

// buildTree recursively builds ExtractedNode tree from the flat AX node map.
func buildTree(
	nodeMap map[proto.AccessibilityAXNodeID]*proto.AccessibilityAXNode,
	nodeID proto.AccessibilityAXNodeID,
	level ExtractLevel,
	scope map[proto.AccessibilityAXNodeID]bool,
	refCounter *int,
	refs map[string]ExtractedNode,
	stats *ExtractionStats,
) []ExtractedNode {
	axNode, ok := nodeMap[nodeID]
	if !ok {
		return nil
	}

	// If scoped, skip nodes not in scope.
	if scope != nil && !scope[nodeID] {
		return nil
	}

	// Skip ignored nodes but still recurse into children.
	if axNode.Ignored {
		var result []ExtractedNode
		for _, childID := range axNode.ChildIDs {
			result = append(result, buildTree(nodeMap, childID, level, scope, refCounter, refs, stats)...)
		}
		return result
	}

	role := axValueStr(axNode.Role)
	name := axValueStr(axNode.Name)

	// Apply level filter.
	if !shouldInclude(role, name, level) {
		// Still recurse — children might be relevant.
		var result []ExtractedNode
		for _, childID := range axNode.ChildIDs {
			result = append(result, buildTree(nodeMap, childID, level, scope, refCounter, refs, stats)...)
		}
		return result
	}

	node := ExtractedNode{
		Role:          role,
		Name:          name,
		BackendNodeID: axNode.BackendDOMNodeID,
	}

	// Extract value.
	if axNode.Value != nil {
		node.Value = axValueStr(axNode.Value)
	}

	// Extract properties.
	for _, prop := range axNode.Properties {
		switch prop.Name {
		case proto.AccessibilityAXPropertyNameLevel:
			if prop.Value != nil {
				if v, ok := prop.Value.Value.Val().(float64); ok {
					node.Level = int(v)
				}
			}
		case proto.AccessibilityAXPropertyNameChecked:
			if prop.Value != nil {
				val := axValueStr(prop.Value) == "true"
				node.Checked = &val
			}
		case proto.AccessibilityAXPropertyNameDisabled:
			if prop.Value != nil {
				node.Disabled = axValueStr(prop.Value) == "true"
			}
		case proto.AccessibilityAXPropertyNameURL:
			if prop.Value != nil {
				node.Href = axValueStr(prop.Value)
			}
		}
	}

	// Assign ref to interactive elements.
	if interactiveRoles[role] && axNode.BackendDOMNodeID != 0 {
		*refCounter++
		node.Ref = fmt.Sprintf("@%d", *refCounter)
		stats.InteractiveCount++
	}

	// Recurse children.
	for _, childID := range axNode.ChildIDs {
		node.Children = append(node.Children, buildTree(nodeMap, childID, level, scope, refCounter, refs, stats)...)
	}

	stats.FilteredNodes++

	// Store in refs map if interactive.
	if node.Ref != "" {
		refs[node.Ref] = node
	}

	return []ExtractedNode{node}
}

// shouldInclude checks if a node should be included based on the extraction level.
func shouldInclude(role, name string, level ExtractLevel) bool {
	// Skip generic/none roles unless they have meaningful content.
	if role == "" || role == "none" || role == "generic" {
		return false
	}

	switch level {
	case LevelSkeleton:
		return skeletonRoles[role]
	case LevelContent:
		return skeletonRoles[role] || contentRoles[role]
	case LevelFull:
		// Include everything with a non-empty name or a known role.
		return name != "" || skeletonRoles[role] || contentRoles[role]
	}
	return false
}

// ValidateExtractLevel ensures the extraction level is supported.
func ValidateExtractLevel(level ExtractLevel) error {
	switch level {
	case LevelSkeleton, LevelContent, LevelFull:
		return nil
	default:
		return fmt.Errorf("invalid level %q: use skeleton, content, or full", level)
	}
}

// axValueStr extracts the string representation from an AXValue.
func axValueStr(v *proto.AccessibilityAXValue) string {
	if v == nil {
		return ""
	}
	raw := v.Value.Val()
	if raw == nil {
		return ""
	}
	switch val := raw.(type) {
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

// FormatText renders the extraction result as compact text (human profile).
func FormatText(result *ExtractionResult) string {
	return FormatTextProfile(result, ProfileHuman("text"))
}

// FormatTextProfile renders the extraction result using the given profile.
// The agent profile uses one-letter role tags, truncates long labels and
// shortens hrefs (see TruncateURL).
func FormatTextProfile(result *ExtractionResult, p RenderProfile) string {
	var buf strings.Builder
	state := renderState{lastKey: "", repeat: 0}
	for _, node := range result.Nodes {
		formatNode(&buf, node, 0, p, &state)
	}
	return strings.TrimRight(buf.String(), "\n")
}

// renderState tracks cross-node state used by the agent renderer to dedupe
// adjacent siblings with identical role + label (e.g. 30 "Read more" links).
// The key combines role and label so that a heading and its inner StaticText
// with the same text (very common) are NOT merged.
type renderState struct {
	lastKey string
	repeat  int
}

// formatNode writes a single node and its children to the buffer.
func formatNode(buf *strings.Builder, node ExtractedNode, depth int, p RenderProfile, state *renderState) {
	indent := strings.Repeat("  ", depth)
	if p.Agent {
		// Single-space indent in agent mode — still conveys hierarchy but saves
		// bytes on deep trees.
		indent = strings.Repeat(" ", depth)
	}

	if p.Agent {
		writeNodeAgent(buf, indent, node, p, state)
	} else {
		writeNodeHuman(buf, indent, node)
	}

	for _, child := range node.Children {
		formatNode(buf, child, depth+1, p, state)
	}
}

// writeNodeHuman keeps the legacy "[tag @ref attr=val] label" format.
func writeNodeHuman(buf *strings.Builder, indent string, node ExtractedNode) {
	tag := roleToTag(node.Role)
	parts := []string{tag}

	if node.Ref != "" {
		parts = append(parts, node.Ref)
	}
	if node.Href != "" {
		parts = append(parts, "href="+node.Href)
	}
	if node.Type != "" {
		parts = append(parts, "type="+node.Type)
	}
	if node.Checked != nil {
		if *node.Checked {
			parts = append(parts, "checked")
		} else {
			parts = append(parts, "unchecked")
		}
	}
	if node.Disabled {
		parts = append(parts, "disabled")
	}

	if node.Level > 0 && node.Role == "heading" {
		parts[0] = fmt.Sprintf("h%d", node.Level)
	}

	label := strings.Join(parts, " ")
	switch {
	case node.Name != "":
		fmt.Fprintf(buf, "%s[%s] %s\n", indent, label, node.Name)
	case node.Value != "":
		fmt.Fprintf(buf, "%s[%s] %s\n", indent, label, node.Value)
	default:
		fmt.Fprintf(buf, "%s[%s]\n", indent, label)
	}
}

// writeNodeAgent emits a compact representation:
//
//	@3 b Save                         (interactive, no href)
//	@1 a>iana.org/x Learn more        (link with truncated href)
//	h1 Example Domain                 (non-interactive, heading with level)
//	p Lorem ipsum…                    (non-interactive, label truncated)
//
// It also strips common navigation prefixes ("Aller à", "Go to", "Read more
// about") from the label and replaces runs of identical adjacent labels with a
// single `(xN)` marker on the last occurrence.
func writeNodeAgent(buf *strings.Builder, indent string, node ExtractedNode, p RenderProfile, state *renderState) {
	tag := agentTag(node)

	var head strings.Builder
	head.WriteString(indent)
	if node.Ref != "" {
		head.WriteString(node.Ref)
		head.WriteString(" ")
	}
	head.WriteString(tag)

	if node.Href != "" {
		head.WriteString(">")
		head.WriteString(TruncateURL(node.Href, 40))
	}
	if node.Type != "" {
		head.WriteString(" t=")
		head.WriteString(node.Type)
	}
	if node.Checked != nil {
		if *node.Checked {
			head.WriteString(" ✓")
		} else {
			head.WriteString(" ✗")
		}
	}
	if node.Disabled {
		head.WriteString(" off")
	}

	label := node.Name
	if label == "" {
		label = node.Value
	}
	label = stripNavNoise(label)
	// Drop redundant label when it duplicates the href.
	if label != "" && node.Href != "" && label == node.Href {
		label = ""
	}
	label = truncateLabel(label, p.MaxLabelLen)

	// Dedupe adjacent identical role+label (common in listings).
	dedupeKey := ""
	if label != "" {
		dedupeKey = tag + "|" + label
	}
	if dedupeKey != "" && dedupeKey == state.lastKey {
		state.repeat++
		return
	}
	if state.repeat > 0 {
		trimTrailingNewline(buf)
		fmt.Fprintf(buf, " (×%d)\n", state.repeat+1)
		state.repeat = 0
	}
	state.lastKey = dedupeKey

	if label != "" {
		fmt.Fprintf(buf, "%s %s\n", head.String(), label)
	} else {
		fmt.Fprintf(buf, "%s\n", head.String())
	}
}

// trimTrailingNewline removes the final '\n' from buf if present.
func trimTrailingNewline(buf *strings.Builder) {
	s := buf.String()
	if strings.HasSuffix(s, "\n") {
		buf.Reset()
		buf.WriteString(s[:len(s)-1])
	}
}

// navNoisePrefixes strips common accessibility-verbose prefixes that add no
// signal for LLM agents.
var navNoisePrefixes = []string{
	"Aller à ",
	"Aller au ",
	"Go to ",
	"Lire la suite de ",
	"Lire la suite sur ",
	"Read more about ",
	"Read more on ",
	"Voir ",
	"Link to ",
	"Lien vers ",
}

func stripNavNoise(label string) string {
	for _, prefix := range navNoisePrefixes {
		if strings.HasPrefix(label, prefix) {
			return strings.TrimSpace(label[len(prefix):])
		}
	}
	return label
}

// agentTag returns the 1-2 character role tag used in agent mode.
func agentTag(node ExtractedNode) string {
	if node.Role == "heading" && node.Level > 0 {
		return fmt.Sprintf("h%d", node.Level)
	}
	if abbrev, ok := roleAbbrev[node.Role]; ok {
		return abbrev
	}
	return node.Role
}

// roleAbbrev maps accessibility roles to short tags used in agent mode.
var roleAbbrev = map[string]string{
	"button":       "b",
	"link":         "a",
	"textbox":      "t",
	"checkbox":     "c",
	"radio":        "r",
	"combobox":     "s",
	"menuitem":     "m",
	"tab":          "x",
	"heading":      "h",
	"navigation":   "nav",
	"form":         "form",
	"search":       "search",
	"banner":       "hdr",
	"main":         "main",
	"complementary": "aside",
	"contentinfo":  "ftr",
	"paragraph":    "p",
	"listitem":     "li",
	"img":          "img",
	"image":        "img",
	"StaticText":   "txt",
	"text":         "txt",
	"table":        "tbl",
	"rowheader":    "rh",
	"columnheader": "ch",
}

// truncateLabel shortens a label to maxLen characters, appending an ellipsis
// if truncated. A maxLen of 0 disables truncation.
func truncateLabel(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return s[:maxLen-1] + "…"
}

// roleToTag maps accessibility roles to compact text tags.
func roleToTag(role string) string {
	switch role {
	case "button":
		return "btn"
	case "link":
		return "link"
	case "textbox":
		return "input"
	case "checkbox":
		return "checkbox"
	case "radio":
		return "radio"
	case "combobox":
		return "select"
	case "menuitem":
		return "menuitem"
	case "tab":
		return "tab"
	case "heading":
		return "h"
	case "navigation":
		return "nav"
	case "form":
		return "form"
	case "search":
		return "search"
	case "banner":
		return "banner"
	case "main":
		return "main"
	case "complementary":
		return "aside"
	case "contentinfo":
		return "footer"
	case "paragraph":
		return "p"
	case "listitem":
		return "li"
	case "img", "image":
		return "img"
	case "StaticText", "text":
		return "text"
	case "table":
		return "table"
	case "rowheader":
		return "rowheader"
	case "columnheader":
		return "colheader"
	default:
		return role
	}
}
