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
// Marker balancing (the fix that made this parser push-safe). The naive toggle
// model treats every lone "*", "==", "`", "~~", or "<u>" as a formatting marker,
// which FABRICATES formatting and DROPS the delimiter on perfectly ordinary cell
// values — "2 * 3 = 6", "5 * 4 * 3", "100% == done", a stray backtick. That is
// exactly the corruption #95 set out to kill, so it must not reach Notion on push.
//
// Instead the parser tokenizes first, then resolves which delimiters are real:
//
//   - Toggle markers (** * ~~ ==) obey CommonMark-style flanking: a marker with
//     whitespace immediately after it cannot open, and one with whitespace
//     immediately before it cannot close. A marker that can neither open nor close
//     — typically because it is surrounded by spaces, like the "*" in "2 * 3" or
//     the "==" in "100% == done" — is demoted to literal text. Surviving markers
//     are matched open→close; any opener left unclosed at end of input is also
//     literal. So "2 * 3" and "5 * 4 * 3" keep their asterisks, "x == y == z"
//     keeps its "==", and "a*b*c*d" yields one italic run plus a literal tail
//     rather than deleting every marker.
//   - <u>/</u> are matched as an explicit open/close pair via a stack; an
//     unmatched <u> or </u> is literal.
//   - Code spans take content literally up to the next backtick; an unterminated
//     backtick is literal, not a code span swallowing the rest of the line.
//   - Links match their delimiters with nesting awareness: the text's "]" is the
//     bracket-balanced close and the URL's ")" is the paren-balanced close, so a
//     parenthesized URL such as
//     "[x](https://en.wikipedia.org/wiki/Foo_(bar))" keeps its full URL instead
//     of truncating at "Foo_(bar" and emitting a phantom ")" segment.
//
// Pure function — no push wiring lives here.
func ParseRichText(md string) []notion.RichText {
	toks := tokenize(md)
	resolveActive(toks)
	p := &inlineParser{}
	p.emit(toks)
	if len(p.out) == 0 {
		return nil
	}
	return p.out
}

// tokKind classifies a lexed token. Delimiter kinds carry their literal source in
// token.text so an unresolved (orphan) delimiter can be re-emitted verbatim.
type tokKind int

const (
	tkText      tokKind = iota // literal text run
	tkBold                     // **
	tkItalic                   // *
	tkStrike                   // ~~
	tkHighlight                // ==
	tkUOpen                    // <u>
	tkUClose                   // </u>
	tkCode                     // `...` (text = literal content)
	tkLink                     // [text](url)
)

// token is one lexed unit. For delimiter kinds, text holds the literal marker so
// an inactive (unpaired) delimiter falls back to plain text. canOpen/canClose are
// the flanking flags for toggle markers; active is filled in by resolveActive:
// true means the delimiter acts as formatting, false means it is emitted literally.
type token struct {
	kind              tokKind
	text              string // literal text, code content, link text, or raw delimiter
	url               string // link URL (tkLink only)
	canOpen, canClose bool   // flanking: may this toggle marker open / close emphasis?
	active            bool   // delimiter resolved to real formatting (pass 2)
}

// tokenize lexes the input into text and delimiter tokens. Code spans and links
// are lexed greedily (their inner markers are literal); a "[" that does not form a
// valid link and an unterminated backtick fall back to literal text. Toggle
// markers record their flanking so resolveActive can pair them.
func tokenize(src string) []token {
	var toks []token
	appendText := func(s string) {
		if n := len(toks); n > 0 && toks[n-1].kind == tkText {
			toks[n-1].text += s
		} else {
			toks = append(toks, token{kind: tkText, text: s})
		}
	}
	toggle := func(i, length int, kind tokKind, lit string) token {
		open, close := flanking(src, i, length)
		return token{kind: kind, text: lit, canOpen: open, canClose: close}
	}
	i := 0
	for i < len(src) {
		rest := src[i:]
		switch {
		case strings.HasPrefix(rest, "`"):
			if content, n, ok := scanCode(rest); ok {
				toks = append(toks, token{kind: tkCode, text: content})
				i += n
			} else {
				appendText("`")
				i++
			}
		case strings.HasPrefix(rest, "["):
			if text, url, n, ok := scanLink(rest); ok {
				toks = append(toks, token{kind: tkLink, text: text, url: url})
				i += n
			} else {
				appendText("[")
				i++
			}
		case strings.HasPrefix(rest, "<u>"):
			toks = append(toks, token{kind: tkUOpen, text: "<u>"})
			i += 3
		case strings.HasPrefix(rest, "</u>"):
			toks = append(toks, token{kind: tkUClose, text: "</u>"})
			i += 4
		case strings.HasPrefix(rest, "**"):
			toks = append(toks, toggle(i, 2, tkBold, "**"))
			i += 2
		case strings.HasPrefix(rest, "~~"):
			toks = append(toks, toggle(i, 2, tkStrike, "~~"))
			i += 2
		case strings.HasPrefix(rest, "=="):
			toks = append(toks, toggle(i, 2, tkHighlight, "=="))
			i += 2
		case strings.HasPrefix(rest, "*"):
			toks = append(toks, toggle(i, 1, tkItalic, "*"))
			i++
		default:
			appendText(src[i : i+1])
			i++
		}
	}
	return toks
}

