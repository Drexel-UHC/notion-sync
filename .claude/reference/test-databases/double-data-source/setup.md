# Test Database: Double Data Source

**URL:** https://www.notion.so/c9aa5ab2b470429cba9c86c853782bb2
**Database ID:** `c9aa5ab2-b470-429c-ba9c-86c853782bb2`

A database with two independent data sources, used to test multi-data-source import and refresh logic introduced with the Notion API v2 (`2025-09-03`).

---

## Data Source 1: "my first data source"

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
| ID         | unique_id    | prefix: DS1                                         |

### Pages (3 rows)

| Name           | Category    | Score | Tags              | Approved |
|----------------|-------------|-------|-------------------|----------|
| Alpha Report   | Research    | 92    | urgent, frontend  | true     |
| Beta Analysis  | Engineering | 67    | backend           | false    |
| Gamma Design   | Design      | 88    | frontend, backend | true     |

---

## Data Source 2: "my second data source"

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

### Pages (3 rows)

| Name         | Region | Revenue | Active | Notes                              |
|--------------|--------|---------|--------|------------------------------------|
| Delta Corp   | North  | 125000  | true   | Primary client in the northern region |
| Echo Systems | South  | 89000   | true   | Expanding into new markets         |
| Foxtrot Ltd  | East   | 42000   | false  | Contract paused                    |

---

## Expected Import Behavior

When imported with `notion-sync import c9aa5ab2-b470-429c-ba9c-86c853782bb2 --output ./test-output`:

```
test-output/
└── test database - double data source/
    ├── my first data source/
    │   ├── _database.json       (dataSourceId: 3bd3383c-...)
    │   ├── Alpha Report.md
    │   ├── Beta Analysis.md
    │   └── Gamma Design.md
    └── my second data source/
        ├── _database.json       (dataSourceId: 30957008-...)
        ├── Delta Corp.md
        ├── Echo Systems.md
        └── Foxtrot Ltd.md
```

Each data source gets its own subfolder with independent `_database.json` metadata.

### Key Test Points

1. **Subfolder layout** — multi-source databases create one subfolder per data source
2. **Independent schemas** — source 1 and source 2 have completely different property columns
3. **Per-source refresh** — `refresh` on the parent folder delegates to each subfolder
4. **Per-source metadata** — each `_database.json` has its own `dataSourceId` and `entryCount`

---

## How This Database Was Created

1. Created database via Notion MCP `create-pages` tool at workspace level
2. Source 1 schema configured via `update-data-source` with full property set
3. Source 1 pages created via `create-pages` with `data_source_id`
4. Source 2 added **manually in Notion UI** ("+ New source" button) — cannot be done via API
5. Source 2 schema configured via `update-data-source`
6. Source 2 pages created via `create-pages` with `data_source_id`
