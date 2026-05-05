# Testing Framework

## Test Pyramid

```
┌─────────────────────────────────────────────────────────┐
│                    SYSTEM TESTS                         │
│              (Claude skills, real Notion API)            │
│                                                         │
│  /test-single-datasource-db  /test-double-datasource-db │
│  - import → refresh → --ids  - multi-source subfolders  │
│  - --force, no-pin, props    - cross-source relations   │
│  - SQLite verification       - edge cases (nulls, 0,    │
│  - scoped cleanup per DB       unicode, negative nums)  │
│                                                         │
│  Catches: API contract changes, CLI regressions,        │
│           end-to-end data flow                          │
├─────────────────────────────────────────────────────────┤
│                 INTEGRATION TESTS                       │
│          (mock NotionClient, real filesystem)            │
│                                                         │
│  sync/database_integration_test.go                      │
│  - resolveDataSources (single/multi/zero/fallback)      │
│  - deletion detection (notion-deleted: true)            │
│  - refreshMultiSource aggregation                       │
│  - output mode: markdown-only, sqlite-only, both        │
│                                                         │
│  Catches: orchestration bugs, multi-source logic,       │
│           deletion edge cases — without Notion API       │
├─────────────────────────────────────────────────────────┤
│                     UNIT TESTS                          │
│              (pure functions, no I/O)                    │
│                                                         │
│  markdown/       frontmatter/     notion/               │
│  converter ✅    parse ✅         NormalizeID ✅        │
│  richtext ✅     writer ✅        config/ ✅            │
│  nested blocks ✅ round-trip ✅                          │
│                                                         │
│  sync/           store/           util/                 │
│  page props ✅   SQLite CRUD ✅   SanitizeFileName ✅   │
│  database ✅     FTS ✅           JoinPath ✅           │
│  metadata ✅     triggers ✅      cmd/main ✅           │
│                                                         │
│  Catches: conversion bugs, YAML edge cases,             │
│           property mapping, filename sanitization,       │
│           CLI flag parsing, config key priority          │
└─────────────────────────────────────────────────────────┘
```

---

## How to Run

### Unit + Integration (no API key needed)

```sh
go test ./...                    # all packages
go test ./internal/sync/         # just sync tests
go test -run TestRefresh ./internal/sync/  # specific test
go test -v ./...                 # verbose output
```

### System Tests (require Notion API key)

```sh
/test-single-datasource-db              # single data source lifecycle
/test-single-datasource-db --verbose    # step-by-step with confirmation
/test-single-datasource-db --no-cleanup # keep test-output/ for inspection

/test-double-datasource-db              # multi-source layout + edge cases
/test-double-datasource-db --verbose
```

### Everything Together

```sh
/test                      # unit → single-source → double-source (sequential)
/test --skip-unit          # system tests only
/test --skip-system        # unit/integration only
/test --verbose            # verbose passed to system tests
```

---

## Coverage by Package

| Package | Test File(s) | Tests | What's Covered |
|---------|-------------|-------|---------------|
| cmd/notion-sync | `main_test.go` | 12 | `reorderArgs` (9 cases), CLI exit codes, --version |
| internal/config | `config_test.go` | 5 | env > file priority, defaults, XDG path, outputMode |
| internal/frontmatter | `frontmatter_test.go` | ~30 | Parse, GetBody, round-trip (negatives, unicode, booleans, timestamps), yamlEscapeString, boundary numbers |
| internal/markdown | `converter_test.go` | ~35 | 28 block types, rich text (24 variants), nested blocks with mock BlockFetcher (lists, toggles, callouts, tables) |
| internal/notion | `client_test.go` | 9 | NormalizeNotionID (hex, UUID, URL, errors) |
| internal/store | `store_test.go` | 22 | SQLite CRUD, FTS, triggers, reopen, special chars, multi-DB |
| internal/sync | `database_test.go` | ~16 | scanLocalFiles, markAsDeleted, timestampsEqual, findSubSourceFolders |
| internal/sync | `database_integration_test.go` | ~10 | resolveDataSources, deletion detection, refreshMultiSource, output modes |
| internal/sync | `page_test.go` | ~12 | mapPropertiesToFrontmatter (17 property types), getPageTitle |
| internal/sync | `metadata_test.go` | 5 | _database.json read/write/list |
| internal/util | `path_test.go` | ~8 | SanitizeFileName, JoinPath |
| **System** | 2 Claude skills | 28 steps | Full CLI lifecycle against real Notion API |
| **Total** | **12 test files + 2 skills** | **~236 unit/integration + 28 system steps** | |

---

## Key Test Infrastructure

### Interfaces (prod code, enable mocking)

- **`sync.NotionClient`** (`internal/sync/client.go`) — 5 methods: GetDatabase, GetDataSource, QueryAllEntries, GetPage, FetchAllBlocks
- **`markdown.BlockFetcher`** (`internal/markdown/converter.go`) — 1 method: FetchAllBlocks

### Mocks (test-only)

- **`mockNotionClient`** (`internal/sync/mock_client_test.go`) — map-based, returns pre-configured data by ID
- **`mockBlockFetcher`** (`internal/markdown/converter_test.go`) — returns pre-configured blocks for testing nested conversion
- **`testPage()`** helper — creates a minimal `notion.Page` with title and timestamp

### Test Databases (real Notion)

| Name | Database ID | Skill | Pages |
|------|-------------|-------|-------|
| Single data source (complex) | `2fe57008-e885-8003-b1f3-cc05981dc6b0` | `/test-single-datasource-db` | 11 |
| Double data source | `c9aa5ab2-b470-429c-ba9c-86c853782bb2` | `/test-double-datasource-db` | ~14 |

---

## Adding Tests

### New unit test for a sync function

1. If the function calls `NotionClient` methods, use `mockNotionClient` from `mock_client_test.go`
2. Use `t.TempDir()` for filesystem operations
3. Use `WriteDatabaseMetadata()` to set up `_database.json` fixtures
4. Run: `go test -v ./internal/sync/ -run TestYourTest`

### New block conversion test with children

1. Create a `mockBlockFetcher` with the child blocks keyed by parent block ID
2. Pass it via `ConvertContext{Client: mock}`
3. Set `HasChildren: true` on parent blocks
4. See `TestConvertBlocksToMarkdown_NestedBulletedList` for example

### New system test step

1. Edit the relevant skill in `.claude/skills/test-*-datasource-db/SKILL.md`
2. Add step with pass criteria
3. Test with: `/test-single-datasource-db --verbose`
