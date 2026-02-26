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

### 1. Test page in Notion
- Page exists at: `31357008-e885-80c3-90f4-d148f0854bba`
- URL: https://www.notion.so/drexel-climate/Test-Notion-sync-single-page-31357008e88580c390f4d148f0854bba
- **Needs**: Connect the Notion integration to this page (share with it), then add some test content (headings, paragraphs, code block, list)
- Use Notion MCP tools to add content

### 2. End-to-end test
- After integration is connected, run: `./notion-sync.exe import 31357008-e885-80c3-90f4-d148f0854bba --output ./test-output`
- Verify folder structure: `test-output/pages/<title>_31357008/` with `_page.json` + `<title>.md`
- Test refresh: `./notion-sync.exe refresh test-output/pages/<folder>`
- Test list: `./notion-sync.exe list ./test-output`

### 3. Reference doc
- Create `.claude/reference/test-pages/standalone/setup.md` documenting the test page

### 4. Test skill
- Create `.claude/skills/test-standalone-page/SKILL.md`
- Update `.claude/skills/test/SKILL.md` to include standalone page test step

### 5. Possible issue: IsNotFoundError false positives
- Notion returns 401 "API token is invalid" both for wrong-type queries AND genuinely invalid tokens
- Current code treats 401 as "not a database, try as page" — if the token is truly invalid, the page fetch also fails with a clear error, so this is safe
- But worth monitoring if edge cases appear
