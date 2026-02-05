package markdown

import (
	"testing"

	"github.com/ran-codes/notion-sync/internal/notion"
)

func TestConvertRichText(t *testing.T) {
	tests := []struct {
		name     string
		input    []notion.RichText
		expected string
	}{
		{
			name:     "empty",
			input:    nil,
			expected: "",
		},
		{
			name: "plain text",
			input: []notion.RichText{
				{Type: "text", PlainText: "Hello", Text: &notion.TextContent{Content: "Hello"}},
			},
			expected: "Hello",
		},
		{
			name: "bold text",
			input: []notion.RichText{
				{
					Type:        "text",
					PlainText:   "Bold",
					Text:        &notion.TextContent{Content: "Bold"},
					Annotations: notion.Annotations{Bold: true, Color: "default"},
				},
			},
			expected: "**Bold**",
		},
		{
			name: "italic text",
			input: []notion.RichText{
				{
					Type:        "text",
					PlainText:   "Italic",
					Text:        &notion.TextContent{Content: "Italic"},
					Annotations: notion.Annotations{Italic: true, Color: "default"},
				},
			},
			expected: "*Italic*",
		},
		{
			name: "code text",
			input: []notion.RichText{
				{
					Type:        "text",
					PlainText:   "code",
					Text:        &notion.TextContent{Content: "code"},
					Annotations: notion.Annotations{Code: true, Color: "default"},
				},
			},
			expected: "`code`",
		},
		{
			name: "strikethrough",
			input: []notion.RichText{
				{
					Type:        "text",
					PlainText:   "deleted",
					Text:        &notion.TextContent{Content: "deleted"},
					Annotations: notion.Annotations{Strikethrough: true, Color: "default"},
				},
			},
			expected: "~~deleted~~",
		},
		{
			name: "underline",
			input: []notion.RichText{
				{
					Type:        "text",
					PlainText:   "underlined",
					Text:        &notion.TextContent{Content: "underlined"},
					Annotations: notion.Annotations{Underline: true, Color: "default"},
				},
			},
			expected: "<u>underlined</u>",
		},
		{
			name: "highlighted (background color)",
			input: []notion.RichText{
				{
					Type:        "text",
					PlainText:   "highlighted",
					Text:        &notion.TextContent{Content: "highlighted"},
					Annotations: notion.Annotations{Color: "yellow_background"},
				},
			},
			expected: "==highlighted==",
		},
		{
			name: "link",
			input: []notion.RichText{
				{
					Type:        "text",
					PlainText:   "Click here",
					Text:        &notion.TextContent{Content: "Click here", Link: &notion.Link{URL: "https://example.com"}},
					Annotations: notion.Annotations{Color: "default"},
				},
			},
			expected: "[Click here](https://example.com)",
		},
		{
			name: "combined annotations",
			input: []notion.RichText{
				{
					Type:        "text",
					PlainText:   "Bold and italic",
					Text:        &notion.TextContent{Content: "Bold and italic"},
					Annotations: notion.Annotations{Bold: true, Italic: true, Color: "default"},
				},
			},
			expected: "***Bold and italic***",
		},
		{
			name: "multi-segment bold with italic sub-span",
			input: []notion.RichText{
				{Type: "text", Text: &notion.TextContent{Content: "bold "},
					Annotations: notion.Annotations{Bold: true, Color: "default"}},
				{Type: "text", Text: &notion.TextContent{Content: "and italic"},
					Annotations: notion.Annotations{Bold: true, Italic: true, Color: "default"}},
				{Type: "text", Text: &notion.TextContent{Content: " combined"},
					Annotations: notion.Annotations{Bold: true, Color: "default"}},
			},
			expected: "**bold *and italic* combined**",
		},
		{
			name: "adjacent bold segments",
			input: []notion.RichText{
				{Type: "text", Text: &notion.TextContent{Content: "hello "},
					Annotations: notion.Annotations{Bold: true, Color: "default"}},
				{Type: "text", Text: &notion.TextContent{Content: "world"},
					Annotations: notion.Annotations{Bold: true, Color: "default"}},
			},
			expected: "**hello world**",
		},
		{
			name: "italic span with bold sub-span",
			input: []notion.RichText{
				{Type: "text", Text: &notion.TextContent{Content: "italic "},
					Annotations: notion.Annotations{Italic: true, Color: "default"}},
				{Type: "text", Text: &notion.TextContent{Content: "and bold"},
					Annotations: notion.Annotations{Bold: true, Italic: true, Color: "default"}},
				{Type: "text", Text: &notion.TextContent{Content: " rest"},
					Annotations: notion.Annotations{Italic: true, Color: "default"}},
			},
			expected: "*italic **and bold** rest*",
		},
		{
			name: "bold then italic separate",
			input: []notion.RichText{
				{Type: "text", Text: &notion.TextContent{Content: "bold"},
					Annotations: notion.Annotations{Bold: true, Color: "default"}},
				{Type: "text", Text: &notion.TextContent{Content: " normal "},
					Annotations: notion.Annotations{Color: "default"}},
				{Type: "text", Text: &notion.TextContent{Content: "italic"},
					Annotations: notion.Annotations{Italic: true, Color: "default"}},
			},
			expected: "**bold** normal *italic*",
		},
		{
			name: "inline equation",
			input: []notion.RichText{
				{
					Type:      "equation",
					PlainText: "E=mc^2",
					Equation:  &notion.Equation{Expression: "E=mc^2"},
				},
			},
			expected: "$E=mc^2$",
		},
		{
			name: "page mention",
			input: []notion.RichText{
				{
					Type:      "mention",
					PlainText: "My Page",
					Mention:   &notion.Mention{Type: "page", Page: &notion.PageMention{ID: "abc123"}},
				},
			},
			expected: "[[notion-id: abc123]]",
		},
		{
			name: "user mention",
			input: []notion.RichText{
				{
					Type:      "mention",
					PlainText: "John Doe",
					Mention:   &notion.Mention{Type: "user"},
				},
			},
			expected: "@John Doe",
		},
		{
			name: "date mention",
			input: []notion.RichText{
				{
					Type:      "mention",
					PlainText: "2024-01-15",
					Mention:   &notion.Mention{Type: "date", Date: &notion.DateValue{Start: "2024-01-15"}},
				},
			},
			expected: "2024-01-15",
		},
		{
			name: "date range mention",
			input: []notion.RichText{
				{
					Type:      "mention",
					PlainText: "Jan 15-20",
					Mention:   &notion.Mention{Type: "date", Date: &notion.DateValue{Start: "2024-01-15", End: strPtr("2024-01-20")}},
				},
			},
			expected: "2024-01-15 → 2024-01-20",
		},
		{
			name: "bold + strikethrough",
			input: []notion.RichText{
				{Type: "text", Text: &notion.TextContent{Content: "deleted"},
					Annotations: notion.Annotations{Bold: true, Strikethrough: true, Color: "default"}},
			},
			expected: "**~~deleted~~**",
		},
		{
			name: "bold link",
			input: []notion.RichText{
				{Type: "text", Text: &notion.TextContent{Content: "Click", Link: &notion.Link{URL: "https://example.com"}},
					Annotations: notion.Annotations{Bold: true, Color: "default"}},
			},
			expected: "**[Click](https://example.com)**",
		},
		{
			name: "multi-segment bold with code sub-span",
			input: []notion.RichText{
				{Type: "text", Text: &notion.TextContent{Content: "text "},
					Annotations: notion.Annotations{Bold: true, Color: "default"}},
				{Type: "text", Text: &notion.TextContent{Content: "func"},
					Annotations: notion.Annotations{Bold: true, Code: true, Color: "default"}},
				{Type: "text", Text: &notion.TextContent{Content: " more"},
					Annotations: notion.Annotations{Bold: true, Color: "default"}},
			},
			expected: "**text `func` more**",
		},
		{
			name: "adjacent bold then italic (no gap)",
			input: []notion.RichText{
				{Type: "text", Text: &notion.TextContent{Content: "A"},
					Annotations: notion.Annotations{Bold: true, Color: "default"}},
				{Type: "text", Text: &notion.TextContent{Content: "B"},
					Annotations: notion.Annotations{Italic: true, Color: "default"}},
			},
			expected: "**A***B*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertRichText(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertRichText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertBlocksToMarkdown(t *testing.T) {
	ctx := &ConvertContext{Client: nil, IndentLevel: 0}

	tests := []struct {
		name     string
		blocks   []notion.Block
		expected string
	}{
		{
			name:     "empty",
			blocks:   nil,
			expected: "",
		},
		{
			name: "paragraph",
			blocks: []notion.Block{
				{Type: "paragraph", Paragraph: &notion.ParagraphBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "Hello world"}}},
				}},
			},
			expected: "Hello world",
		},
		{
			name: "heading 1",
			blocks: []notion.Block{
				{Type: "heading_1", Heading1: &notion.HeadingBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "Title"}}},
				}},
			},
			expected: "# Title",
		},
		{
			name: "heading 2",
			blocks: []notion.Block{
				{Type: "heading_2", Heading2: &notion.HeadingBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "Subtitle"}}},
				}},
			},
			expected: "## Subtitle",
		},
		{
			name: "heading 3",
			blocks: []notion.Block{
				{Type: "heading_3", Heading3: &notion.HeadingBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "Section"}}},
				}},
			},
			expected: "### Section",
		},
		{
			name: "bulleted list",
			blocks: []notion.Block{
				{Type: "bulleted_list_item", BulletedListItem: &notion.ListItemBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "Item 1"}}},
				}},
				{Type: "bulleted_list_item", BulletedListItem: &notion.ListItemBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "Item 2"}}},
				}},
			},
			expected: "- Item 1\n- Item 2",
		},
		{
			name: "numbered list",
			blocks: []notion.Block{
				{Type: "numbered_list_item", NumberedListItem: &notion.ListItemBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "First"}}},
				}},
				{Type: "numbered_list_item", NumberedListItem: &notion.ListItemBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "Second"}}},
				}},
			},
			expected: "1. First\n2. Second",
		},
		{
			name: "to-do unchecked",
			blocks: []notion.Block{
				{Type: "to_do", ToDo: &notion.ToDoBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "Task"}}},
					Checked:  false,
				}},
			},
			expected: "- [ ] Task",
		},
		{
			name: "to-do checked",
			blocks: []notion.Block{
				{Type: "to_do", ToDo: &notion.ToDoBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "Done task"}}},
					Checked:  true,
				}},
			},
			expected: "- [x] Done task",
		},
		{
			name: "code block",
			blocks: []notion.Block{
				{Type: "code", Code: &notion.CodeBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "console.log('hi')"}}},
					Language: "javascript",
				}},
			},
			expected: "```javascript\nconsole.log('hi')\n```",
		},
		{
			name: "code block plain text",
			blocks: []notion.Block{
				{Type: "code", Code: &notion.CodeBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "plain"}}},
					Language: "plain text",
				}},
			},
			expected: "```\nplain\n```",
		},
		{
			name: "quote",
			blocks: []notion.Block{
				{Type: "quote", Quote: &notion.QuoteBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "A wise quote"}}},
				}},
			},
			expected: "> A wise quote",
		},
		{
			name: "equation",
			blocks: []notion.Block{
				{Type: "equation", Equation: &notion.EquationBlock{Expression: "x^2 + y^2 = z^2"}},
			},
			expected: "$$\nx^2 + y^2 = z^2\n$$",
		},
		{
			name: "divider",
			blocks: []notion.Block{
				{Type: "divider", Divider: &struct{}{}},
			},
			expected: "---",
		},
		{
			name: "child page",
			blocks: []notion.Block{
				{Type: "child_page", ChildPage: &notion.ChildPageBlock{Title: "My Subpage"}},
			},
			expected: "[[My Subpage]]",
		},
		{
			name: "child database",
			blocks: []notion.Block{
				{Type: "child_database", ChildDatabase: &notion.ChildDatabaseBlock{Title: "My DB"}},
			},
			expected: "<!-- child database: My DB -->",
		},
		{
			name: "image with caption",
			blocks: []notion.Block{
				{Type: "image", Image: &notion.MediaBlock{
					Type:     "external",
					External: &notion.ExternalURL{URL: "https://example.com/img.png"},
					Caption:  []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "My image"}}},
				}},
			},
			expected: "![My image](https://example.com/img.png)",
		},
		{
			name: "image without caption",
			blocks: []notion.Block{
				{Type: "image", Image: &notion.MediaBlock{
					Type:     "external",
					External: &notion.ExternalURL{URL: "https://example.com/img.png"},
				}},
			},
			expected: "![image](https://example.com/img.png)",
		},
		{
			name: "bookmark with caption",
			blocks: []notion.Block{
				{Type: "bookmark", Bookmark: &notion.BookmarkBlock{
					URL:     "https://example.com",
					Caption: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "Example"}}},
				}},
			},
			expected: "[Example](https://example.com)",
		},
		{
			name: "bookmark without caption",
			blocks: []notion.Block{
				{Type: "bookmark", Bookmark: &notion.BookmarkBlock{URL: "https://example.com"}},
			},
			expected: "https://example.com",
		},
		{
			name: "embed",
			blocks: []notion.Block{
				{Type: "embed", Embed: &notion.EmbedBlock{
					URL:     "https://youtube.com/watch?v=123",
					Caption: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "Video"}}},
				}},
			},
			expected: "[Video](https://youtube.com/watch?v=123)",
		},
		{
			name: "link to page",
			blocks: []notion.Block{
				{Type: "link_to_page", LinkToPage: &notion.LinkToPageBlock{
					Type:   "page_id",
					PageID: "page-123",
				}},
			},
			expected: "[[notion-id: page-123]]",
		},
		{
			name: "link to database",
			blocks: []notion.Block{
				{Type: "link_to_page", LinkToPage: &notion.LinkToPageBlock{
					Type:       "database_id",
					DatabaseID: "db-456",
				}},
			},
			expected: "<!-- linked database: db-456 -->",
		},
		{
			name: "table of contents (skipped)",
			blocks: []notion.Block{
				{Type: "table_of_contents", TableOfContents: &struct{}{}},
			},
			expected: "",
		},
		{
			name: "breadcrumb (skipped)",
			blocks: []notion.Block{
				{Type: "breadcrumb", Breadcrumb: &struct{}{}},
			},
			expected: "",
		},
		{
			name: "callout with tip emoji",
			blocks: []notion.Block{
				{Type: "callout", Callout: &notion.CalloutBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "Pro tip!"}}},
					Icon:     &notion.Icon{Type: "emoji", Emoji: "💡"},
				}},
			},
			expected: "> [!tip]\n> Pro tip!",
		},
		{
			name: "callout with warning emoji",
			blocks: []notion.Block{
				{Type: "callout", Callout: &notion.CalloutBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "Be careful!"}}},
					Icon:     &notion.Icon{Type: "emoji", Emoji: "⚠️"},
				}},
			},
			expected: "> [!warning]\n> Be careful!",
		},
		{
			name: "numbered list resets",
			blocks: []notion.Block{
				{Type: "numbered_list_item", NumberedListItem: &notion.ListItemBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "A"}}},
				}},
				{Type: "numbered_list_item", NumberedListItem: &notion.ListItemBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "B"}}},
				}},
				{Type: "paragraph", Paragraph: &notion.ParagraphBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "Break"}}},
				}},
				{Type: "numbered_list_item", NumberedListItem: &notion.ListItemBlock{
					RichText: []notion.RichText{{Type: "text", Text: &notion.TextContent{Content: "C"}}},
				}},
			},
			expected: "1. A\n2. B\nBreak\n1. C",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertBlocksToMarkdown(tt.blocks, ctx)
			if err != nil {
				t.Fatalf("ConvertBlocksToMarkdown error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("ConvertBlocksToMarkdown() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestEmojiToCalloutType(t *testing.T) {
	tests := []struct {
		icon     *notion.Icon
		expected string
	}{
		{nil, "info"},
		{&notion.Icon{Type: "external"}, "info"},
		{&notion.Icon{Type: "emoji", Emoji: ""}, "info"},
		{&notion.Icon{Type: "emoji", Emoji: "💡"}, "tip"},
		{&notion.Icon{Type: "emoji", Emoji: "⚠️"}, "warning"},
		{&notion.Icon{Type: "emoji", Emoji: "❗"}, "danger"},
		{&notion.Icon{Type: "emoji", Emoji: "❓"}, "question"},
		{&notion.Icon{Type: "emoji", Emoji: "📝"}, "note"},
		{&notion.Icon{Type: "emoji", Emoji: "✅"}, "success"},
		{&notion.Icon{Type: "emoji", Emoji: "🐛"}, "bug"},
		{&notion.Icon{Type: "emoji", Emoji: "📖"}, "quote"},
		{&notion.Icon{Type: "emoji", Emoji: "🎯"}, "example"},
		{&notion.Icon{Type: "emoji", Emoji: "🙂"}, "info"}, // unknown emoji
	}

	for _, tt := range tests {
		name := "nil"
		if tt.icon != nil {
			name = tt.icon.Emoji
			if name == "" {
				name = tt.icon.Type
			}
		}
		t.Run(name, func(t *testing.T) {
			result := emojiToCalloutType(tt.icon)
			if result != tt.expected {
				t.Errorf("emojiToCalloutType() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
