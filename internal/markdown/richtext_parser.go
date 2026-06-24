package markdown

import (
	"strings"

	"github.com/ran-codes/notion-sync/internal/notion"
)

// ParseRichText is the inverse of ConvertRichText: it deserializes a Markdown
// inline string back into a Notion rich-text array. It exists so an edited
// rich_text / title cell round-trips with formatting intact on push instead of
// landing in Notion as literal markers (e.g. "**bold**" as 8 plain characters).
// See issue #95 (Gap 2).
//
// Recognized inline syntax (the inverse of what ConvertRichText emits):
//
//	**bold**           bold
//	*italic*           italic
//	`code`             code (content is literal — markers inside are not parsed)
//	~~strike~~         strikethrough
//	==text==           highlight → yellow_background
//	<u>text</u>        underline
//	[text](url)        link
//
// Annotations nest: "**bold *and italic* bold**" yields three segments whose
// annotation sets reflect the active markers at each boundary, matching Notion's
// flat rich-text model. Color/`@user` mention identity are intentionally NOT
// handled — they are lost on import (Gap 1) and documented as not round-tripped.
//
// Known limitation — unbalanced markers corrupt content. The parser uses a flat
// toggle model with NO balance checking, so any lone "*", "==", "`", "~~", or
// "<u>" in ordinary cell text is treated as a formatting toggle. This is not mere
// imprecise round-tripping: on valid input the parser actively FABRICATES
// formatting and DROPS the orphan delimiter, e.g.
//
//	"2 * 3 = 6"     → " 3 = 6" emitted italic; the literal "*" is dropped
//	"100% == done"  → " done" gets a phantom yellow_background highlight
//	"a*b*c*d"       → alternating italic runs; all three "*" deleted
//
// Links share the defect through a different mechanism: tryLink matches the
// FIRST ")" (and the first "]"), so a URL or bracketed text that itself contains
// those characters is silently mis-parsed, e.g.
//
//	"[x](https://en.wikipedia.org/wiki/Foo_(bar))"
//	    → URL truncated to ".../Foo_(bar" plus a phantom ")" text segment
//
// Here the Markdown STRING still round-trips (so a fixed-point test stays blind),
// but the rich-text STRUCTURE sent to Notion is wrong — wrong URL, extra run.
//
// Multiplication, globs, footnote asterisks, "100% ==", and parenthesized URLs
// (Wikipedia and friends) are real database cell values, so this MUST be fixed —
// balance the markers (track open delimiter positions; treat a marker left
// unclosed at EOF as literal text) and match link delimiters with nesting/escape
// awareness — BEFORE this parser is wired into push, or it reintroduces the cell
// corruption #95 set out to fix. It is unwired today — ParseRichText has zero
// production callers (wiring is deferred to epic #55) — so the corruption is
// latent, not live.
//
// Pure function — no push wiring lives here.
func ParseRichText(md string) []notion.RichText {
	p := &inlineParser{src: md}
	p.run()
	if len(p.out) == 0 {
		return nil
	}
	return p.out
}

// inlineState is the set of annotations currently active at the scan position.
// highlight is tracked separately from Notion's single Color field and only
// resolved to a color name when a segment is emitted.
type inlineState struct {
	bold, italic, strike, underline, highlight bool
}

type inlineParser struct {
	src string
	pos int
	buf strings.Builder
	st  inlineState
	out []notion.RichText
}

func (p *inlineParser) run() {
	for p.pos < len(p.src) {
		switch {
		case p.consume("`"):
			p.flush()
			p.emitCode()
		case p.startsWith("["):
			if !p.tryLink() {
				p.buf.WriteByte(p.src[p.pos])
				p.pos++
			}
		case p.consume("<u>"):
			p.flush()
			p.st.underline = true
		case p.consume("</u>"):
			p.flush()
			p.st.underline = false
		case p.consume("**"):
			p.flush()
			p.st.bold = !p.st.bold
		case p.consume("~~"):
			p.flush()
			p.st.strike = !p.st.strike
		case p.consume("=="):
			p.flush()
			p.st.highlight = !p.st.highlight
		case p.consume("*"):
			p.flush()
			p.st.italic = !p.st.italic
		default:
			p.buf.WriteByte(p.src[p.pos])
			p.pos++
		}
	}
	p.flush()
}

// startsWith reports whether the remaining input begins with s.
func (p *inlineParser) startsWith(s string) bool {
	return strings.HasPrefix(p.src[p.pos:], s)
}

// consume advances past s and returns true if the remaining input begins with it.
func (p *inlineParser) consume(s string) bool {
	if p.startsWith(s) {
		p.pos += len(s)
		return true
	}
	return false
}

// flush emits the buffered text (if any) as a segment carrying the current
// annotation state, then clears the buffer.
func (p *inlineParser) flush() {
	if p.buf.Len() == 0 {
		return
	}
	p.out = append(p.out, p.segment(p.buf.String(), nil))
	p.buf.Reset()
}

// emitCode reads a code span: everything up to the next backtick is literal
// content (no nested markers), emitted as one segment with Code set.
func (p *inlineParser) emitCode() {
	end := strings.IndexByte(p.src[p.pos:], '`')
	if end < 0 {
		// Unterminated code span: treat the rest as literal code content.
		end = len(p.src) - p.pos
	}
	content := p.src[p.pos : p.pos+end]
	rt := p.segment(content, nil)
	rt.Annotations.Code = true
	p.out = append(p.out, rt)
	p.pos += end
	if p.pos < len(p.src) {
		p.pos++ // skip closing backtick
	}
}

// tryLink parses "[text](url)" at the current position. The bracketed text is
// treated as literal (the renderer never nests markers inside the brackets).
// Returns false if the syntax doesn't match so the caller can fall back to
// treating "[" as plain text.
func (p *inlineParser) tryLink() bool {
	rest := p.src[p.pos:]
	if !strings.HasPrefix(rest, "[") {
		return false
	}
	closeText := strings.IndexByte(rest, ']')
	if closeText < 0 || closeText+1 >= len(rest) || rest[closeText+1] != '(' {
		return false
	}
	closeURL := strings.IndexByte(rest[closeText+2:], ')')
	if closeURL < 0 {
		return false
	}
	text := rest[1:closeText]
	url := rest[closeText+2 : closeText+2+closeURL]
	p.flush()
	rt := p.segment(text, &url)
	p.out = append(p.out, rt)
	p.pos += closeText + 2 + closeURL + 1
	return true
}

// segment builds a rich-text segment for content under the current annotation
// state. A non-nil url attaches a link.
func (p *inlineParser) segment(content string, url *string) notion.RichText {
	color := "default"
	if p.st.highlight {
		color = "yellow_background"
	}
	tc := &notion.TextContent{Content: content}
	if url != nil {
		tc.Link = &notion.Link{URL: *url}
	}
	return notion.RichText{
		Type:      "text",
		PlainText: content,
		Text:      tc,
		Annotations: notion.Annotations{
			Bold:          p.st.bold,
			Italic:        p.st.italic,
			Strikethrough: p.st.strike,
			Underline:     p.st.underline,
			Color:         color,
		},
	}
}
