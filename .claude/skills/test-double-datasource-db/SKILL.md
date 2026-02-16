---
name: test-double-datasource-db
description: Run integration test against the double-data-source test database (multi-source import, subfolder layout, SQLite, edge cases)
version: 1.0.0
args: "[--verbose] [--no-cleanup]"
---

# Integration Test: Double Data Source Database

Run an integration test using the **double-data-source test database** (`c9aa5ab2-b470-429c-ba9c-86c853782bb2`).

- **Notion URL:** https://www.notion.so/c9aa5ab2b470429cba9c86c853782bb2
- **Reference:** `.claude/reference/test-databases/double-data-source/`
- **Data sources:** "Projects" (7 pages) + "Clients" (6-7 pages, may include stray "test" page)

## Mode

Check if `--verbose` was passed in the skill args.

- **Default (concise):** Run all steps automatically without asking questions. Print a one-line status per step as you go (e.g., `Step 1: Build... done`). At the end, print the summary table and pass/fail result. Do NOT use `AskUserQuestion` at all.
- **Verbose (`--verbose`):** Interactive mode. Use `AskUserQuestion` with selectable options before every command. Show exact CLI calls, wait for confirmation. Never jump ahead.
- **No cleanup (`--no-cleanup`):** Skip Step 14 (delete `test-output/`). The test output is left on disk so you can examine it manually.

## Verbose-Only Interaction Rules

These rules only apply in `--verbose` mode:

1. **Always use `AskUserQuestion` with selectable options** — never ask questions as plain text.
2. **Before running any command**, show the exact CLI call and use `AskUserQuestion` to confirm.
3. **Never jump ahead** — always wait for my selection between steps.

## Steps

### Step 0: Build
Run: `go build ./cmd/notion-sync`
- **Pass criteria:** exit code 0, `notion-sync.exe` exists.

### Step 1: Clean slate
- If `test-output/test database - double data source/` exists: delete only that subfolder (in verbose mode, ask first).
- If it doesn't exist: skip.
- **Important:** Do NOT delete `test-output/` itself or any other subfolders — other test databases may live there.

### Step 2: Fresh import
Run: `./notion-sync.exe import c9aa5ab2-b470-429c-ba9c-86c853782bb2 --output ./test-output`
- **Pass criteria:** exit code 0, created >= 13.

### Step 3: Verify subfolder layout
Check that the import produced the correct multi-source directory structure:

```
test-output/
├── _notion_sync.db
└── test database - double data source/
    ├── _database.json           (top-level, NO dataSourceId)
    ├── Projects/
    │   ├── _database.json       (HAS dataSourceId)
    │   └── *.md files
    └── Clients/
        ├── _database.json       (HAS dataSourceId)
        └── *.md files
```

Checks:
1. `Projects/` and `Clients/` subfolders exist
2. Each subfolder has its own `_database.json`
3. Top-level `_database.json` exists (no `dataSourceId` field)
4. Each sub-level `_database.json` has a `dataSourceId` field
5. `Projects/_database.json` entryCount >= 7
6. `Clients/_database.json` entryCount >= 6
7. `_notion_sync.db` exists at the `test-output/` root

- **Pass criteria:** All 7 checks pass.

### Step 4: Verify Projects markdown files
Check the `.md` files in `Projects/` subfolder:

| Check | How |
|-------|-----|
| Alpha Report.md exists | File exists |
| Beta Analysis.md exists | File exists |
| Gamma Design.md exists | File exists |
| Edge- All Nulls.md exists | File exists |
| Duplicate Name.md exists | File exists |
| Special chars file exists | A file matching `Edge- Special*` exists |
| Long unicode file exists | A file matching `Edge- Très*` or `Edge- Tr*` exists |
| All files have `notion-id` | Grep frontmatter |
| All files have `notion-database-id` | Value = `c9aa5ab2-b470-429c-ba9c-86c853782bb2` |
| Relation property present | `Alpha Report.md` has `Client:` with at least one page ID |
| Multi-relation | `Beta Analysis.md` has `Client:` with 2 page IDs |
| Null handling | `Edge- All Nulls.md` has `Score: null` and `Category: null` |
| Zero value | Special chars file has `Score: 0` (not null, not empty) |
| Large number | Long title file has `Score: 999999.99` |
| Empty tags | `Edge- All Nulls.md` has `Tags: []` |

- **Pass criteria:** All checks pass.

### Step 5: Verify Clients markdown files
Check the `.md` files in `Clients/` subfolder:

