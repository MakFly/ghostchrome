package engine

import (
	"strings"
	"testing"
)

func TestFormatTextBasic(t *testing.T) {
	checked := true
	result := &ExtractionResult{
		Nodes: []ExtractedNode{
			{
				Role:  "heading",
				Name:  "Example Domain",
				Level: 1,
			},
			{
				Role: "link",
				Ref:  "@1",
				Name: "About Us",
				Href: "/about",
			},
			{
				Role: "navigation",
				Name: "Main Navigation",
				Children: []ExtractedNode{
					{
						Role: "link",
						Ref:  "@2",
						Name: "Home",
						Href: "/home",
					},
					{
						Role: "link",
						Ref:  "@3",
						Name: "Products",
						Href: "/products",
					},
				},
			},
			{
				Role: "form",
				Children: []ExtractedNode{
					{
						Role: "textbox",
						Ref:  "@4",
						Type: "text",
						Name: "Search",
					},
					{
						Role: "button",
						Ref:  "@5",
						Name: "Submit",
					},
				},
			},
			{
				Role: "paragraph",
				Name: "This domain is for illustrative examples.",
			},
			{
				Role:    "checkbox",
				Ref:     "@6",
				Name:    "Accept terms",
				Checked: &checked,
			},
		},
		Refs:  map[string]ExtractedNode{},
		Stats: ExtractionStats{},
	}

	text := FormatText(result)

	// Verify key elements are present.
	expectations := []string{
		"[h1] Example Domain",
		"[link @1 href=/about] About Us",
		"[nav] Main Navigation",
		"  [link @2 href=/home] Home",
		"  [link @3 href=/products] Products",
		"[form]",
		"  [input @4 type=text] Search",
		"  [btn @5] Submit",
		"[p] This domain is for illustrative examples.",
		"[checkbox @6 checked] Accept terms",
	}

	for _, exp := range expectations {
		if !strings.Contains(text, exp) {
			t.Errorf("expected output to contain %q\nGot:\n%s", exp, text)
		}
	}
}

func TestFormatTextIndentation(t *testing.T) {
	result := &ExtractionResult{
		Nodes: []ExtractedNode{
			{
				Role: "main",
				Children: []ExtractedNode{
					{
						Role: "navigation",
						Children: []ExtractedNode{
							{
								Role: "link",
								Ref:  "@1",
								Name: "Deep Link",
								Href: "/deep",
							},
						},
					},
				},
			},
		},
		Refs:  map[string]ExtractedNode{},
		Stats: ExtractionStats{},
	}

	text := FormatText(result)

	if !strings.Contains(text, "    [link @1 href=/deep] Deep Link") {
		t.Errorf("expected 4-space indent for depth-2 node\nGot:\n%s", text)
	}
}

func TestShouldInclude(t *testing.T) {
	tests := []struct {
		role  string
		name  string
		level ExtractLevel
		want  bool
	}{
		{"button", "Click", LevelSkeleton, true},
		{"heading", "Title", LevelSkeleton, true},
		{"paragraph", "Text", LevelSkeleton, false},
		{"paragraph", "Text", LevelContent, true},
		{"StaticText", "hello", LevelContent, true},
		{"StaticText", "hello", LevelSkeleton, false},
		{"generic", "", LevelFull, false},
		{"none", "", LevelFull, false},
		{"group", "Named Group", LevelFull, true},
		{"group", "", LevelFull, false},
	}

	for _, tt := range tests {
		got := shouldInclude(tt.role, tt.name, tt.level)
		if got != tt.want {
			t.Errorf("shouldInclude(%q, %q, %q) = %v, want %v", tt.role, tt.name, tt.level, got, tt.want)
		}
	}
}

func TestRoleToTag(t *testing.T) {
	tests := []struct {
		role string
		want string
	}{
		{"button", "btn"},
		{"link", "link"},
		{"textbox", "input"},
		{"heading", "h"},
		{"navigation", "nav"},
		{"complementary", "aside"},
		{"contentinfo", "footer"},
		{"paragraph", "p"},
		{"unknown_role", "unknown_role"},
	}

	for _, tt := range tests {
		got := roleToTag(tt.role)
		if got != tt.want {
			t.Errorf("roleToTag(%q) = %q, want %q", tt.role, got, tt.want)
		}
	}
}

