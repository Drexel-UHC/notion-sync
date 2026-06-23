// Package sync implements database and page syncing from Notion to local Markdown files.
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
	"github.com/ran-codes/notion-sync/internal/pathutil"
)

// DatabaseImportOptions contains options for importing a database.
type DatabaseImportOptions struct {
	Client         NotionClient
	DatabaseID     string
	OutputFolder   string
	StripPresigned bool // Strip rotating AWS pre-signed query strings from file URLs
}

// dataSourceInfo holds resolved info for a single data source to import.
type dataSourceInfo struct {
	ID         string
	Title      string // data source title (may differ from database title)
	FolderPath string // where pages land
}

// resolveDataSources determines the data sources and folder layout for a database.
// Single-source databases stay flat; multi-source databases get subfolders.
func resolveDataSources(client NotionClient, database *notion.Database, dbTitle, baseFolderPath string) ([]dataSourceInfo, error) {
	if len(database.DataSources) == 0 {
		return nil, fmt.Errorf("database has no data sources; ensure you are using Notion API version 2025-09-03 or later")
	}

	if len(database.DataSources) == 1 {
		// Single data source — flat layout (no subfolder), same as before
		ds := database.DataSources[0]
		return []dataSourceInfo{{
			ID:         ds.ID,
			Title:      dbTitle,
			FolderPath: baseFolderPath,
		}}, nil
	}

	// Multiple data sources — each gets a subfolder
	var sources []dataSourceInfo
	for _, ds := range database.DataSources {
		detail, err := client.GetDataSource(ds.ID)
		if err != nil {
			return nil, fmt.Errorf("verify data source %s: %w", ds.ID, err)
		}
		dsTitle := markdown.ConvertRichText(detail.Title)
		if dsTitle == "" {
			dsTitle = "Data Source " + ds.ID[:8]
		}
		safeDSName := pathutil.SanitizeFileName(dsTitle)
		sources = append(sources, dataSourceInfo{
			ID:         ds.ID,
			Title:      dsTitle,
			FolderPath: filepath.Join(baseFolderPath, safeDSName),
		})
	}
	return sources, nil
}

// FreshDatabaseImport imports all entries from a Notion database.
func FreshDatabaseImport(opts DatabaseImportOptions, onProgress ProgressCallback) (*DatabaseFreezeResult, error) {
	if onProgress != nil {
		onProgress(ProgressPhase{Phase: PhaseQuerying})
	}

	// Fetch database metadata
	database, err := opts.Client.GetDatabase(opts.DatabaseID)
	if err != nil {
		return nil, fmt.Errorf("fetch database: %w", err)
	}

	dbTitle := markdown.ConvertRichText(database.Title)
	if dbTitle == "" {
		dbTitle = "Untitled Database"
	}
	safeName := pathutil.SanitizeFileName(dbTitle)
	folderPath := filepath.Join(opts.OutputFolder, safeName)

	// Create folder
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		return nil, fmt.Errorf("create folder: %w", err)
	}

	// Resolve data sources and folder layout
	sources, err := resolveDataSources(opts.Client, database, dbTitle, folderPath)
	if err != nil {
		return nil, err
	}

	// Track results
	result := &DatabaseFreezeResult{
		Title:      dbTitle,
		FolderPath: folderPath,
	}

	// Import each data source
	for _, src := range sources {
		if err := os.MkdirAll(src.FolderPath, 0755); err != nil {
			return nil, fmt.Errorf("create folder %s: %w", src.FolderPath, err)
		}
		entries, err := opts.Client.QueryAllEntries(src.ID)
		if err != nil {
			return nil, fmt.Errorf("query data source %s: %w", src.Title, err)
		}
		countBefore := result.Total
		importEntries(opts.Client, entries, src.FolderPath, opts.DatabaseID, result, src.Title, opts.StripPresigned, onProgress)

		// Write per-source metadata for multi-source databases
		if len(sources) > 1 {
			srcMeta := &FrozenDatabase{
				DatabaseID:   opts.DatabaseID,
				DataSourceID: src.ID,
				Title:        src.Title,
				URL:          database.URL,
				FolderPath:   src.FolderPath,
				LastSyncedAt: time.Now().UTC().Format(time.RFC3339),
				EntryCount:   result.Total - countBefore,
			}
			if err := WriteDatabaseMetadata(src.FolderPath, srcMeta); err != nil {
				return nil, fmt.Errorf("write metadata for %s: %w", src.Title, err)
			}
		}
	}

	// Write top-level database metadata
	metadata := &FrozenDatabase{
		DatabaseID:   opts.DatabaseID,
		Title:        dbTitle,
		URL:          database.URL,
		FolderPath:   folderPath,
		LastSyncedAt: time.Now().UTC().Format(time.RFC3339),
		EntryCount:   result.Total,
	}
	if len(sources) == 1 {
		metadata.DataSourceID = sources[0].ID
	}
	if err := WriteDatabaseMetadata(folderPath, metadata); err != nil {
		return nil, fmt.Errorf("write metadata: %w", err)
	}

	// Write AGENTS.md at workspace root (only on first import, won't overwrite)
	if err := WriteAgentsMD(opts.OutputFolder); err != nil {
		log.Printf("warning: failed to write AGENTS.md: %v", err)
	}

	if onProgress != nil {
		onProgress(ProgressPhase{Phase: PhaseComplete})
	}

	return result, nil
}

