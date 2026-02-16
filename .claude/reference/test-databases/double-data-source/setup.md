# Test Database: Double Data Source

**URL:** https://www.notion.so/c9aa5ab2b470429cba9c86c853782bb2
**Database ID:** `c9aa5ab2-b470-429c-ba9c-86c853782bb2`

A database with two independent data sources, used to test multi-data-source import and refresh logic introduced with the Notion API v2 (`2025-09-03`).

---

## Data Source 1: "Projects"

**Data Source ID:** `3bd3383c-9d2d-4ac6-a7da-278d870ce57f`

### Schema

| Property   | Type         | Config                                              |
|------------|--------------|-----------------------------------------------------|
| Name       | title        | (default)                                           |
| Description| rich_text    |                                                     |
| Score      | number       | format: number                                      |
| Category   | select       | Research (blue), Engineering (green), Design (purple)|
| Tags       | multi_select | urgent (red), frontend (blue), backend (green)      |
| Due Date   | date         |                                                     |
| Approved   | checkbox     |                                                     |
| Website    | url          |                                                     |
| Client     | relation     | → Clients data source                               |
| ID         | unique_id    | prefix: DS1                                         |

### Pages (7 rows)

| Name | Category | Score | Tags | Client | Edge Case |
|------|----------|-------|------|--------|-----------|
| Alpha Report | Research | 90 | urgent, frontend | Delta Corp | Normal row |
| Beta Analysis | Engineering | 67 | backend | Echo Systems, Foxtrot Ltd | Multi-relation |
| Gamma Design | Design | 88 | frontend, backend | Delta Corp | Normal row |
| Edge: All Nulls | null | null | [] | [] | All optional properties null/empty |
| Edge: Special Ch@rs & "Quotes" / Slashes \ (Parens) | Research | 0 | urgent | — | Filename sanitization (colons, quotes, slashes) |
| Edge: Très Long Tïtle With Ünïcödé Characters... | Engineering | 999999.99 | urgent, frontend, backend | — | Unicode + very long title + large number |
| Duplicate Name | Design | 50 | [] | — | Same title exists in Clients source |

---

## Data Source 2: "Clients"

**Data Source ID:** `30957008-e885-8041-888e-000becc596ae`

### Schema

| Property | Type     | Config                                       |
|----------|----------|----------------------------------------------|
| Name     | title    | (default)                                    |
| Text     | rich_text|                                              |
| Region   | select   | North (blue), South (green), East (orange), West (purple) |
| Revenue  | number   | format: number                               |
| Active   | checkbox |                                              |
| Notes    | rich_text|                                              |

### Pages (6 rows)

| Name | Region | Revenue | Active | Edge Case |
|------|--------|---------|--------|-----------|
| Delta Corp | North | 125000 | true | Normal row |
| Echo Systems | South | 89000 | true | Normal row |
| Foxtrot Ltd | East | 42000 | false | Inactive (checkbox false) |
| Duplicate Name | West | 0 | true | Same title exists in Projects source |
| Edge: Empty Everything | null | null | false | All optional properties null |
| Edge: Numeric-Like Title 12345 | North | -500.75 | true | Numeric-looking title + negative number |

---

## Edge Cases Covered

| Edge Case | What It Tests |
|-----------|---------------|
| All null properties | Null handling for select, number, url, date, relation, multi_select |
| Special characters in title | Filename sanitization: `@`, `&`, `"`, `/`, `\`, `(`, `)` |
| Very long title with unicode | Filename truncation + unicode preservation (accented chars) |
| Duplicate title across sources | No collision since sources live in separate subfolders |
| Zero and large numbers | Score=0, Score=999999.99 — boundary values |
| Negative number | Revenue=-500.75 — negative float in frontmatter |
| Empty page content | Page with frontmatter only, no body |
| Cross-source relation | Projects.Client → Clients pages (relation IDs resolve across sources) |
| Multi-relation | Beta Analysis → 2 clients (array with multiple IDs) |

---

## Expected Import Behavior

When imported with `notion-sync import c9aa5ab2-b470-429c-ba9c-86c853782bb2 --output ./test-output`:

```
test-output/
└── test database - double data source/
    ├── _database.json
    ├── Projects/
    │   ├── _database.json       (dataSourceId: 3bd3383c-...)
    │   ├── Alpha Report.md
    │   ├── Beta Analysis.md
    │   ├── Gamma Design.md
    │   ├── Edge- All Nulls.md
    │   ├── Edge- Special Ch@rs & -Quotes- - Slashes  (Parens).md
    │   ├── Edge- Très Long Tïtle With Ünïcödé Characters....md
    │   └── Duplicate Name.md
    └── Clients/
        ├── _database.json       (dataSourceId: 30957008-...)
        ├── Delta Corp.md
        ├── Echo Systems.md
        ├── Foxtrot Ltd.md
        ├── Duplicate Name.md
        ├── Edge- Empty Everything.md
        └── Edge- Numeric-Like Title 12345.md
```

Total: 13 pages (7 Projects + 6 Clients).

### Key Test Points

1. **Subfolder layout** — multi-source databases create one subfolder per data source
2. **Independent schemas** — Projects and Clients have completely different property columns
3. **Cross-source relations** — Projects.Client contains Clients page IDs
4. **Per-source refresh** — `refresh` on the parent folder delegates to each subfolder
5. **Per-source metadata** — each `_database.json` has its own `dataSourceId` and `entryCount`
6. **Duplicate names** — same title in both sources lives in separate folders, no conflict
7. **Null property handling** — null select/number/url/date render correctly in frontmatter
8. **Filename sanitization** — special chars in titles are safely converted for filesystem
9. **Negative numbers** — preserved correctly in frontmatter YAML

---

## How This Database Was Created

1. Created database via Notion MCP `create-pages` tool at workspace level
2. Source 1 ("Projects") schema configured via `update-data-source` with full property set
3. Source 1 pages created via `create-pages` with `data_source_id`
4. Source 2 added **manually in Notion UI** ("+ New source" button) — cannot be done via API
5. Source 2 ("Clients") schema configured via `update-data-source`
6. Source 2 pages created via `create-pages` with `data_source_id`
7. Cross-source relation property (`Client`) added to Projects, pointing to Clients data source
8. Relations wired: Alpha→Delta, Beta→Echo+Foxtrot, Gamma→Delta
9. Edge case pages added to both sources
