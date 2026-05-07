---
name: test-push
description: E2E test for the push command's full contract against real Notion — confirmation gate, validation halts, cell-level diff, and run summary. Grows phase-by-phase as the v1.4.0 push redesign lands.
version: 1.0.0
args: "[--verbose] [--no-cleanup]"
---

# E2E Test: Push Command

End-to-end test of `notion-sync push` against a real Notion database. Covers behavior the unit and CLI-e2e tests can't observe — **what Notion actually receives** (or, for negative cases, what it *doesn't* receive).

Reference: epic [issue #55](https://github.com/Drexel-UHC/notion-sync/issues/55), DAG `.context/features/push/dag-v1.4.0.mmd`, gate spec `.context/features/push/features/confirmation-gate.md`.

## Why this skill exists separately from `/test-single-datasource-db`

| Concern | Skill |
|---|---|
| Single-source schema integration (import / refresh / --ids / --force / push runs) | `/test-single-datasource-db` |
| Push command **contract** (gate, halts, cell-level, summary) — grows with v1.4.0 epic | **`/test-push`** (this skill) |

`/test-single-datasource-db` keeps a sanity-only push step. Deep push contract lives here so neither skill turns into a kitchen sink as the push feature grows.

## Phase coverage

Step-group letters map to the four-phase v1.4.0 push DAG. Each phase PR appends its group; existing steps don't get renumbered.

| Phase | DAG nodes | Step prefix | Status |
|---|---|---|---|
| 1: Confirmation gate | n12b → n13 → n13a | **G** | ✅ included (PR #77) |
| 2: Validation halts | n21 series → n22a/b | **V** | ⏳ TODO |
| 3: Cell-level push + verify | n31 → n37 | **C** | ⏳ TODO |
| 4: Run summary JSON | n41 | **S** | ⏳ TODO |

When adding a phase: append a new `## Phase N — <name>` section with new step IDs (`V1`, `V2`, ... or `C1`, ...). Don't modify existing G/V/C/S blocks unless the phase explicitly redefines that contract.

## Test database

- **DB ID:** `35957008-e885-80c5-9e34-f4191fd83907` (dedicated push e2e fixture DB — `notion-sync-test-database-push`)
- **Notion URL:** https://www.notion.so/35957008e88580c59e34f4191fd83907
- **Schema reference:** `.claude/reference/test-databases/push-e2e/setup.md`
- **Output folder for this skill:** `test-output/push-e2e/` — distinct from `/test-single-datasource-db`'s folder so the two skills can run side-by-side without collision.

Imported fresh on every run (Step 2). Reverted to original state on every run (final step).

## Mode

Check the skill args:

- **Default (concise):** Run all steps automatically, no questions. One-line status per step (e.g., `Step G1: Cancel without --yes... PASS`). Print summary table + final pass/fail at the end. Do NOT use `AskUserQuestion`.
- **`--verbose`:** Interactive. `AskUserQuestion` with selectable options before every command. Show exact CLI calls, wait for confirmation, show `git diff --stat` + a `Git Analysis:` section after every action.
- **`--no-cleanup`:** Skip the final cleanup step. Leave `test-output/push-e2e/` on disk for manual inspection.

## State invariants (re-runnability)

This skill must be **idempotent** — runnable cleanly on a fresh checkout AND immediately after a previous run. Every step that mutates Notion has a corresponding revert step. If a step fails mid-run, the cleanup section's revert + summary will tell you what state needs manual fix-up.

The push e2e DB is dedicated to this skill, but the `setup.md` "do not edit" conventions still apply across runs and across phases. Don't leave behind:
- Locally-edited `.md` files with non-original property values (Step 3 documents the originals; F1 verifies them)
- Notion-side drift on any of the 7 fixtures — especially Page 4's rich-text annotations (the phase-3 regression target; see `setup.md` "Things to NEVER do")
- Auto-created `select` / `multi_select` options. Use only the spec'd options: Research / Engineering / Design / Marketing for `Category`; alpha / beta / gamma / delta for `Tags`.

---

## Setup steps (run for every skill invocation)

### Step 0: Build

Run: `go build ./cmd/notion-sync`

- **Pass:** exit 0, `notion-sync.exe` exists at repo root.

### Step 1: Clean slate

- If `test-output/push-e2e/` exists, delete it (in `--verbose` mode, ask first).
- Don't touch `test-output/` itself or any sibling folders — `/test-single-datasource-db` may be running there.

### Step 2: Fresh import

Run: `./notion-sync.exe import 35957008-e885-80c5-9e34-f4191fd83907 --output ./test-output/push-e2e`

- **Pass:** created > 0, exit 0, `_database.json` present at `./test-output/push-e2e/notion-sync-test-database-push/_database.json`.

### Step 3: Snapshot the canary fixture (Page 1 — `Push: Canary`)

The push DB has 7 fixed fixtures. See [`.claude/reference/test-databases/push-e2e/setup.md`](../../reference/test-databases/push-e2e/setup.md) for the full per-fixture reference. Phase 1's canary is **Page 1 — `Push: Canary`** (https://www.notion.so/35957008e885813a886bcbb6dd7c1598).

Use Notion MCP `notion-fetch` to snapshot its property values — needed for negative assertions (cancel paths) and the F1 final state check.

| Property | Frontmatter key | Original value | Notes |
|---|---|---|---|
| `title` | `Name` | `Push: Canary` | |
| `rich_text` | `Description` | `Phase 1 fixture — confirmation gate cancel/proceed/dry-run.` | |
| `select` | `Category` | `Research` | Never invent options — use only Research / Engineering / Design / Marketing |
| `number` | `Score` | `100` | Phase 1 canary — gets edited to 9999 / 8888 across G1-G4 |
| `date` | `Due Date` | `2026-06-01` | |

**🚨 NEVER use Page 4 (`Push: Formatting Fixture`) in phase 1.** Its rich-text annotations are the phase-3 fixture; touching it from a phase-1 step pollutes phase-3 verification.

Record the page `notion-id` and the 5 values in the run notes.

### Step 4: Isolate canary for phase 1 (delete Pages 2-7 from local folder)

The push command iterates **every** `.md` file in the folder and sends each one's full property payload to Notion. Until phase 3 (cell-level diff) lands, this means a single `push --yes` against the full folder strips Page 4's rich-text annotations on Notion — silently corrupting the phase-3 fixture.

**Phase 1 only needs Page 1.** Delete the other six pages' `.md` files so the push queue contains exactly the canary:

- Keep: `Push- Canary.md` (the `:` in `Push: Canary` is sanitized to `-` on import)
- Delete: every other `.md` file in `./test-output/push-e2e/notion-sync-test-database-push/`
- Don't touch: `_database.json`, `AGENTS.md`

Verify: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --dry-run` should show `Push queue (1 file)` listing only the canary.

When phase 3 lands and cell-level diff is in place, this step becomes optional — until then it's the cheapest way to keep the phase-3 fixture intact across phase-1 runs.

---

## Phase 1 — Confirmation gate (DAG n12b → n13 → n13a)

### Step G1: Cancel without `--yes` — verify no Notion write

Edit the canary page's local `.md`: change `Score` to `9999` (single-property edit is enough — push currently sends every populated property, but only `Score` is the canary here).

Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push"`

- **Pass:**
  - Exit code **0** (cancel is not a failure)
  - Combined output contains `Cancelled` and `--yes`
  - Combined output contains `Push queue (` (preview fired before the gate)
  - Combined output does **NOT** contain `Pushing properties to Notion...` (gate fired before push flow)
  - **Notion MCP fetch** of the canary page: `Score` is the **original snapshot value**, NOT 9999. (This is the critical real-Notion negative assertion — proves the gate fires before any API write.)

**Revert local edit before next step:** restore the canary page's `Score` to the original snapshot value.

### Step G2: `--yes` proceeds and writes to Notion

Edit the canary page's local `.md`: change `Score` to `9999`.

Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit code 0
  - Combined output contains `Proceeding (--yes).`
  - `Pushed >= 1`, `Pushed + Skipped == Total`
  - No `Conflicts:` or `Failed:` lines
  - The canary page's local `.md` now has `notion-last-pushed:`
  - **Notion MCP fetch** of the canary page: `Score` is now `9999`.

### Step G3: Revert via push (`--yes`)

Restore the canary page's local `.md`: change `Score` back to the original snapshot value.

Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit code 0
  - **Notion MCP fetch** of the canary page: `Score` matches the original snapshot value again.

### Step G4: `--dry-run` skips the gate AND doesn't write

Edit the canary page's local `.md`: change `Score` to `8888`.

Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --dry-run`

- **Pass:**
  - Exit code 0
  - Output starts with `Pushing properties (dry run)...`
  - Output contains `Dry run:` and `Would push:` (not `Pushed:`)
  - Output does **NOT** contain `Push queue (` (gate skipped — `--dry-run` short-circuits before the preview)
  - **Notion MCP fetch** of the canary page: `Score` is the original snapshot value, NOT 8888 (dry-run touched no API state).

**Revert local edit before next step:** restore the canary page's local `Score` to the original snapshot value. No push needed — Notion was never written to.

---

## Phase 2 — Validation halts (TODO — added by phase 2 PR)

When phase 2 lands (n21 series + n22a halt aggregation), this section gets steps `V1`...`Vn`. Expected coverage:
- Multi-file conflict aggregation: every halt reason listed, **nothing** pushed.
- Single conflict halts the run (current behavior is per-row partial — phase 2 changes this).
- Non-row file types (AGENTS.md, notion-deleted) classified correctly.

## Phase 3 — Cell-level push (TODO — added by phase 3 PR)

Steps `C1`...`Cn`. Expected coverage (the original #55 symptom):
- Edit one field locally; push.
- **Other fields' rich-text formatting (bold, links, mentions) survives on Notion** — the original epic motivation.
- Untouched fields don't bump `last_edited_time` on Notion's side beyond the changed cell.

## Phase 4 — Run summary JSON (TODO — added by phase 4 PR)

Steps `S1`...`Sn`. Expected coverage:
- JSON schema matches `dag-v1.4.0.mmd` header comment.
- `status` enum: `clean` / `partial` / `halted` / `cancelled`.
- Exit code matrix: `0` for clean/cancelled, `1` for partial/halted.
- Per-row `pushed` / `skippedNoOp` / `skippedNonRow` / `failed` / `halted` arrays populated correctly.

---

## Final steps

### Step F1: Final state verification

Notion MCP fetch of the canary page. Every snapshotted property must match the **Step 3 originals**. If anything drifted, mark the run as TESTS FAILED and list the field + got/want values — don't try to auto-fix; investigate.

### Step F2: Cleanup

If `--no-cleanup` was passed: skip and print `Step F2: Skipped (--no-cleanup)`.

Otherwise:
1. Delete `test-output/push-e2e/`.
2. If `test-output/` is now empty, delete it.
3. Don't touch `notion-sync.exe` at the repo root — other skills use it.

---

## Done

Print a summary table:

```
| Step | Action                                  | Result |
|------|-----------------------------------------|--------|
| 0    | Build                                   | PASS   |
| 1    | Clean slate                             | PASS   |
| 2    | Fresh import                            | PASS   |
| 3    | Snapshot the canary page                | PASS   |
| 4    | Isolate canary (delete Pages 2-7 .md)   | PASS   |
| G1   | Cancel without --yes (no Notion write)  | PASS   |
| G2   | --yes proceeds, Notion updated          | PASS   |
| G3   | Revert via push --yes                   | PASS   |
| G4   | --dry-run skips gate, Notion untouched  | PASS   |
| F1   | Final state matches snapshot            | PASS   |
| F2   | Cleanup                                 | PASS   |
```

If all pass:

```
ALL TESTS PASSED
```

If any failed:

```
TESTS FAILED
```

followed by a bullet list of each failed step + what went wrong + whether Notion state needs manual repair.
