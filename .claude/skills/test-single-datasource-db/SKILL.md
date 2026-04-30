---
name: test-single-datasource-db
description: Run integration test against the single-data-source test database (import, refresh, --ids, --force, push)
version: 2.1.0
args: "[--verbose] [--no-cleanup]"
---

# Integration Test: Single Data Source Database

Run an integration test using the **single-data-source test database** (`2fe57008-e885-8003-b1f3-cc05981dc6b0`).

- **Notion URL:** https://www.notion.so/2fe57008e8858003b1f3cc05981dc6b0
- **Reference:** `.claude/reference/test-databases/single-data-source/`

## Mode

Check if `--verbose` was passed in the skill args.

- **Default (concise):** Run all steps automatically without asking questions. Print a one-line status per step as you go (e.g., `Step 1: Clean slate... done`). At the end, print the summary table and pass/fail result. Do NOT use `AskUserQuestion` at all.
- **Verbose (`--verbose`):** Interactive mode. Use `AskUserQuestion` with selectable options before every command. Show exact CLI calls, wait for confirmation, and provide detailed Git Analysis after each action. Never jump ahead.
- **No cleanup (`--no-cleanup`):** Skip Step 11 (delete `test-output/`). The test output is left on disk so you can examine it manually.

## Verbose-Only Interaction Rules

These rules only apply in `--verbose` mode:

1. **Always use `AskUserQuestion` with selectable options** — never ask questions as plain text.
2. **Before running any command**, show the exact CLI call and use `AskUserQuestion` to confirm.
3. **Never jump ahead** — always wait for my selection between steps.
4. **Git Analysis after every action** — run `git diff --stat` and provide a **"Git Analysis:"** section summarizing what git sees. This is inline with each step, not a separate step.

## Steps

### Step 0: Build
Run: `go build ./cmd/notion-sync`
- **Pass criteria:** exit code 0, `notion-sync.exe` exists.

### Step 1: Clean slate
- If `test-output/test database obsdiain complex/` exists: delete only that subfolder (in verbose mode, ask first).
- If it doesn't exist: skip.
- **Important:** Do NOT delete `test-output/` itself or any other subfolders — other test databases may live there.

### Step 2: Fresh import
Run: `./notion-sync.exe import 2fe57008-e885-8003-b1f3-cc05981dc6b0 --output ./test-output`
- **Pass criteria:** created > 0, exit code 0.

### Step 2b: Verify duplicate title disambiguation
Check that pages with duplicate titles ("Headings & Rich Text") were disambiguated:
- `Headings & Rich Text.md` should NOT exist (clean name must not exist since the title is duplicated)
- 2 files matching `Headings & Rich Text-*.md` should exist
- Grep both files for `notion-id` — they should have different IDs
- Both should have `notion-database-id: 2fe57008-e885-8003-b1f3-cc05981dc6b0`
- **Pass criteria:** All checks pass.

### Step 3: No-op refresh
Run: `./notion-sync.exe refresh "./test-output/test database obsdiain complex"`
- **Pass criteria:** updated = 0, skipped = total, deleted = 0.

### Step 4: Make a change via Notion MCP
Use the Notion MCP tools to make **both** a property edit **and** a content edit to one of the pages. Property-only edits may not bump Notion's `last_edited_time`, so you must also edit the page content (e.g., append `<!-- sync-test -->`) to ensure the timestamp changes.

**Remember the original values so you can revert in Step 9.**

### Step 5: Incremental refresh
Run: `./notion-sync.exe refresh "./test-output/test database obsdiain complex"`
- **Pass criteria:** updated = 1 (the edited page), skipped = total - 1.

### Step 6: Test --ids flag
Pick a page ID from the synced files. Run:
`./notion-sync.exe refresh "./test-output/test database obsdiain complex" --ids <page-id>`
- **Pass criteria:** updated = 1, skipped = 0.

### Step 7: Test --force flag
Run: `./notion-sync.exe refresh "./test-output/test database obsdiain complex" --force`
- **Pass criteria:** updated = total, skipped = 0.

### Step 8: Verify property output
Grep the synced `.md` files in `./test-output/test database obsdiain complex/` for the following frontmatter keys and validate:

| Key | Check |
|---|---|
| `unique_id` | Present in at least one file, value matches pattern `PREFIX-N` or just `N` (digits) |
| `created_by` | Present in at least one file, value is a non-empty string (user name or ID) |
| `last_edited_by` | Present in at least one file, value is a non-empty string (user name or ID) |

