---
name: test-standalone-page
description: Run integration test for standalone page import/refresh/list
version: 2.0.0
args: "[--verbose] [--no-cleanup]"
---

# Integration Test: Standalone Page Sync

Run an integration test using the **standalone test page** (`31357008-e885-80c3-90f4-d148f0854bba`).

- **Notion URL:** https://www.notion.so/drexel-climate/Test-Notion-sync-single-page-31357008e88580c390f4d148f0854bba
- **Reference:** `.claude/reference/v0.3/features/page-level-sync.md`

## Mode

Check if `--verbose` was passed in the skill args.

- **Default (concise):** Run all steps automatically without asking questions. Print a one-line status per step as you go (e.g., `Step 1: Clean slate... done`). At the end, print the summary table and pass/fail result. Do NOT use `AskUserQuestion` at all.
- **Verbose (`--verbose`):** Interactive mode. Use `AskUserQuestion` with selectable options before every command. Show exact CLI calls, wait for confirmation, and provide detailed output after each action. Never jump ahead.
- **No cleanup (`--no-cleanup`):** Skip Step 7 (cleanup). The test output is left on disk so you can examine it manually.

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
- If `test-output/pages/` exists: delete only that subfolder (in verbose mode, ask first).
- If it doesn't exist: skip.
- **Important:** Do NOT delete `test-output/` itself or any other subfolders.

### Step 2: Import standalone page
Run: `./notion-sync.exe import 31357008-e885-80c3-90f4-d148f0854bba --output ./test-output`
- **Pass criteria:** Exit code 0, output shows `Status: created`.

### Step 3: Verify folder structure
Check that the import created the correct structure:
- Folder `test-output/pages/Test - Notion sync - single page_31357008/` exists
- `_page.json` exists inside it with `pageId` = `31357008-e885-80c3-90f4-d148f0854bba`
- A `.md` file exists inside it
- The `.md` file has `notion-id: 31357008-e885-80c3-90f4-d148f0854bba` in frontmatter
- The `.md` file does NOT have `notion-database-id` in frontmatter
- The `.md` file contains expected content (at least a heading and a code block)
- **Pass criteria:** All checks pass.

### Step 4: No-op refresh
Run: `./notion-sync.exe refresh "test-output/pages/Test - Notion sync - single page_31357008"`
- **Pass criteria:** Output shows `Status: skipped`.

### Step 5: Force refresh
Run: `./notion-sync.exe refresh "test-output/pages/Test - Notion sync - single page_31357008" --force`
- **Pass criteria:** Output shows `Status: updated`.

### Step 6: List
Run: `./notion-sync.exe list ./test-output`
- **Pass criteria:** Output contains `Test - Notion sync - single page` in the "Synced pages" section with the correct page ID.

### Step 7: Clean up
If `--no-cleanup` was passed, **skip this step** and print `Step 7: Skipped (--no-cleanup)`.
Otherwise:
1. Delete `test-output/pages/` folder.
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
