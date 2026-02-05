package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ran-codes/notion-sync/internal/frontmatter"
	"github.com/ran-codes/notion-sync/internal/markdown"
	"github.com/ran-codes/notion-sync/internal/notion"
	"github.com/ran-codes/notion-sync/internal/util"
)

// FreezePageOptions contains options for freezing a page.
type FreezePageOptions struct {
	Client       *notion.Client
	NotionID     string
	OutputFolder string
	DatabaseID   string
	Page         *notion.Page // Pre-fetched page (optional)
}

// FreezePage fetches a page from Notion and writes it as a Markdown file.
func FreezePage(opts FreezePageOptions) (*PageFreezeResult, error) {
	// Use pre-fetched page if provided, otherwise fetch from API
	page := opts.Page
	if page == nil {
		var err error
		page, err = opts.Client.GetPage(opts.NotionID)
		if err != nil {
			return nil, fmt.Errorf("fetch page: %w", err)
		}
	}

	title := getPageTitle(page)
	safeName := util.SanitizeFileName(title)
	if safeName == "" {
		safeName = "Untitled"
	}
	filePath := filepath.Join(opts.OutputFolder, safeName+".md")

	// Check for re-freeze: compare last_edited_time
	exists := fileExists(filePath)
	if exists {
		content, err := os.ReadFile(filePath)
		if err == nil {
			fm, _ := frontmatter.Parse(string(content))
			if fm != nil {
				if storedEdited, ok := fm["notion-last-edited"].(string); ok {
					if storedEdited == page.LastEditedTime {
						return &PageFreezeResult{Status: "skipped", FilePath: filePath, Title: safeName}, nil
					}
				}
			}
		}
	}

	// Fetch all blocks
	blocks, err := opts.Client.FetchAllBlocks(opts.NotionID)
	if err != nil {
		return nil, fmt.Errorf("fetch blocks: %w", err)
	}

	// Convert blocks to markdown
	ctx := &markdown.ConvertContext{Client: opts.Client, IndentLevel: 0}
	md, err := markdown.ConvertBlocksToMarkdown(blocks, ctx)
	if err != nil {
		return nil, fmt.Errorf("convert blocks: %w", err)
	}

	// Build frontmatter
	fm := map[string]interface{}{
		"notion-id":          opts.NotionID,
		"notion-url":         page.URL,
		"notion-frozen-at":   time.Now().UTC().Format(time.RFC3339),
		"notion-last-edited": page.LastEditedTime,
	}
	if opts.DatabaseID != "" {
		fm["notion-database-id"] = opts.DatabaseID
	}

	// Map database entry properties to frontmatter
	if opts.DatabaseID != "" {
		mapPropertiesToFrontmatter(page.Properties, fm)
	}

	// Define key order for consistent output
	keyOrder := []string{
		"notion-id",
		"notion-url",
		"notion-frozen-at",
		"notion-last-edited",
		"notion-database-id",
	}

	content := frontmatter.BuildOrdered(fm, keyOrder, md)

	// Ensure output folder exists
	if err := os.MkdirAll(opts.OutputFolder, 0755); err != nil {
		return nil, fmt.Errorf("create output folder: %w", err)
	}

	// Write file
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	status := "created"
	if exists {
		status = "updated"
	}

	return &PageFreezeResult{Status: status, FilePath: filePath, Title: safeName}, nil
}

func getPageTitle(page *notion.Page) string {
	for _, prop := range page.Properties {
		if prop.Type == "title" && len(prop.Title) > 0 {
			return markdown.ConvertRichText(prop.Title)
		}
	}
	return "Untitled"
}

func mapPropertiesToFrontmatter(properties map[string]notion.Property, fm map[string]interface{}) {
	for key, prop := range properties {
		switch prop.Type {
		case "title":
			// Already used as filename, skip

		case "rich_text":
			fm[key] = markdown.ConvertRichText(prop.RichText)

		case "number":
			if prop.Number != nil {
				fm[key] = *prop.Number
			} else {
				fm[key] = nil
			}

		case "select":
			if prop.Select != nil {
				fm[key] = prop.Select.Name
			} else {
				fm[key] = nil
			}

		case "multi_select":
			var names []interface{}
			for _, s := range prop.MultiSelect {
				names = append(names, s.Name)
			}
			if len(names) == 0 {
				fm[key] = []interface{}{}
			} else {
				fm[key] = names
			}

		case "status":
			if prop.Status != nil {
				fm[key] = prop.Status.Name
			} else {
				fm[key] = nil
			}

		case "date":
			if prop.Date != nil {
				if prop.Date.End != nil {
					fm[key] = fmt.Sprintf("%s → %s", prop.Date.Start, *prop.Date.End)
				} else {
					fm[key] = prop.Date.Start
				}
			} else {
				fm[key] = nil
			}

		case "checkbox":
			fm[key] = prop.Checkbox

		case "url":
			if prop.URL != nil {
				fm[key] = *prop.URL
			} else {
				fm[key] = nil
			}

		case "email":
			if prop.Email != nil {
				fm[key] = *prop.Email
			} else {
				fm[key] = nil
			}

		case "phone_number":
			if prop.PhoneNumber != nil {
				fm[key] = *prop.PhoneNumber
			} else {
				fm[key] = nil
			}

		case "relation":
			var ids []interface{}
			for _, r := range prop.Relation {
				ids = append(ids, r.ID)
			}
			if len(ids) == 0 {
				fm[key] = []interface{}{}
			} else {
				fm[key] = ids
			}

		case "people":
			var names []interface{}
			for _, p := range prop.People {
				if p.Name != nil {
					names = append(names, *p.Name)
				} else {
					names = append(names, p.ID)
				}
			}
			if len(names) == 0 {
				fm[key] = []interface{}{}
			} else {
				fm[key] = names
			}

		case "files":
			var urls []interface{}
			for _, f := range prop.Files {
				if f.Type == "file" && f.File != nil {
					urls = append(urls, f.File.URL)
				} else if f.External != nil {
					urls = append(urls, f.External.URL)
				}
			}
			if len(urls) == 0 {
				fm[key] = []interface{}{}
			} else {
				fm[key] = urls
			}

		case "created_time":
			fm[key] = prop.CreatedTime

		case "last_edited_time":
			fm[key] = prop.LastEditedTime

		// Skip formula, rollup, button, unique_id, verification — complex or non-user types
		default:
			// Skip unknown property types
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
