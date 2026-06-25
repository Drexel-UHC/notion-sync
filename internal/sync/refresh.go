package sync

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ran-codes/notion-sync/internal/markdown"
	"github.com/ran-codes/notion-sync/internal/notion"
)

// RefreshOptions contains options for refreshing a database.
type RefreshOptions struct {
	Client         NotionClient
	FolderPath     string
	Force          bool     // Skip timestamp comparison and resync all entries
	PageIDs        []string // If set, only refresh these specific page IDs
	StripPresigned bool     // Strip rotating AWS pre-signed query strings from file URLs
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

	// Multi-source detection: if top-level metadata has no dataSourceId,
	// check if subfolders have their own _database.json with dataSourceId.
	// If so, refresh each subfolder independently.
	if metadata.DataSourceID == "" {
		subSources := findSubSourceFolders(opts.FolderPath)
		if len(subSources) > 0 {
			return refreshMultiSource(opts, subSources, metadata, onProgress)
		}
	}

	workspacePath := filepath.Dir(opts.FolderPath)

	// Ensure AGENTS.md at workspace root: backfill for older imports, and
	// force-upgrade it when this binary is newer than the stamped version.
	if err := syncAgentsMD(workspacePath); err != nil {
		log.Printf("warning: failed to write AGENTS.md: %v", err)
	}

	// Clean up legacy SQLite database files (removed in v0.3)
	removeLegacySQLite(workspacePath)

	// --ids mode: fetch only specific pages, skip full query/diff/delete
	if len(opts.PageIDs) > 0 {
		// Migrate any existing title-based filenames to UUID-based.
		localFiles, _ := scanLocalFiles(opts.FolderPath)
		migrateToUUIDFilenames(opts.FolderPath, localFiles)

		total := len(opts.PageIDs)
		dbTitle := metadata.Title

		result := &DatabaseFreezeResult{
			Title:      dbTitle,
			FolderPath: opts.FolderPath,
			Total:      total,
		}

		if onProgress != nil {
			onProgress(ProgressPhase{Phase: PhaseStaleDetected, Stale: total, Total: total})
		}

		for i, pageID := range opts.PageIDs {
			if onProgress != nil {
				onProgress(ProgressPhase{Phase: PhaseImporting, Current: i + 1, Total: total, Title: dbTitle})
			}

			pageResult, err := FreezePage(FreezePageOptions{
				Client:         opts.Client,
				NotionID:       pageID,
				OutputFolder:   opts.FolderPath,
				DatabaseID:     databaseID,
				Force:          true,
				StripPresigned: opts.StripPresigned,
			})

			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Entry %s: %v", pageID, err))
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

		// Update metadata timestamp but preserve entry count
		metadata.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
		if err := WriteDatabaseMetadata(opts.FolderPath, metadata); err != nil {
			return nil, fmt.Errorf("write metadata: %w", err)
		}

		if onProgress != nil {
			onProgress(ProgressPhase{Phase: PhaseComplete})
		}

		return result, nil
	}

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

	// Determine which data source to query.
	dataSourceID := metadata.DataSourceID
	if dataSourceID == "" && len(database.DataSources) > 0 {
		dataSourceID = database.DataSources[0].ID
	}
	if dataSourceID == "" {
		return nil, fmt.Errorf("database has no data sources; ensure you are using Notion API version 2025-09-03 or later")
	}
	if _, err := opts.Client.GetDataSource(dataSourceID); err != nil {
		return nil, fmt.Errorf("verify data source access: %w", err)
	}
	entries, err := opts.Client.QueryAllEntries(dataSourceID)
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

	// Migrate title-based filenames to UUID-based filenames (one-time).
	migrateToUUIDFilenames(opts.FolderPath, localFiles)

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
			Client:         opts.Client,
			NotionID:       entry.ID,
			OutputFolder:   opts.FolderPath,
			DatabaseID:     databaseID,
			Page:           &entry,
			Force:          opts.Force,
			StripPresigned: opts.StripPresigned,
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

	// Mark deleted entries (from filesystem scan)
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
		DataSourceID: dataSourceID,
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

// findSubSourceFolders scans for subfolders with _database.json that have a dataSourceId.
func findSubSourceFolders(folderPath string) []string {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil
	}
	var folders []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subPath := filepath.Join(folderPath, entry.Name())
		meta, err := ReadDatabaseMetadata(subPath)
		if err != nil || meta == nil {
			continue
		}
		if meta.DataSourceID != "" {
			folders = append(folders, subPath)
		}
	}
	return folders
}

// refreshMultiSource refreshes each sub-source folder independently and aggregates results.
func refreshMultiSource(opts RefreshOptions, subFolders []string, parentMeta *FrozenDatabase, onProgress ProgressCallback) (*DatabaseFreezeResult, error) {
	result := &DatabaseFreezeResult{
		Title:      parentMeta.Title,
		FolderPath: opts.FolderPath,
	}

	for _, subFolder := range subFolders {
		subOpts := RefreshOptions{
			Client:         opts.Client,
			FolderPath:     subFolder,
			Force:          opts.Force,
			PageIDs:        opts.PageIDs,
			StripPresigned: opts.StripPresigned,
		}
		subResult, err := RefreshDatabase(subOpts, onProgress)
		if err != nil {
			return nil, fmt.Errorf("refresh %s: %w", filepath.Base(subFolder), err)
		}
		result.Total += subResult.Total
		result.Created += subResult.Created
		result.Updated += subResult.Updated
		result.Skipped += subResult.Skipped
		result.Deleted += subResult.Deleted
		result.Failed += subResult.Failed
		result.Errors = append(result.Errors, subResult.Errors...)
	}

	// Update top-level metadata timestamp
	parentMeta.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
	parentMeta.EntryCount = result.Total
	if err := WriteDatabaseMetadata(opts.FolderPath, parentMeta); err != nil {
		return nil, fmt.Errorf("write metadata: %w", err)
	}

	return result, nil
}
