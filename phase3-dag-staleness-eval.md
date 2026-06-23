# Phase 3 — DAG staleness evaluation (before we start)

Scratch note. Evaluates `.context/features/push/dag-v1.4.0.mmd` against shipped code + recorded
decisions, for **all of Phase 3 (3a/3b/3c/3d)**. Goal: be clear on what we're actually building
before writing a line.

## Sources of truth, and how they relate

- **Issue #55 = SOT for scope + sequencing.** Body just lists `3a–3d`; the TDD comment table
  expands each phase to DAG node IDs.
- The node IDs (`n31`, `n34h`, …) only resolve in **`dag-v1.4.0.mmd`** — so #55 *delegates* the
  per-step behavioral spec to the DAG by reference. DAG is not a competing plan; it's #55's detail.
- DAG was authored in **#76 (`929d0ec`) and never edited since** — frozen design doc.

## Framing: Phase 3 staleness is the *opposite* of Phase 2's

- **Phase 2** went stale because code raced **ahead** of the DAG: `validation.go` shipped **9**
  halt/skip classes; the DAG draws only `n21a–f`. Net-new in code, absent from DAG:
  `n21g ClassHaltMalformed`, `n21h ClassHaltUnreadable`, `n21i ClassHaltInvalidOption` (the #90 work).
- **Phase 3 is entirely unbuilt.** The per-row loop in `push.go` is the **legacy naive push** that
  predates this epic. So the DAG is the **forward target**; shipped code is *behind* it. The DAG
  isn't "wrong about intent" — staleness = where the target collides with code/decisions already
  on disk.

## Per sub-phase verdict

| Phase | DAG nodes | On disk today | DAG stale? |
|---|---|---|---|
| **3a** Per-cell diff | n31, n32, n32a | ❌ none — `buildPropertyPayload` sends **all** writable fields, never diffs; the `len==0 → Skipped` path is near-dead | **Accurate, unbuilt.** Date normalization half-explored already (`stripMidnightUTC` + `backlog/date-only-roundtrip.md`). Clean to build. |
| **3b** Send + read-back verify | n33, n34✓, n34d, n34e, n35a, n36a | ⚠️ **half** — restamp (n35a/n36a = the #57 precise-timestamp refetch) **exists**; read-back verify (n34d/n34e) **missing** | **Accurate.** Build = add verify gate, route the *existing* restamp through it. |
| **3c** Error taxonomy + retry | n34a–c, n34f–h | ❌ blanket `Failed++`, no taxonomy, no retry, no auth-halt | ⚠️ **Real drift — see decision #1.** |
| **3d** Restamp on retry success | n35b, n36b | ❌ no retry branch exists | **Depends on 3c.** Likely collapses — see decision #2. |

## Two decisions to lock before coding

### 1. 3c retry location — DAG vs reality collision
DAG `n34a/n34b` prescribes a **push-level** retry (2× w/ 5s→15s backoff). But the **Notion HTTP
client already retries transients** — max 5×, codes 429/500/502/503/504, exponential backoff,
honors `Retry-After` (`internal/notion/client.go`, per CLAUDE.md). Building n34a/n34b literally =
**double retry** (client 5× × push 2×).

- **Recommendation:** transient retry stays at the client layer. 3c becomes **classify-only**:
  transient-exhausted → loud-fail; per-row 4xx (400/422) → loud-fail + continue (`n34c`); auth
  401/403 → halt run (`n34h`). The net-new nodes (n34c, n34h) are accurate and not stale.

### 2. 3d may not exist
`n35b/n36b` ("restamp after retry success") is identical to 3b's `n35a/n36a` on a separate retry
branch. If retry lives at the client layer (decision #1), there is **no push-level "retry
succeeded" branch** → **3d collapses into 3b**. #55 itself hedges "merge 3c+3d." Expectation: 3d
disappears.

### (also) per-row TOCTOU conflict recheck
Legacy loop does a per-row `GetPage` + timestamp check → `Conflicts++`. The DAG models conflict only
at the Phase-2 gate (`n21d`), **not** in Phase 3. Decide: keep the belt-and-suspenders per-row
recheck, or trust the gate per the DAG.

## Cross-cutting gap (touches all of 3a–3d)
`PushResult` is still flat counters (`Pushed/Skipped/Conflicts/Failed int`). The DAG run-summary
schema wants **arrays**: `pushed[]{file,fields}`, `skippedNoOp[]`, `skippedNonRow[]`,
`failed[]{file,reason,fix}`, `halted[]{…,phase}`. Officially Phase 4, but 3a–3d must **feed** it, so
the struct grows as we go. Not "DAG stale" — "type behind DAG."

## Bottom line
- **3a / 3b:** DAG accurate. 3a is clean greenfield; 3b is half-built in our favor (restamp exists,
  add verify).
- **3c:** DAG naive on retry — reduce to classify-only over the existing client retry.
- **3d:** likely vanishes into 3b.

## Recommended next step
Lock decisions #1 and #2, then **refresh the DAG** (also backfill Phase 2's `n21g/h/i`) so it's an
honest SOT — *then* start 3a via `/tdd`.
