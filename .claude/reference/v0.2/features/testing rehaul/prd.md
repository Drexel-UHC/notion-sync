# Testing Protocol

## Current Coverage

| Layer | Files | Tests | What's Covered |
|-------|-------|-------|---------------|
| Unit: markdown | `converter_test.go` | ~30 | Block→MD conversion (28 block types), rich text (24 variants) |
| Unit: frontmatter | `frontmatter_test.go` | ~16 | YAML parse, body extraction, string escaping |
| Unit: notion | `client_test.go` | 9 | `NormalizeNotionID` (hex, UUID, URL, errors) |
| Unit: util | `path_test.go` | ~8 | `SanitizeFileName`, `JoinPath` |
| Unit: sync/page | `page_test.go` | ~12 | `mapPropertiesToFrontmatter` (17 property types), `getPageTitle` |
| Unit: sync/database | `database_test.go` | ~12 | `scanLocalFiles`, `markAsDeleted`, `timestampsEqual` |
| Unit: sync/metadata | `metadata_test.go` | 5 | `_database.json` read/write/list |
| Unit: store | `store_test.go` | 22 | SQLite CRUD, FTS, triggers, reopen, special chars, multi-DB |
| System: single-source | Claude skill | 13 steps | Full CLI lifecycle (import→refresh→ids→force→verify→SQLite) |
| System: double-source | Claude skill | 15 steps | Multi-source layout, SQLite, edge cases |
| **Total** | **8 test files + 2 skills** | **178 unit + 28 system steps** | |

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    SYSTEM TESTS                         │
│              (Claude skills, real Notion API)            │
│                                                         │
│  /test-single-datasource-db  /test-double-datasource-db │
│  - import, refresh, --ids    - subfolder layout         │
│  - --force, mtime, props     - SQLite, edge cases       │
│  - incremental sync, SQLite  - cross-source relations   │
│  - scoped cleanup per DB     - scoped cleanup per DB    │
│                                                         │
│  Catches: API contract changes, CLI regressions,        │
│           end-to-end data flow                          │
├─────────────────────────────────────────────────────────┤
│                                                         │
│                 INTEGRATION TESTS                       │
│              (mocked client, real filesystem)            │
│                                                         │
│              ┌─────────────────────────┐                │
│              │     ⚠ MISSING GAP ⚠     │                │
│              │                         │                │
│              │  • resolveDataSources() │                │
│              │  • refreshMultiSource() │                │
│              │  • deletion detection   │                │
│              │  • importEntries() flow │                │
│              │  • --output-mode paths  │                │
│              │  • config key priority  │                │
│              └─────────────────────────┘                │
│                                                         │
│  Would catch: orchestration bugs, multi-source logic,   │
│               deletion edge cases — without Notion API   │
├─────────────────────────────────────────────────────────┤
│                     UNIT TESTS                          │
│              (pure functions, no I/O)                    │
│                                                         │
│  markdown/       frontmatter/     notion/               │
│  converter ✅    parse ✅         NormalizeID ✅        │
│  richtext ✅     writer ✅                              │
│                                                         │
│  sync/           store/           util/                 │
│  page ✅         SQLite CRUD ✅   SanitizeFileName ✅   │
│  database ✅     FTS ✅           JoinPath ✅           │
│  metadata ✅     triggers ✅                            │
│                                                         │
│  Catches: conversion bugs, YAML edge cases,             │
│           property mapping, filename sanitization        │
├─────────────────────────────────────────────────────────┤
│                   TOOLS & NOT TESTED                    │
│                                                         │
│  sqlite3 CLI — read-only inspection of _notion_sync.db  │
│  (used in system test skills; never write directly)     │
│                                                         │
│  cmd/notion-sync/main.go  — CLI flag parsing, routing   │
│  internal/config/         — API key priority chain      │
└─────────────────────────────────────────────────────────┘
```

---

## Proposed Testing Enhancements

| Priority | Test | Level | Changes Prod Code? | Risk | Effort |
|----------|------|-------|--------------------|------|--------|
| P0 | `findSubSourceFolders()` — subfolder detection with/without `dataSourceId` in metadata | Unit | No | Zero | 30 min |
| P0 | `resolveDataSources()` — mock client, test single/multi/zero source paths | Integration | Yes (extract `NotionClient` interface) | Very low | 1-2 hr |
| P0 | Deletion detection — mock client returning fewer pages than local files, verify `notion-deleted: true` | Integration | Yes (same interface) | Very low | 1 hr |
| P1 | Frontmatter round-trip — write then read, assert equality for negative numbers, colons, unicode, empty arrays, null | Unit | No | Zero | 30 min |
| P1 | `refreshMultiSource()` aggregation — verify totals sum correctly, errors propagate | Integration | No (mock sub-folders on disk) | Zero | 30 min |
| P1 | Deletion step in system skills — create temp page, import, delete in Notion, refresh, verify soft delete | System | No (skill file only) | Zero | 15 min |
| P2 | `--output-mode sqlite` / `--output-mode markdown` — verify only expected outputs produced | System | No (skill file only) | Zero | 15 min |
| P2 | Config key priority — test flag > env > keyring > file precedence | Unit | Maybe (env var access pattern) | Very low | 30 min |
| P2 | Nested block conversion — indented lists, toggle children, callout with child blocks | Unit | No | Zero | 30 min |
| P3 | CLI flag parsing — valid flags, missing required args, unknown flags, exit codes | Unit | No (test via `exec.Command`) | Zero | 30 min |
| P3 | Boundary number serialization — negative, zero, very large, `Inf`, `NaN` in frontmatter writer | Unit | No | Zero | 15 min |

### Summary

- **P0 (3 items):** Fill the integration test gap for multi-data-source orchestration and deletion. One requires extracting a `NotionClient` interface (safe, additive refactor).
- **P1 (3 items):** Harden data integrity (frontmatter round-trip, aggregation math, deletion E2E).
- **P2 (3 items):** Cover output modes, config, and nested blocks.
- **P3 (2 items):** Polish — CLI parsing and number edge cases.

~85% of items are pure test additions (zero risk). ~15% require a small production refactor (extract interface).

---

## Recent Changes

- **v1 API removed:** `QueryDatabase`/`QueryAllEntriesFromDatabase` deleted. All queries now go through `/data_sources/{id}/query` (v2 API). No legacy fallback paths to test.
- **SQLite verification:** Added to single-datasource skill (Step 10) — checks page count, body_markdown, timestamps, FTS index.
- **Scoped cleanup:** Both skills now only delete their own subfolder + clean their rows from SQLite by `database_id`. No more nuking the entire `test-output/` directory.
