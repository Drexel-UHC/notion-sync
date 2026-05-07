# Halt validation when Notion is unreachable

**Status:** backlog (v1.4.1 hardening)
**DAG node:** `n21f`
**Tier:** edge-case protection

## What

During phase 2 validation, the tool reads each row's `last_edited_time` from Notion to compare against `local notion-last-edited`. If that read fails (network error, timeout, 5xx with retries exhausted), the row cannot be safely classified as ready-to-push.

Add a halt branch (`n21f`) that stops the run when validation can't reach Notion. Surface the affected file(s) and the network error.

## Why

Without it, a transient network failure during the validation window could let a row fall through to "ready" or get silently skipped — pushing against potentially stale local state.

## Why deferred

Real risk but narrow: only fires on network failure *during the validation read specifically*, not during the push itself. Rare in practice. The confirmation gate (v1.4.0) gives the user a chance to abort; this is belt-and-suspenders for unattended runs.

## Acceptance

- Simulated network failure on a single row's `last_edited_time` read → run halts, no rows pushed, that row appears in `halted[]` with `phase: "validation"`.
- Other rows in the same batch are not pushed (validation is all-or-nothing — see `n22`).
