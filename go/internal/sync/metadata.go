package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const DatabaseMetadataFile = "_database.json"

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
