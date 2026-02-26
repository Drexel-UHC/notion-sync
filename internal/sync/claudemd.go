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
- ` + "`_database.json`" + ` — metadata (databaseId, title, url, lastSyncedAt, entryCount)

**Multi-source databases** (e.g. Notion databases with multiple linked sources) have an extra level of subfolders, one per data source.

## SQLite store

The file ` + "`_notion_sync.sqlite`" + ` at the workspace root is a structured mirror of all synced pages. Use it for fast search and querying.

### Schema

` + "```" + `sql
-- Metadata
_meta (key TEXT PK, value TEXT)

-- All synced pages
pages (
  id             TEXT PK,     -- Notion page UUID
  title          TEXT,
  url            TEXT,         -- Notion URL
  file_path      TEXT,         -- path to .md file (relative)
  body_markdown  TEXT,         -- full page content as Markdown
  properties_json TEXT,        -- JSON object of all Notion properties
  created_time   TEXT,
  last_edited_time TEXT,
  frozen_at      TEXT,         -- when notion-sync last wrote this row
  deleted        INTEGER,      -- 1 = soft-deleted (removed from Notion)
  database_id    TEXT
)

-- Full-text search (FTS5) over title + body_markdown
pages_fts (title, body_markdown)
` + "```" + `

### Progressive disclosure (recommended query pattern)

1. **FTS search** — find pages by keyword:
` + "```" + `sql
SELECT p.id, p.title, p.url
FROM pages_fts fts
JOIN pages p ON p.rowid = fts.rowid
WHERE pages_fts MATCH 'search terms'
  AND p.deleted = 0;
` + "```" + `

2. **Properties** — inspect structured metadata:
` + "```" + `sql
SELECT id, title, properties_json FROM pages
WHERE database_id = '<db-id>' AND deleted = 0;
` + "```" + `

3. **Full content** — read the Markdown body:
` + "```" + `sql
SELECT body_markdown FROM pages WHERE id = '<page-id>';
` + "```" + `

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

Pages removed from Notion are not deleted from disk. Instead:
- SQLite: ` + "`deleted = 1`" + `
- Markdown: ` + "`notion-deleted: true`" + ` added to frontmatter

Filter them out with ` + "`WHERE deleted = 0`" + ` in SQL or by checking frontmatter.
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