| Check | How |
|-------|-----|
| Delta Corp.md exists | File exists |
| Echo Systems.md exists | File exists |
| Foxtrot Ltd.md exists | File exists |
| Duplicate Name.md exists | File exists (same title as Projects, no collision) |
| Edge- Empty Everything.md exists | File exists |
| Edge- Numeric-Like Title 12345.md exists | File exists |
| Negative number | `Edge- Numeric-Like Title 12345.md` has `Revenue: -500.75` |
| Null properties | `Edge- Empty Everything.md` has `Region: null` and `Revenue: null` |
| Checkbox false | `Foxtrot Ltd.md` has `Active: false` |
| Checkbox true | `Delta Corp.md` has `Active: true` |
| Empty page body | `Edge- Empty Everything.md` has no content after frontmatter closing `---` (or only whitespace) |

- **Pass criteria:** All checks pass.

### Step 6: Verify SQLite database
Query `_notion_sync.db` at `test-output/` root:

```bash
sqlite3 test-output/_notion_sync.db "SELECT COUNT(*) FROM pages"
sqlite3 test-output/_notion_sync.db "SELECT COUNT(DISTINCT database_id) FROM pages"
sqlite3 test-output/_notion_sync.db "SELECT id, title FROM pages WHERE title = 'Duplicate Name'"
sqlite3 test-output/_notion_sync.db "SELECT id, title FROM pages WHERE title LIKE 'Edge:%'"
```

| Check | Expected |
|-------|----------|
| Total page count | >= 13 |
| Distinct database_id count | 1 (all pages share same parent database ID) |
| "Duplicate Name" rows | Exactly 2 (one from each source) |
| Edge case rows | >= 5 (All Nulls, Special, Long, Empty Everything, Numeric-Like) |
| All pages have non-empty `last_edited_time` | `SELECT COUNT(*) FROM pages WHERE last_edited_time IS NULL OR last_edited_time = ''` = 0 |

- **Pass criteria:** All checks pass.

### Step 7: No-op refresh (top-level)
Run refresh on the **parent** folder (not a subfolder) to test multi-source delegation:
`./notion-sync.exe refresh "./test-output/test database - double data source"`
- **Pass criteria:** updated = 0, skipped = total (>= 13).

### Step 8: No-op refresh (per-source)
Run refresh on each subfolder individually:
```
./notion-sync.exe refresh "./test-output/test database - double data source/Projects"
./notion-sync.exe refresh "./test-output/test database - double data source/Clients"
```
- **Pass criteria:** Both return updated = 0, skipped = their respective entry counts.

### Step 9: Make a change via Notion MCP
Use Notion MCP tools to edit one page in the **Clients** source (e.g., Delta Corp):
- Property edit: change `Revenue` to a different value
- Content edit: append `<!-- double-source-test -->` to ensure timestamp changes

**Remember the original values so you can revert in Step 13.**

### Step 10: Incremental refresh (top-level)
Run: `./notion-sync.exe refresh "./test-output/test database - double data source"`
- **Pass criteria:** updated = 1 (Delta Corp), skipped = total - 1.

### Step 11: Force refresh (top-level)
Run: `./notion-sync.exe refresh "./test-output/test database - double data source" --force`
- **Pass criteria:** updated = total (>= 13), skipped = 0.

### Step 12: Verify file mtime preservation
For each synced `.md` file in both `Projects/` and `Clients/`, compare the file's modification time (via `stat`) against the `notion-last-edited` value in its frontmatter.
- **Pass criteria:** File mtime matches `notion-last-edited` timestamp (within 1-second tolerance).

### Step 13: Revert Notion changes
Use Notion MCP tools to restore the page edited in Step 9 back to its original property values and content. This keeps the test database clean for the next run.

### Step 14: Clean up
If `--no-cleanup` was passed, **skip this step** and print `Step 14: Skipped (--no-cleanup)`.
Otherwise:
1. Delete only `test-output/test database - double data source/` (not the entire `test-output/` directory).
2. Clean SQLite: delete rows from `pages` table in `test-output/_notion_sync.db` where `database_id` matches this test's database ID (`c9aa5ab2-b470-429c-ba9c-86c853782bb2`). Use Python or Go to run the SQL.
3. If `_notion_sync.db` has zero rows remaining in `pages`, delete the `.db` file entirely.
4. If `test-output/` is now empty, delete it.

## Done
Summarize all step results in a table with columns: Step | Action | Result.

If all steps passed, output:

```
✅ ALL TESTS PASSED
```

If any steps failed, output:

```
❌ TESTS FAILED
```
followed by a bullet list of each failed step and what went wrong.
