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
