package markdown

import (
	"fmt"
	"strings"

	"github.com/ran-codes/notion-sync/internal/notion"
)

// ConvertRichText converts Notion rich text items to Markdown.
// Bold and italic delimiters are placed at annotation boundaries (transitions)
// to avoid garbled asterisks when adjacent segments share bold/italic state.
func ConvertRichText(richTexts []notion.RichText) string {
	var result strings.Builder

	for i, item := range richTexts {
		curr := item.Annotations

		// Get prev/next bold/italic state
		var prevBold, prevItalic, nextBold, nextItalic bool
		if i > 0 {
			prevBold = richTexts[i-1].Annotations.Bold
			prevItalic = richTexts[i-1].Annotations.Italic
		}
		if i < len(richTexts)-1 {
			nextBold = richTexts[i+1].Annotations.Bold
			nextItalic = richTexts[i+1].Annotations.Italic
		}

		// Open delimiters (bold outside, italic inside)
		if curr.Bold && !prevBold {
			result.WriteString("**")
		}
		if curr.Italic && !prevItalic {
			result.WriteString("*")
		}

		// Write text with non-colliding annotations applied
		result.WriteString(convertRichTextContent(item))

		// Close delimiters (italic inside, bold outside — reverse order)
		if curr.Italic && !nextItalic {
			result.WriteString("*")
		}
		if curr.Bold && !nextBold {
			result.WriteString("**")
		}
	}

	return result.String()
}

// convertRichTextContent extracts text content and applies non-colliding annotations
// (code, strikethrough, underline, highlight). Bold and italic are handled by ConvertRichText.
func convertRichTextContent(item notion.RichText) string {
	var text string

	switch item.Type {
	case "equation":
		if item.Equation != nil {
			text = fmt.Sprintf("$%s$", item.Equation.Expression)
		} else {
			text = item.PlainText
		}
	case "mention":
		text = convertMention(item)
	default: // "text"
		if item.Text != nil {
			if item.Text.Link != nil {
				text = fmt.Sprintf("[%s](%s)", item.Text.Content, item.Text.Link.URL)
			} else {
				text = item.Text.Content
			}
		} else {
			text = item.PlainText
		}
	}

	// Apply non-colliding annotations only
	a := item.Annotations
	if a.Code {
		text = fmt.Sprintf("`%s`", text)
	}
	if a.Strikethrough {
		text = fmt.Sprintf("~~%s~~", text)
	}
	if a.Underline {
		text = fmt.Sprintf("<u>%s</u>", text)
	}
	if a.Color != "default" && strings.HasSuffix(a.Color, "_background") {
		text = fmt.Sprintf("==%s==", text)
	}

	return text
}

func convertMention(item notion.RichText) string {
	if item.Type != "mention" || item.Mention == nil {
		return item.PlainText
	}

	mention := item.Mention
	switch mention.Type {
	case "page":
		if mention.Page != nil {
			return fmt.Sprintf("[[notion-id: %s]]", mention.Page.ID)
		}
	case "database":
		if mention.Database != nil {
			return fmt.Sprintf("[[notion-id: %s]]", mention.Database.ID)
		}
	case "date":
		if mention.Date != nil {
			if mention.Date.End != nil {
				return fmt.Sprintf("%s → %s", mention.Date.Start, *mention.Date.End)
			}
			return mention.Date.Start
		}
	case "user":
		return fmt.Sprintf("@%s", item.PlainText)
	case "link_preview":
		if mention.LinkPreview != nil {
			return fmt.Sprintf("[%s](%s)", item.PlainText, mention.LinkPreview.URL)
		}
	}

	return item.PlainText
}