// importEntries processes a batch of entries, updating result counters and calling onProgress.
func importEntries(
	client NotionClient,
	entries []notion.Page,
	folderPath, databaseID string,
	result *DatabaseFreezeResult,
	title string,
	stripPresigned bool,
	onProgress ProgressCallback,
) {
	total := len(entries)
	startIdx := result.Total
	result.Total += total

	if onProgress != nil {
		onProgress(ProgressPhase{Phase: PhaseStaleDetected, Stale: total, Total: result.Total})
	}

	for i, entry := range entries {
		if onProgress != nil {
			onProgress(ProgressPhase{Phase: PhaseImporting, Current: startIdx + i + 1, Total: result.Total, Title: title})
		}

		pageResult, err := FreezePage(FreezePageOptions{
			Client:         client,
			NotionID:       entry.ID,
			OutputFolder:   folderPath,
			DatabaseID:     databaseID,
			Page:           &entry,
			StripPresigned: stripPresigned,
		})

		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("Entry %s: %v", entry.ID, err))
			continue
		}

		switch pageResult.Status {
		case "created":
			result.Created++
		case "updated":
			result.Updated++
		case "skipped":
			result.Skipped++
		}
	}
}

type localFileInfo struct {
	filePath   string
	lastEdited string
}

// removeLegacySQLite deletes leftover _notion_sync.sqlite and _notion_sync.db
// files from the workspace root. These were removed in v0.3.
func removeLegacySQLite(workspacePath string) {
	for _, name := range []string{"_notion_sync.sqlite", "_notion_sync.db"} {
		p := filepath.Join(workspacePath, name)
		if _, err := os.Stat(p); err == nil {
			if err := os.Remove(p); err == nil {
				log.Printf("removed legacy %s", name)
			}
		}
	}
}

// migrateToUUIDFilenames renames title-based .md files to UUID-based filenames.
// For each file in localFiles, if the filename doesn't match "{notion-id}.md",
// it renames the file and updates the map entry in place.
func migrateToUUIDFilenames(folderPath string, localFiles map[string]localFileInfo) {
	for notionID, info := range localFiles {
		expectedName := notionID + ".md"
		expectedPath := filepath.Join(folderPath, expectedName)
		if info.filePath == expectedPath {
			continue
		}
		if err := os.Rename(info.filePath, expectedPath); err != nil {
			log.Printf("warning: failed to rename %s to %s: %v", filepath.Base(info.filePath), expectedName, err)
			continue
		}
		log.Printf("migrated: %s -> %s", filepath.Base(info.filePath), expectedName)
		localFiles[notionID] = localFileInfo{
			filePath:   expectedPath,
			lastEdited: info.lastEdited,
		}
	}
}

func scanLocalFiles(folderPath string) (map[string]localFileInfo, error) {
	result := make(map[string]localFileInfo)

	entries, err := os.ReadDir(folderPath)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(folderPath, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		fm, err := frontmatter.Parse(string(content))
		if err != nil || fm == nil {
			continue
		}

		notionID, ok := fm["notion-id"].(string)
		if !ok || notionID == "" {
			continue
		}

		var lastEdited string
		if le, ok := fm["notion-last-edited"].(string); ok {
			lastEdited = le
		}

		result[notionID] = localFileInfo{
			filePath:   filePath,
			lastEdited: lastEdited,
		}
	}

	return result, nil
}

// timestampsEqual compares two RFC3339 timestamp strings, tolerating
// differences like ".000Z" vs "Z" that represent the same instant.
func timestampsEqual(a, b string) bool {
	if a == b {
		return true
	}
	ta, errA := time.Parse(time.RFC3339Nano, a)
	tb, errB := time.Parse(time.RFC3339Nano, b)
	if errA != nil || errB != nil {
		return false
	}
	return ta.Equal(tb)
}

func markAsDeleted(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	contentStr := strings.ReplaceAll(string(content), "\r\n", "\n")

	// Check if already marked
	if strings.Contains(contentStr, "notion-deleted: true") {
		return nil
	}

	// Insert notion-deleted into frontmatter
	if strings.HasPrefix(contentStr, "---\n") {
		endIdx := strings.Index(contentStr[4:], "\n---")
		if endIdx != -1 {
			before := contentStr[:4+endIdx]
			after := contentStr[4+endIdx:]
			newContent := before + "\nnotion-deleted: true" + after
			return os.WriteFile(filePath, []byte(newContent), 0644)
		}
	}

	// No frontmatter found, add it
	fmStr := "---\nnotion-deleted: true\n---\n"
	newContent := fmStr + contentStr
	return os.WriteFile(filePath, []byte(newContent), 0644)
}
