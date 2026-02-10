// Package sync implements database and page syncing from Notion to local Markdown files.
package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ran-codes/notion-sync/internal/frontmatter"
	"github.com/ran-codes/notion-sync/internal/markdown"
	"github.com/ran-codes/notion-sync/internal/notion"
	"github.com/ran-codes/notion-sync/internal/util"
)

// DatabaseImportOptions contains options for importing a database.
type DatabaseImportOptions struct {
	Client       *notion.Client
	DatabaseID   string
	OutputFolder string
}

// RefreshOptions contains options for refreshing a database.
type RefreshOptions struct {
	Client     *notion.Client
	FolderPath string
	Force      bool // Skip timestamp comparison and resync all entries
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
	safeName := util.SanitizeFileName(dbTitle)
	folderPath := filepath.Join(opts.OutputFolder, safeName)

	// Create folder
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		return nil, fmt.Errorf("create folder: %w", err)
	}

	// Query all entries — use data_sources API if available, otherwise fall back to classic endpoint
	var entries []notion.Page
	if len(database.DataSources) > 0 {
		dataSourceID := database.DataSources[0].ID
		if err := opts.Client.GetDataSource(dataSourceID); err != nil {
			return nil, fmt.Errorf("verify data source access: %w", err)
		}
		entries, err = opts.Client.QueryAllEntries(dataSourceID)
	} else {
		entries, err = opts.Client.QueryAllEntriesFromDatabase(opts.DatabaseID)
	}
	if err != nil {
		return nil, fmt.Errorf("query entries: %w", err)
	}

	total := len(entries)
	if onProgress != nil {
		onProgress(ProgressPhase{Phase: PhaseStaleDetected, Stale: total, Total: total})
	}

	// Track results
	result := &DatabaseFreezeResult{
		Title:      dbTitle,
		FolderPath: folderPath,
		Total:      total,
	}

	// Process all entries
	for i, entry := range entries {
		if onProgress != nil {
			onProgress(ProgressPhase{Phase: PhaseImporting, Current: i + 1, Total: total, Title: dbTitle})
		}

		pageResult, err := FreezePage(FreezePageOptions{
			Client:       opts.Client,
			NotionID:     entry.ID,
			OutputFolder: folderPath,
			DatabaseID:   opts.DatabaseID,
			Page:         &entry,
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

	// Write database metadata
	metadata := &FrozenDatabase{
		DatabaseID:   opts.DatabaseID,
		Title:        dbTitle,
		URL:          database.URL,
		FolderPath:   folderPath,
		LastSyncedAt: time.Now().UTC().Format(time.RFC3339),
		EntryCount:   total,
	}
	if err := WriteDatabaseMetadata(folderPath, metadata); err != nil {
		return nil, fmt.Errorf("write metadata: %w", err)
	}

	if onProgress != nil {
		onProgress(ProgressPhase{Phase: PhaseComplete})
	}

	return result, nil
}

// RefreshDatabase refreshes an existing synced database.
// Only processes entries that have changed since the last sync.
func RefreshDatabase(opts RefreshOptions, onProgress ProgressCallback) (*DatabaseFreezeResult, error) {
	// Read existing metadata
	metadata, err := ReadDatabaseMetadata(opts.FolderPath)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}
	if metadata == nil {
		return nil, fmt.Errorf("no %s found in %s. Use 'import' to import the database first", DatabaseMetadataFile, opts.FolderPath)
	}

	databaseID := metadata.DatabaseID

	if onProgress != nil {
		onProgress(ProgressPhase{Phase: PhaseQuerying})
	}

	// Fetch database metadata
	database, err := opts.Client.GetDatabase(databaseID)
	if err != nil {
		return nil, fmt.Errorf("fetch database: %w", err)
	}

	dbTitle := markdown.ConvertRichText(database.Title)
	if dbTitle == "" {
		dbTitle = "Untitled Database"
	}

	// Query all entries — use data_sources API if available, otherwise fall back to classic endpoint
	var entries []notion.Page
	if len(database.DataSources) > 0 {
		dataSourceID := database.DataSources[0].ID
		if err := opts.Client.GetDataSource(dataSourceID); err != nil {
			return nil, fmt.Errorf("verify data source access: %w", err)
		}
		entries, err = opts.Client.QueryAllEntries(dataSourceID)
	} else {
		entries, err = opts.Client.QueryAllEntriesFromDatabase(databaseID)
	}
	if err != nil {
		return nil, fmt.Errorf("query entries: %w", err)
	}

	total := len(entries)
	if onProgress != nil {
		onProgress(ProgressPhase{Phase: PhaseDiffing, Total: total})
	}

	// Scan existing local files
	localFiles, err := scanLocalFiles(opts.FolderPath)
	if err != nil {
		return nil, fmt.Errorf("scan local files: %w", err)
	}

	// Track results
	result := &DatabaseFreezeResult{
		Title:      dbTitle,
		FolderPath: opts.FolderPath,
		Total:      total,
	}

	// Build set of all entry IDs
	allEntryIDs := make(map[string]bool)
	for _, e := range entries {
		allEntryIDs[e.ID] = true
	}

	// Pre-filter: skip entries whose last_edited_time matches stored frontmatter
	var entriesToProcess []notion.Page
	for _, entry := range entries {
		if opts.Force {
			entriesToProcess = append(entriesToProcess, entry)
			continue
		}

		if local, ok := localFiles[entry.ID]; ok {
			if timestampsEqual(local.lastEdited, entry.LastEditedTime) {
				result.Skipped++
				continue
			}
		}
		entriesToProcess = append(entriesToProcess, entry)
	}

	staleCount := len(entriesToProcess)
	if onProgress != nil {
		onProgress(ProgressPhase{Phase: PhaseStaleDetected, Stale: staleCount, Total: total})
	}

	// Process only changed/new entries
	for i, entry := range entriesToProcess {
		if onProgress != nil {
			onProgress(ProgressPhase{Phase: PhaseImporting, Current: i + 1, Total: staleCount, Title: dbTitle})
		}

		pageResult, err := FreezePage(FreezePageOptions{
			Client:       opts.Client,
			NotionID:     entry.ID,
			OutputFolder: opts.FolderPath,
			DatabaseID:   databaseID,
			Page:         &entry,
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

	// Mark deleted entries
	for id, info := range localFiles {
		if !allEntryIDs[id] {
			if err := markAsDeleted(info.filePath); err == nil {
				result.Deleted++
			}
		}
	}

	// Update database metadata
	updatedMetadata := &FrozenDatabase{
		DatabaseID:   databaseID,
		Title:        dbTitle,
		URL:          database.URL,
		FolderPath:   opts.FolderPath,
		LastSyncedAt: time.Now().UTC().Format(time.RFC3339),
		EntryCount:   total,
	}
	if err := WriteDatabaseMetadata(opts.FolderPath, updatedMetadata); err != nil {
		return nil, fmt.Errorf("write metadata: %w", err)
	}

	if onProgress != nil {
		onProgress(ProgressPhase{Phase: PhaseComplete})
	}

	return result, nil
}

type localFileInfo struct {
	filePath   string
	lastEdited string
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

	contentStr := string(content)

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
