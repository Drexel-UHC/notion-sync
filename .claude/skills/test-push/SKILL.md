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

## Phase 2 — Validation halts (DAG n21 series → n22a/b)

The validation gate classifies every `.md` against 8 outcomes (n21a–h). Any halt-class file aborts the **entire** run before any Notion write — all-or-nothing. `--force` bypasses the entire gate.

**🚨 NEVER push Page 4 in this phase.** Same rule as Phase 1 — Page 4's rich-text annotations are the phase-3 fixture. Every V step below either operates on a single page's `.md` (Pages 2 / 3 / 6 / 7) or explicitly excludes Page 4 from the folder. If you can't guarantee Page 4 is excluded, stop and re-run Step 1 (clean slate).

**Halt → exit 1.** A halted run prints `Halted: "<title>"` and an enumerated halt list to stdout, plus `push halted by validation gate (N halt(s))` to stderr, and exits **1**. Cancel (Phase 1) is exit 0; halt is exit 1. Don't conflate them.

### Step V0: Re-import for Phase 2

Phase 1's Step 4 deleted Pages 2–7 from disk. Phase 2 needs them back.

Run: `./notion-sync.exe import 35957008-e885-80c5-9e34-f4191fd83907 --output ./test-output/push-e2e`

- **Pass:** all 7 `.md` files present in `./test-output/push-e2e/notion-sync-test-database-push/`. `_database.json` and `AGENTS.md` also present.

### Step V1: Single conflict halts the run (n21d)

Edit Page 2's local `.md` (`Push- Conflict Subject A.md`): change `notion-last-edited` to `2020-01-01T00:00:00Z` (definitively stale). Don't touch any property values.

Isolate to Page 2: delete every other page's `.md` so the gate halts on Page 2 alone. Keep `_database.json` and `AGENTS.md`.

Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit code **1**
  - stdout contains `Halted:` and `[conflict]`
  - stderr contains `push halted by validation gate`
  - **Notion MCP fetch** of Page 2: `Score` is still **200** (canonical from `setup.md`), proving no UpdatePage fired.

**Revert local edit:** restore Page 2's `notion-last-edited` to its pre-edit value (or just re-run V0 to re-import fresh). No Notion revert needed — nothing was written.

### Step V2: Multi-halt aggregation (n22a)

Re-run V0 if needed for a clean folder. Then:

1. Stale-stamp Page 2's `notion-last-edited` → `2020-01-01T00:00:00Z`.
2. Stale-stamp Page 3's (`Push- Conflict Subject B.md`) `notion-last-edited` → `2020-01-01T00:00:00Z`.
3. Drop a `random-stray.md` in the folder with no frontmatter (just `# stray` body).
4. Delete every other page's `.md` (including Page 4 — critical) so the gate sees exactly Page 2 + Page 3 + the stray.

Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit code **1**
  - stdout enumerates **3 halts**: Page 2 `[conflict]`, Page 3 `[conflict]`, `random-stray.md` `[stray]`. Fix-once-rerun-once UX — all three listed in one pass, not "fix the first then come back."
  - `Halts: 3` line in summary.
  - **Notion MCP fetch** of Page 2 (`Score` 200) and Page 3 (`Score` 300): both unchanged.

**Revert:** re-run V0 to re-import fresh. No Notion revert needed.

### Step V3: Soft-deleted skip (n21b)

Re-run V0. Then:

