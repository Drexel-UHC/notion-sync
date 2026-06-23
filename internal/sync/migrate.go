package sync

import (
	"log"
	"os"
	"path/filepath"
)

// This file holds one-off, backward-compatibility migrations that run during
// import/refresh. They are not part of the core sync flow and exist only to
// clean up artifacts from older versions; each is safe to delete once no
// workspace in the wild predates the change it handles.

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
