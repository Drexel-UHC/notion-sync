package sync

import (
	"os"
	"path/filepath"
)

// agentsMDContent is written to the workspace root as AGENTS.md so that any
// downstream LLM/agent that lands in a notion-sync output folder understands
// the layout, conventions, and gotchas of the synced data.
//
// AGENTS.md is the cross-vendor convention (Cursor, OpenAI's agents.md spec,
// and others) for provider-neutral agent instructions. Claude Code reads it
// alongside CLAUDE.md.
const agentsMDContent = `# notion-sync workspace

This folder contains data synced from Notion using [notion-sync](https://github.com/ran-codes/notion-sync).
This file (AGENTS.md) tells downstream LLM/agent tools how to interpret the contents.

## What is here

- One subfolder per synced Notion **database**, containing markdown files (one per page) and a ` + "`_database.json`" + ` metadata file.
- A ` + "`pages/`" + ` subfolder containing one folder per synced **standalone page** (page imported by URL, not part of a database). Each has a ` + "`_page.json`" + ` and one ` + "`.md`" + ` file.

` + "```" + `
<workspace-root>/
├── AGENTS.md                      <-- this file
├── <Database Title>/
│   ├── _database.json
│   └── <notion-id>.md             (one per database entry)
├── <Multi-Source Database>/
│   ├── _database.json             (top-level, no dataSourceId)
│   ├── <Source 1 Title>/
│   │   ├── _database.json         (with dataSourceId)
│   │   └── <notion-id>.md
│   └── <Source 2 Title>/
│       └── ...
└── pages/
    └── <Page Title>_<short-id>/
        ├── _page.json
        └── <Page Title>.md
` + "```" + `

## Filenames

- **Database entries**: ` + "`{notion-id}.md`" + ` (UUID-based). Stable — renaming the page in Notion does not change the local filename. The page title lives in the ` + "`title`" + ` (or named title property) of the frontmatter.
- **Standalone pages**: title-based filename inside a folder named ` + "`<sanitized-title>_<8-char-id>/`" + `.

## Frontmatter format

Every ` + "`.md`" + ` file starts with YAML frontmatter. The first block is always notion-sync metadata; everything after is the Notion page's properties verbatim.

` + "```" + `yaml
---
notion-id: "<page-uuid>"
notion-url: "https://www.notion.so/..."
notion-frozen-at: "<RFC 3339 — when this file was last written>"
notion-last-edited: "<RFC 3339 — Notion's last_edited_time>"
notion-database-id: "<database-uuid>"   # only present for database entries
# notion-deleted: true                  # only present if the entry was removed in Notion (soft delete)

# ... all Notion properties below ...
Title Property: "Page Name"
Status: "In Progress"
Tags: [a, b, c]
PDF:
  - "https://prod-files-secure.s3.us-west-2.amazonaws.com/<bucket>/<uuid>/file.pdf"
---
` + "```" + `

### Property → frontmatter mapping

| Notion type                     | Frontmatter value                          |
| ------------------------------- | ------------------------------------------ |
| ` + "`title`" + `                         | filename (database entries) or ` + "`title`" + ` key (pages) |
| ` + "`rich_text`" + `                     | plain markdown string                      |
| ` + "`number`" + `                        | number or ` + "`null`" + `                            |
| ` + "`select`" + `                        | option name or ` + "`null`" + `                       |
| ` + "`multi_select`" + `                  | array of option names                      |
| ` + "`status`" + `                        | status name or ` + "`null`" + `                       |
| ` + "`date`" + `                          | ` + "`start`" + ` or ` + "`start → end`" + `                     |
| ` + "`checkbox`" + `                      | ` + "`true`" + ` or ` + "`false`" + `                            |
| ` + "`url`" + ` / ` + "`email`" + ` / ` + "`phone_number`" + ` | string or ` + "`null`" + `                            |
| ` + "`relation`" + `                      | array of page IDs                          |
| ` + "`people`" + `                        | array of names (or IDs as fallback)        |
| ` + "`files`" + `                         | array of URLs (see "File URLs" below)      |
| ` + "`created_time`" + ` / ` + "`last_edited_time`" + ` | RFC 3339 timestamp                |
| ` + "`unique_id`" + `                     | ` + "`PREFIX-N`" + ` or ` + "`N`" + `                            |
| ` + "`created_by`" + ` / ` + "`last_edited_by`" + ` | user name (or ID as fallback)        |

Skipped (not in frontmatter): ` + "`formula`" + `, ` + "`rollup`" + `, ` + "`button`" + `, ` + "`verification`" + ` — they're computed or non-portable.

## File URLs (important for downstream consumers)

URLs in ` + "`files`" + ` properties and in markdown body image/PDF/video/file/audio embeds may have had their **AWS S3 pre-signed query string stripped**:

- **Original** (from Notion API): ` + "`https://prod-files-secure.s3.us-west-2.amazonaws.com/<bucket>/<uuid>/file.pdf?X-Amz-Algorithm=...&X-Amz-Signature=...&X-Amz-Date=...`" + `
- **In this snapshot** (default behavior): ` + "`https://prod-files-secure.s3.us-west-2.amazonaws.com/<bucket>/<uuid>/file.pdf`" + `

Why: Notion rotates the ` + "`X-Amz-Signature`" + ` query string every hour. Without stripping, every refresh produces a giant noisy diff even when nothing actually changed.

What this means for you:

- The path (including the file UUID and filename) **is stable** — use it as the file's identifier.
- The stripped URL **will not return file bytes** if you GET it directly — the auth params have been removed and AWS rejects unsigned requests to ` + "`prod-files-secure`" + `.
- To fetch the actual bytes, re-query the Notion API for the parent page and use the freshly-signed URL it returns.
- If a snapshot was produced with ` + "`--keep-presigned-params`" + `, URLs include the auth string but the signature is **already expired** (1-hour TTL).

External URLs (set by users in Notion as "external" file references, not uploaded into Notion) are never stripped — they pass through verbatim.

## Soft deletes

Pages removed from Notion are **not** deleted from disk on refresh. Instead, ` + "`notion-deleted: true`" + ` is added to the frontmatter. Treat any file with that key as historical.

## Metadata files

### ` + "`_database.json`" + ` (per database folder)

` + "```" + `json
{
  "databaseId": "<uuid>",
  "dataSourceId": "<uuid>",
  "title": "Human Title",
  "url": "https://www.notion.so/...",
  "folderPath": "<absolute path>",
  "lastSyncedAt": "<RFC 3339>",
  "entryCount": 42,
  "syncVersion": "v0.5.0"
}
` + "```" + `

For multi-source databases, the **top-level** ` + "`_database.json`" + ` has no ` + "`dataSourceId`" + `; each per-source subfolder has its own with ` + "`dataSourceId`" + ` set. ` + "`entryCount`" + ` at the top level is the total across all sources.

### ` + "`_page.json`" + ` (per standalone-page folder)

` + "```" + `json
{
  "pageId": "<uuid>",
  "title": "Page Title",
  "url": "https://www.notion.so/...",
  "folderPath": "<absolute path>",
  "lastSyncedAt": "<RFC 3339>",
  "syncVersion": "v0.5.0"
}
` + "```" + `

## Refresh semantics (helpful when reasoning about diffs)

- Default ` + "`refresh`" + ` is incremental: entries whose ` + "`notion-last-edited`" + ` matches the local copy are skipped.
- ` + "`refresh --force`" + ` resyncs every entry regardless of timestamp.
- ` + "`refresh --ids id1,id2`" + ` resyncs specific pages by ID.
- ` + "`clean <folder>`" + ` strips presigned URLs from existing files **without** any API call — used as a one-time backfill after upgrading.
`

// WriteAgentsMD writes the generated AGENTS.md to the workspace root.
// It only writes if the file doesn't already exist (preserves user edits).
func WriteAgentsMD(workspacePath string) error {
	dest := filepath.Join(workspacePath, "AGENTS.md")
	if _, err := os.Stat(dest); err == nil {
		return nil // file exists, don't overwrite
	}
	return os.WriteFile(dest, []byte(agentsMDContent), 0644)
}
