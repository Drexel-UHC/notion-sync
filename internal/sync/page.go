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
	"github.com/ran-codes/notion-sync/internal/store"
	"github.com/ran-codes/notion-sync/internal/util"
)

// FreezePageOptions contains options for freezing a page.
type FreezePageOptions struct {
	Client           NotionClient
	NotionID         string
	OutputFolder     string
	DatabaseID       string
	Page             *notion.Page // Pre-fetched page (optional)
	Force            bool         // Skip timestamp check and always re-freeze
	SQLStore         *store.Store // SQLite store (nil = skip SQLite writes)
	OutputMode       OutputMode   // Controls markdown vs sqlite output
	OverrideFileName string       // If set, use this instead of computing from title (without .md extension)
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
	safeName := opts.OverrideFileName
	if safeName == "" {
		safeName = util.SanitizeFileName(title)
		if safeName == "" {
			safeName = "Untitled"
		}
	}
	filePath := filepath.Join(opts.OutputFolder, safeName+".md")

	// Check for re-freeze: compare last_edited_time
	exists := fileExists(filePath)
	if !opts.Force {
		var storedEdited string

		// Try markdown file first (for both/markdown modes)
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

		// Only skip if the markdown file confirms the page is unchanged.
		// SQLite is intentionally NOT consulted — if the .md file is missing,
		// we always re-fetch to avoid stale SQLite state blocking re-imports.
		if exists && storedEdited != "" && timestampsEqual(storedEdited, page.LastEditedTime) {
			return &PageFreezeResult{Status: "skipped", FilePath: filePath, Title: safeName}, nil
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
	ctx := &markdown.ConvertContext{Client: &markdown.CachedBlockFetcher{Tree: tree}, IndentLevel: 0}
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

	mode := opts.OutputMode
	if mode == "" {
		mode = OutputBoth
	}

	// Write markdown file (unless sqlite-only mode)
	if mode != OutputSQLite {
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
	}

	// Write to SQLite store (warnings only, never blocks markdown sync)
	if opts.SQLStore != nil {
		frozenAt := time.Now().UTC().Format(time.RFC3339)
		propsJSON, err := store.SerializeProperties(fm)
		if err != nil {
			log.Printf("warning: serialize properties for %s: %v", opts.NotionID, err)
			propsJSON = "{}"
		}
		// Only store file_path if a markdown file was actually written
		storedFilePath := ""
		if mode != OutputSQLite {
			storedFilePath = filePath
		}
		if err := opts.SQLStore.UpsertPage(store.PageData{
			ID:             opts.NotionID,
			Title:          title,
			URL:            page.URL,
			FilePath:       storedFilePath,
			BodyMarkdown:   md,
			PropertiesJSON: propsJSON,
			CreatedTime:    formatTimeIfNotZero(page.CreatedTime),
			LastEditedTime: page.LastEditedTime,
			FrozenAt:       frozenAt,
			DatabaseID:     opts.DatabaseID,
		}); err != nil {
			log.Printf("warning: SQLite upsert %s: %v", opts.NotionID, err)
		}
	}

	status := "created"
	if exists {
		status = "updated"
	}

	return &PageFreezeResult{Status: status, FilePath: filePath, Title: safeName}, nil
}

// StandalonePageImportOptions contains options for importing a standalone page.
type StandalonePageImportOptions struct {
	Client       NotionClient
	PageID       string
	OutputFolder string
	OutputMode   OutputMode
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

	// Open SQLite store at workspace root
	mode := resolveOutputMode(opts.OutputMode)
	sqlStore := openStoreIfNeeded(mode, opts.OutputFolder)
	if sqlStore != nil {
		defer sqlStore.Close()
	}

	result, err := FreezePage(FreezePageOptions{
		Client:       opts.Client,
		NotionID:     opts.PageID,
		OutputFolder: folderPath,
		DatabaseID:   "", // standalone page — no database
		Page:         page,
		SQLStore:     sqlStore,
		OutputMode:   mode,
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

	// Write CLAUDE.md at workspace root
	if err := WriteClaudeMD(opts.OutputFolder); err != nil {
		log.Printf("warning: failed to write CLAUDE.md: %v", err)
	}

	result.FolderPath = folderPath
	return result, nil
}

// RefreshStandalonePageOptions contains options for refreshing a standalone page.
type RefreshStandalonePageOptions struct {
	Client     NotionClient
	FolderPath string
	Force      bool
	OutputMode OutputMode
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

	mode := resolveOutputMode(opts.OutputMode)
	workspacePath := filepath.Dir(filepath.Dir(opts.FolderPath)) // pages/<folder> → workspace
	sqlStore := openStoreIfNeeded(mode, workspacePath)
	if sqlStore != nil {
		defer sqlStore.Close()
	}

	// Backfill CLAUDE.md at workspace root
	if err := WriteClaudeMD(workspacePath); err != nil {
		log.Printf("warning: failed to write CLAUDE.md: %v", err)
	}

	result, err := FreezePage(FreezePageOptions{
		Client:       opts.Client,
		NotionID:     meta.PageID,
		OutputFolder: opts.FolderPath,
		DatabaseID:   "",
		Force:        opts.Force,
		SQLStore:     sqlStore,
		OutputMode:   mode,
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

func formatTimeIfNotZero(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
