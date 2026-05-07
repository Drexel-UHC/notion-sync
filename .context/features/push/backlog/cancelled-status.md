# Add `status: "cancelled"` for user-declined runs

**Status:** backlog (v1.4.1 polish)
**DAG node:** `n13a` → `n41`
**Tier:** semantic clarity

## What

Add a fourth value to the run-summary `status` field: `cancelled`. Emitted when the user declines at the `n13` confirmation prompt (or `--yes` is missing in non-interactive mode).

```json
{ "status": "cancelled", "pushed": [], "skippedNoOp": [], ... }
```

## Why

Today, a cancelled run would emit `status: "clean"` (nothing failed, nothing halted). That's technically true but semantically misleading — agents can't distinguish "ran successfully and had nothing to push" from "user said no."

## Why deferred

Agents that read the JSON can already disambiguate by checking `pushed[]` length and exit code. This is a clarity polish, not a correctness fix.

## Acceptance

- User answers `n` at the prompt → JSON has `"status": "cancelled"`.
- Non-interactive run without `--yes` → JSON has `"status": "cancelled"`.
- Exit code `0` (cancellation is not a failure — see `exit-code-cancelled.md`).

## Coupled to

- `status-rules-explicit.md` — adds `cancelled` to the rule table.
- `exit-code-cancelled.md` — defines exit code for the new status.
