# Testing Overhaul Plan (v0.2)

## Context

The codebase has strong unit tests (178 tests) and system tests (2 Claude skills hitting real Notion API), but **no integration tests** — the middle layer that tests orchestration logic with mocked API and real filesystem. This plan fills that gap and adds the unified `/test` skill.

---

## Phase 1: Extract Interfaces (Prod Refactor — Prerequisite)

One safe, additive refactor: define interfaces so sync functions accept mocks in tests.

### 1a. New file: `internal/sync/client.go`

```go
type NotionClient interface {
    GetDatabase(databaseID string) (*notion.Database, error)
    GetDataSource(dataSourceID string) (*notion.DataSourceDetail, error)
    QueryAllEntries(dataSourceID string) ([]notion.Page, error)
    GetPage(pageID string) (*notion.Page, error)
    FetchAllBlocks(blockID string) ([]notion.Block, error)
}
```

### 1b. Update 5 refs in `internal/sync/`

| File | Field/Param | Change |
|------|-------------|--------|
| `database.go:21` | `DatabaseImportOptions.Client` | `*notion.Client` → `NotionClient` |
| `database.go:29` | `RefreshOptions.Client` | `*notion.Client` → `NotionClient` |
| `database.go:67` | `resolveDataSources()` param | `*notion.Client` → `NotionClient` |
| `database.go:200` | `importEntries()` param | `*notion.Client` → `NotionClient` |
| `page.go:19` | `FreezePageOptions.Client` | `*notion.Client` → `NotionClient` |

### 1c. New interface in `internal/markdown/converter.go`

```go
type BlockFetcher interface {
    FetchAllBlocks(blockID string) ([]notion.Block, error)
}
```

Change `ConvertContext.Client` from `*notion.Client` → `BlockFetcher` (only method used is `FetchAllBlocks`).

### 1d. Verify: `go build ./cmd/notion-sync && go test ./...` — all 178 tests pass unchanged.

---

## Phase 2: Mock Client (Test Infrastructure)

### New file: `internal/sync/mock_client_test.go`

Map-based mock implementing `NotionClient`. Returns pre-configured data by ID. Follows pattern from `store_test.go`'s `setupTestStore(t)`.

---

## Phase 3: P0 Tests — Fill Integration Gap

### 3a. `findSubSourceFolders()` unit test
- **File:** `internal/sync/database_test.go`
- **Cases:** empty dir, subfolder with/without `dataSourceId` in metadata, mixed, non-dir entries

### 3b. `resolveDataSources()` integration test
- **File:** `internal/sync/database_integration_test.go` (new)
- **Cases:** single source (flat layout), multiple sources (subfolders), zero sources (error), empty title fallback

### 3c. Deletion detection integration test
- **File:** `internal/sync/database_integration_test.go`
- **Setup:** temp dir with 3 `.md` files, mock client returns only 2 pages
- **Assert:** missing page's `.md` gets `notion-deleted: true`

---

## Phase 4: P1 Tests — Data Integrity

### 4a. Frontmatter round-trip
- **File:** `internal/frontmatter/frontmatter_test.go`
- **Cases:** negative numbers, colons, unicode, empty arrays, nil, boolean-like strings, timestamps

### 4b. `refreshMultiSource()` aggregation
- **File:** `internal/sync/database_integration_test.go`
- **Assert:** totals sum correctly across subfolders, errors propagate

### 4c. Deletion step in system skills
- **Files:** `.claude/skills/test-single-datasource-db/SKILL.md`, `.claude/skills/test-double-datasource-db/SKILL.md`
- **Add step:** create temp page → import → delete page → refresh → verify `notion-deleted: true` → cleanup

---

## Phase 5: P2 Tests — Coverage

### 5a. `--output-mode` tests
- **File:** `internal/sync/database_integration_test.go`
- **Cases:** markdown-only (no `.db`), sqlite-only (no `.md`), both

### 5b. Config key priority
- **File:** `internal/config/config_test.go` (new)
- **Cases:** env > file, file fallback, neither set. Uses `t.Setenv()`.

### 5c. Nested block conversion
- **File:** `internal/markdown/converter_test.go`
- **Mock:** `mockBlockFetcher` in test file
- **Cases:** indented lists, toggle children, callout with child blocks, table rows

---

## Phase 6: P3 Tests — Polish

### 6a. CLI flag parsing
- **File:** `cmd/notion-sync/main_test.go` (new)
- **Test:** `reorderArgs()` directly + `exec.Command` for exit codes

### 6b. Boundary number serialization
- **File:** `internal/frontmatter/frontmatter_test.go`
- **Cases:** negative, zero, large, Inf, NaN, negative zero

---

## Phase 7: Unified `/test` Skill

### New file: `.claude/skills/test/SKILL.md`

Sequential orchestration (API rate limits prevent concurrency):
1. `go test ./...` (unit tests)
2. Invoke `/test-single-datasource-db`
3. Invoke `/test-double-datasource-db`
4. Print combined summary table with pass/fail per suite

Supports `--verbose`, `--no-cleanup`, `--skip-unit` flags passthrough.

---

## Implementation Order

| # | What | New/Modified Files |
|---|------|--------------------|
| 1 | Extract interfaces | `sync/client.go` (new), `sync/database.go`, `sync/page.go`, `markdown/converter.go` |
| 2 | Verify build + tests | — |
| 3 | Mock client | `sync/mock_client_test.go` (new) |
| 4 | P0: findSubSourceFolders test | `sync/database_test.go` |
| 5 | P0: resolveDataSources test | `sync/database_integration_test.go` (new) |
| 6 | P0: Deletion detection test | `sync/database_integration_test.go` |
| 7 | P1: Frontmatter round-trip | `frontmatter/frontmatter_test.go` |
| 8 | P1: refreshMultiSource test | `sync/database_integration_test.go` |
| 9 | P1: Deletion in system skills | `.claude/skills/test-*-datasource-db/SKILL.md` |
| 10 | P2: Output mode tests | `sync/database_integration_test.go` |
| 11 | P2: Config priority test | `config/config_test.go` (new) |
| 12 | P2: Nested block tests | `markdown/converter_test.go` |
| 13 | P3: CLI parsing tests | `cmd/notion-sync/main_test.go` (new) |
| 14 | P3: Boundary numbers | `frontmatter/frontmatter_test.go` |
| 15 | Unified /test skill | `.claude/skills/test/SKILL.md` (new) |

---

## Verification

1. After interface extraction: `go build ./cmd/notion-sync && go test ./...` (178 existing tests pass)
2. After each new test: `go test -v ./internal/sync/ -run TestName`
3. After all Go tests: `go test ./... -count=1` (expect ~200+ tests)
4. After skill updates: `/test-single-datasource-db` and `/test-double-datasource-db` individually
5. Final: `/test` unified skill — all suites green
