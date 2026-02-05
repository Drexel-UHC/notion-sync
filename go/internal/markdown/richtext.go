package markdown

import (
	"fmt"
	"strings"

	"github.com/ran-codes/notion-sync/internal/notion"
)

// ConvertRichText converts Notion rich text items to Markdown.
func ConvertRichText(richTexts []notion.RichText) string {
	var parts []string
	for _, item := range richTexts {
		parts = append(parts, convertRichTextItem(item))
	}
	return strings.Join(parts, "")
}

func convertRichTextItem(item notion.RichText) string {
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

	// Apply annotations
	a := item.Annotations
	if a.Code {
		text = fmt.Sprintf("`%s`", text)
	}
	if a.Bold {
		text = fmt.Sprintf("**%s**", text)
	}
	if a.Italic {
		text = fmt.Sprintf("*%s*", text)
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
