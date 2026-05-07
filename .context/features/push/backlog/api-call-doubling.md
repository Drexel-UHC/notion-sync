# Halve GetPage call volume by reusing the validation pass result

**Status:** backlog (v1.4.0 phase 3)
**DAG node:** `n21+n22 → n23+`
**Tier:** performance / cost

## What

Phase 2 added a validation gate (`ValidatePushQueue`) that calls `GetPage` for every linked `.md` to read Notion's `last_edited_time`. The per-row push loop in `PushDatabase` *also* calls `GetPage` for every linked file — first as a TOCTOU defense before `UpdatePage`, then again after the write to grab the precise (non-minute-quantized) timestamp.

For an N-row push with M actual writes, the new flow makes **`2N + 2M` `GetPage` calls** plus M `UpdatePage` calls. The Notion client throttles to ~3 req/s, so the validation pass alone takes ~`N/3` seconds before any write happens.

## Why this matters

- **100-row database, 10 actual property changes:** validation = ~33 s wall time before the first `UpdatePage`. With pre-phase-2 behavior the same push was ~3 s.
- **All linked files pay**, even ones that won't be modified — `buildPropertyPayload` returning empty (no diff) still incurred the validation `GetPage`.
- The work is duplicated: the validation pass already fetched Notion's `last_edited_time` for every row, but the push loop refetches it.

## Why deferred to phase 3

Phase 2 is intentionally a pure read+classify pass per the DAG spec (`n21` is a classification node, not a state-passing node). Plumbing the `*ValidationReport` through to the push loop is a phase-3 concern because:

1. Phase 3 will introduce cell-level diffing (`n23+`), which restructures the per-row inner loop anyway. Adding plumbing now then rewriting it next phase is churn.
2. The TOCTOU defense at `push.go:158-171` is real (Notion can be edited between the gate and the per-row check). Phase 3's per-cell diff will need its own answer to TOCTOU; the optimization should land alongside that design.

## Acceptance

- `PushDatabase` consumes `ValidationReport.Files` instead of calling `GetPage` again in the per-row loop. For Ready-class rows, the gate's `Page` is reused.
- TOCTOU is either explicitly accepted (gate-then-push is optimistic) or the per-row check stays for the narrow race window — but the GetPage call itself is gone for the common case.
- A 100-row push with 0 changes makes ≤ 100 + small-constant `GetPage` calls, not ~200.
- Test: `TestPushDatabase_DoesNotDoubleGetPageCalls` counts `mockNotionClient.getPageCalls` and asserts the bound.

## Touch points

- `internal/sync/push.go` — `PushDatabase` per-row loop (lines ~148-227 today)
- `internal/sync/validation.go` — possibly expose a `ReadyFiles()` helper on `*ValidationReport`
- `internal/sync/mock_client_test.go` — add a call counter for `GetPage` so the test above can assert
