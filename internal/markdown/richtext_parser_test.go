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