// flanking reports whether a toggle marker of the given length at position i may
// open and/or close emphasis (CommonMark-simplified): a marker may open only if
// the character immediately after it is not whitespace, and may close only if the
// character immediately before it is not whitespace.
func flanking(src string, i, length int) (canOpen, canClose bool) {
	if i+length < len(src) {
		canOpen = !isInlineSpace(src[i+length])
	}
	if i > 0 {
		canClose = !isInlineSpace(src[i-1])
	}
	return
}

func isInlineSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// scanCode reads a code span starting at a backtick. The content is everything up
// to the next backtick (markers inside are literal). Returns ok=false for an
// unterminated span so the caller emits the backtick as literal text.
func scanCode(rest string) (content string, n int, ok bool) {
	end := strings.IndexByte(rest[1:], '`')
	if end < 0 {
		return "", 0, false
	}
	return rest[1 : 1+end], 1 + end + 1, true
}

// scanLink parses "[text](url)" with nesting-aware delimiter matching: the text's
// closing "]" is bracket-balanced and the URL's closing ")" is paren-balanced, so
// a parenthesized URL is not truncated. Returns ok=false if the syntax does not
// match so the caller falls back to treating "[" as literal text.
func scanLink(rest string) (text, url string, n int, ok bool) {
	closeText := matchDelim(rest, 0, '[', ']')
	if closeText < 0 || closeText+1 >= len(rest) || rest[closeText+1] != '(' {
		return "", "", 0, false
	}
	closeURL := matchDelim(rest, closeText+1, '(', ')')
	if closeURL < 0 {
		return "", "", 0, false
	}
	return rest[1:closeText], rest[closeText+2 : closeURL], closeURL + 1, true
}

// matchDelim returns the index of the close delimiter that balances the open
// delimiter at position start, or -1 if unbalanced. s[start] must equal open.
func matchDelim(s string, start int, open, close byte) int {
	depth := 0
	for j := start; j < len(s); j++ {
		switch s[j] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// resolveActive marks which delimiter tokens are real formatting (matched) versus
// lone markers (emitted literally). Toggle kinds are paired open→close per type
// using flanking: a closer pops the nearest open opener; an opener left unmatched
// at end of input stays inactive. <u>/</u> match as an explicit open/close pair.
func resolveActive(toks []token) {
	for _, kind := range []tokKind{tkBold, tkItalic, tkStrike, tkHighlight} {
		var open []int
		for i := range toks {
			if toks[i].kind != kind {
				continue
			}
			switch {
			case toks[i].canClose && len(open) > 0:
				o := open[len(open)-1]
				open = open[:len(open)-1]
				toks[o].active = true
				toks[i].active = true
			case toks[i].canOpen:
				open = append(open, i)
			}
		}
	}
	var stack []int
	for i := range toks {
		switch toks[i].kind {
		case tkUOpen:
			stack = append(stack, i)
		case tkUClose:
			if len(stack) > 0 {
				o := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				toks[o].active = true
				toks[i].active = true
			}
		}
	}
}

// inlineState is the set of annotations currently active at the scan position.
// highlight is tracked separately from Notion's single Color field and only
// resolved to a color name when a segment is emitted.
type inlineState struct {
	bold, italic, strike, underline, highlight bool
}

type inlineParser struct {
	buf strings.Builder
	st  inlineState
	out []notion.RichText
}

// emit walks the resolved tokens, flipping annotation state on active delimiters,
// buffering text (and inactive delimiters' literal markers), and flushing a
// segment whenever the state changes or a code/link token is reached.
func (p *inlineParser) emit(toks []token) {
	for _, t := range toks {
		switch t.kind {
		case tkText:
			p.buf.WriteString(t.text)
		case tkBold:
			p.toggle(t, &p.st.bold)
		case tkItalic:
			p.toggle(t, &p.st.italic)
		case tkStrike:
			p.toggle(t, &p.st.strike)
		case tkHighlight:
			p.toggle(t, &p.st.highlight)
		case tkUOpen:
			p.set(t, &p.st.underline, true)
		case tkUClose:
			p.set(t, &p.st.underline, false)
		case tkCode:
			p.flush()
			rt := p.segment(t.text, nil)
			rt.Annotations.Code = true
			p.out = append(p.out, rt)
		case tkLink:
			p.flush()
			url := t.url
			p.out = append(p.out, p.segment(t.text, &url))
		}
	}
	p.flush()
}

// toggle flips a boolean annotation for an active delimiter (flushing the segment
// before the boundary); an inactive (lone) delimiter is emitted as literal text.
func (p *inlineParser) toggle(t token, flag *bool) {
	if !t.active {
		p.buf.WriteString(t.text)
		return
	}
	p.flush()
	*flag = !*flag
}

// set assigns a boolean annotation for an active open/close delimiter (flushing
// first); an inactive delimiter is emitted as literal text.
func (p *inlineParser) set(t token, flag *bool, v bool) {
	if !t.active {
		p.buf.WriteString(t.text)
		return
	}
	p.flush()
	*flag = v
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
