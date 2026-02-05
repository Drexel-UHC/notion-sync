# notion-sync

Monorepo that syncs Notion databases to local Markdown files with YAML frontmatter.

## Quick Start (for agents)

```sh
npm install          # install deps
npm run build        # build all packages
npm run test         # run tests (83 tests in core)
```

## Repo Layout

```
packages/
  core/     @notion-sync/core       Platform-agnostic sync engine (all business logic)
  cli/      notion-sync             CLI tool using node:fs
  vscode/   notion-sync-vscode      VS Code extension using vscode.workspace.fs
```

---

## Architecture

### Data Flow

```
Notion Database
       ↓
freshDatabaseImport() or refreshDatabase()
       ↓
   freezePage() (per entry)
       ↓
   .md files with YAML frontmatter
```

### Dependency Injection

Core never touches the filesystem directly. Platform adapters implement two interfaces:

```typescript
interface FileSystem {
  readFile, writeFile, fileExists, mkdir, listMarkdownFiles, listDirectories
}

interface FrontmatterReader {
  readFrontmatter(filePath): Promise<Record<string, unknown> | null>
}
```

- **CLI** implements with `node:fs/promises`
- **VS Code** implements with `vscode.workspace.fs`
- **Tests** use in-memory mocks

### Orchestration Functions

| Function | Use Case | Behavior |
|----------|----------|----------|
| `freshDatabaseImport()` | First-time import | Imports all entries, writes `_database.json` |
| `refreshDatabase()` | Incremental update | Reads `_database.json`, compares timestamps, skips unchanged |
| `listSyncedDatabases()` | Discovery | Scans folder for `_database.json` files |
| `readDatabaseMetadata()` | Single folder | Reads `_database.json` from a folder |

### Metadata File

Each synced database folder contains `_database.json`:
```json
{ "databaseId": "...", "title": "...", "folderPath": "...", "lastSyncedAt": "...", "entryCount": N }
```
This allows `refreshDatabase()` to work from just a folder path without external state.

### Progress Phases

Progress callback reports: `querying` → `diffing` → `stale-detected` → `importing` → `complete`

---

## Key Code Locations

| To understand... | Look at... |
|------------------|------------|
| Core public API | `packages/core/src/index.ts` |
| Types & interfaces | `packages/core/src/types.ts` |
| Database sync logic | `packages/core/src/database-freezer.ts` |
| Page/entry processing | `packages/core/src/page-freezer.ts` |
| Block → Markdown | `packages/core/src/block-converter.ts` |
| Rate limiting & retry | `packages/core/src/notion-client.ts` |
| CLI commands | `packages/cli/src/main.ts` |
| VS Code commands | `packages/vscode/src/commands.ts` |

---

## Key Design Decisions

- **Database-only sync** — no individual page syncing
- **Metadata file** — `_database.json` in each folder stores databaseId, title, last sync time
- **Notion dataSources API** — uses `client.dataSources.query()` (not `databases.query()`)
- **Forward-slash paths** — core uses `/` internally; adapters resolve to OS paths
- **Manual YAML serialization** — `yaml` package used only for *parsing*
- **Soft deletes** — removed entries get `notion-deleted: true` in frontmatter
- **Incremental sync** — compares `notion-last-edited` timestamps

---

## Common Tasks

### Add a new Notion block type

1. Add case to `convertBlocksToMarkdown()` in `packages/core/src/block-converter.ts`
2. Add tests in `packages/core/tests/block-converter.test.ts`
3. Run `npm test`

### Add a new property type

1. Add case to `mapPropertiesToFrontmatter()` in `packages/core/src/page-freezer.ts`
2. Add tests in `packages/core/tests/page-freezer.test.ts`
3. Run `npm test`

### Modify progress reporting

1. Update `ProgressPhase` type in `packages/core/src/types.ts`
2. Update phase emissions in `packages/core/src/database-freezer.ts`
3. Update `formatProgress()` in CLI (`packages/cli/src/main.ts`) and VS Code (`packages/vscode/src/commands.ts`)

---

## Dependencies

| Package | Version | Used by |
|---------|---------|---------|
| `@notionhq/client` | ^5.3.0 | core |
| `yaml` | ^2.7.0 | core |
| `@napi-rs/keyring` | ^1.1.0 | cli |
| `vitest` | ^3.0.0 | core (dev) |
| `esbuild` | ^0.25.0 | vscode (dev) |

---

## Origin

Extracted from [obsidian-notion-database-sync](https://github.com/ran-codes/obsidian-notion-database-sync).
