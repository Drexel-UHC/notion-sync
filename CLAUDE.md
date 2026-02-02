# notion-sync

Monorepo that syncs Notion pages and databases to local Markdown files with YAML frontmatter. Three packages share one core library.

## Repo layout

```
packages/
  core/     @notion-sync/core  — platform-agnostic sync engine (all business logic)
  cli/      notion-sync        — CLI wrapper wiring node:fs to core
  vscode/   notion-sync-vscode — VS Code extension wiring vscode.workspace.fs to core
```

## Commands

```sh
npm run build          # build all 3 packages (tsc for core/cli, esbuild for vscode)
npm run test           # run core unit tests (vitest)
npm run build:core     # build only core
npm run build:cli      # build only cli
npm run build:vscode   # build only vscode extension
```

## Architecture

Core is platform-agnostic. It never imports `node:fs`, `vscode`, or any platform API directly. Platform-specific behaviour is injected via two interfaces defined in `packages/core/src/types.ts`:

- **`FileSystem`** — readFile, writeFile, fileExists, mkdir, listMarkdownFiles
- **`FrontmatterReader`** — readFrontmatter (parse YAML between `---` markers)

Each consumer (CLI, VS Code) provides its own implementation of these interfaces.

## Key design decisions

- The Notion client uses `client.dataSources.query()` (not `databases.query()`) for database entries. This is a newer Notion API.
- Core uses forward-slash paths internally. Platform adapters resolve to OS-native paths.
- YAML frontmatter is serialized manually (not via a library) to control formatting. The `yaml` npm package is used only for *parsing* frontmatter.
- Incremental sync via `last_edited_time` comparison — skips pages that haven't changed.
- Deletion tracking marks files with `notion-deleted: true` in frontmatter rather than deleting them.

## Dependencies

- `@notionhq/client` ^5.3.0 — Notion SDK (core)
- `yaml` ^2.7.0 — YAML parsing for frontmatter (core, cli, vscode)
- `vitest` ^3.0.0 — testing (core devDep)
- `esbuild` ^0.25.0 — bundling (vscode devDep)

## Origin

Extracted from the Obsidian plugin [obsidian-notion-database-sync](https://github.com/ran-codes/obsidian-notion-database-sync). The ADR is at `.claude/reference/v0.1/initial-ideas.md`.
