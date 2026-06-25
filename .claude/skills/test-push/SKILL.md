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
| 2: Validation halts | n21 series → n22a/b | **V** | ✅ included |
| 3a: Per-cell diff + skip no-op rows | n31 → n32a/b/c | **C** | ✅ included (PR #97) |
| 3b/3c: Per-field payload + store-verify + restamp + auth halt | n33 → n34d/e → n35a/n36a; n34h | **C** | ✅ included (PR #98) |
| 3d: Rich-text un-skip (rich_text re-included in diff/write/verify) | n31 rich_text re-include (#95/#99) | **R** | ✅ included (#100) |
| 4: Run summary JSON | n41 | **S** | ✅ included (PR #101) |

When adding a phase: append a new `## Phase N — <name>` section with new step IDs (`V1`, `V2`, ... or `C1`, ...). Don't modify existing G/V/C/S/R blocks unless the phase explicitly redefines that contract. **Phase 3d (#99 rich-text un-skip) is one such redefinition:** it flips `C3` from a no-op assertion to a positive-push assertion and drops the "rich_text excluded from the diff" notes in the 3a/3b prose — those edits are intentional, not drift.

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
- Notion-side drift on any of the 8 fixtures — especially Page 4's rich-text annotations (the phase-3 regression target; see `setup.md` "Things to NEVER do")
- Auto-created `select` / `multi_select` options. Use only the spec'd options: Research / Engineering / Design / Marketing for `Category`; alpha / beta / gamma / delta for `Tags`.

## Notion-read strategy (performance — issue #104)

Wall-clock is dominated by serial Notion round-trips. Two rules cut the read count with **no coverage loss** — apply them everywhere the per-step text says "Notion MCP fetch":

- **B — one `notion-query-data-sources` sweep instead of per-page `notion-fetch` for multi-page *scalar* checks.** SQL mode takes the **data source URL** as the table name — for this DB that is `collection://35957008-e885-8068-9080-000b89086bb3` (the **Data Source ID** from `setup.md:7`, **not** the Database ID `35957008-e885-80c5-9e34-f4191fd83907`; the two are different values, so a bare DB ID in `FROM "collection://…"` errors). So the sweep is `SELECT * FROM "collection://35957008-e885-8068-9080-000b89086bb3"` — one call returns every row's scalar columns: `Name`, `Description` (plain text), `Category`, `Score`, `Due Date` (start), `Tags`. Use it for F1's canonical sweep and any mid-run multi-page scalar check (V2, V5). **Read-after-write timing:** the mid-run sweeps read live Notion right after a push, but that is the **same** read-after-write window the per-page `notion-fetch` calls they replace already had — no new flake source. **Carve-out — the query flattens rich_text to plain text (zero annotations) and (pending first-run confirmation) is assumed not to project `is_datetime`,** so keep a full `notion-fetch` for exactly three things: **Page 4** rich-text byte-identity, **Page 8** rich-text byte-identity, and the **`Due Date` `is_datetime`** flag. (**TODO — confirm on the first real run whether SQL mode projects `is_datetime`; if it does, fold Page 1's date-type check into the sweep and drop that full fetch.** Until confirmed, assert `is_datetime` via a full fetch regardless.)
- **C — trust the push read-back store-verify for *positive* writes.** Post-3b every successful push re-fetches and verifies each sent field against what Notion stored (n34d) and reports it in the run-summary JSON. So an independent `notion-fetch` to re-confirm a write that *succeeded* is redundant — assert instead on `Pushed: N` + no `Failed:`/`Conflicts:` line + the summary's `pushed[].fields` (captured in the S-steps). **Keep an independent fetch for what the CLI can't vouch for:** no-write / negative paths (G1 cancel, G4 dry-run, V1/V2/V4 halts, C9 auth) must *independently* prove Notion did **not** change; **Page 4's `Description`** is never *pushed* (C7 edits only `Score`) so store-verify never covers it; and the **R-step round-trip / clear proofs (R2 / R5 / R6)** are independent round-trip checks, **not** redundant positive confirms — never drop them. **Tradeoff:** leaning on store-verify means some positive-write reverts (e.g. C2, C8) have no independent check until F1's end-of-run sweep — an accepted cost, but a revert that silently doesn't land surfaces at F1, not at the step, so failure-localization is coarser.

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

The push DB has 8 fixed fixtures (Page 8 added for Phase 3d rich-text round-trip). See [`.claude/reference/test-databases/push-e2e/setup.md`](../../reference/test-databases/push-e2e/setup.md) for the full per-fixture reference. Phase 1's canary is **Page 1 — `Push: Canary`** (https://www.notion.so/35957008e885813a886bcbb6dd7c1598).

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

### Step 4: Isolate canary for phase 1 (delete Pages 2-8 from local folder)

The push command iterates **every** `.md` file in the folder and sends each changed row's payload to Notion. A pre-3a full-folder `push --yes` would strip Page 4's rich-text annotations — and post-#99 a `--force` full-folder push still re-serializes Page 4's equation. Isolation keeps phase 1 clean of both.

**Phase 1 only needs Page 1.** Delete the other seven pages' `.md` files so the push queue contains exactly the canary:

- **Filename convention:** the importer writes `.md` files named `<notion-id>.md` (e.g. `35957008-e885-813a-886b-cbb6dd7c1598.md`), NOT title-derived names. Every "edit Page X's `.md`" step below means "edit the file whose name matches Page X's notion-id from the `setup.md` fixture table."
- Keep: `35957008-e885-813a-886b-cbb6dd7c1598.md` (Page 1 — Canary)
- Delete: the seven `.md` files for Pages 2–8 (notion-ids in `setup.md`; Page 8 = `38a57008-e885-81c3-88c4-eec03393dcad.md`)
- Don't touch: `_database.json`, `AGENTS.md`

Verify: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --dry-run` should show `Push queue (1 file)` listing only the canary.

Phase 3a (PR #97) has landed: the cell-level diff now skips unchanged rows, so a full-folder push **without `--force`** no longer clobbers Page 4 — this isolation step is **optional** for non-force runs. A `--force` run still re-sends every row, so keep the isolation for any `--force` step (see C6).

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
  - *(No independent fetch — the push store-verifies the write against Notion; **S1** asserts the summary's `pushed[].fields:["Score"]`. See Notion-read strategy C.)*

### Step G3: Revert via push (`--yes`)

Restore the canary page's local `.md`: change `Score` back to the original snapshot value.

Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit code 0
  - `Pushed: 1` (the revert is a real write), no `Conflicts:`/`Failed:` line
  - *(No independent fetch — store-verify covers the write. **G4's kept negative fetch transitively confirms the revert landed**: G4 expects `Score` = original snapshot value, which is only true if this revert reached Notion. See Notion-read strategy C.)*

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

## Phase 2 — Validation halts (DAG n21 series → n22a)

The validation gate classifies every `.md` against 8 outcomes (n21a–h). Any halt-class file aborts the **entire** run before any Notion write — all-or-nothing. `--force` bypasses the entire gate.

**🚨 NEVER push Page 4 in this phase.** Same rule as Phase 1 — Page 4's rich-text annotations are the phase-3 fixture. Every V step below either operates on a single page's `.md` (Pages 2 / 3 / 6 / 7) or explicitly excludes Page 4 from the folder. If you can't guarantee Page 4 is excluded, stop and re-run Step 1 (clean slate).

**Halt → exit 1.** A halted run prints `Halted: "<title>"` and an enumerated halt list to stdout, plus `push halted by validation gate (N halt(s))` to stderr, and exits **1**. Cancel (Phase 1) is exit 0; halt is exit 1. Don't conflate them.

**Filename quick reference** (from `setup.md`; importer writes `<notion-id>.md`):

| Page | notion-id (filename without `.md`) |
|---|---|
| 1 — Canary | `35957008-e885-813a-886b-cbb6dd7c1598` |
| 2 — Conflict A | `35957008-e885-811d-ae4b-eb73607cc037` |
| 3 — Conflict B | `35957008-e885-8141-9e44-ef7c58e4a487` |
| 4 — Formatting (NEVER touch) | `35957008-e885-8192-ab0f-c75e6a011b10` |
| 5 — Cell-Level | `35957008-e885-815e-8e73-ea79c22f96d4` |
| 6 — Soft Deleted | `35957008-e885-81e0-83c1-ff9fb4dbfda5` |
| 7 — Null Edges | `35957008-e885-814f-9f19-c401d454b08d` |
| 8 — Rich-Text Roundtrip | `38a57008-e885-81c3-88c4-eec03393dcad` |

### Step V0: Re-import for Phase 2

Phase 1's Step 4 deleted Pages 2–8 from disk. Phase 2 needs them back.

Run: `./notion-sync.exe import 35957008-e885-80c5-9e34-f4191fd83907 --output ./test-output/push-e2e`

- **Pass:** all 8 `.md` files present in `./test-output/push-e2e/notion-sync-test-database-push/`. `_database.json` and `AGENTS.md` also present.

### Step V1: Single conflict halts the run (n21d)

Edit Page 2's local `.md` (`35957008-e885-811d-ae4b-eb73607cc037.md`): change `notion-last-edited` to `2020-01-01T00:00:00Z` (definitively stale). Don't touch any property values.

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
2. Stale-stamp Page 3's (`35957008-e885-8141-9e44-ef7c58e4a487.md`) `notion-last-edited` → `2020-01-01T00:00:00Z`.
3. Drop a `random-stray.md` in the folder with no frontmatter (just `# stray` body).
4. Delete every other page's `.md` (including Page 4 — critical) so the gate sees exactly Page 2 + Page 3 + the stray.

Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit code **1**
  - stdout enumerates **3 halts**: Page 2 `[conflict]`, Page 3 `[conflict]`, `random-stray.md` `[stray]`. Fix-once-rerun-once UX — all three listed in one pass, not "fix the first then come back."
  - Summary line shows a halts count of **3** (the renderer prints `Halts:` followed by aligned whitespace then the number — match loosely on the count, not the spacing).
  - **One `notion-query-data-sources` sweep** (not two per-page fetches): Page 2 `Score` = **200** and Page 3 `Score` = **300** — both unchanged, proving no UpdatePage fired. See Notion-read strategy B.

**Revert:** re-run V0 to re-import fresh. No Notion revert needed.

### Step V3: Soft-deleted skip (n21b)

Re-run V0. Then:

1. Edit Page 6's local `.md`: add `notion-deleted: true` to its frontmatter.
2. Keep Page 5's `.md` alongside Page 6 — without a non-deleted file, the queue ends up empty and the CLI short-circuits with `"Nothing to push: no synced .md files in folder."` *before* the validation gate fires (so n21b never gets exercised). Page 5 is the safest neighbor: clean by default, not the phase-3 fixture, and pushing it round-trips its current canonical values without drift.
3. Delete the other 6 pages' `.md` (Page 4 critical) so the folder has Pages 5 + 6 only.

Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit code **0** — soft-deleted is skip, not halt.
  - stdout does NOT contain `Halted:`.
  - stdout shows a Pushed count of **1** (Page 5 round-trip) and a summary line, NOT `"Nothing to push"` (gate ran). The renderer aligns the `Pushed:` line with whitespace — match on the count, not the spacing.
  - **Notion MCP fetch** of Page 6: `Score` still **600**, all properties unchanged (skip path proven).
  - **Notion MCP fetch** of Page 5: `Score` still **500** (round-trip with no value drift).

**Revert:** re-run V0 to re-import fresh. Note: V3's push bumps Page 5's Notion `last_edited_time` (the round-trip is a real write). The next V0 re-import realigns local timestamps, so this is harmless across runs — just don't be surprised if Page 5's `last_edited_time` keeps drifting forward across V3 invocations.

### Step V4: Malformed YAML halts (n21g)

Re-run V0. Then:

1. Corrupt Page 7's local `.md` (`35957008-e885-814f-9f19-c401d454b08d.md`): introduce an unclosed quoted string in the frontmatter (e.g. change a property value to `"unclosed`).
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
  - stdout shows a Pushed count of **2** (Page 2 + Page 3; the stray has no `notion-id` so `scanPushable` filters it). Match on the count, not the spacing.
  - **One `notion-query-data-sources` sweep** (not two per-page fetches): Page 2 `Score` = **2222**, Page 3 `Score` = **3333**. See Notion-read strategy B.

**Revert (mandatory — V5 actually wrote to Notion).** Pick one branch and follow it in order — the two branches need different orderings because re-importing rewrites the entire `.md`, including any local Score edit.

*Branch A — re-import (faster, recommended):*
1. Drop the stray `random-stray.md`.
2. Re-run V0 (`./notion-sync.exe import ...`). This pulls Notion's current state — Score `2222`/`3333` and matching `notion-last-edited` — into local Pages 2 & 3.
3. Edit Page 2's local `Score` → `200` and Page 3's local `Score` → `300`. Timestamps already match Notion, so the gate will pass.
4. Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes` (no `--force` — proves the gate clears now that local state is sane).
5. **One `notion-query-data-sources` sweep** (not two per-page fetches): Page 2 `Score` = 200 and Page 3 `Score` = 300 — both back to canonical. See Notion-read strategy B.

*Branch B — hand-fix (no re-import):*
1. Restore Page 2's local `Score` → `200` and Page 3's local `Score` → `300`.
2. Drop the stray `random-stray.md`.
3. Hand-edit Page 2's & Page 3's `notion-last-edited` to match Notion's current `last_edited_time` (fetch via Notion MCP). Required for the gate to clear without `--force`.
4. Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`.
5. **One `notion-query-data-sources` sweep** (not two per-page fetches): Page 2 `Score` = 200 and Page 3 `Score` = 300 — both back to canonical. See Notion-read strategy B.

⚠️ Don't mix branches — restoring Score first and then re-importing in step 3 will overwrite your Score edit and round-trip `2222`/`3333` to Notion on the next push.

## Phase 3a — Per-cell diff + skip no-op rows (DAG n31 → n32a/b/c)

Phase 3a (PR #97) adds a **per-cell diff** against the snapshot the validation gate already stashed, then **skips whole rows that didn't change** (`skippedNoOp`). Three granularities — be precise about which one each step asserts:

- **Diff = per-cell.** `diffRow` compares each schema-backed field local-vs-snapshot and returns the changed keys.
- **Skip = per-row.** Zero changed cells → the whole row is skipped, no `UpdatePage` call.
- **Write = whole-row (NOT cell-scoped yet).** A *changed* row still re-sends its **entire** payload — sending only the changed fields is `n33`, deferred to 3b. **Do not assert cell-scoped writes here.**

New summary line this phase introduces: `Unchanged: N (already in sync — nothing to push)`, printed only when `skippedNoOp > 0`. Distinct from `Skipped:` (rows with no pushable fields at all).

Two behaviors the steps below pin:
- **rich_text now participates in the diff (#99 un-skip landed).** The original "3a skip" — rich_text excluded so a rich_text-only edit was a no-op — is **reversed**. #95's parser hardening + #99 make rich_text round-trip with formatting intact, so a rich_text-only edit is a real change again (`Pushed: 1`). C3's old no-op assertion is flipped to R1 (Phase 3d). The remaining no-op steps below (C1/C4/C5) stay valid because their edits are genuinely no-change (fresh import, multi_select reorder, null round-trip), not rich_text edits.
- **The TOCTOU guard is timestamp-based, not a re-diff.** A changed row is re-fetched and its `notion-last-edited` compared; a moved timestamp → `Conflicts`, row skipped. (The DAG calls n32b "re-diff"; the code guards on the timestamp — `push.go:179`.) Reliably racing live Notion mid-run isn't scriptable, so the conflict path stays **unit-tested** — no live C-step depends on a race.

**🚨 Page 4 rule still applies — but the reason narrowed post-#99.** Every step below either pushes the full folder with Page 4 *unchanged* (the per-cell diff skips it — safe) or, for C6, **excludes Page 4**. The hazard is no longer "all rich_text re-sends as plain text" — #99 makes the 6 supported formats round-trip. It's now narrower: Page 4's `Description` holds an **inline equation** (`$E = mc^2$`) that the parser does **not** round-trip (#95 scoped equations out), so a `--force` push (which sends the whole row, bypassing the diff) re-serializes and **degrades the equation**. Never `--force` the full folder.

### Step C0: Re-import for Phase 3

Phase 2 leaves drift / a partial folder. Re-import a clean all-8 folder.

Run: `./notion-sync.exe import 35957008-e885-80c5-9e34-f4191fd83907 --output ./test-output/push-e2e`

- **Pass:** all 8 `.md` present; `_database.json` + `AGENTS.md` present.

### Step C1: Fresh import → every row is no-op (n31 equality traps, n32a)

On the untouched import nothing differs from Notion, so the per-cell diff must find zero changes across all 11 property types — **including rich_text** (post-#99 it participates in the diff; a fresh import's `Description` must compare equal to Notion's, so Page 4's and Page 8's formatted rich text produce no phantom diff).

Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --dry-run`

- **Pass:**
  - Exit 0
  - `Total: 8`, `Would push: 0`
  - `Unchanged: 8 (already in sync — nothing to push)`
  - No Notion write (dry-run). Proves the int/float, nil/empty, multi_select-reorder, date-midnight, **and rich_text round-trip** equality traps all hold on live data — a clean round-trip produces no phantom diff.

No revert (nothing written).

### Step C2: One-cell edit → only that row writes; other pages' formatting survives (#55 core, n32a)

The original epic symptom: editing one field must not clobber another page's rich text.

1. **Snapshot Page 4** (`35957008-e885-8192-ab0f-c75e6a011b10`) via Notion MCP `notion-fetch`. Record its `Name` + `Description` rich-text payload (the annotation runs: bold / italic / link / inline-code / strikethrough / equation) and `Score` (400).
2. Edit Page 5's local `.md` (`35957008-e885-815e-8e73-ea79c22f96d4.md`): `Score` `500` → `555`. Touch nothing else.
3. Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit 0
  - `Pushed: 1`, `Unchanged: 7`
  - **Notion MCP fetch Page 4 (KEEP — load-bearing negative check):** `Name` + `Description` annotation payload byte-identical to the step-1 snapshot; `Score` still `400` (Page 4 unchanged → skipped → no `UpdatePage`). This is the "editing Page 5 didn't clobber Page 4's formatting" assertion — never drop it.
  - *(No independent Page 5 fetch — the push store-verifies the `Score` write; `Pushed: 1` + the S-step `pushed[].fields:["Score"]` confirm it landed, and F1's sweep re-checks Page 5 = canonical. Only `Score` was edited locally, so n33's cell-scoped write sends only `Score` — `Related` and every other field are absent from the payload and can't have changed. See Notion-read strategy C.)*

**Revert:** restore Page 5's local `Score` → `500`, re-run the same `push --yes` (`Pushed: 1`, `Unchanged: 7`). No independent fetch — `Pushed: 1` + store-verify confirm the revert; F1's sweep re-checks Page 5 = `500`.

### Step C3: rich_text-only edit — ⚠️ CONTRACT FLIPPED in Phase 3d (#99), see R1

**This step's original 3a assertion is reversed and now lives in [R1](#step-r1-rich_text-only-edit-pushes-positive-flip-of-c3).** Pre-#99, a rich_text-only edit was a deliberate **no-op** (`Pushed: 0`) because rich_text was excluded from the diff — sending it would have flattened formatting to plain text. #95's parser hardening + #99's un-skip make rich_text round-trip safely, so a rich_text-only edit is now a **real change** (`Pushed: 1`).

Left here as a signpost so the Phase 3a section isn't stale and C-numbering doesn't shift. **Do not run an assertion here** — the positive case runs as R1 in the Phase 3d block below. (If you're cross-referencing the old skill: the `Pushed: 0` / "rich_text excluded" assertion is gone on purpose.)

### Step C4: multi_select reorder is a no-op (n31 set-equality)

1. On a clean folder, reorder Page 5's local `Tags`: `[beta, gamma]` → `[gamma, beta]`. No value change.
2. Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - `Unchanged: 8`, `Pushed: 0`.
  - **Notion MCP fetch Page 5:** `Tags` still the set `{beta, gamma}`.

**Revert:** restore local order or re-run C0. No Notion write.

### Step C5: Null-edges row round-trips clean (Page 7, n31 nil/empty)

Page 7 (`35957008-e885-814f-9f19-c401d454b08d`) has null `Score` / `Category` / `Due Date` / `Website` / `Email` / `Phone` and empty `Tags []`. Its fresh-import row must diff to no-op — nil/empty must not read as a phantom change.

On the clean folder, run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Page 7 is counted in `Unchanged`, never `Pushed` / `Failed`.
  - **Notion MCP fetch Page 7:** all properties still null/empty — no spurious value or auto-created select option.

(C1 already lands Page 7 in `Unchanged: 8`; this step is the explicit null-edge assertion + Notion check.)

### Step C6: `--force` bypasses the diff (n32a negative)

`--force` has no stash and must overwrite every row, even in-sync ones.

⚠️ **Isolate to Page 5 — `--force` would clobber Page 4.**

1. Re-run C0. Delete every `.md` except Page 5's (`35957008-e885-815e-8e73-ea79c22f96d4.md`). Keep `_database.json` + `AGENTS.md`. **Page 4 must be gone.**
2. Verify isolation: `push --dry-run` reports `Total: 1` (just Page 5). Note `--dry-run` short-circuits *before* the gate preview, so it prints `Total:`, **not** the `Push queue (1 file)` line — that line only appears in the gated, non-`--yes` path (see G4).
3. Without editing anything, run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes --force`

- **Pass:**
  - Exit 0
  - `Pushed: 1` (Page 5 re-sent despite being in sync); **no `Unchanged:` line** (force bypassed the diff → `skippedNoOp == 0`).
  - **Notion MCP fetch Page 5:** values still canonical (force overwrote with identical values).

**Revert:** re-run C0 (restores the full folder). Page 5's `last_edited_time` bumps — harmless, realigned on the next import.

## Phase 3b / 3c — Per-field payload, store-verify, restamp (DAG n33 → n34d/e → n35a/n36a; n34h)

Phase 3b/3c (PR #98) makes three changes to the *write* itself, plus an auth-halt classification:

- **Cell-scoped write (n33).** A changed row now sends **only the changed fields**, not its whole payload (`buildPropertyPayloadFor(..., changedFields)`). 3a still re-sent the entire row; 3b narrows the PATCH to the diff.
- **Read-back store-verify (n34d/n34e).** After each write the row is re-fetched and every *sent* field is compared to what Notion stored. A mismatch is a **LOUD per-row failure** and the row is **NOT restamped** (so the next run re-attempts).
- **Precise restamp (n35a/n36a).** The same re-fetch reads Notion's authoritative `last_edited_time` (precise to the second) for the local restamp — *not* UpdatePage's minute-quantized echo (issue #57).
- **Auth halt (n34h).** A write returning 401/403 is run-wide (the credential, not the row), so the loop halts once and skips the rest instead of failing N identical rows. Exit 1, `Auth halted:` summary, rows pushed before the halt stay pushed.

**rich_text now participates in the diff, write, and store-verify (#99 un-skip).** The earlier 3a/3b note that "rich_text is excluded" is reversed — `diffRow`/`verifyStoredFields`/`buildPropertyValue` all handle rich_text via `ParseRichText` (`richtext_parser.go`). So **C3 is flipped, not a no-op** (see R1), and the read-back store-verify covers rich_text too (R7). The mismatch branch (n34e) stays unit-only as before.

**🚨 Page 4 rule — one deliberate carve-out this phase.** The blanket "never touch Page 4" relaxes *only* for **C7**: n33 means a **scalar** edit on Page 4 under a **non-force** push sends just that scalar, leaving its rich-text `Description` untouched — that survival is exactly what C7 proves. Everywhere else the rule is unchanged: **never `--force` a folder containing Page 4** (force sends `changedFields=nil` = whole row = re-serializes `Description`, which degrades its inline **equation** to a literal `$…$` run — the 6 supported formats survive, the equation does not), and **never edit Page 4's `Description`/rich_text locally**.

### Step C7: Scalar edit on the formatting fixture preserves its rich text (n33 cell-scoped write)

The in-scope version of the #55 epic symptom: editing one *scalar* field on a page must not drag that page's rich-text along as a formatting-corrupting plain-text push. In 3a this was impossible to test on Page 4 (a whole-row write would corrupt it); n33 makes it safe and asserts it.

1. Re-run C0 (clean folder — Page 4's `Description` must equal Notion, i.e. not itself "changed").
2. **Snapshot Page 4** (`35957008-e885-8192-ab0f-c75e6a011b10`) via Notion MCP `notion-fetch`. Record its `Description` rich-text payload (the annotation runs: bold / italic / link / inline-code / strikethrough / equation) and `Score` (400).
3. Edit **only** Page 4's local `Score`: `400` → `444`. Touch nothing else. Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes` (**non-force**).

- **Pass:**
  - Exit 0
  - `Pushed: 1`, `Unchanged: 7`
  - **Notion MCP fetch Page 4:** `Score` is now `444` **AND** `Description`'s annotation payload is **byte-identical** to the step-2 snapshot (every run + its annotations intact). A 3a whole-row write would have flattened `Description` to a single plain-text run — that regression is exactly what this step guards.

**Revert:** restore Page 4's local `Score` → `400`, re-run the same `push --yes` (`Pushed: 1`, `Unchanged: 7`), then **fetch Page 4** to confirm `Score` is `400` again and `Description` is still byte-identical to the snapshot.

### Step C8: Restamp keeps local aligned → a changed-row re-push doesn't spuriously conflict (n34d happy path + n35a/n36a)

Proves the read-back verify passed *and* the restamp wrote back Notion's authoritative `last_edited_time` — observable because a stale/wrong stamp would fail the *next* changed row's TOCTOU compare and surface as a phantom conflict.

> **Granularity note (verified live 2026-06-24).** The push-e2e DB's Notion API floors `last_edited_time` to the whole minute — even the GetPage refetch returns `…:00.000Z` (a write at `:33` reads back `2026-06-24T18:33:00.000Z`). So **do not assert sub-minute precision** — "to the second" is unobservable here. The observable n35a/n36a signal is that the restamp keeps local in lockstep with whatever Notion returns, so the *next* changed-row push doesn't conflict. (A no-op re-push does **not** test this — a no-op short-circuits on the per-cell diff before the TOCTOU compare, so the re-edit in step 2 is required.)

1. Re-run C0 if needed. Edit Page 5's local `.md` (`35957008-e885-815e-8e73-ea79c22f96d4.md`): `Score` `500` → `567`. Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`.

- **Pass (first push):**
  - Exit 0, `Pushed: 1`, `Unchanged: 7`
  - Page 5's local `.md` now has `notion-last-pushed:` set and `notion-last-edited:` rewritten (to Notion's minute-floored value — expected; see the granularity note).

2. **Edit Page 5's `Score` again** `567` → `568` and run the same `push --yes`. A changed row re-fetches and runs the TOCTOU timestamp compare — this is the real restamp catch.

- **Pass (second push):**
  - Exit 0, `Pushed: 1`, **no `Conflicts:` line**. A stale or wrong restamp from step 1 would fail the TOCTOU compare here and show a phantom `Conflicts: 1` — a clean push is the proof the restamp stored the value Notion reports on the next read.

**Revert:** restore Page 5's local `Score` → `500`, re-run `push --yes` (`Pushed: 1`). No independent fetch — store-verify + `Pushed: 1` confirm the revert; F1's sweep re-checks Page 5 = `500`. (See Notion-read strategy C.)

### Step C9: Auth failure halts the run once and writes nothing (n34h) — ⏭️ SKIP until a read-only token exists

⏭️ **SKIPPED by default.** Requires a second Notion integration with **Read content** capability but **NOT Update content**, shared to the push-e2e DB, with its token available (e.g. env `NOTION_SYNC_RO_KEY`). With a read-only token, the schema fetch and `GetPage` (reads) succeed but `UpdatePage` (the PATCH) returns 403 — the only scriptable way to reach n34h against live Notion. If the token isn't configured, print `Step C9: SKIPPED (no read-only token)` and move on.

When the token exists:

1. Re-run C0. Edit Page 2's local `Score` → `2200` and Page 3's local `Score` → `3300`. Isolate the folder to Pages 2 + 3 (delete the rest, **Page 4 critical**). Keep `_database.json` + `AGENTS.md`.
2. Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes --api-key <read-only-token>`.

- **Pass:**
  - Exit code **1**
  - stdout contains `Auth halted:` and an `authentication failed` line mentioning write access (the `AuthError` reason+fix).
  - **One** halt, not two per-row `Failed:` lines — the run `break`s on the first 403 (n34h), it doesn't fail every row.
  - **Notion MCP fetch** of Page 2 (`Score` 200) and Page 3 (`Score` 300): both **unchanged** — the 403 wrote nothing.

**Revert:** re-run C0 (no Notion write happened, so this just restores the local folder).

> **Unit-only (not E2E-scriptable), recorded here so the gap is explicit:**
> - **Verify *mismatch* (n34e)** — making live Notion store a value ≠ what was sent isn't reliable (the validation gate pre-validates select/status/multi_select), so the mismatch branch stays unit-tested in `push_test.go`. Same rationale as 3a's TOCTOU race.
> - **`Pushed before halt: N>0`** — a static read-only token 403s from row 1, so partial progress before an auth halt can't be staged live. Unit-covered.
> - **Empty-payload → `Skipped`** (`push.go:214`) — needs a changed field whose `buildPropertyValue` returns nil; narrow, unit territory.

## Phase 3d — Rich-text un-skip (DAG n31 rich_text re-include, #95/#99)

#99 reverses the 3a "rich_text skip": rich_text now **participates in the diff, the cell-scoped write, and the read-back store-verify**, deserialized by `ParseRichText` (`richtext_parser.go`). The `R` steps prove the round-trip lands with formatting intact **and** that ordinary cell values with markdown-ambiguous characters survive uncorrupted.

**Supported formats (round-trip clean):** bold `**`, italic `*`, inline code `` ` ``, link `[t](url)`, strikethrough `~~`, highlight `==` (→ `yellow_background`).
**Documented limitations (do NOT assert clean round-trip — they degrade, covered unit-only):** inline **equation** `$…$` (Page 4 — flattens to literal `$…$`), **text/background color** other than yellow highlight (non-yellow `*_background` collapses to yellow; text colors lost), **`@user` mention** identity. These are the Gap-1 family — `agents.go` documents them downstream.

**Primary fixtures:** Page 8 (`38a57008-e885-81c3-88c4-eec03393dcad`) — the full supported-format round-trip target; Page 5 (`35957008-e885-815e-8e73-ea79c22f96d4`) — plain-`Description` workhorse that reverts cleanly. **Never Page 4** (equation) and never `--force` a folder containing it.

**Dialect reminder:** the local `.md` uses notion-sync's dialect (`==highlight==`, `<u>underline</u>`), not the MCP create dialect (`<span color="yellow_bg">`). Page 8's canonical local `Description` after import is `**Bold run.** *Italic run.* \`inline code\` [link to xkcd](https://xkcd.com) ~~struck~~ ==highlighted==` — pin the exact string from the run's own `import` (Notion may re-segment).

### Step R0: Re-import for Phase 3d

Run: `./notion-sync.exe import 35957008-e885-80c5-9e34-f4191fd83907 --output ./test-output/push-e2e`

- **Pass:** all 8 `.md` present (incl. Page 8); `_database.json` + `AGENTS.md` present. Snapshot Page 8's `Description` annotation payload via Notion MCP `notion-fetch` now — it's the canonical for R2/R6/F1.

### Step R1: rich_text-only edit pushes (positive flip of C3)

The reversal of the old 3a skip: a rich_text-only edit is now a real change.

1. On a clean folder (re-run R0 if needed), edit **only** Page 5's local `Description` (append ` EDITED`). Change no other field.
2. Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit 0
  - `Pushed: 1`, `Unchanged: 7` — Page 5's row is **no longer skipped** (rich_text is in the diff now).
  - **Notion MCP fetch Page 5:** `Description` ends with ` EDITED` (the edit landed; Page 5's text is plain, so nothing to corrupt — this step proves the *push fires*, R2 proves *formatting survives*).

**Revert:** restore Page 5's local `Description` to canonical, re-run `push --yes` (`Pushed: 1`), then **fetch Page 5** to confirm `Description` is canonical again. (Or re-run R0.)

### Step R2: Full supported round-trip on Page 8 (the core un-skip proof)

1. On a clean folder (R0), confirm Page 8's local `Description` is the canonical 6-format string. **Snapshot Page 8** via Notion MCP `notion-fetch` and record the annotation runs (bold / italic / code / link / strikethrough / `yellow_background`).
2. Edit **only** Page 8's local `Description`: change the bold run's text `**Bold run.**` → `**Bold edit.**`. Keep every other marker exactly. Touch no other field.
3. Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit 0, `Pushed: 1`, `Unchanged: 7`
  - **Notion MCP fetch Page 8:** `Description` is **6 annotation runs**, NOT literal markers — bold on `Bold edit.`, italic on `Italic run.`, code on `inline code`, link on `link to xkcd` → `https://xkcd.com`, strikethrough on `struck`, `yellow_background` on `highlighted`. A pre-#99 push would have stored the literal string `**Bold edit.** *Italic run.* …` as one plain run — that's the regression this kills.
  - **Fixed point:** re-run R0 (re-import), then `push --dry-run` → `Unchanged: 8`. The round-trip is idempotent — import→edit→push→re-import produces no phantom diff.

**Revert:** restore Page 8's local `Description` to canonical (`**Bold run.** …`), re-run `push --yes` (`Pushed: 1`), then **fetch Page 8** → annotation payload **byte-identical to the step-1 snapshot**.

### Step R3: Hardening — markdown-ambiguous plain text survives uncorrupted (#575 edge cases)

The non-corruption guarantee: ordinary cell values with `*`, `_`, math operators, unicode, and parenthesized URLs must round-trip with **zero fabricated formatting**. String bundles the real edge cases from [salurbal.org #575](https://github.com/SALURBAL-Climate/salurbal.org/pull/575)'s first mock push.

1. On a clean folder (R0), set **only** Page 5's local `Description` to:
   ```
   SPVBMIOBESE adults 20-89: BMI ≥ 30, 2 * 3 checks, 5 µg/m³ — see [BMI](https://en.wikipedia.org/wiki/Body_mass_index_(disorder)); strata_id Sex_Female.
   ```
2. Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit 0, `Pushed: 1`
  - **Notion MCP fetch Page 5:** `Description` is **plain text + exactly one link run**, no other annotations:
    - snake_case underscores (`strata_id`, `Sex_Female`, `SPVBMIOBESE`) are **literal** — NOT italicized (the parser has no `_` token).
    - the space-flanked `2 * 3` keeps both asterisks (flanking demotes them to literal) — no fabricated italic, no dropped `*`.
    - unicode `≥` and `µg/m³` and the em-dash `—` survive byte-for-byte.
    - the link is on `BMI` with the **full** URL `https://en.wikipedia.org/wiki/Body_mass_index_(disorder)` — the inner `(disorder)` parens are **not** truncated and there is no phantom trailing `)` run.

**Revert:** restore Page 5's local `Description` to canonical, re-run `push --yes` (`Pushed: 1`), fetch to confirm. (Or re-run R0.)

### Step R4: Mixed scalar + rich_text edit — both land, formatting intact (n33)

1. On a clean folder (R0), edit Page 5: `Score` `500` → `575` **and** set `Description` to `**bold mix** and plain tail`. (Two changed fields.)
2. Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit 0, `Pushed: 1`, `Unchanged: 7`
  - **Notion MCP fetch Page 5:** `Score` is `575` **AND** `Description` is two runs — bold on `bold mix`, plain ` and plain tail`. n33 sends both changed fields; neither clobbers the other.

**Revert:** restore Page 5's `Score` → `500` and `Description` → canonical, re-run `push --yes` (`Pushed: 1`), fetch to confirm both. (Or re-run R0.)

### Step R5: Clear a rich_text cell — empty-array clear, not a literal empty run

1. On a clean folder (R0), blank Page 5's local `Description` (set it to an empty value). Touch nothing else.
2. Run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes`

- **Pass:**
  - Exit 0, `Pushed: 1` (a clear is a real write, not `Skipped`).
  - **Notion MCP fetch Page 5:** `Description` is **empty** — `richTextPayload([])` sends an empty array that clears the property. NOT a one-run rich text containing `""`, and NOT the literal text `""`.

**Revert:** restore Page 5's local `Description` to canonical, re-run `push --yes` (`Pushed: 1`), fetch to confirm. (Or re-run R0.)

### Step R6: `--force` preserves supported formatting — isolate to Page 8

`--force` re-serializes every row blind (no diff). For the 6 supported formats this is **safe** (proves the round-trip is faithful even on the overwrite-blind path); for Page 4's equation it is **not** — hence the isolation.

⚠️ **Isolate to Page 8 — `--force` would degrade Page 4's equation.**

1. Re-run R0. Delete every `.md` except Page 8's (`38a57008-e885-81c3-88c4-eec03393dcad.md`). Keep `_database.json` + `AGENTS.md`. **Page 4 must be gone.**
2. Verify isolation: `push --dry-run` reports `Total: 1` (just Page 8).
3. Without editing anything, run: `./notion-sync.exe push "./test-output/push-e2e/notion-sync-test-database-push" --yes --force`

- **Pass:**
  - Exit 0
  - `Pushed: 1` (Page 8 re-sent despite being in sync); **no `Unchanged:` line** (force bypassed the diff).
  - **Notion MCP fetch Page 8:** all 6 annotation runs **byte-identical to the R0 canonical snapshot** — force re-serialized the rich text through `ParseRichText` and the supported formats survived intact.

**Revert:** re-run R0 (restores the full folder). Page 8's `last_edited_time` bumps — harmless, realigned on the next import.

### Step R7: Store-verify covers rich_text (n34d happy path) — overlay on R2

No new push. The read-back store-verify (`verifyStoredFields`) now compares rich_text too: after R2's push, the row is re-fetched and `Description` is compared to what `ParseRichText` sent.

- **Pass:** R2's push showed **no `Failed:` line** — the rich_text store-verify passed (Notion stored exactly what was sent). Had the parser produced a payload Notion stored differently, n34e would have surfaced a LOUD per-row failure and skipped the restamp. The **mismatch branch (n34e) stays unit-only** (`push_test.go`) — making live Notion store a value ≠ what was sent isn't reliably scriptable (same rationale as 3a's TOCTOU race and the verify-mismatch gap recorded under C9).

## Phase 4 — Run summary JSON (DAG n41)

Phase 4 emits a machine-readable JSON `RunSummary` as the **leading object on stdout** of every push (cancel, dry-run, halt included). Banners, the `\r` progress line, the queue preview, and the final error all moved to **stderr**. So this phase is a **sanity overlay**, not new pushes: it re-runs a few representative G/V/C steps with the streams split and eyeballs the leading JSON. No formal assertions, no new fixtures.

The summary contract is already unit/CLI tested (mock client, in `summary_test.go` / `push_test.go`). These `S` steps add **real-Notion** confidence that live pushes produce the right `status` / arrays / exit — a sanity pass, not the source of truth.

### How to capture (per step)

Run the step's existing push with the two streams teed to files (PowerShell shown; `$LASTEXITCODE` is the exit code — don't trust `$?` for native-exe exit):

```powershell
./notion-sync.exe push "<folder>" [flags] 1>summary.txt 2>progress.txt
"exit=$LASTEXITCODE"
```

- `summary.txt` (stdout) leads with the JSON object, then the `─────` divider, then the human counts.
- `progress.txt` (stderr) holds banners + the `\r` progress line + the queue preview + the final halt/error line.
- **The summary is the first complete `{...}` object.** It's pretty-printed (multi-line), so read down to the divider line, not just line 1. The structural check is "first non-space byte of stdout is `{`".
- Splitting the streams means the existing G/V/C "combined output contains X" assertions now live across **both** files — check the right stream (preview/banners/halt line in `progress.txt`, JSON + human counts in `summary.txt`).

### Scenario → expected summary

| Rides on step | `status` | Key array content | exit |
|---|---|---|---|
| **G1** cancel (non-TTY, no `--yes`) | `cancelled` | **all arrays `[]`** — cancel emits the summary from an empty result *before* any classify, so even `skippedNonRow` is empty | 0 |
| **G2** `--yes`, Score edited | `clean` | `pushed:[{file:"<canary>.md", fields:["Score"]}]`, `skippedNonRow:[{file:"AGENTS.md", reason:"AGENTS.md"}]` | 0 |
| **C1** `--dry-run`, fresh import | `clean` | `pushed:[]`, `skippedNoOp` lists all 8 rows, `skippedNonRow:[{AGENTS.md}]` (dry-run still classifies) | 0 |
| **V2** multi-halt | `halted` | `halted[]` = 3 × `{phase:"validation", file, reason, fix}` (2 conflict + 1 stray), `pushed:[]`, `skippedNonRow:[{AGENTS.md}]` | 1 |
| **V3** soft-deleted | `clean` | `skippedNonRow` includes Page 6 `{reason:"notion-deleted"}` | 0 |
| **V5 / C6** `--force` | `clean` | `pushed[].fields` = full **sorted payload keys** (force has no per-cell diff → not just the edited field) | 0 |

**`skippedNonRow` appears on every path that reaches the validation classifier — normal, halt, `--force`, dry-run — but NOT cancel** (cancel short-circuits before the classifier runs).

### Step S1 — `clean` + per-field `fields` (rides G2)
Capture G2's `push --yes` stdout.
- **Pass:** leading byte `{`; `status:"clean"`; `pushed` has the canary with `fields:["Score"]` (only the edited field — n33 cell-scoped, not the whole payload); `skippedNonRow:[{AGENTS.md}]`; `failed`/`halted`/`skippedNoOp` empty; exit 0.

### Step S2 — `cancelled` + empty arrays (rides G1)
Capture G1's gated (no-`--yes`) stdout.
- **Pass:** `status:"cancelled"`; **every** array `[]` (incl. `skippedNonRow` — classifier never ran); exit 0. The queue preview + `Cancelled` line are in `progress.txt`, not the summary.

### Step S3 — `clean` dry-run, all-no-op arrays (rides C1)
Capture C1's `push --dry-run` stdout.
- **Pass:** `status:"clean"`; `pushed:[]`; `skippedNoOp` lists all 8 basenames; `skippedNonRow:[{AGENTS.md}]`; exit 0. Proves dry-run classifies and reports would-skip rows without a write.

### Step S4 — `halted` with per-halt entries (rides V2)
Capture V2's `push --yes` stdout.
- **Pass:** leading byte `{`; `status:"halted"`; `halted[]` has 3 entries, each `phase:"validation"` with non-empty `file`/`reason`/`fix` (Page 2 + Page 3 conflict, `random-stray.md` stray); `pushed:[]`; `skippedNonRow:[{AGENTS.md}]`; exit 1. The JSON precedes the human halt list on stdout; `push halted by validation gate` is in `progress.txt`.

### Step S5 — `skippedNonRow` soft-delete reason (rides V3)
Capture V3's `push --yes` stdout.
- **Pass:** `skippedNonRow` includes Page 6 with `reason:"notion-deleted"` (alongside `{AGENTS.md}`); `status:"clean"`; exit 0. (Don't pin Page 5's bucket — post-3a an unedited Page 5 diffs to a no-op and lands in `skippedNoOp`, not `pushed`.)

### Step S6 — `--force` `fields` fallback (rides V5 or C6)
Capture the `--force` push stdout from V5 (Pages 2+3) or C6 (Page 5).
- **Pass:** `pushed[].fields` is the full **sorted** set of pushable keys for the row (not just the edited field) — force has no snapshot to diff, so it overwrites blind; **no `skippedNoOp`**; `status:"clean"`; exit 0.

> **Unit-only — NOT checked live (recorded so the gap is explicit):**
> - **`status:"partial"` / `failed[]`** — every live route to a per-row failure is either a network artifact (re-fetch fail), an unscriptable mid-run race (n32c), a read-back mismatch the gate pre-empts (n34e), or a 4xx the validation gate already turns into a *halt*. No G/V/C step produces `partial`; it stays unit-tested in `push_test.go` / `summary_test.go`. **Don't hunt for it live.**
> - **Auth halt `{phase:"auth"}`** — only reachable with a read-only token (see **C9**, skipped by default). When C9 runs, also confirm its summary's `halted:[{phase:"auth", file:"", reason, fix}]` and exit 1.

---

## Final steps

### Step F1: Final state verification

**Read via one `notion-query-data-sources` sweep** — `SELECT * FROM "collection://35957008-e885-8068-9080-000b89086bb3"` (the **Data Source URL**, SQL mode; see Notion-read strategy B — **not** the Database ID) — **not** a per-page `notion-fetch` of Pages 1–8 (run this once any phase 2+ step has executed). The sweep returns every row's **scalar** properties (`Name`, `Description` as plain text, `Category`, `Score`, `Due Date` start, `Tags`, …) in a single call. Add exactly **three targeted full `notion-fetch` calls** for what the query flattens or omits (see Notion-read strategy B): **Page 4** `Description` rich-text byte-identity, **Page 8** `Description` rich-text byte-identity, and **Page 1's `Due Date` `is_datetime`** flag. Compare against **the canonical values hardcoded in `setup.md`** (per-page property tables) — NOT just the run's own Step 3 fetch. Within-run-only comparison is unsafe: if a prior run left a fixture drifted, a fresh Step 3 fetch records the drifted state and F1 then "matches" itself, silently passing while the bug persists. F1's job is to detect drift against the source-of-truth canonical, full stop.

**Phase 1 minimum — Page 1 (canary):**

| Property | Canonical value | Notion shape to assert |
|---|---|---|
| `Name` | `Push: Canary` | `title` text == canonical |
| `Description` | `Phase 1 fixture — confirmation gate cancel/proceed/dry-run.` | `rich_text` plain text == canonical |
| `Category` | `Research` | `select.name` == canonical |
| `Score` | `100` | `number` == canonical |
| `Due Date` | `2026-06-01` (date-only) | `date.start` == canonical AND `is_datetime` == `0`/`false` |

**Phase 2 additions — check Pages 2, 3, 6, 7 (all scalar → from the sweep) against `setup.md` canonicals.** V1/V2 stale-stamp Pages 2 & 3 (must end at `Score` 200 / 300). V3 marks Page 6 deleted in the local file only (Notion-side `Score` 600 must be untouched). V4 corrupts Page 7's local YAML (Notion-side unchanged). V5 actually writes to Notion — its mandatory revert step must restore Page 2 → 200 and Page 3 → 300 before F1 runs. Don't duplicate the canonical values here — read them from `setup.md`'s per-page sections (Pages 2/3/6/7).

**Phase 3a additions — check Pages 4, 5, 7 against `setup.md` canonicals** (Page 5 / Page 7 scalars from the sweep; **Page 4's `Description` annotation payload via the targeted full fetch**). C2 writes then reverts Page 5 (`Score` must end at 500; `Related` = [Page 4]). **Page 4 is the load-bearing check:** C2/C6 are designed never to write it, so any drift in its `Name` / `Description` annotation payload means the cell-diff skip (C2) or the `--force` isolation (C6) failed — fail the run loudly. Page 7 must remain all-null. C3 edits Page 5's `Description` locally only (no Notion write) — confirm Notion-side `Description` is canonical.

**Phase 3b/3c additions — Page 4 is *doubly* load-bearing now.** C7 deliberately writes+reverts Page 4's `Score` (must end at `400`) and its `Description` annotation payload must be **byte-identical to canonical** — drift means n33's cell-scoped write regressed and re-sent the whole row. C8 writes+reverts Page 5's `Score` (must end at `500`). C9, if it ran (token present), wrote nothing, so Pages 2 & 3 must equal canonical (`Score` 200 / 300); if C9 was skipped, no extra check.

**Phase 3d additions — check Pages 5 and 8 against `setup.md` canonicals** (Page 5 scalars from the sweep; **Page 8's `Description` annotation payload via the targeted full fetch**). The `R` steps write+revert Page 5's `Description` (R1/R3/R4/R5) and `Score` (R4) and Page 8's `Description` (R2/R6) — all must end at canonical:
- **Page 8 is the load-bearing rich-text check:** its `Description` annotation payload must be **byte-identical** to the setup.md canonical (the 6 supported runs, highlight = `yellow_background`). Drift means a round-trip (R2) or `--force` (R6) regressed — fail loudly. R6 never edits Page 8 (force re-sends in-sync), so any annotation drift there means `--force` corrupted a *supported* format, a real bug.
- **Page 5** must end at `Score` 500 and its plain canonical `Description` — R1/R3/R4/R5 each revert it; a leftover ` EDITED`, the #575 bundle string, `**bold mix**`, or an empty `Description` means a revert was skipped.
- **Page 4 stays the equation canary:** the R steps never touch it, so its `Description` (incl. the `$E = mc^2$` run) must equal canonical — any drift means an R step leaked onto Page 4 (isolation failure).

If any property's Notion shape doesn't match the canonical, mark the run as TESTS FAILED and list the field + got/want values — don't try to auto-fix; investigate.

**Note on `Due Date`:** the `is_datetime` flag is load-bearing here. A common bug class is push promoting date-only properties to UTC datetimes (the original parser-roundtrip bug). F1 must assert *both* `start` matches AND `is_datetime` is false; matching only on the calendar day misses the type drift. The sweep gives `start`; **`is_datetime` comes from the targeted full fetch on Page 1** (SQL mode's `is_datetime` projection is unconfirmed — see strategy B's first-run TODO; until confirmed, take it from the full fetch) — one of F1's three full fetches.

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
| 4    | Isolate canary (delete Pages 2-8 .md)   | PASS   |
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
| C0   | Re-import for Phase 3                    | PASS   |
| C1   | Fresh import → every row no-op          | PASS   |
| C2   | One-cell edit, formatting survives      | PASS   |
| C3   | rich_text-only edit — flipped, see R1   | N/A    |
| C4   | multi_select reorder = no-op            | PASS   |
| C5   | Null-edges row round-trips clean        | PASS   |
| C6   | --force bypasses the diff               | PASS   |
| C7   | Scalar edit on Page 4 keeps rich text   | PASS   |
| C8   | Restamp aligns local; re-push no conflict | PASS  |
| C9   | Auth 403 halts once, writes nothing     | SKIP   |
| R0   | Re-import for Phase 3d (snapshot Pg 8)   | PASS   |
| R1   | rich_text-only edit pushes (flip of C3)  | PASS   |
| R2   | Full supported round-trip on Page 8      | PASS   |
| R3   | Hardening — #575 markdown-ambiguous text | PASS   |
| R4   | Mixed scalar + rich_text edit, both land | PASS   |
| R5   | Clear rich_text → empty-array clear      | PASS   |
| R6   | --force preserves formatting (Page 8)    | PASS   |
| R7   | Store-verify covers rich_text (rides R2) | PASS   |
| S1   | clean + per-field fields (rides G2)     | PASS   |
| S2   | cancelled + empty arrays (rides G1)     | PASS   |
| S3   | dry-run all-no-op arrays (rides C1)     | PASS   |
| S4   | halted entries, exit 1 (rides V2)       | PASS   |
| S5   | skippedNonRow soft-delete (rides V3)    | PASS   |
| S6   | --force fields fallback (rides V5/C6)   | PASS   |
| F1   | Final state matches canonical (1–8)     | PASS   |
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
