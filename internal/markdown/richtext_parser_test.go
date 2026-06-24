package markdown

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/ran-codes/notion-sync/internal/notion"
)

// ParseRichText is the inverse of ConvertRichText: it deserializes a Markdown
// inline string back into a Notion rich-text array so edited cells round-trip
// on push instead of landing as literal markers. See issue #95 (Gap 2).
func TestParseRichText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []notion.RichText
	}{
		{
			name:  "empty string yields no segments",
			input: "",
			want:  nil,
		},
		{
			name:  "plain text",
			input: "Hello",
			want: []notion.RichText{
				txt("Hello", notion.Annotations{Color: "default"}),
			},
		},
		{
			name:  "bold",
			input: "**bold**",
			want: []notion.RichText{
				txt("bold", notion.Annotations{Bold: true, Color: "default"}),
			},
		},
		{
			name:  "italic",
			input: "*italic*",
			want: []notion.RichText{
				txt("italic", notion.Annotations{Italic: true, Color: "default"}),
			},
		},
		{
			name:  "inline code",
			input: "`code`",
			want: []notion.RichText{
				txt("code", notion.Annotations{Code: true, Color: "default"}),
			},
		},
		{
			name:  "strikethrough",
			input: "~~gone~~",
			want: []notion.RichText{
				txt("gone", notion.Annotations{Strikethrough: true, Color: "default"}),
			},
		},
		{
			name:  "highlight maps to yellow_background",
			input: "==hi==",
			want: []notion.RichText{
				txt("hi", notion.Annotations{Color: "yellow_background"}),
			},
		},
		{
			name:  "underline",
			input: "<u>under</u>",
			want: []notion.RichText{
				txt("under", notion.Annotations{Underline: true, Color: "default"}),
			},
		},
		{
			name:  "link",
			input: "[text](https://example.com)",
			want: []notion.RichText{
				link("text", "https://example.com", notion.Annotations{Color: "default"}),
			},
		},
		{
			name:  "formatting in the middle of plain text",
			input: "a **b** c",
			want: []notion.RichText{
				txt("a ", notion.Annotations{Color: "default"}),
				txt("b", notion.Annotations{Bold: true, Color: "default"}),
				txt(" c", notion.Annotations{Color: "default"}),
			},
		},
		{
			name:  "bold wrapping italic yields three segments",
			input: "**bold *and italic* bold**",
			want: []notion.RichText{
				txt("bold ", notion.Annotations{Bold: true, Color: "default"}),
				txt("and italic", notion.Annotations{Bold: true, Italic: true, Color: "default"}),
				txt(" bold", notion.Annotations{Bold: true, Color: "default"}),
			},
		},
		{
			name:  "bold link",
			input: "**[text](https://example.com)**",
			want: []notion.RichText{
				link("text", "https://example.com", notion.Annotations{Bold: true, Color: "default"}),
			},
		},
		{
			name:  "code span content is literal (markers inside are not parsed)",
			input: "`a**b**c`",
			want: []notion.RichText{
				txt("a**b**c", notion.Annotations{Code: true, Color: "default"}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRichText(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRichText(%q):\n got = %s\nwant = %s", tt.input, dump(got), dump(tt.want))
			}
		})
	}
}

// txt builds a plain "text" rich-text segment with the given content and annotations.
func txt(content string, a notion.Annotations) notion.RichText {
	return notion.RichText{
		Type:        "text",
		PlainText:   content,
		Text:        &notion.TextContent{Content: content},
		Annotations: a,
	}
}

// TestParseRichText_RoundTrip is the issue #95 acceptance spec: a cell's
// formatting survives import → push. Rendering with ConvertRichText (import) then
// parsing back with ParseRichText (push) and re-rendering must be a fixed point —
// i.e. ConvertRichText(ParseRichText(s)) == s for every form the renderer emits.
func TestParseRichText_RoundTrip(t *testing.T) {
	cases := []string{
		"Hello",
		"**bold**",
		"*italic*",
		"`code`",
		"~~gone~~",
		"==hi==",
		"<u>under</u>",
		"[text](https://example.com)",
		"a **b** c",
		"**bold *and italic* bold**",
		"**[text](https://example.com)**",
		"plain `inline code` and **bold** mixed",
	}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			if got := ConvertRichText(ParseRichText(s)); got != s {
				t.Errorf("round-trip not a fixed point:\n in  = %q\n out = %q", s, got)
			}
		})
	}
}

// TestParseRichText_StructureFidelity is the parser+renderer "formatting intact"
// guarantee (issue #95, Gap 2, runbook Part 1a). It starts from a []notion.RichText
// array as Notion returns it (one segment per formatting run), renders it with
// ConvertRichText (import direction), then parses it back with ParseRichText (push
// direction) and asserts annotations survive.
//
// We deliberately avoid reflect.DeepEqual on the arrays: ConvertRichText re-segments
// at bold/italic boundaries and ParseRichText sets Text.Link rather than Href, so
// exact struct equality is brittle and tests the wrong thing. Instead we assert two
// invariants that DO hold: the render→parse→render fixed point, and a canonical
// projection compare (annotation flags + link, with highlight normalized across
// background shades and adjacent identical runs merged).
func TestParseRichText_StructureFidelity(t *testing.T) {
	input := []notion.RichText{
		txt("bold", notion.Annotations{Bold: true, Color: "default"}),
		txt(" ", notion.Annotations{Color: "default"}),
		txt("italic", notion.Annotations{Italic: true, Color: "default"}),
		txt(" ", notion.Annotations{Color: "default"}),
		txt("code", notion.Annotations{Code: true, Color: "default"}),
		txt(" ", notion.Annotations{Color: "default"}),
		txt("strike", notion.Annotations{Strikethrough: true, Color: "default"}),
		txt(" ", notion.Annotations{Color: "default"}),
		txt("hi", notion.Annotations{Color: "yellow_background"}),
		txt(" ", notion.Annotations{Color: "default"}),
		txt("under", notion.Annotations{Underline: true, Color: "default"}),
		txt(" ", notion.Annotations{Color: "default"}),
		link("link", "https://example.com", notion.Annotations{Color: "default"}),
	}

	md := ConvertRichText(input)
	got := ParseRichText(md)

	// Fixed point: parsing the rendered Markdown and re-rendering reproduces it.
	if rendered := ConvertRichText(got); rendered != md {
		t.Errorf("round-trip not a fixed point:\n in  = %q\n out = %q", md, rendered)
	}

	// Canonical projection: formatting flags + link survive, ignoring re-segmentation.
	wantProj := projectRichText(input)
	gotProj := projectRichText(got)
	if !reflect.DeepEqual(gotProj, wantProj) {
		t.Errorf("projection mismatch for md=%q:\n got  = %+v\n want = %+v", md, gotProj, wantProj)
	}
}

