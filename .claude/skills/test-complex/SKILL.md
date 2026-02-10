---
name: test-complex
description: Run guided integration test against the complex test database (import, refresh, --ids, --force)
version: 1.3.0
---

# Integration Test: Complex Test Database

Walk me through a step-by-step manual integration test using the **complex test database** (`2fe57008-e885-8003-b1f3-cc05981dc6b0`).

- **Notion URL:** https://www.notion.so/2fe57008e8858003b1f3cc05981dc6b0
- **Reference:** `.claude/reference/v0.1/test-databases/complex/`

## Interaction Rules

1. **Always use `AskUserQuestion` with selectable options** — never ask questions as plain text. Use arrow-key-friendly options so I can just navigate and press enter.
2. **Before running any command**, show the exact CLI call and use `AskUserQuestion` to confirm.
3. **Never jump ahead** — always wait for my selection between steps.
4. **Git Analysis after every action** — after every action (deletion, import, refresh, edit, etc.), run `git diff --stat` and provide a **"Git Analysis:"** section summarizing what git sees (e.g., "Confirmed deletion of 26 tracked files", "25 new untracked files added", "1 file modified: X.md", "No changes detected"). This is inline with each step, not a separate step.

## Steps

### Step 1: Clean slate
Check if `test-output/` folder exists.
- If it exists: use `AskUserQuestion` with options like "Delete it" / "Keep it and skip".
- If it doesn't exist: tell me and use `AskUserQuestion` to proceed.
- **Git Analysis:** after deletion, run `git diff --stat` and report tracked file changes.

### Step 2: Fresh import
Show me the command:
`./notion-sync.exe import 2fe57008-e885-8003-b1f3-cc05981dc6b0 --output ./test-output`
Use `AskUserQuestion` to confirm execution. Then run it, report results (created/updated/skipped), list files created.
- **Git Analysis:** run `git diff --stat` and `git status --short` to report new/changed files.

### Step 3: No-op refresh
Show me the command:
`./notion-sync.exe refresh "./test-output/test database obsdiain complex"`
Use `AskUserQuestion` to confirm. Nothing should change — expect all skipped. Report results.
- **Git Analysis:** run `git diff --stat` to confirm no new changes appeared.

### Step 4: Make a change via Notion MCP
Use the Notion MCP tools to make **both** a property edit **and** a content edit to one of the pages in the test database. Property-only edits may not bump Notion's `last_edited_time`, so you must also edit the page content (e.g., append a test line like "<!-- sync-test -->") to ensure the timestamp changes and the incremental refresh can detect it.
Tell me what you changed and which page.
Use `AskUserQuestion` to confirm when ready to proceed.

### Step 5: Incremental refresh
Show me the command:
`./notion-sync.exe refresh "./test-output/test database obsdiain complex"`
Use `AskUserQuestion` to confirm. Verify the changed page was picked up as updated. Report results.
- **Git Analysis:** run `git diff --stat` and `git diff` — evaluate whether the diff matches only the page that was edited.

### Step 6: Test --ids flag
Pick a specific page ID from the synced files. Show me the command:
`./notion-sync.exe refresh "./test-output/test database obsdiain complex" --ids <page-id>`
Use `AskUserQuestion` to confirm. Should show updated: 1, skipped: 0. Report results.
- **Git Analysis:** run `git diff --stat` and `git diff` — evaluate changes.

### Step 7: Test --force flag
Show me the command:
`./notion-sync.exe refresh "./test-output/test database obsdiain complex" --force`
Use `AskUserQuestion` to confirm. Should re-download all pages (updated: N, skipped: 0). Report results.
- **Git Analysis:** run `git diff --stat` and `git diff` — all files should appear but content should be identical or near-identical.

## Done
Summarize all step results in a table with columns: Step | Action | Result | Git Analysis. Flag any failures.