// Package markdown converts Notion blocks and rich text to Markdown.
package markdown

import (
	"fmt"
	"strings"

	"github.com/ran-codes/notion-sync/internal/notion"
)

// ConvertContext holds context for block conversion.
type ConvertContext struct {
	Client      *notion.Client
	IndentLevel int
}

// ConvertBlocksToMarkdown converts a slice of Notion blocks to Markdown.
func ConvertBlocksToMarkdown(blocks []notion.Block, ctx *ConvertContext) (string, error) {
	var lines []string
	numberedIndex := 1

	for _, block := range blocks {
		// Reset numbered list counter when block type changes
		if block.Type != "numbered_list_item" {
			numberedIndex = 1
		}

		result, err := convertBlock(block, ctx, numberedIndex)
		if err != nil {
			return "", err
		}

		if block.Type == "numbered_list_item" {
			numberedIndex++
		}

		lines = append(lines, result)
	}

	return strings.Join(lines, "\n"), nil
}

func convertBlock(block notion.Block, ctx *ConvertContext, numberedIndex int) (string, error) {
	indent := strings.Repeat("    ", ctx.IndentLevel)

	switch block.Type {
	case "paragraph":
		if block.Paragraph == nil {
			return "", nil
		}
		text := ConvertRichText(block.Paragraph.RichText)
		children, err := maybeConvertChildren(block, ctx)
		if err != nil {
			return "", err
		}
		return text + children, nil

	case "heading_1":
		if block.Heading1 == nil {
			return "", nil
		}
		text := ConvertRichText(block.Heading1.RichText)
		if block.Heading1.IsToggleable {
			childMd, err := maybeConvertChildren(block, ctx)
			if err != nil {
				return "", err
			}
			lines := strings.Split(childMd, "\n")
			for i, line := range lines {
				lines[i] = "> " + line
			}
			return fmt.Sprintf("> [!note]+ # %s\n%s", text, strings.Join(lines, "\n")), nil
		}
		return fmt.Sprintf("# %s", text), nil

	case "heading_2":
		if block.Heading2 == nil {
			return "", nil
		}
		text := ConvertRichText(block.Heading2.RichText)
		if block.Heading2.IsToggleable {
			childMd, err := maybeConvertChildren(block, ctx)
			if err != nil {
				return "", err
			}
			lines := strings.Split(childMd, "\n")
			for i, line := range lines {
				lines[i] = "> " + line
			}
			return fmt.Sprintf("> [!note]+ ## %s\n%s", text, strings.Join(lines, "\n")), nil
		}
		return fmt.Sprintf("## %s", text), nil

	case "heading_3":
		if block.Heading3 == nil {
			return "", nil
		}
		text := ConvertRichText(block.Heading3.RichText)
		if block.Heading3.IsToggleable {
			childMd, err := maybeConvertChildren(block, ctx)
			if err != nil {
				return "", err
			}
			lines := strings.Split(childMd, "\n")
			for i, line := range lines {
				lines[i] = "> " + line
			}
			return fmt.Sprintf("> [!note]+ ### %s\n%s", text, strings.Join(lines, "\n")), nil
		}
		return fmt.Sprintf("### %s", text), nil

	case "bulleted_list_item":
		if block.BulletedListItem == nil {
			return "", nil
		}
		text := ConvertRichText(block.BulletedListItem.RichText)
		childCtx := &ConvertContext{Client: ctx.Client, IndentLevel: ctx.IndentLevel + 1}
		children, err := maybeConvertChildren(block, childCtx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s- %s%s", indent, text, children), nil

	case "numbered_list_item":
		if block.NumberedListItem == nil {
			return "", nil
		}
		text := ConvertRichText(block.NumberedListItem.RichText)
		childCtx := &ConvertContext{Client: ctx.Client, IndentLevel: ctx.IndentLevel + 1}
		children, err := maybeConvertChildren(block, childCtx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s%d. %s%s", indent, numberedIndex, text, children), nil

	case "to_do":
		if block.ToDo == nil {
			return "", nil
		}
		text := ConvertRichText(block.ToDo.RichText)
		check := " "
		if block.ToDo.Checked {
			check = "x"
		}
		childCtx := &ConvertContext{Client: ctx.Client, IndentLevel: ctx.IndentLevel + 1}
		children, err := maybeConvertChildren(block, childCtx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s- [%s] %s%s", indent, check, text, children), nil

	case "code":
		if block.Code == nil {
			return "", nil
		}
		text := ConvertRichText(block.Code.RichText)
		lang := block.Code.Language
		if lang == "plain text" {
			lang = ""
		}
		return fmt.Sprintf("```%s\n%s\n```", lang, text), nil

	case "quote":
		if block.Quote == nil {
			return "", nil
		}
		text := ConvertRichText(block.Quote.RichText)
		childMd, err := maybeConvertChildren(block, ctx)
		if err != nil {
			return "", err
		}
		combined := text + childMd
		lines := strings.Split(combined, "\n")
		for i, line := range lines {
			lines[i] = "> " + line
		}
		return strings.Join(lines, "\n"), nil

	case "callout":
		if block.Callout == nil {
			return "", nil
		}
		text := ConvertRichText(block.Callout.RichText)
		calloutType := emojiToCalloutType(block.Callout.Icon)
		childMd, err := maybeConvertChildren(block, ctx)
		if err != nil {
			return "", err
		}
		body := text + childMd
		lines := strings.Split(body, "\n")
		for i, line := range lines {
			lines[i] = "> " + line
		}
		return fmt.Sprintf("> [!%s]\n%s", calloutType, strings.Join(lines, "\n")), nil

	case "equation":
		if block.Equation == nil {
			return "", nil
		}
		return fmt.Sprintf("$$\n%s\n$$", block.Equation.Expression), nil

	case "divider":
		return "---", nil

	case "toggle":
		if block.Toggle == nil {
			return "", nil
		}
		text := ConvertRichText(block.Toggle.RichText)
		childMd, err := maybeConvertChildren(block, ctx)
		if err != nil {
			return "", err
		}
		lines := strings.Split(childMd, "\n")
		for i, line := range lines {
			lines[i] = "> " + line
		}
		return fmt.Sprintf("> [!note]+ %s\n%s", text, strings.Join(lines, "\n")), nil

	case "child_page":
		if block.ChildPage == nil {
			return "", nil
		}
		return fmt.Sprintf("[[%s]]", block.ChildPage.Title), nil

	case "child_database":
		if block.ChildDatabase == nil {
			return "", nil
		}
		return fmt.Sprintf("<!-- child database: %s -->", block.ChildDatabase.Title), nil

	case "image":
		return convertMediaBlock(block.Image, "image"), nil

	case "video":
		return convertMediaBlock(block.Video, ""), nil

	case "audio":
		return convertMediaBlock(block.Audio, ""), nil

	case "file":
		return convertMediaBlock(block.File, ""), nil

	case "pdf":
		return convertMediaBlock(block.PDF, ""), nil

	case "bookmark":
		if block.Bookmark == nil {
			return "", nil
		}
		caption := ConvertRichText(block.Bookmark.Caption)
		url := block.Bookmark.URL
		if caption != "" {
			return fmt.Sprintf("[%s](%s)", caption, url), nil
		}
		return url, nil

	case "embed":
		if block.Embed == nil {
			return "", nil
		}
		caption := ConvertRichText(block.Embed.Caption)
		url := block.Embed.URL
		if caption != "" {
			return fmt.Sprintf("[%s](%s)", caption, url), nil
		}
		return url, nil

	case "link_to_page":
		if block.LinkToPage == nil {
			return "", nil
		}
		switch block.LinkToPage.Type {
		case "page_id":
			return fmt.Sprintf("[[notion-id: %s]]", block.LinkToPage.PageID), nil
		case "database_id":
			return fmt.Sprintf("<!-- linked database: %s -->", block.LinkToPage.DatabaseID), nil
		}
		return "", nil

	case "synced_block":
		// Fetch children of the synced block (or original)
		children, err := maybeConvertChildren(block, ctx)
		if err != nil {
			return "", err
		}
		return children, nil

	case "table":
		return convertTable(block, ctx)

	case "column_list":
		return convertColumnList(block, ctx)

	case "column":
		// Handled by column_list
		return "", nil

	case "table_of_contents", "breadcrumb":
		// UI-only elements, skip
		return "", nil

	default:
		// Unsupported block type
		return "", nil
	}
}

func convertMediaBlock(media *notion.MediaBlock, defaultAlt string) string {
	if media == nil {
		return ""
	}

	var url string
	if media.Type == "external" && media.External != nil {
		url = media.External.URL
	} else if media.File != nil {
		url = media.File.URL
	}

	caption := ConvertRichText(media.Caption)

	// For images, use markdown image syntax
	if defaultAlt == "image" {
		alt := caption
		if alt == "" {
			alt = defaultAlt
		}
		return fmt.Sprintf("![%s](%s)", alt, url)
	}

	// For other media, use link syntax
	if caption != "" {
		return fmt.Sprintf("[%s](%s)", caption, url)
	}
	return url
}

func maybeConvertChildren(block notion.Block, ctx *ConvertContext) (string, error) {
	if !block.HasChildren || ctx.Client == nil {
		return "", nil
	}

	children, err := ctx.Client.FetchAllBlocks(block.ID)
	if err != nil {
		return "", err
	}

	childMd, err := ConvertBlocksToMarkdown(children, ctx)
	if err != nil {
		return "", err
	}

	if childMd == "" {
		return "", nil
	}

	return "\n" + childMd, nil
}

func convertTable(block notion.Block, ctx *ConvertContext) (string, error) {
	if ctx.Client == nil {
		return "", nil
	}

	rows, err := ctx.Client.FetchAllBlocks(block.ID)
	if err != nil {
		return "", err
	}

	if len(rows) == 0 {
		return "", nil
	}

	var lines []string
	for i, row := range rows {
		if row.Type != "table_row" || row.TableRow == nil {
			continue
		}

		var cells []string
		for _, cell := range row.TableRow.Cells {
			cells = append(cells, ConvertRichText(cell))
		}

		lines = append(lines, fmt.Sprintf("| %s |", strings.Join(cells, " | ")))

		if i == 0 {
			var separators []string
			for range cells {
				separators = append(separators, "---")
			}
			lines = append(lines, fmt.Sprintf("| %s |", strings.Join(separators, " | ")))
		}
	}

	return strings.Join(lines, "\n"), nil
}

func convertColumnList(block notion.Block, ctx *ConvertContext) (string, error) {
	if ctx.Client == nil {
		return "", nil
	}

	columns, err := ctx.Client.FetchAllBlocks(block.ID)
	if err != nil {
		return "", err
	}

	var parts []string
	for _, col := range columns {
		if col.Type != "column" {
			continue
		}

		children, err := ctx.Client.FetchAllBlocks(col.ID)
		if err != nil {
			return "", err
		}

		md, err := ConvertBlocksToMarkdown(children, ctx)
		if err != nil {
			return "", err
		}

		if md != "" {
			parts = append(parts, md)
		}
	}

	return strings.Join(parts, "\n\n---\n\n"), nil
}

var emojiCalloutMap = map[string]string{
	"💡": "tip",
	"⚠️": "warning",
	"❗": "danger",
	"❓": "question",
	"📝": "note",
	"🔥": "danger",
	"✅": "success",
	"📌": "important",
	"🚨": "danger",
	"💀": "danger",
	"🐛": "bug",
	"📖": "quote",
	"💬": "quote",
	"🗣️": "quote",
	"ℹ️": "info",
	"📋": "abstract",
	"🎯": "example",
	"🔗": "info",
}

func emojiToCalloutType(icon *notion.Icon) string {
	if icon == nil || icon.Type != "emoji" || icon.Emoji == "" {
		return "info"
	}

	if calloutType, ok := emojiCalloutMap[icon.Emoji]; ok {
		return calloutType
	}

	return "info"
}
