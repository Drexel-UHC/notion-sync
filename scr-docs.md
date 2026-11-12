# scr — notion-sync docs site

## Goal

One Docusaurus site = single source of truth (SOT) for user docs — humans and agents.
`README.md` and `AGENTS.md` derive from it.

## Layout

- `docusaurus/` at repo root — mirrors `SALURBAL-Climate/salurbal-obt`.
- Go build unaffected: no `.go` files there.
- Guard: empty `docusaurus/go.mod` so `go test ./...` never walks `node_modules`.

## AGENTS.md today

- Template: `internal/sync/agents.md.tmpl` — **244 lines**.
- Embedded via `//go:embed`; written to workspace root on every `import` / `refresh`.
- `go:embed` can't read outside its package dir → can't point straight at `docusaurus/`.

## Decision: short AGENTS.md, not link-only

Link-only is risky:

- Agents often offline / won't fetch URLs.
- Site = latest; user's binary may be older → version mismatch.

Industry pattern: short AGENTS.md with critical rules inline + link out for the rest. Site also serves `llms.txt`.

**Keep inline (~60–80 lines):**

- Push guardrails (safety-critical — must never depend on a fetch)
- Frontmatter format + property mapping essentials
- Soft deletes

**Link out:** everything else (metadata files, refresh/clean details, rich-text fidelity, run summary schema).

## SOT flow

```
docusaurus/docs/**        (SOT)
      │  generate script
      ▼
internal/sync/agents.md.tmpl   (generated, committed)
      │  go:embed
      ▼
<workspace>/AGENTS.md          (written on import/refresh)
```

- Script generates the template from site source.
- CI drift check: fails if template ≠ generated output.

## README

- Shrinks to: one-liner, install, link to docs site.
- Currently stale vs AGENTS.md:
  - `--out` vs `--output`
  - filenames: README implies title-based; actually `{notion-id}.md`
  - missing commands: `push`, `clean`, `agents-md`, `--ids`

## Open questions

- Hosting: GitHub Pages vs Cloudflare?
- Versioned docs (like salurbal-obt `versioned_docs/`) — needed now?
- Exact inline set for short AGENTS.md.
