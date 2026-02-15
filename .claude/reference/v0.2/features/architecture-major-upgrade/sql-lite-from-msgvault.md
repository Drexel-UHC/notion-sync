# Phase 1: Add SQLite Output to notion-sync

## Context

We evaluated msgvault's features (search, stats, MCP, TUI) and identified that they all depend on having a queryable SQLite store. This phase adds SQLite as a second output alongside existing markdown files — the foundation for all future features. Markdown output is completely unchanged; SQLite is additive.

## Result

```
notion/Project Tracker/
├── _database.json          # unchanged
├── _notion_sync.db         # NEW
├── Auth system redesign.md # unchanged
└── ...
```

## New Package: `internal/store/store.go`

Pure Go SQLite via `modernc.org/sqlite` (no CGO, FTS5 built-in).

**Schema:**
```sql
-- Version tracking for future migrations
CREATE TABLE IF NOT EXISTS _meta (
    key TEXT PRIMARY KEY,
    value TEXT
);
INSERT OR IGNORE INTO _meta (key, value) VALUES ('schema_version', '1');

-- Main pages table
CREATE TABLE IF NOT EXISTS pages (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    url TEXT NOT NULL,
    body_markdown TEXT NOT NULL DEFAULT '',
    properties_json TEXT NOT NULL DEFAULT '{}',
    created_time TEXT,
    last_edited_time TEXT NOT NULL,
    frozen_at TEXT NOT NULL,
    deleted INTEGER DEFAULT 0,
    database_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_pages_database_id ON pages(database_id);
CREATE INDEX IF NOT EXISTS idx_pages_last_edited ON pages(last_edited_time);

-- FTS5 for full-text search (content-sync mode)
CREATE VIRTUAL TABLE IF NOT EXISTS pages_fts USING fts5(
    title, body_markdown, content=pages, content_rowid=rowid
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS pages_ai AFTER INSERT ON pages BEGIN
    INSERT INTO pages_fts(rowid, title, body_markdown)
    VALUES (new.rowid, new.title, new.body_markdown);
END;

CREATE TRIGGER IF NOT EXISTS pages_ad AFTER DELETE ON pages BEGIN
    INSERT INTO pages_fts(pages_fts, rowid, title, body_markdown)
    VALUES('delete', old.rowid, old.title, old.body_markdown);
END;

CREATE TRIGGER IF NOT EXISTS pages_au AFTER UPDATE ON pages BEGIN
    INSERT INTO pages_fts(pages_fts, rowid, title, body_markdown)
    VALUES('delete', old.rowid, old.title, old.body_markdown);
    INSERT INTO pages_fts(rowid, title, body_markdown)
    VALUES (new.rowid, new.title, new.body_markdown);
END;
```

**Functions:**
- `OpenStore(folderPath string) (*Store, error)` — open/create DB, run schema init, enable WAL mode
- `Close() error`
- `UpsertPage(data PageData) error` — INSERT OR REPLACE with ON CONFLICT
- `MarkDeleted(pageID string) error` — sets `deleted = 1`
- `SerializeProperties(fm map[string]interface{}) (string, error)` — helper to JSON-encode the frontmatter map

**PageData struct:**
```go
type PageData struct {
    ID             string
    Title          string
    URL            string
    BodyMarkdown   string
    PropertiesJSON string
    CreatedTime    string
    LastEditedTime string
    FrozenAt       string
    DatabaseID     string
}
```

## Changes to Existing Files

### `internal/sync/types.go`
- Add `OutputMode` type: `"both"` (default) | `"markdown"` | `"sqlite"`
- Add `OutputMode` field to `DatabaseImportOptions`
- Add `OutputMode` field to `RefreshOptions`

### `internal/sync/page.go`
- Add `SQLStore *store.Store` field to `FreezePageOptions`
- After line 106 (after writing `.md` file), add SQLite write block:
  - Serialize `fm` map to JSON via `store.SerializeProperties(fm)`
  - Build `store.PageData` from available vars: `opts.NotionID`, `title`, `page.URL`, `md`, `page.LastEditedTime`, `page.CreatedTime`, `opts.DatabaseID`
  - Call `opts.SQLStore.UpsertPage(data)`
  - **On error: log warning to stderr, do NOT fail** — markdown sync continues unaffected
- When `OutputMode == "sqlite"` (no markdown), skip the `os.WriteFile` call

### `internal/sync/database.go`
- In `FreshDatabaseImport()`: after folder creation (line 52), open store if mode is `"both"` or `"sqlite"`. Pass store to each `FreezePage()` call. Defer `store.Close()`. If store open fails, warn and fall back to markdown-only.
- In `RefreshDatabase()`: same pattern — open store early, pass to FreezePage calls, close at end. Also call `store.MarkDeleted()` in the deleted entries loop (line 312-318).
- In `RefreshDatabase()` `--ids` mode (line 148-202): same — open store, pass through.

### `internal/config/config.go`
- Add `OutputMode string` field to `Config` struct
- Default to `"both"` in `DefaultConfig()`
- Load from config file if present
- Add `"outputMode"` to valid keys in `SaveConfig()`

### `cmd/notion-sync/main.go`
- Add `--output-mode` flag to `runImport()` and `runRefresh()` flag sets
- Resolve: flag > config > default (`"both"`)
- Pass to `DatabaseImportOptions.OutputMode` / `RefreshOptions.OutputMode`
- Update usage string to document the flag
- Add `"outputMode"` to valid config keys in `runConfig()`

## Error Handling

SQLite errors are **warnings only** — they never block markdown sync:
- Store open failure → warn, fall back to markdown-only
- Page upsert failure → warn per page, continue
- Mark-deleted failure → warn, continue

## Dependencies

Add to `go.mod`:
```
modernc.org/sqlite  (latest, currently v1.36.0)
```

This is a pure Go package — no CGO, no C compiler needed. Works on all platforms (Windows, macOS, Linux) and architectures already in the release matrix.

## File Summary

| File | Action |
|------|--------|
| `internal/store/store.go` | **NEW** — Store struct, OpenStore, UpsertPage, MarkDeleted, schema |
| `internal/sync/types.go` | **EDIT** — add OutputMode type + fields on option structs |
| `internal/sync/page.go` | **EDIT** — add SQLStore to FreezePageOptions, write to DB after md |
| `internal/sync/database.go` | **EDIT** — open/close store, pass to FreezePage, mark deleted |
| `internal/config/config.go` | **EDIT** — add OutputMode to Config |
| `cmd/notion-sync/main.go` | **EDIT** — add --output-mode flag, pass through |

## Verification

1. `go build ./cmd/notion-sync` — compiles cleanly
2. `go test ./...` — existing tests pass (no behavior change for markdown)
3. `go test ./internal/store/` — new store tests pass (unit test UpsertPage, MarkDeleted, schema init)
4. Manual: `notion-sync import <db-id>` → verify `_notion_sync.db` appears alongside `.md` files
5. Manual: open DB with `sqlite3 _notion_sync.db` → verify pages table has rows, FTS works: `SELECT * FROM pages_fts WHERE pages_fts MATCH 'search term'`
6. Manual: `notion-sync import <db-id> --output-mode markdown` → verify NO `.db` file created
