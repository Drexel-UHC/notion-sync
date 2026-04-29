package sync

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ran-codes/notion-sync/internal/frontmatter"
	"github.com/ran-codes/notion-sync/internal/markdown"
	"github.com/ran-codes/notion-sync/internal/notion"
	"github.com/ran-codes/notion-sync/internal/util"
)

// FreezePageOptions contains options for freezing a page.
type FreezePageOptions struct {
	Client         NotionClient
	NotionID       string
	OutputFolder   string
	DatabaseID     string
	Page           *notion.Page // Pre-fetched page (optional)
	Force          bool         // Skip timestamp check and always re-freeze
	StripPresigned bool         // Strip rotating AWS pre-signed query strings from file URLs
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

	var filePath string
	if opts.DatabaseID != "" {
		// Database entry: use UUID filename
		filePath = filepath.Join(opts.OutputFolder, opts.NotionID+".md")
	} else {
		// Standalone page: keep title-based filename
		safeName := util.SanitizeFileName(title)
		if safeName == "" {
			safeName = "Untitled"
		}
		filePath = filepath.Join(opts.OutputFolder, safeName+".md")
	}

	// Check for re-freeze: compare last_edited_time
	exists := fileExists(filePath)
	if !opts.Force {
		var storedEdited string

		if exists {
			content, err := os.ReadFile(filePath)
			if err == nil {
				fm, _ := frontmatter.Parse(string(content))
				if fm != nil {
					if se, ok := fm["notion-last-edited"].(string); ok {
						storedEdited = se
					}
				}
			}
		}

		if exists && storedEdited != "" && timestampsEqual(storedEdited, page.LastEditedTime) {
			return &PageFreezeResult{Status: "skipped", FilePath: filePath, Title: title}, nil
		}
	}

	// Fetch entire block tree concurrently with progress
	tree, err := opts.Client.FetchBlockTree(opts.NotionID, func(fetched, found int) {
		fmt.Fprintf(os.Stderr, "\r  Fetching blocks... %d/%d (discovering nested content)", fetched, found)
	})
	if err != nil {
		return nil, fmt.Errorf("fetch blocks: %w", err)
	}
	// Count total blocks fetched
	totalBlocks := 0
	for _, children := range tree.Children {
		totalBlocks += len(children)
	}
	fmt.Fprintf(os.Stderr, "\r  Fetching blocks... done (%d blocks fetched)          \n", totalBlocks)

	blocks := tree.Children[opts.NotionID]

	// Convert blocks to markdown using cached tree (no more API calls)
	ctx := &markdown.ConvertContext{Client: &markdown.CachedBlockFetcher{Tree: tree}, IndentLevel: 0, StripPresigned: opts.StripPresigned}
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
		mapPropertiesToFrontmatter(page.Properties, fm, opts.StripPresigned)
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

	// Write markdown file
	if err := os.MkdirAll(opts.OutputFolder, 0755); err != nil {
		return nil, fmt.Errorf("create output folder: %w", err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}
	// Preserve file mtime from Notion's last_edited_time
	if lastEdited, err := time.Parse(time.RFC3339, page.LastEditedTime); err == nil {
		os.Chtimes(filePath, lastEdited, lastEdited)
	}

	status := "created"
	if exists {
		status = "updated"
	}

	return &PageFreezeResult{Status: status, FilePath: filePath, Title: title}, nil
}

// StandalonePageImportOptions contains options for importing a standalone page.
type StandalonePageImportOptions struct {
	Client         NotionClient
	PageID         string
	OutputFolder   string
	StripPresigned bool
}

// FreezeStandalonePage imports a standalone Notion page (not a database entry).
func FreezeStandalonePage(opts StandalonePageImportOptions) (*PageFreezeResult, error) {
	// Fetch the page to get title
	page, err := opts.Client.GetPage(opts.PageID)
	if err != nil {
		return nil, fmt.Errorf("fetch page: %w", err)
	}

	title := getPageTitle(page)
	safeName := util.SanitizeFileName(title)
	if safeName == "" {
		safeName = "Untitled"
	}

	// Build folder: <outputFolder>/pages/<title>_<shortID>/
	shortID := strings.ReplaceAll(opts.PageID, "-", "")
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	folderName := safeName + "_" + shortID
	folderPath := filepath.Join(opts.OutputFolder, "pages", folderName)

	if err := os.MkdirAll(folderPath, 0755); err != nil {
		return nil, fmt.Errorf("create folder: %w", err)
	}

	result, err := FreezePage(FreezePageOptions{
		Client:         opts.Client,
		NotionID:       opts.PageID,
		OutputFolder:   folderPath,
		DatabaseID:     "", // standalone page — no database
		Page:           page,
		StripPresigned: opts.StripPresigned,
	})
	if err != nil {
		return nil, err
	}

	// Write page metadata
	meta := &FrozenPage{
		PageID:       opts.PageID,
		Title:        title,
		URL:          page.URL,
		FolderPath:   folderPath,
		LastSyncedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := WritePageMetadata(folderPath, meta); err != nil {
		return nil, fmt.Errorf("write metadata: %w", err)
	}

	// Write AGENTS.md at workspace root
	if err := WriteAgentsMD(opts.OutputFolder); err != nil {
		log.Printf("warning: failed to write AGENTS.md: %v", err)
	}

	result.FolderPath = folderPath
	return result, nil
}

// RefreshStandalonePageOptions contains options for refreshing a standalone page.
type RefreshStandalonePageOptions struct {
	Client         NotionClient
	FolderPath     string
	Force          bool
	StripPresigned bool
}

// RefreshStandalonePage refreshes a previously imported standalone page.
func RefreshStandalonePage(opts RefreshStandalonePageOptions) (*PageFreezeResult, error) {
	meta, err := ReadPageMetadata(opts.FolderPath)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}
	if meta == nil {
		return nil, fmt.Errorf("no %s found in %s", PageMetadataFile, opts.FolderPath)
	}

	workspacePath := filepath.Dir(filepath.Dir(opts.FolderPath)) // pages/<folder> → workspace

	// Backfill AGENTS.md at workspace root
	if err := WriteAgentsMD(workspacePath); err != nil {
		log.Printf("warning: failed to write AGENTS.md: %v", err)
	}

	result, err := FreezePage(FreezePageOptions{
		Client:         opts.Client,
		NotionID:       meta.PageID,
		OutputFolder:   opts.FolderPath,
		DatabaseID:     "",
		Force:          opts.Force,
		StripPresigned: opts.StripPresigned,
	})
	if err != nil {
		return nil, err
	}

	// Update metadata timestamp
	meta.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
	if err := WritePageMetadata(opts.FolderPath, meta); err != nil {
		return nil, fmt.Errorf("write metadata: %w", err)
	}

	return result, nil
}

func getPageTitle(page *notion.Page) string {
	for _, prop := range page.Properties {
		if prop.Type == "title" && len(prop.Title) > 0 {
			return markdown.ConvertRichText(prop.Title)
		}
	}
	return "Untitled"
}

func mapPropertiesToFrontmatter(properties map[string]notion.Property, fm map[string]interface{}, stripPresigned bool) {
	for key, prop := range properties {
		switch prop.Type {
		case "title":
			if len(prop.Title) > 0 {
				fm[key] = markdown.ConvertRichText(prop.Title)
			}

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
				var u string
				if f.Type == "file" && f.File != nil {
					u = f.File.URL
				} else if f.External != nil {
					u = f.External.URL
				} else {
					continue
				}
				if stripPresigned {
					u = notion.StripPresignedParams(u)
				}
				urls = append(urls, u)
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

		case "unique_id":
			if prop.UniqueID != nil {
				if prop.UniqueID.Prefix != "" {
					fm[key] = fmt.Sprintf("%s-%d", prop.UniqueID.Prefix, prop.UniqueID.Number)
				} else {
					fm[key] = fmt.Sprintf("%d", prop.UniqueID.Number)
				}
			} else {
				fm[key] = nil
			}

		case "created_by":
			if prop.CreatedBy != nil {
				fm[key] = getUserName(prop.CreatedBy)
			} else {
				fm[key] = nil
			}

		case "last_edited_by":
			if prop.LastEditedBy != nil {
				fm[key] = getUserName(prop.LastEditedBy)
			} else {
				fm[key] = nil
			}

		// Skip formula, rollup, button, verification — complex or non-user types
		default:
			// Skip unknown property types
		}
	}
}

func getUserName(p *notion.Person) string {
	if p.Name != nil {
		return *p.Name
	}
	return p.ID
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
