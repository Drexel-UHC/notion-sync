# notion-sync

Sync Notion pages and databases to local Markdown files with YAML frontmatter. Supports incremental sync, deletion tracking, and 30+ block types.

## Architecture

```
packages/
  core/     ← all business logic, zero platform dependencies
  cli/      ← thin wrapper using node:fs
  vscode/   ← thin wrapper using vscode.workspace.fs
```

- **Core** owns everything: Notion API calls, block-to-Markdown conversion, frontmatter generation, sync logic
- **CLI** and **VS Code** are adapters — they implement two small interfaces (`FileSystem`, `FrontmatterReader`) and wire them to core
- Core never imports `node:fs` or `vscode` — platform behavior is injected, not hardcoded

### Origin

- Extracted from [obsidian-notion-database-sync](https://github.com/ran-codes/obsidian-notion-database-sync), an Obsidian plugin
  - All Obsidian-specific code (`app.vault`, `requestUrl`, modals, settings UI) was replaced with injectable interfaces
  - The block converter, retry logic, and property mapper were carried over unchanged
  - ADR documenting the extraction: `.claude/reference/v0.1/initial-ideas.md`

## Workflow

### Setup

```sh
npm install
```

### Build

```sh
npm run build          # all packages
npm run build:core     # just core
npm run build:cli      # just cli
npm run build:vscode   # just vscode (esbuild bundle)
```

### Test

```sh
npm test               # runs vitest in packages/core
```

76 unit tests covering block conversion, Notion client, page freezer, and database freezer. Tests mock the `FileSystem` and `FrontmatterReader` interfaces — no Notion API calls.

### Run (CLI)

```sh
node packages/cli/dist/main.js sync <notion-url-or-id> --api-key <key> --output ./out
node packages/cli/dist/main.js resync ./out/SomePage.md --api-key <key>
node packages/cli/dist/main.js config set apiKey <key>
```

### Run (VS Code)

1. Open this repo in VS Code
2. Press `F5` to launch Extension Development Host
3. Run "Notion Sync: Sync Page or Database" from the Command Palette
4. Set `notionSync.apiKey` in VS Code settings

### Develop

- Edit core logic in `packages/core/src/` — this is where sync behavior lives
- Edit CLI-specific code in `packages/cli/src/` (arg parsing, config, node adapters)
- Edit VS Code-specific code in `packages/vscode/src/` (commands, vscode adapters)
- After editing core, rebuild before testing CLI/VS Code: `npm run build:core`

### Deploy

- **CLI**: publish to npm as `notion-sync` (`npm publish -w packages/cli`)
- **VS Code**: bundle and publish (`cd packages/vscode && npx @vscode/vsce package`)
