# Page-Level Sync

**Issue:** [#31](https://github.com/ran-codes/notion-sync/issues/31) — CDC for standalone Notion pages (not just databases)

## Summary

Extend `import` and `refresh` to transparently handle standalone Notion page IDs alongside database IDs. No new CLI commands — auto-detect whether the provided ID is a database or a page.

## Folder Structure

Standalone pages live under `pages/` in the workspace, each in its own subfolder:

```
<outputFolder>/
  pages/
    <title>_<short-id>/        # short-id = first 8 chars of UUID, no dashes
      _page.json               # metadata (like _database.json for databases)
      <title>.md               # frozen markdown
    <title2>_<short-id2>/
      _page.json
      <title2>.md
  <Database Name>/             # existing database folders unchanged
    _database.json
    *.md
```

## Implementation

### 1. Auto-detect in `import` (`cmd/notion-sync/main.go` → `runImport`)

After normalizing the ID:
1. Try `client.GetDatabase(id)` — if 200, proceed with existing database import
2. If 404 (`*notion.ErrorResponse` with `Status == 404`), try `client.GetPage(id)`
3. If page → call `sync.FreezeStandalonePage()`
4. If both fail → error

Add `IsNotFoundError(err) bool` helper in `internal/notion/client.go`.

### 2. `FreezeStandalonePage()` (`internal/sync/page.go`)

New function:
1. Fetch page via `client.GetPage(id)` to get title
2. Build folder path: `<outputFolder>/pages/<sanitizedTitle>_<shortID>/`
3. Create folder
4. Call existing `FreezePage()` with `DatabaseID: ""` (already supported — page.go:100-102 skips `notion-database-id` when empty)
5. Write `_page.json` metadata
6. Return result

### 3. Page metadata (`internal/sync/metadata.go`)

```go
const PageMetadataFile = "_page.json"

type FrozenPage struct {
    PageID       string `json:"pageId"`
    Title        string `json:"title"`
    URL          string `json:"url"`
    FolderPath   string `json:"folderPath"`
    LastSyncedAt string `json:"lastSyncedAt"`
}
```

Functions:
- `WritePageMetadata(folderPath, *FrozenPage) error`
- `ReadPageMetadata(folderPath) (*FrozenPage, error)` — reads `_page.json`
- `ListSyncedPages(outputFolder) ([]FrozenPage, error)` — scans `pages/*/` for `_page.json`

### 4. Refresh support

New `RefreshStandalonePage()` in `internal/sync/page.go`:
1. Read `_page.json` from folder
2. Call `FreezePage()` with page ID and `Force: opts.Force`
3. Update `_page.json` timestamp

In `runRefresh()` (`main.go`):
- If folder contains `_page.json` → single page refresh
- If folder contains `pages/` subfolders with `_page.json` → refresh all standalone pages
- Otherwise → existing database refresh

### 5. List (`cmd/notion-sync/main.go` → `runList`)

After listing databases, call `ListSyncedPages(outputFolder)` and print:

```
Synced pages in ./notion:

  My Page Title
    Folder:      pages/My Page Title_abc12345/
    Page ID:     abc12345-...
    Last synced: 2026-02-26T...
```

## Files to Modify

| File | Changes |
|------|---------|
| `internal/notion/client.go` | `IsNotFoundError()` helper |
| `internal/sync/types.go` | `FrozenPage` struct |
| `internal/sync/metadata.go` | `WritePageMetadata`, `ReadPageMetadata`, `ListSyncedPages` |
| `internal/sync/page.go` | `FreezeStandalonePage()`, `RefreshStandalonePage()` |
| `cmd/notion-sync/main.go` | Auto-detect in `runImport`, page support in `runRefresh`, pages in `runList`, updated usage text |

## Existing Code to Reuse

- `FreezePage()` in `sync/page.go` — already handles `DatabaseID: ""` (skips database-id frontmatter)
- `notion.NormalizeNotionID()` — works for both page and database IDs/URLs
- `util.SanitizeFileName()` — for folder naming
- `notion.ErrorResponse` with `Status` field — for 404 detection
- `openStoreIfNeeded()` / SQLite store — pass through to `FreezePage` for SQLite support

## Test Infrastructure

### Test page setup

Create a standalone Notion page (not in a database) with mixed block types. Document in `.claude/reference/test-pages/standalone/setup.md`.

### Test skill: `/test-standalone-page`

File: `.claude/skills/test-standalone-page/SKILL.md`

| Step | Action | Pass Criteria |
|------|--------|---------------|
| 0 | Build | `go build ./cmd/notion-sync` succeeds |
| 1 | Clean slate | Delete `test-output/pages/` if exists |
| 2 | Import standalone page | Creates `test-output/pages/<title>_<id>/` with `.md` + `_page.json` |
| 3 | Verify structure | `_page.json` has correct `pageId`; `.md` has `notion-id`, no `notion-database-id` |
| 4 | No-op refresh | `refresh <page-folder>` → skipped = 1 |
| 5 | Force refresh | `refresh <page-folder> --force` → updated = 1 |
| 6 | List | `list ./test-output` → shows the page |
| 7 | SQLite check | Page in `_notion_sync.sqlite` with empty `database_id` |
| 8 | Clean up | Delete `test-output/pages/`, clean SQLite |

### Update `/test` skill

Add standalone page test as a new step between system tests and cross-integration.
