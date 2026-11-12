# Docs strategy

How notion-sync documents itself for humans and agents. One source of truth, two thin
pointers to it. Research behind this: [research-nov2026.md](research-nov2026.md).

## Three layers, one SOT

| Layer | Path | Role | Changes when |
| --- | --- | --- | --- |
| **Docs site (`next`)** | `docusaurus/docs/**` | **Source of truth.** Full detail: tables, schemas, examples, flags, run-summary JSON, caveats. Published at `/next/` with `llms.txt`, `llms-full.txt`, and a `.md` copy of every page | Any PR that changes user-visible behavior — commands, flags, output, frontmatter, metadata files, push contract. Enforced by `/docs-update` via `## User Docs` in root `CLAUDE.md` |
| **Versioned snapshot** | `docusaurus/versioned_docs/version-vX.Y.Z/` | Frozen copy of the site per release. Never edited after the cut | Release time only — `pnpm docusaurus docs:version vX.Y.Z` (step in `/release`) |
| **`AGENTS.md`** | `internal/sync/agents.md.tmpl` → written to the user's workspace root on `import` / `refresh` / `clean` | **Slim pointer** (~80–100 lines). Only what an agent needs *without network*: what the folder is, layout, metadata keys, soft deletes, push guardrails, halt table, links to the site | Only when an inline essential changes. Everything else is a link, so most feature PRs don't touch it |

Not a layer: `CLAUDE.md` at repo root is for agents working *on* notion-sync's code, not
*with* its output. It never ships to users.

## Rules

- **Site wins for detail; AGENTS.md wins for truth.** AGENTS.md describes the binary that
  wrote the files; the site describes the latest release. If they disagree, trust AGENTS.md.
- **Safety-critical rules stay inline in AGENTS.md, never link-only.** Agents may be
  offline or unable to fetch URLs. The push guardrails ("default to NOT pushing", never
  silence a guardrail, halt → report, never clear `create-pending`) are copied verbatim,
  not summarised.
- **Links are version-pinned.** AGENTS.md links use
  `https://drexel-uhc.github.io/notion-sync/{{VERSION}}/…` so the page matches the
  installed binary. A dev build (no release tag) falls back to `/next/`.
- **Write each fact once.** If a detail is on the site, AGENTS.md points at it. Duplicated
  prose is the thing that drifted before this strategy existed (#120 shipped with no site
  update; #121 backfilled it).
- **No changelog, no status vocabulary.** GitHub Releases are the release notes.

## What goes where

| Content | AGENTS.md | Site page |
| --- | --- | --- |
| Folder identity, layout tree, filename rule, "tool-owned, don't edit" | inline | `reference/workspace-layout` |
| `notion-*` metadata keys | inline (one block) | `reference/frontmatter` |
| Soft deletes = historical; S3 URLs stable but won't download | inline (gotcha only) | `reference/frontmatter#soft-deletes`, `reference/file-urls` |
| Push guardrails, 5-condition checklist, override rules | inline, verbatim | `agents`, `guides/push` |
| Halt-class table (class → meaning → fix) | inline, compressed | `guides/push#validation-gate` |
| Full property mapping table | link | `reference/frontmatter#property-mapping` |
| `_database.json` / `_page.json` schemas | link | `reference/metadata-files` |
| `clean`, `agents-md`, refresh flags | link | `reference/commands` |
| Push mechanics, flags, run-summary JSON, row creation, rich-text fidelity | link | `guides/push` |

## Workflow

1. Feature PR changes behavior → `/docs-update` edits `docusaurus/docs/**` (and the
   template only if an inline essential moved) before merge.
2. Merge to `main` → GitHub Pages workflow rebuilds the site; `next` is live.
3. `/release` → tag, build binaries, cut `docs:version vX.Y.Z`, so `/vX.Y.Z/` exists for the
   `{{VERSION}}` links the new binary writes.

Tracking issue: #123.
