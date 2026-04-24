package sync

import (
	"os"
	"path/filepath"
)

const claudeMDContent = `# notion-sync workspace

This folder contains data synced from Notion using [notion-sync](https://github.com/ran-codes/notion-sync).

## Folder structure

Each synced Notion database gets its own subfolder containing:
- Markdown files (one per page) with YAML frontmatter
- ` + "`_database.json`" + ` — metadata (databaseId, title, url, lastSyncedAt, entryCount, syncVersion)

**Multi-source databases** (e.g. Notion databases with multiple linked sources) have an extra level of subfolders, one per data source.

## Filenames

Database entries use UUID-based filenames (` + "`{notion-id}.md`" + `). This makes filenames stable — renaming a page in Notion does not change the local filename. The page title is available in frontmatter.

Standalone pages (imported via ` + "`notion-sync import --page`" + `) use title-based filenames.

## Frontmatter format

Each .md file starts with YAML frontmatter:

` + "```" + `yaml
---
notion-id: "<page-uuid>"
notion-last-edited: "<ISO 8601>"
notion-url: "<url>"
# ... all Notion properties as key-value pairs ...
---
` + "```" + `

## Soft deletes

Pages removed from Notion are not deleted from disk. Instead, ` + "`notion-deleted: true`" + ` is added to frontmatter.

## Migration notes

**Title-based → UUID filenames (v0.4+):** Older versions used title-based filenames (e.g. ` + "`My Page.md`" + `). Migration to UUID filenames is automatic — just run ` + "`notion-sync refresh <folder>`" + ` and existing files are renamed in place. No manual steps required.
`

// WriteClaudeMD writes a generic CLAUDE.md to the workspace root.
// It only writes if the file doesn't already exist (preserves user edits).
func WriteClaudeMD(workspacePath string) error {
	dest := filepath.Join(workspacePath, "CLAUDE.md")
	if _, err := os.Stat(dest); err == nil {
		return nil // file exists, don't overwrite
	}
	return os.WriteFile(dest, []byte(claudeMDContent), 0644)
}
