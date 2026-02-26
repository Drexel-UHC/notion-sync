# Page-Level Sync — Implementation Notes

## What's Done

### Code changes (all compiling, all tests passing)

1. **`internal/notion/client.go`** — Added `IsNotFoundError(err) bool`
   - Checks for 404, `object_not_found` code, and 401 "API token is invalid" (Notion returns this when querying a page ID against `/databases/`)

2. **`internal/sync/types.go`** — Added `FrozenPage` struct and `FolderPath` field to `PageFreezeResult`

3. **`internal/sync/metadata.go`** — Added:
   - `PageMetadataFile = "_page.json"`
   - `ReadPageMetadata()`, `WritePageMetadata()`, `ListSyncedPages()`

4. **`internal/sync/page.go`** — Added:
   - `StandalonePageImportOptions` struct
   - `FreezeStandalonePage()` — imports a standalone page into `pages/<title>_<shortID>/`
   - `RefreshStandalonePageOptions` struct
   - `RefreshStandalonePage()` — refreshes a previously imported standalone page

5. **`cmd/notion-sync/main.go`** — Updated:
   - `runImport()` — auto-detects database vs page (tries `GetDatabase` first, falls back to `GetPage` on not-found)
   - `runImportPage()` — new helper for standalone page import CLI output
   - `runRefresh()` — checks for `_page.json` before database refresh, routes to `runRefreshPage()`
   - `runRefreshPage()` — new helper for standalone page refresh CLI output
   - `runList()` — now also calls `ListSyncedPages()` and prints pages section
   - Usage text updated to reflect database-or-page support

## What's Left

All items complete. Feature is ready for merge.

### Completed items
1. Test page connected to integration and populated with content (h1-h3, rich text, lists, code, quote, divider)
2. E2E test passed: import, no-op refresh, force refresh, list, SQLite — all verified
3. Reference doc created: `.claude/reference/test-pages/standalone/setup.md`
4. Test skill created: `/test-standalone-page`
5. `/test` skill updated with standalone page step (Step 4)
6. API key validation added: `config.ValidateAPIKey()` checks length and prefix on all commands + `config set`
7. `config get` command added: shows all config with masked API key

### Note: IsNotFoundError false positives
- Notion returns 401 "API token is invalid" both for wrong-type queries AND genuinely invalid tokens
- Current code treats 401 as "not a database, try as page" — if the token is truly invalid, the page fetch also fails with a clear error, so this is safe
- API key validation (added in this session) catches most bad-key scenarios before any API call is made