// TestParseRichText_ColorNonCorruption is the Gap 1 contract for foreground color:
// a colored run renders to plain text on import (color is dropped) and parses back
// as plain text with default color — it degrades, it does not corrupt (no stray
// markers, no broken segment).
func TestParseRichText_ColorNonCorruption(t *testing.T) {
	input := []notion.RichText{
		txt("red", notion.Annotations{Color: "red"}), // FOREGROUND color
	}

	md := ConvertRichText(input)
	if md != "red" {
		t.Fatalf("expected foreground color to render as plain text, got md = %q", md)
	}

	got := ParseRichText(md)
	want := []notion.RichText{txt("red", notion.Annotations{Color: "default"})}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("color corrupted on parse:\n got = %s\nwant = %s", dump(got), dump(want))
	}
}

// TestParseRichText_UserMentionNonCorruption is the Gap 1 contract for @user
// mentions: a user mention renders to a literal "@name" on import (mention identity
// is lost) and parses back as a single plain-text segment — no link, no
// [[notion-id]], no broken markers.
func TestParseRichText_UserMentionNonCorruption(t *testing.T) {
	name := "Alice"
	input := []notion.RichText{{
		Type:        "mention",
		PlainText:   "Alice",
		Mention:     &notion.Mention{Type: "user", User: &notion.Person{ID: "u1", Name: &name}},
		Annotations: notion.Annotations{Color: "default"},
	}}

	md := ConvertRichText(input)
	if md != "@Alice" {
		t.Fatalf("expected user mention to render as @name, got md = %q", md)
	}

	got := ParseRichText(md)
	want := []notion.RichText{txt("@Alice", notion.Annotations{Color: "default"})}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("user mention corrupted on parse:\n got = %s\nwant = %s", dump(got), dump(want))
	}
}

// rtProj is a canonical projection of a rich-text segment used by structure-fidelity
// comparisons: only the annotation flags and link survive, with highlight normalized
// across background shades so e.g. blue_background and yellow_background both count.
type rtProj struct {
	Text                                             string
	Bold, Italic, Code, Strike, Underline, Highlight bool
	Link                                             string
}

// projectRichText projects a rich-text slice to []rtProj, merging adjacent segments
// that carry identical annotations and link so re-segmentation by the renderer/parser
// doesn't cause spurious mismatches.
func projectRichText(rts []notion.RichText) []rtProj {
	var out []rtProj
	for _, rt := range rts {
		content := rt.PlainText
		if rt.Text != nil {
			content = rt.Text.Content
		}
		linkURL := ""
		if rt.Text != nil && rt.Text.Link != nil {
			linkURL = rt.Text.Link.URL
		}
		a := rt.Annotations
		p := rtProj{
			Text:      content,
			Bold:      a.Bold,
			Italic:    a.Italic,
			Code:      a.Code,
			Strike:    a.Strikethrough,
			Underline: a.Underline,
			Highlight: strings.HasSuffix(a.Color, "_background"),
			Link:      linkURL,
		}
		if n := len(out); n > 0 && sameProjAnnotations(out[n-1], p) {
			out[n-1].Text += p.Text
		} else {
			out = append(out, p)
		}
	}
	return out
}

// sameProjAnnotations reports whether two projections carry identical formatting and
// link (ignoring text content) — the condition for merging adjacent runs.
func sameProjAnnotations(a, b rtProj) bool {
	return a.Bold == b.Bold && a.Italic == b.Italic && a.Code == b.Code &&
		a.Strike == b.Strike && a.Underline == b.Underline &&
		a.Highlight == b.Highlight && a.Link == b.Link
}

// link builds a "text" rich-text segment carrying a link.
func link(content, url string, a notion.Annotations) notion.RichText {
	return notion.RichText{
		Type:        "text",
		PlainText:   content,
		Text:        &notion.TextContent{Content: content, Link: &notion.Link{URL: url}},
		Annotations: a,
	}
}

// dump renders a rich-text slice compactly for readable test failures.
func dump(rts []notion.RichText) string {
	var b strings.Builder
	b.WriteString("[")
	for i, rt := range rts {
		if i > 0 {
			b.WriteString(", ")
		}
		content := rt.PlainText
		if rt.Text != nil {
			content = rt.Text.Content
		}
		fmt.Fprintf(&b, "{%s content=%q ann=%+v", rt.Type, content, rt.Annotations)
		if rt.Text != nil && rt.Text.Link != nil {
			fmt.Fprintf(&b, " link=%q", rt.Text.Link.URL)
		}
		b.WriteString("}")
	}
	b.WriteString("]")
	return b.String()
}
