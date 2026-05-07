# Halt run on auth failure (401 / 403)

**Status:** backlog (v1.4.1 hardening)
**DAG node:** `n34h`
**Tier:** edge-case protection

## What

Today's design treats every non-2xx push response as a per-row failure (`n34c`). But auth failures (401, 403) are run-wide — every subsequent row will fail for the same reason.

Split the 4xx branch:

- **Per-row 4xx (400, 422)** → continue at `n34c` (LOUD failure, queue continues).
- **Run-wide 4xx (401, 403)** → halt at `n34h` (stop the queue, report once, exit 1).

Subsequent rows skip the API call entirely; the credential, not the row, is the failure.

## Why

Without it: a bad token produces N noisy per-row failures instead of one clean halt. Burns API quota, pollutes the JSON summary, makes it harder to see what actually broke.

## Why deferred

Annoying, not destructive. The tool still terminates and surfaces the error; users can read past the noise. Land after the cell-diff core works.

## Acceptance

- Push run with an invalid token → first API call returns 401, run halts immediately, no further rows attempted.
- `halted[]` contains one entry with `phase: "auth"`.
- Exit code `1`.
- API call count = 1 (not N).

## Coupled to

`halt-schema-with-phase.md` — the schema rename is required to land this.
