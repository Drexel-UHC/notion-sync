package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const DatabaseMetadataFile = "_database.json"
const PageMetadataFile = "_page.json"

// ReadDatabaseMetadata reads _database.json from a folder.
// Returns nil if the file doesn't exist.
func ReadDatabaseMetadata(folderPath string) (*FrozenDatabase, error) {
	metaPath := filepath.Join(folderPath, DatabaseMetadataFile)

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var metadata FrozenDatabase
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// WriteDatabaseMetadata writes _database.json to a folder.
func WriteDatabaseMetadata(folderPath string, metadata *FrozenDatabase) error {
	metaPath := filepath.Join(folderPath, DatabaseMetadataFile)

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaPath, data, 0644)
}

// ReadPageMetadata reads _page.json from a folder.
// Returns nil if the file doesn't exist.
func ReadPageMetadata(folderPath string) (*FrozenPage, error) {
	metaPath := filepath.Join(folderPath, PageMetadataFile)

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var metadata FrozenPage
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// WritePageMetadata writes _page.json to a folder.
func WritePageMetadata(folderPath string, metadata *FrozenPage) error {
	metaPath := filepath.Join(folderPath, PageMetadataFile)

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaPath, data, 0644)
}

// ListSyncedPages scans the pages/ subdirectory for folders containing _page.json.
func ListSyncedPages(outputFolder string) ([]FrozenPage, error) {
	var pages []FrozenPage

	pagesDir := filepath.Join(outputFolder, "pages")
	entries, err := os.ReadDir(pagesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return pages, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		folderPath := filepath.Join(pagesDir, entry.Name())
		metadata, err := ReadPageMetadata(folderPath)
		if err != nil {
			continue
		}
		if metadata != nil {
			pages = append(pages, *metadata)
		}
	}

	return pages, nil
}

// ListSyncedDatabases scans a folder for subdirectories containing _database.json.
func ListSyncedDatabases(outputFolder string) ([]FrozenDatabase, error) {
	var databases []FrozenDatabase

	entries, err := os.ReadDir(outputFolder)
	if err != nil {
		if os.IsNotExist(err) {
			return databases, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		folderPath := filepath.Join(outputFolder, entry.Name())
		metadata, err := ReadDatabaseMetadata(folderPath)
		if err != nil {
			continue // Skip folders with invalid metadata
		}
		if metadata != nil {
			databases = append(databases, *metadata)
		}
	}

	return databases, nil
}
