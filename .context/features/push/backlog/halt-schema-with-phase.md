# Unify halt schema with `phase` field

**Status:** backlog (v1.4.1 hardening)
**Tier:** schema cleanup
**Coupled to:** `auth-failure-run-halt.md`

## What

Rename the run-summary key `haltedInValidation` → `halted`, and add a `phase` field per entry indicating where the halt originated:

```json
"halted": [
  {"file": "...", "reason": "...", "fix": "...", "phase": "validation" | "auth"}
]
```

## Why

Today's schema only models validation halts. Adding auth halts (`auth-failure-run-halt.md`) introduces a second halt source. Rather than create a parallel `haltedAtAuth` key, fold both under one `halted[]` array discriminated by `phase`.

## Why deferred

Pure schema refactor. Only meaningful once auth halts exist. Ship together with `auth-failure-run-halt.md` as one PR.

## Acceptance

- Validation halts emit `phase: "validation"`.
- Auth halts emit `phase: "auth"`.
- Old key `haltedInValidation` is removed (clean break — push JSON is not yet stable).
- Status rule: `status: "halted"` iff `halted[]` is non-empty (regardless of phase).

## Migration

None — push command is pre-1.0 in terms of stable JSON contract; we change the schema cleanly.