1. Edit Page 6's local `.md` (`Push- Soft Deleted.md`): add `notion-deleted: true` to its frontmatter.
2. Delete every other page's `.md` (Page 4 critical) so the folder has only Page 6.

Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit code **0** — soft-deleted is skip, not halt.
  - stdout does NOT contain `Halted:`.
  - stdout shows `Total: 0` (deleted rows aren't counted as pushable) or equivalent skip-only summary.
  - **Notion MCP fetch** of Page 6: `Score` still **600**, all properties unchanged.

**Revert:** re-run V0 to re-import fresh.

### Step V4: Malformed YAML halts (n21g)

Re-run V0. Then:

1. Corrupt Page 7's local `.md` (`Push- Null Edges.md`): introduce an unclosed quoted string in the frontmatter (e.g. change a property value to `"unclosed`).
2. Delete every other page's `.md` (Page 4 critical) so the folder has only the broken Page 7.

Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit code **1**
  - stdout contains `Halted:` and `[malformed]`
  - The halt's reason mentions `YAML` (so the user knows to fix frontmatter, not hunt for a stray).
  - **Notion MCP fetch** of Page 7: unchanged (whatever its canonical state was).

**Revert:** re-run V0 to re-import fresh.

### Step V5: `--force` bypasses every halt class

Re-run V0. Then build the worst-case mixed folder:

1. Stale-stamp Page 2's `notion-last-edited` → `2020-01-01T00:00:00Z` (would trigger n21d).
2. Stale-stamp Page 3's `notion-last-edited` → `2020-01-01T00:00:00Z` (would trigger n21d).
3. Drop `random-stray.md` with no frontmatter (would trigger n21e).
4. Edit Page 2's `Score` locally → `2222` and Page 3's `Score` locally → `3333` (the actual writes we expect to land).
5. Delete every other page's `.md` (Page 4 **critical** — `--force` would otherwise push it and clobber phase-3's formatting fixture).

Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes --force`

- **Pass:**
  - Exit code **0**
  - stdout does NOT contain `Halted:` — gate fully bypassed.
  - stdout shows `Pushed: 2` (Page 2 + Page 3; the stray has no `notion-id` so `scanPushable` filters it).
  - **Notion MCP fetch** of Page 2: `Score` is now **2222**.
  - **Notion MCP fetch** of Page 3: `Score` is now **3333**.

**Revert (mandatory — V5 actually wrote to Notion):**
1. Restore Page 2's local `Score` → `200` and Page 3's local `Score` → `300`.
2. Drop the stray `random-stray.md`.
3. Restore `notion-last-edited` on Pages 2 & 3 (re-import or hand-fix). Either way, `notion-last-edited` must match Notion before the next push.
4. Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes` (no `--force` — proves the gate clears now that local state is sane).
5. **Notion MCP fetch** of Page 2 (`Score` 200) and Page 3 (`Score` 300): both back to canonical.

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

Notion MCP fetch of **every page touched by the run** — Pages 1–7 once any phase 2+ step has executed. Compare against **the canonical values hardcoded in `setup.md`** (per-page property tables) — NOT just the run's own Step 3 fetch. Within-run-only comparison is unsafe: if a prior run left a fixture drifted, a fresh Step 3 fetch records the drifted state and F1 then "matches" itself, silently passing while the bug persists. F1's job is to detect drift against the source-of-truth canonical, full stop.

**Phase 1 minimum — Page 1 (canary):**

| Property | Canonical value | Notion shape to assert |
|---|---|---|
| `Name` | `Push: Canary` | `title` text == canonical |
| `Description` | `Phase 1 fixture — confirmation gate cancel/proceed/dry-run.` | `rich_text` plain text == canonical |
| `Category` | `Research` | `select.name` == canonical |
| `Score` | `100` | `number` == canonical |
| `Due Date` | `2026-06-01` (date-only) | `date.start` == canonical AND `is_datetime` == `0`/`false` |

**Phase 2 additions — re-fetch Pages 2, 3, 6, 7 against `setup.md` canonicals.** V1/V2 stale-stamp Pages 2 & 3 (must end at `Score` 200 / 300). V3 marks Page 6 deleted in the local file only (Notion-side `Score` 600 must be untouched). V4 corrupts Page 7's local YAML (Notion-side unchanged). V5 actually writes to Notion — its mandatory revert step must restore Page 2 → 200 and Page 3 → 300 before F1 runs. Don't duplicate the canonical values here — read them from `setup.md`'s per-page sections (Pages 2/3/6/7).

If any property's Notion shape doesn't match the canonical, mark the run as TESTS FAILED and list the field + got/want values — don't try to auto-fix; investigate.

**Note on `Due Date`:** the `is_datetime` flag is load-bearing here. A common bug class is push promoting date-only properties to UTC datetimes (the original parser-roundtrip bug). F1 must assert *both* `start` matches AND `is_datetime` is false; matching only on the calendar day misses the type drift.

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
| V0   | Re-import for Phase 2                   | PASS   |
| V1   | Single conflict halts (n21d)            | PASS   |
| V2   | Multi-halt aggregation (n22a)           | PASS   |
| V3   | Soft-deleted skip (n21b)                | PASS   |
| V4   | Malformed YAML halts (n21g)             | PASS   |
| V5   | --force bypasses every halt class       | PASS   |
| F1   | Final state matches canonical (1–7)     | PASS   |
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
