# Phase 1: Workspace-Level SQLite Store

## Context

Two architecture upgrades are planned: SQLite queryable output (foundation for search/stats/MCP/TUI) and SyncSession multi-DB orchestrator (relation resolution, child page recursion, related DB auto-import). Analysis showed these share the same backbone — the SQLite `pages` table IS the cross-database PageRegistry that SyncSession needs. Building SQLite workspace-level from day 1 avoids a migration later.

See: `sqlite-syncsession-synergy.md` for full analysis.

## Verified Technical Assumptions

- **modernc.org/sqlite v1.45.0** (latest, Feb 2026): Pure Go, no CGO. FTS5 + JSON1 built-in by default. WAL works on Windows.
- **FTS5 trigger caveat**: `PRAGMA recursive_triggers = 1` is REQUIRED when using `INSERT OR REPLACE`, otherwise the AFTER DELETE trigger doesn't fire on the implicit delete, and FTS gets out of sync. Must set this in `OpenStore()`.
- **Notion API relations**: Target database ID is available in database schema (`GET /databases/{id}`) as `relation.data_source_id` on the property definition. Page-level properties only contain target page IDs. Our `Database` struct currently doesn't parse property schemas — needs adding in a later phase.
- **types.go already has `UniqueID`, `CreatedBy`, `LastEditedBy` structs** (lines 60-62, 66-69, 90-94). Issue #14 (easy properties) only needs new cases in `mapPropertiesToFrontmatter()`.

## Result

```
notion/                          <- OutputFolder (workspace root)
├── _notion_sync.db              <- NEW: workspace-level SQLite DB
├── Project Tracker/
│   ├── _database.json           <- unchanged
│   ├── Auth system redesign.md  <- unchanged
│   └── ...
├── Another Database/
│   ├── _database.json
│   └── ...
```

The DB lives at the workspace root (OutputFolder), not inside each database folder. The `database_id` column partitions data per database. This supports future cross-database queries for relation resolution.

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
    file_path TEXT,
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

**Critical init pragmas:**
```go
func OpenStore(workspacePath string) (*Store, error) {
    // ...open DB at workspacePath/_notion_sync.db...
    db.Exec("PRAGMA journal_mode = WAL")
    db.Exec("PRAGMA recursive_triggers = 1")  // REQUIRED for FTS5 + INSERT OR REPLACE
    // ...run schema...
}
```

**Functions:**
- `OpenStore(workspacePath string) (*Store, error)` — open/create DB at workspace root, run schema, enable WAL + recursive_triggers
- `Close() error`
- `UpsertPage(data PageData) error` — INSERT OR REPLACE
- `MarkDeleted(pageID string) error` — sets `deleted = 1`
- `SerializeProperties(fm map[string]interface{}) (string, error)` — JSON-encode frontmatter map

**PageData struct:**
```go
type PageData struct {
    ID             string
    Title          string
    URL            string
    FilePath       string  // NEW: for future link resolution
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
  - Build `store.PageData` from available vars: `opts.NotionID`, `title`, `filePath`, `page.URL`, `md`, `page.LastEditedTime`, `page.CreatedTime`, `opts.DatabaseID`
  - Call `opts.SQLStore.UpsertPage(data)`
  - **On error: log warning to stderr, do NOT fail** — markdown sync continues unaffected
- When `OutputMode == "sqlite"` (no markdown), skip the `os.WriteFile` call

### `internal/sync/database.go`
- In `FreshDatabaseImport()`: after folder creation (line 52), open store at **parent OutputFolder** (not database subfolder) if mode is `"both"` or `"sqlite"`. Pass store to each `FreezePage()` call. Defer `store.Close()`. If store open fails, warn and fall back to markdown-only.
- In `RefreshDatabase()`: same pattern — open store at parent of FolderPath, pass to FreezePage calls, close at end. Also call `store.MarkDeleted()` in the deleted entries loop (line 312-318).
- In `RefreshDatabase()` `--ids` mode (line 148-202): same — open store, pass through.

### `internal/config/config.go`
- Add `OutputMode string` field to `Config` struct
- Default to `"both"` in `DefaultConfig()`

### `cmd/notion-sync/main.go`
- Add `--output-mode` flag to `runImport()` and `runRefresh()` flag sets
- Resolve: flag > config > default (`"both"`)
- Pass to `DatabaseImportOptions.OutputMode` / `RefreshOptions.OutputMode`

## Error Handling

SQLite errors are **warnings only** — they never block markdown sync:
- Store open failure → warn, fall back to markdown-only
- Page upsert failure → warn per page, continue
- Mark-deleted failure → warn, continue

## Dependencies

Add to `go.mod`:
```
modernc.org/sqlite  v1.45.0
```

Pure Go — no CGO, no C compiler. Works on all platforms and architectures already in the release matrix.

## File Summary

| File | Action |
|------|--------|
| `internal/store/store.go` | **NEW** — Store struct, OpenStore, UpsertPage, MarkDeleted, schema |
| `internal/sync/types.go` | **EDIT** — add OutputMode type + fields on option structs |
| `internal/sync/page.go` | **EDIT** — add SQLStore to FreezePageOptions, write to DB after md |
| `internal/sync/database.go` | **EDIT** — open/close store at workspace root, pass to FreezePage, mark deleted |
| `internal/config/config.go` | **EDIT** — add OutputMode to Config |
| `cmd/notion-sync/main.go` | **EDIT** — add --output-mode flag, pass through |

## Design Decisions & Rationale

1. **Workspace-level DB** (not per-database): Enables cross-database queries for future relation resolution. The `database_id` column partitions data. `OutputFolder` is already the workspace root in current code.
2. **`file_path` column included now**: Costs nothing to add, avoids schema migration when SyncSession PostProcess needs it.
3. **`PRAGMA recursive_triggers = 1`**: Without this, `INSERT OR REPLACE` (which is DELETE + INSERT internally) won't fire the AFTER DELETE trigger, causing FTS index to go stale. This is a confirmed SQLite behavior, not a modernc.org bug.
4. **`_database.json` kept**: Not replaced yet. SQLite is additive. `_database.json` is still the source of truth for refresh. A future phase can add a `databases` table and migrate.

## Verification

1. `go build ./cmd/notion-sync` — compiles cleanly
2. `go test ./...` — existing tests pass (no behavior change for markdown)
3. `go test ./internal/store/` — new store tests pass (unit test UpsertPage, MarkDeleted, schema init, FTS sync)
4. Manual: `notion-sync import <db-id>` → verify `_notion_sync.db` appears at workspace root alongside database folders
5. Manual: open DB with `sqlite3 _notion_sync.db` → verify pages table has rows with `file_path` populated, FTS works: `SELECT * FROM pages_fts WHERE pages_fts MATCH 'search term'`
6. Manual: `notion-sync import <db-id> --output-mode markdown` → verify NO `.db` file created
7. Manual: import two different databases to same output folder → verify both appear in single `_notion_sync.db` with different `database_id` values