// sharedResult builds a minimal ExtractionResult for boundary-marker tests.
func sharedBoundaryResult() *ExtractionResult {
	return &ExtractionResult{
		Nodes: []ExtractedNode{
			{Role: "heading", Name: "Example Domain", Level: 1},
			{Role: "link", Ref: "@1", Name: "More information", Href: "/info"},
			{Role: "button", Ref: "@2", Name: "Submit"},
		},
		Refs:  map[string]ExtractedNode{},
		Stats: ExtractionStats{},
	}
}

// TestContentBoundaryDefaultOff ensures that without ContentBoundary, output is
// byte-identical to the baseline (no markers introduced).
func TestContentBoundaryDefaultOff(t *testing.T) {
	result := sharedBoundaryResult()

	// Both calls must produce the same output.
	baseline := FormatText(result)
	withOff := FormatTextProfile(result, ProfileHuman("text"))

	if baseline != withOff {
		t.Errorf("FormatTextProfile with default profile differs from FormatText\ngot: %q\nwant: %q", withOff, baseline)
	}
	if strings.Contains(baseline, ContentBoundaryOpen) {
		t.Errorf("default output must not contain boundary marker %q, got:\n%s", ContentBoundaryOpen, baseline)
	}
}

// TestContentBoundaryHumanProfile verifies the human profile applies markers
// around node names/values when ContentBoundary=true.
func TestContentBoundaryHumanProfile(t *testing.T) {
	result := sharedBoundaryResult()

	p := ProfileHuman("text")
	p.ContentBoundary = true
	marked := FormatTextProfile(result, p)

	// Every text label must be wrapped.
	if !strings.Contains(marked, ContentBoundaryOpen+"Example Domain"+ContentBoundaryClose) {
		t.Errorf("heading name not wrapped; got:\n%s", marked)
	}
	if !strings.Contains(marked, ContentBoundaryOpen+"More information"+ContentBoundaryClose) {
		t.Errorf("link name not wrapped; got:\n%s", marked)
	}
	if !strings.Contains(marked, ContentBoundaryOpen+"Submit"+ContentBoundaryClose) {
		t.Errorf("button name not wrapped; got:\n%s", marked)
	}

	// Structural tokens (@ref, role tags) must NOT be inside the markers.
	if strings.Contains(marked, ContentBoundaryOpen+"@1") {
		t.Errorf("ref @1 must not be inside boundary markers; got:\n%s", marked)
	}
	if strings.Contains(marked, ContentBoundaryOpen+"[") {
		t.Errorf("role tag must not be inside boundary markers; got:\n%s", marked)
	}
}

// TestContentBoundaryAgentProfile verifies the agent profile also applies markers.
func TestContentBoundaryAgentProfile(t *testing.T) {
	result := sharedBoundaryResult()

	p := ProfileAgent("text")
	p.ContentBoundary = true
	marked := FormatTextProfile(result, p)

	if !strings.Contains(marked, ContentBoundaryOpen) {
		t.Errorf("agent profile with ContentBoundary must contain markers; got:\n%s", marked)
	}
	// Markers must appear exactly once per label (not double-wrapped).
	count := strings.Count(marked, ContentBoundaryOpen)
	// Three nodes each with a name → three open markers.
	if count != 3 {
		t.Errorf("expected 3 boundary-open markers (one per named node), got %d; output:\n%s", count, marked)
	}
}

// TestContentBoundaryEmptyLabel ensures nodes without a name or value emit no
// markers (wrapContent must not wrap empty strings).
func TestContentBoundaryEmptyLabel(t *testing.T) {
	result := &ExtractionResult{
		Nodes: []ExtractedNode{
			{Role: "navigation"}, // no name, no value
			{Role: "form"},
		},
		Refs:  map[string]ExtractedNode{},
		Stats: ExtractionStats{},
	}

	p := ProfileHuman("text")
	p.ContentBoundary = true
	out := FormatTextProfile(result, p)

	if strings.Contains(out, ContentBoundaryOpen) {
		t.Errorf("empty-label nodes must not produce boundary markers; got:\n%s", out)
	}
}