- **Pass criteria:** All 3 keys found with valid non-null values.

### Step 9: Verify file mtime preservation
For each synced `.md` file, compare the file's modification time (via `stat`) against the `notion-last-edited` value in its frontmatter.
- **Pass criteria:** File mtime matches `notion-last-edited` timestamp (within 1-second tolerance).

### Step 9b: Pick a push test page and snapshot originals
Pick a page **other than the one edited in Step 4** (call it page B). Read its local `.md` file and record the original values for these properties (you'll need them for Step 9d):

| Property type | Example key in this DB | Why |
|---|---|---|
| `title` | (whatever the title property is named) | Regression-protect ref #51 |
| `rich_text` | any rich_text column | Regression-protect 2000-char guard |
| `select` | any select column | |
| `number` | any number column | |
| `date` | any date column | |

If the DB doesn't have a column of one of these types, skip that type and note it in the summary.

### Step 9c: Edit page B locally and push
Edit page B's `.md` file: change the 5 property values in the frontmatter to clearly distinguishable test values (e.g., prefix strings with `e2e-push-test-`, set number to `9999`, set date to `2026-04-30`).

Run: `./notion-sync.exe push "./test-output/test database obsdiain complex"`

- **Pass criteria:**
  - Exit code 0
  - Output contains `Pushed:` and `Pushed == Total` (push writes every file by design — not just edited ones)
  - `Conflicts: 0`, `Failed: 0`
  - Page B's `.md` now has a `notion-last-pushed:` frontmatter key
  - Page B's `notion-last-edited:` was updated to a fresh timestamp

Then use Notion MCP (`notion-fetch` on page B's URL) to verify Notion now reflects each of the 5 edited values.

### Step 9d: Revert page B via push
Restore page B's `.md` frontmatter to the original values recorded in Step 9b. Run push again:

`./notion-sync.exe push "./test-output/test database obsdiain complex"`

- **Pass criteria:** exit 0, `Pushed == Total`, `Conflicts: 0`.

Use Notion MCP to confirm page B's properties are back to their original values.

### Step 9e: Conflict detection
Pick a third page (page C, not page A from Step 4 and not page B). Manually edit page C's `.md` to set `notion-last-edited:` to a clearly stale value like `2020-01-01T00:00:00.000Z`. Don't change any other properties.

Run: `./notion-sync.exe push "./test-output/test database obsdiain complex"`

- **Pass criteria:**
  - Exit code is **non-zero**
  - Output contains `Conflicts: 1` and lists page C's filename under `- <filename>`
  - Notion's actual values for page C are unchanged (verify via MCP — push refuses conflicted files entirely)

After verifying, restore page C's `notion-last-edited:` to its real value (run a `refresh` to repair it cleanly: `./notion-sync.exe refresh "./test-output/test database obsdiain complex" --ids <page-C-id>`).

### Step 9f: Dry-run mode
Edit page B's `.md` again — change just the `select` value to something like `e2e-dryrun-test`. Run:

`./notion-sync.exe push "./test-output/test database obsdiain complex" --dry-run`

- **Pass criteria:**
  - Exit code 0
  - Output starts with `Pushing properties (dry run)...`
  - Output contains `Dry run:` and `Would push:` (not `Pushed:`)
  - Notion MCP fetch of page B shows the select value is **unchanged** from Step 9d (i.e., still the original value, not `e2e-dryrun-test`)

Revert page B's local edit by restoring the original select value (no push needed — Notion was never written to).

### Step 10: Revert Notion changes
Use Notion MCP tools to restore the page edited in Step 4 back to its original property values and content. This keeps the test database clean for the next run.

(Page B and page C are already back to their original states from Steps 9d and 9e.)

### Step 11: Clean up
If `--no-cleanup` was passed, **skip this step** and print `Step 11: Skipped (--no-cleanup)`.
Otherwise:
1. Delete only `test-output/test database obsdiain complex/` (not the entire `test-output/` directory).
2. If `test-output/` is now empty, delete it.

## Done
Summarize all step results in a table with columns: Step | Action | Result.

If all steps passed, output:

```
ALL TESTS PASSED
```

If any steps failed, output:

```
TESTS FAILED
```
followed by a bullet list of each failed step and what went wrong.
