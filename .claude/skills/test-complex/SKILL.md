---
name: test-complex
description: Run guided integration test against the complex test database (import, refresh, --ids, --force)
version: 1.0.0
---

# Integration Test: Complex Test Database

Walk me through a step-by-step manual integration test using the **complex test database** (`2fe57008-e885-8003-b1f3-cc05981dc6b0`). Reference: `.claude/reference/v0.1/test-databases/complex/`

Execute each step one at a time. After each step, tell me what happened, what to review, and ask me when I'm ready to proceed to the next step (with a brief description of what's next).

## Steps

### Step 1: Clean slate
Check if `test-output/` folder exists. If it does, ask me to confirm deletion before removing it. If it doesn't exist, skip to Step 2.

### Step 2: Fresh import
Run: `./notion-sync.exe import 2fe57008-e885-8003-b1f3-cc05981dc6b0 --output ./test-output`
Report the results (created/updated/skipped counts).

### Step 3: Review import
Tell me the folder path and list the files created. Ask me to review them and confirm when ready.

### Step 4: No-op refresh
Run: `./notion-sync.exe refresh "./test-output/test database obsdiain complex"`
Nothing should change since no updates were made. Report results — expect all skipped.

### Step 5: Make a change via Notion MCP
Use the Notion MCP tools to make a small edit to one of the pages in the test database (e.g., update a property or add text). Tell me what you changed and which page.

### Step 6: Incremental refresh
Run: `./notion-sync.exe refresh "./test-output/test database obsdiain complex"`
Verify the changed page was picked up as updated. Report results.

### Step 7: Test --ids flag
Pick a specific page ID from the synced files. Run:
`./notion-sync.exe refresh "./test-output/test database obsdiain complex" --ids <page-id>`
Should show updated: 1, skipped: 0 (force-freezes regardless of timestamp). Report results.

### Step 8: Test --force flag
Run: `./notion-sync.exe refresh "./test-output/test database obsdiain complex" --force`
Should re-download all pages (updated: N, skipped: 0). Report results.

## Done
Summarize all step results in a table and flag any failures.
