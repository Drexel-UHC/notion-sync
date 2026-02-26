---
name: test
description: Run all tests — unit, system, cross-integration SQLite consistency, then cleanup
version: 2.0.0
args: "[--verbose] [--no-cleanup] [--skip-unit] [--skip-system]"
---

# Unified Test Runner

Run all test suites sequentially. Both system tests leave their output on disk so Step 4 can cross-validate the shared SQLite store against all markdown files, then Step 5 cleans up.

## Flags

Parse the skill args for these flags:

| Flag | Effect |
|------|--------|
| `--verbose` | Pass through to system test skills |
| `--no-cleanup` | Skip Step 5 (final cleanup) — leave `test-output/` on disk |
| `--skip-unit` | Skip Step 1 (Go unit tests) |
| `--skip-system` | Skip Steps 2–4 (system + cross-integration) |

## Steps

### Step 1: Unit + Integration Tests

**Skip if `--skip-unit` was passed.**

Run: `go test ./... -count=1`

- **Pass criteria:** exit code 0, all tests pass.
- Print: `Step 1: Unit tests — X packages, PASS/FAIL`

### Step 2: Single Data Source System Test

**Skip if `--skip-system` was passed.**

Invoke `/test-single-datasource-db --no-cleanup`, also passing `--verbose` if present.

The `--no-cleanup` is **always** passed here regardless of user flags — we need the files on disk for Step 4.

- **Pass criteria:** Skill reports `ALL TESTS PASSED`.
- Print: `Step 2: Single datasource — PASS/FAIL`

### Step 3: Double Data Source System Test

**Skip if `--skip-system` was passed.**

Invoke `/test-double-datasource-db --no-cleanup`, also passing `--verbose` if present.

Same as Step 2 — `--no-cleanup` always passed to keep files for Step 4.

- **Pass criteria:** Skill reports `ALL TESTS PASSED`.
- Print: `Step 3: Double datasource — PASS/FAIL`

### Step 4: Standalone Page System Test

**Skip if `--skip-system` was passed.**

Invoke `/test-standalone-page --no-cleanup`, also passing `--verbose` if present.

Same as Steps 2–3 — `--no-cleanup` always passed to keep files for Step 5.

- **Pass criteria:** Skill reports `ALL TESTS PASSED`.
- Print: `Step 4: Standalone page — PASS/FAIL`

### Step 5: Cross-Integration — SQLite ↔ Markdown Consistency

**Skip if `--skip-system` was passed.**

This validates that the shared `_notion_sync.sqlite` at `test-output/` is consistent with all the markdown files produced by both system tests.

Run these checks using `sqlite3` (read-only) and filesystem inspection:

#### 5a. SQLite has pages from both databases and standalone page

```sql
SELECT database_id, COUNT(*) FROM pages WHERE deleted = 0 GROUP BY database_id;
```

- **Pass criteria:** Three groups present:
  - `2fe57008-e885-8003-b1f3-cc05981dc6b0` (single-source) with 11 pages
  - `c9aa5ab2-b470-429c-ba9c-86c853782bb2` (double-source) with >= 13 pages
  - Empty/NULL `database_id` (standalone page) with 1 page

#### 5b. Every non-deleted SQLite page has a matching .md file

For each row in `pages` where `deleted = 0` and `file_path` is not empty:
- Verify the file at `file_path` exists on disk.

- **Pass criteria:** All file paths resolve to existing files.

#### 5c. Every .md file with `notion-id` has a matching SQLite row

Scan all `.md` files under `test-output/` for `notion-id` in frontmatter. For each:
- Query `SELECT id FROM pages WHERE id = '<notion-id>' AND deleted = 0`
- Verify a row exists.

- **Pass criteria:** All markdown notion-ids found in SQLite.

#### 5d. Timestamps match between SQLite and frontmatter

For a sample of 3 pages (pick one from single-source, two from double-source):
- Compare `last_edited_time` in SQLite vs `notion-last-edited` in the `.md` frontmatter.

- **Pass criteria:** Timestamps match (use `timestampsEqual` logic — `.000Z` vs `Z` is OK).

#### 5e. FTS index covers both databases

```sql
SELECT COUNT(*) FROM pages_fts;
```

- **Pass criteria:** Count >= total non-deleted pages across both databases.

Print: `Step 5: Cross-integration SQLite consistency — PASS/FAIL`

### Step 6: Cleanup

**Skip if `--no-cleanup` was passed.** Print `Step 6: Skipped (--no-cleanup)` and leave files.

Otherwise, clean up everything:

1. Delete `test-output/test database obsdiain complex/` (single-source folder)
2. Delete `test-output/test database - double data source/` (double-source folder)
3. Delete `test-output/pages/` (standalone page folder)
4. Clean SQLite: delete rows from `pages` where `database_id` IN the two test database IDs OR `id = '31357008-e885-80c3-90f4-d148f0854bba'`. Use `sqlite3` to execute the SQL.
5. If `_notion_sync.sqlite` has zero rows remaining in `pages`, delete the `.sqlite` file entirely.
6. If `test-output/` is now empty, delete it.

Print: `Step 6: Cleanup — done`

## Summary

Print a combined results table:

```
| Suite                          | Result  |
|--------------------------------|---------|
| Unit tests (go test)           | PASS    |
| Single datasource system       | PASS    |
| Double datasource system       | PASS    |
| Standalone page system         | PASS    |
| Cross-integration (SQLite ↔ MD)| PASS    |
```

Then:
- If all suites passed: `✅ ALL SUITES PASSED`
- If any failed: `❌ SOME SUITES FAILED` followed by which ones failed
