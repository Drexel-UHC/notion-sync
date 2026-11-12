---
sidebar_position: 1
title: Push
---

# Push

*Write local property edits back to Notion — guarded, all-or-nothing.*

`notion-sync push <folder>` reads frontmatter from local `.md` files and writes property changes back to Notion. **Page body content is never touched — only properties.**

`push` is the only command that mutates your live Notion workspace, and its writes aren't trivially reversible. It enforces two guardrails — a [confirmation gate](#confirmation-gate) and a [validation gate](#validation-gate) — before any write.

## Recommended workflow

1. `refresh` the folder and resolve any drift at the source.
2. Edit properties in frontmatter.
3. `push --dry-run` and review what would change.
4. `push` and confirm at the prompt.

```sh
notion-sync refresh "./notion/My Database"
notion-sync push "./notion/My Database" --dry-run
notion-sync push "./notion/My Database"
```

## Confirmation gate

Before any Notion API call, push previews the queue and asks for consent.

- **Interactive (real terminal):** prints the queue to stderr and waits for `y/N`, default **N**.
- **Non-interactive (piped / CI / agent):** no prompt — push **requires `--yes`** or it cancels and writes nothing (`status: "cancelled"`, exit 0).

Pass `--yes` only once a human has reviewed the dry run and approved this push (or has given standing authorization for unattended runs). A cancelled run is the safe default.

## Validation gate

After consent, push classifies every `.md` in the folder. If **any** file hits a halt class, the **entire run aborts before a single write** (`status: "halted"`, non-zero exit). You never get a half-applied batch.

| Halt class | Meaning | Fix |
| --- | --- | --- |
| `conflict` | Notion's row changed since your last sync (`notion-last-edited` drift) | `refresh` (take Notion's values, discards local edits) or `push --force` (overwrite Notion with yours) |
| `stray` | A `.md` with no `notion-id` that isn't `AGENTS.md` | remove it, or add a valid `notion-id` |
| `invalid-option` | A `select` / `status` / `multi_select` value not in the schema | use an existing option, or `--allow-new-options` (select/multi only) |
| `malformed` | YAML frontmatter couldn't be parsed | fix the frontmatter |
| `unreadable` | File couldn't be read from disk | fix permissions |
| `unreachable` | Notion couldn't be reached to check the row | retry when Notion is reachable |

## Flags

### Routine

- `--dry-run` — classify and diff with **no writes**. Still **reads** Notion (schema + each row) to classify and diff. Always run this first.
- `--yes` / `-y` — consent token for the confirmation gate. Required non-interactively.

### Overrides — use only deliberately

These disable safety. Use them only after manual review, for a specific run:

- `--force` / `-f` — skips the **entire** validation gate (every halt class) and blind-overwrites Notion with local values. Local wins unconditionally. Highest blast radius.
- `--allow-new-options` — lets unknown `select` / `multi_select` values create **new** options in Notion's shared schema. `status` is always validated — its options can't be created via the API.

## What gets written

- Only **pushable** properties (see [property mapping](../reference/frontmatter.md#property-mapping)). Notion-managed fields (`people`, `files`, `created_time`, …) are silently skipped.
- Only **changed** cells. Push diffs each row against Notion's current value; unchanged rows are reported as `skippedNoOp`.
- Title properties are pushable — editing the title value renames the page in Notion.
- `notion-deleted: true` files are never pushed (reported as `skippedNonRow`).
- After a successful push, the row gets `notion-last-pushed: <timestamp>` and its `notion-last-edited` is updated to Notion's post-write value, so the next run sees it in sync.

## Run summary

Every terminal path (success, halt, cancel) prints **one JSON object first on stdout**, then a divider and human-readable counts. Queue preview, progress, and prompt go to **stderr**. To script against push: parse the first JSON object on stdout, ignore stderr.

```json
{
  "status": "clean",
  "pushed":        [ { "file": "<id>.md", "fields": ["Status", "Tags"] } ],
  "skippedNoOp":   [ "<id>.md" ],
  "skippedNonRow": [ { "file": "AGENTS.md", "reason": "AGENTS.md" } ],
  "failed":        [ { "file": "<id>.md", "reason": "...", "fix": "..." } ],
  "halted":        [ { "file": "<id>.md", "reason": "...", "fix": "...", "phase": "validation", "cells": [] } ]
}
```

- `status`: `clean` (every row pushed or already in sync), `partial` (finished, some rows failed), `halted` (a gate stopped the run), `cancelled` (consent declined).
- The five arrays are always present (`[]` when empty). `status` is always a string.
- On a `conflict` halt, `cells[]` lists each differing field with both local and Notion values, so you can reconcile without re-fetching. `phase` is `validation` (pre-write gate) or `auth` (a 401/403 mid-run).
- **Exit code:** `0` for `clean` / `cancelled`; non-zero for `partial` / `halted`. A dry run is **not** unconditionally `0` — it still runs the validation gate. Prefer `status` over the exit code when branching.

## Rich-text fidelity

`title` and `rich_text` values are stored as a **flat Markdown string** — on import, Notion's formatting is rendered to Markdown (`**bold**`, `[text](url)`, …). On push, bold, italic, inline code, strikethrough, highlight, underline, and links are reconstructed as real Notion formatting and round-trip intact.

**Lost at import — can never be restored on push:**

- **Foreground (text) color** — only *background* highlight survives (as `==text==`).
- **`@user` mention identity** — rendered as `@name`; the user ID isn't retained, so push can't re-link it.

**Degrades on push:**

- **Inline equation** — imports as literal `$expression$` and pushes back as those characters, so it stops rendering as math.

Highlight color is normalized — any background color round-trips as a single yellow highlight.
