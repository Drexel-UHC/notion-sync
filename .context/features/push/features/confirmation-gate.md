# Confirmation gate before push

**Status:** proposed for v1.4.0
**Target release:** v1.4.0 (ship-blocker)
**DAG nodes:** `n12b` → `n13` → `n13a`
**Source:** pass-2 critical review (commit `1bcc903`)

## What

Before sending any write to Notion, `push` must:

1. **Show the queue** — list every local file it plans to push (paths + count).
2. **Require explicit consent** before any API write:
   - Interactive (TTY): `y/N` prompt, default `N`.
   - Non-interactive (CI, agent, scripted): require `--yes` flag.
3. If declined, or `--yes` missing in non-interactive mode → end with status `cancelled`, exit `0`, send nothing to Notion.

## Why

`push` is the **only** command in notion-sync that writes to a shared system. Every other command (`import`, `refresh`, `list`) is local-only and recoverable via `git`.

An accidental `push`:

- Modifies rows for every human and integration in the workspace.
- Bumps `last_edited_time`, firing automations, webhooks, downstream syncs.
- Has no undo — Notion has no "revert this push" button.

The gate is the only mechanism that prevents the tool from doing the wrong thing in the first place. Everything else in the v1.4.0 push redesign is about reporting errors *after* they happen.

## Acceptance

- Running `push` in a TTY and answering `n` (or pressing Enter on default) → nothing sent to Notion, status `cancelled`, exit `0`.
- Running `push` non-interactively without `--yes` → nothing sent, status `cancelled`, exit `0`. Stderr explains how to opt in.
- Running `push` non-interactively with `--yes` → queue executes as planned.
- Preview output lists every file in the queue *before* the prompt fires.
- Cancelling at the prompt does **not** count as a failure (exit 0, distinct from `partial`/`halted`).

## Out of scope

- **Per-field diff in the preview.** Depends on the cell-level diff (WS2 in the epic). MVP preview shows file paths and a count only.
- **`--dry-run` mode.** Separate concern. This is a consent gate, not a planning tool.
- **Interactive selection** (e.g. uncheck rows from the queue). Not in v1.4.0.

## Implementation notes

- TTY detection: standard `isatty` on stdin/stderr.
- `--yes` flag is the audit trail for unattended runs — present in CI config = someone deliberately allowed automated pushes from this pipeline.
- The preview is emitted to stderr so it doesn't pollute the JSON summary at `n41`.

## References

- Epic: issue #55
- DAG: `.context/features/push/dag-v1.4.0.mmd` nodes `n12b`, `n13`, `n13a`
- Triage: pass-2 review classified this as the only ship-blocker; everything else is hardening (see `.context/features/push/backlog/`)
