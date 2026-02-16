# Test Databases

Reference databases used for integration testing of `notion-sync`.

## Databases

| Name | Type | Database ID | Folder |
|------|------|-------------|--------|
| Complex (Property & Block Coverage) | Single data source | `2fe57008-e885-8003-b1f3-cc05981dc6b0` | [single-data-source/](single-data-source/) |
| Double Data Source | Multi data source (2) | `c9aa5ab2-b470-429c-ba9c-86c853782bb2` | [double-data-source/](double-data-source/) |

## Links

- **Complex:** https://www.notion.so/2fe57008e8858003b1f3cc05981dc6b0
- **Double Data Source:** https://www.notion.so/c9aa5ab2b470429cba9c86c853782bb2

## What They Test

### Single Data Source (Complex)
- All supported property types (title, rich_text, number, select, multi_select, date, checkbox, url, email, phone_number, relation, unique_id, created_by, last_edited_by, etc.)
- All supported block types (headings, lists, code, equations, tables, columns, callouts, toggles, media, etc.)
- Rich text annotations (bold, italic, strikethrough, code, underline, highlight, links)
- 11 pages with varied content

### Double Data Source
- Multi-data-source database import (subfolder-per-source layout)
- Independent schemas across data sources
- Per-source `_database.json` metadata with `dataSourceId`
- Top-level refresh delegating to sub-source folders
- 2 data sources, 3 pages each (6 total)
