---
name: test-single-datasource-db
description: Run integration test against the single-data-source test database (import, refresh, --ids, --force, push)
version: 2.2.0
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

### Step 2b: Verify duplicate title handling
Since PR #43, all synced files are named by UUID (e.g., `<notion-id>.md`), not by title — this trivially handles duplicates without a separate disambiguation pattern. Verify:
- Every `.md` filename matches the UUID format `<8>-<4>-<4>-<4>-<12>.md` (no title-based names like `Headings & Rich Text.md`)
- Each file's `notion-id` frontmatter matches its filename (minus the `.md`)
- Exactly 2 files contain `Name: Headings & Rich Text` (the duplicate-title test pages from `setup.md` Page 1 + Page 6)
- Both duplicate-title files have distinct `notion-id` values
- All files have `notion-database-id: 2fe57008-e885-8003-b1f3-cc05981dc6b0`
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
| `notion-frozen-at` | **Absent from every file** (regression guard — ref #54: the field was removed because it churned every entry's diff on every refresh) |
| `notion-url` | **Present in every file**, and the line matches `^notion-url:[ \t]*"?https://app\.notion\.com/p/[a-f0-9]{32}"?[ \t]*$` (regression guard — ref #63: legacy `www.notion.so/...` URLs must not leak through; the anchored pattern also catches a future writer that appends a query/fragment or drops the field entirely) |

Also check `_database.json` in the synced folder: its `"url"` line must match `"url":[ \t]*"https://app\.notion\.com/p/[a-f0-9]{32}"` (same regression guard, applied to the metadata writer's chokepoint).

- **Pass criteria:** First 3 keys found with valid non-null values; `notion-frozen-at` produces zero matches across all `.md` files; every `.md` file has a `notion-url` line matching the anchored canonical pattern, and `_database.json`'s `"url"` matches the anchored canonical pattern.

### Step 9: Verify file mtime preservation
For each synced `.md` file, compare the file's modification time (via `stat`) against the `notion-last-edited` value in its frontmatter.
- **Pass criteria:** File mtime matches `notion-last-edited` timestamp (within 1-second tolerance).

### Step 9b: Pick a push test page and snapshot originals
Pick a page **other than the one edited in Step 4** (call it page B). Read its local `.md` file and record the original values for these properties (you'll need them for Step 9d):

| Property type | Frontmatter key | Why |
|---|---|---|
| `title` | `Name` | Regression-protect ref #51 |
| `rich_text` | `Description` | Regression-protect 2000-char guard |
| `select` | `Category` (options: Research, Engineering, Design, Marketing) | |
| `number` | `Score` | |
| `date` | `Due Date` | |

These keys are fixed by the test DB schema (see `.claude/reference/test-databases/single-data-source/setup.md`). Pages 2, 3, or 5 work — all 5 properties populated, unique titles. Avoid page 1 (duplicate-title disambiguation), page 4 (null `Due Date`/`Website`/`Contact Email`), and the page edited in Step 4. If the chosen page has a null value for any required key, pick another or note the gap in the summary.

### Step 9c: Edit page B locally and push
Edit page B's `.md` file: change the 5 property values in the frontmatter to clearly distinguishable test values:
- `Name` (title), `Description` (rich_text): prefix with `e2e-push-test-`
- `Score` (number): `9999`
- `Due Date` (date): `2026-04-30`
- `Category` (select): pick a **different existing option** from the schema (Research / Engineering / Design / Marketing — pick whichever is not page B's current value). Do **not** invent a new string — Notion auto-creates select options on push and never garbage-collects them, which would leave cruft in the test DB across runs.

Run: `./notion-sync.exe push "./test-output/test database obsdiain complex"`

- **Pass criteria:**
  - Exit code 0
  - `Pushed >= 1` and `Pushed + Skipped == Total` (push writes every file with a non-empty property payload; `Skipped` only counts files where every property got filtered out by the schema)
  - Output does **not** contain `Conflicts:` or `Failed:` lines (these are only printed when count > 0; exit 0 already guarantees both are zero)
  - Page B's `.md` now has a `notion-last-pushed:` frontmatter key
  - Page B's `notion-last-edited:` is **newer than the value snapshotted in Step 9b**

Then use Notion MCP (`notion-fetch` on page B's URL) to verify Notion now reflects each of the 5 edited values.

### Step 9d: Revert page B via push
Restore page B's `.md` frontmatter to the original values recorded in Step 9b. Run push again:

`./notion-sync.exe push "./test-output/test database obsdiain complex"`

- **Pass criteria:** exit 0, `Pushed + Skipped == Total`, no `Conflicts:` or `Failed:` lines in output.

Use Notion MCP to confirm page B's properties are back to their original values.

### Step 9e: Conflict detection
Pick a third page (page C, not page A from Step 4 and not page B). Manually edit page C's `.md` to set `notion-last-edited:` to a clearly stale value like `2020-01-01T00:00:00.000Z`. Don't change any other properties.

Run: `./notion-sync.exe push "./test-output/test database obsdiain complex"`

- **Pass criteria:**
  - Exit code is **non-zero**
  - Output contains `Conflicts: 1` and lists page C's filename under `- <filename>`
  - Notion's actual values for page C are unchanged (verify via MCP — push refuses conflicted files entirely)

> **Expected behavior:** push continues past the conflict and writes every other non-conflicted file's properties back to Notion. This is by design (push writes every file with a non-empty payload). Page C is the only one that's actually skipped; all other pages get re-pushed with their current local values, which already match Notion. Don't be alarmed by the resulting `Pushed:` count.

### Step 9e2: Force-override the conflict
Page C still has its stale `notion-last-edited:` from the previous step. Re-run push with `--force` to confirm the override path bypasses the conflict check:

> **Expected behavior (continued):** like Step 9e, force push writes every non-empty file. The point is to verify the override path on page C; the other 11 writes are no-ops on data values but still bump Notion timestamps. The resulting `Pushed:` count will be `Total`, not `1`.

`./notion-sync.exe push "./test-output/test database obsdiain complex" --force`

- **Pass criteria:**
  - Exit code 0
  - Output starts with `Force pushing properties (ignoring conflicts)...`
  - No `Conflicts:` line in output (force skips the conflict check entirely)
  - `Pushed >= 1`, `Pushed + Skipped == Total`
  - Page C's `notion-last-edited:` is now refreshed to a real Notion timestamp (since `--force` proceeded with the push and `updateAfterPush` rewrote it)

Verify via Notion MCP that page C's properties match the local frontmatter (since page C wasn't otherwise edited, this should be a no-op for property values, but the timestamp should be current).

### Step 9e3: Repair page C state
After the force push, page C's local frontmatter should already match Notion (in property values). Run a refresh to confirm:

`./notion-sync.exe refresh "./test-output/test database obsdiain complex" --ids <page-C-id>`

- **Pass criteria:** exit 0, `updated = 1` (expected — see note below).

> **Why `updated = 1` instead of 0:** Notion's `UpdatePage` response returns `last_edited_time` quantized to whole minutes (e.g., `22:43:00.000Z`), but `QueryDataSource` (used by refresh) returns the precise timestamp (`22:43:25.xxxZ`). After every push, local frontmatter has the rounded value; refresh sees the drift and rewrites the file. This is a known binary quirk independent of this PR.

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

(Page B and page C are already back to their original states from Steps 9d and 9e3.)

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
