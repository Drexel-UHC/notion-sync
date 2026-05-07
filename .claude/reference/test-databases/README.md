# Test Databases

Reference databases used for integration testing of `notion-sync`.

## Databases

| Name | Type | Database ID | Folder |
|------|------|-------------|--------|
| Complex (Property & Block Coverage) | Single data source | `2fe57008-e885-8003-b1f3-cc05981dc6b0` | [single-data-source/](single-data-source/) |
| Double Data Source | Multi data source (2) | `c9aa5ab2-b470-429c-ba9c-86c853782bb2` | [double-data-source/](double-data-source/) |
| Push E2E Fixtures (`notion-sync-test-database-push`) | Single data source | `35957008-e885-80c5-9e34-f4191fd83907` | [push-e2e/](push-e2e/) |

## Links

- **Complex:** https://www.notion.so/2fe57008e8858003b1f3cc05981dc6b0
- **Double Data Source:** https://www.notion.so/c9aa5ab2b470429cba9c86c853782bb2
- **Push E2E:** https://www.notion.so/35957008e88580c59e34f4191fd83907

## What They Test

### Single Data Source (Complex)
- All supported property types (title, rich_text, number, select, multi_select, date, checkbox, url, email, phone_number, relation, unique_id, created_by, last_edited_by, etc.)
- All supported block types (headings, lists, code, equations, tables, columns, callouts, toggles, media, etc.)
- Rich text annotations (bold, italic, strikethrough, code, underline, highlight, links)
- 11 pages with varied content

### Double Data Source
- Multi-data-source database import (subfolder-per-source layout)
- Independent schemas across data sources ("Projects" and "Clients")
- Cross-source relation property (Projects.Client → Clients pages)
- Per-source `_database.json` metadata with `dataSourceId`
- Top-level refresh delegating to sub-source folders
- Edge cases: null properties, special chars in titles, unicode, long filenames, duplicate names across sources, negative numbers, empty content
- 2 data sources, 13 pages total (7 Projects + 6 Clients)

### Push E2E Fixtures
- Dedicated DB for the v1.4.0 `push` redesign — used by `/test-push`
- 7 pages, schema is a subset of the Complex DB (only push-writable types)
- Phase 1: confirmation gate (cancel / proceed / dry-run)
- Phase 2: validation halts (multi-conflict aggregation, soft-delete skip, null edges)
- Phase 3: cell-level diff + rich-text formatting preservation (the original #55 symptom)
- Phase 4: run summary JSON across every status enum
- See [push-e2e/setup.md](push-e2e/setup.md) for the full protocol, page IDs, and "do not edit" fixture conventions.
