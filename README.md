# notion-sync

CLI tool to sync Notion databases to local Markdown files with YAML frontmatter.

Given a Notion database URL, notion-sync fetches all entries via the Notion API and writes them to `.md` files on disk. Each file gets YAML frontmatter containing the Notion ID, URL, edit timestamp, and all property values. On subsequent runs it compares `last_edited_time` and only re-syncs entries that changed.

## Architecture

```
packages/
  core/     @notion-sync/core       Platform-agnostic sync engine (all business logic)
  cli/      notion-sync             CLI tool using node:fs
```

**Core** contains everything: Notion API calls, block-to-Markdown conversion, frontmatter generation, incremental sync, and deletion tracking. It never imports `node:fs` directly.

**CLI** is a thin adapter that implements two interfaces (`FileSystem` and `FrontmatterReader`) defined in core, then passes them in. This keeps platform-specific code to a few dozen lines.

```
                  +-----------+
                  |    CLI    |
                  | (node:fs) |
                  +-----+-----+
                        |
                        v
              FileSystem + FrontmatterReader  (interfaces)
                        |
                  +-----v-----+
                  |   Core    |
                  | (sync     |
                  |  engine)  |
                  +-----------+
```

## Prerequisites

- **Node.js** >= 18
- **npm** >= 9 (ships with Node)
- A **Notion integration** with access to the databases you want to sync

### Creating a Notion integration

1. Go to [notion.so/my-integrations](https://www.notion.so/my-integrations)
2. Click "New integration"
3. Give it a name (e.g. "notion-sync") and select a workspace
4. Copy the **Internal Integration Secret** (starts with `ntn_`)
5. In Notion, open the database you want to sync
6. Click the `...` menu > "Connections" > add your integration

## Quick start

**All commands run from the repo root** — don't `cd` into package subdirectories.

```sh
# Clone and install
git clone https://github.com/ran-codes/notion-sync.git
cd notion-sync
npm install

# Build all packages
npm run build

# Sync a Notion database
node packages/cli/dist/main.js sync <database-url-or-id> \
  --api-key <your-api-key> \
  --output ./out

# Or save your API key (stored in OS keychain)
node packages/cli/dist/main.js config set apiKey <your-api-key>
node packages/cli/dist/main.js sync <database-url-or-id> --output ./out

# Refresh an existing sync (incremental update)
node packages/cli/dist/main.js refresh ./out/MyDatabase

# Force refresh (resync all, ignoring timestamps)
node packages/cli/dist/main.js refresh ./out/MyDatabase --force

# List synced databases
node packages/cli/dist/main.js list ./out
```

## Commands

```sh
npm run build          # Build both packages
npm run build:core     # Build only core (tsc)
npm run build:cli      # Build only CLI (tsc)
npm run test           # Run core unit tests (vitest, 83 tests)
```

## Package documentation

- [packages/core/README.md](packages/core/README.md) -- Sync engine internals, file-by-file guide, key patterns
- [packages/cli/README.md](packages/cli/README.md) -- CLI install, command reference, configuration

## Key design decisions

- **Incremental sync** -- compares `last_edited_time` from frontmatter and skips unchanged entries
- **Force refresh** -- `--force` flag bypasses timestamp checks to resync all entries (useful when database schema changes)
- **Soft deletes** -- entries removed from a Notion database get `notion-deleted: true` in their frontmatter rather than being deleted from disk
- **Two orchestration functions** -- `freshDatabaseImport()` for first-time imports, `refreshDatabase()` for incremental updates with diff-based optimization
- **Database metadata file** -- each synced database folder contains `_database.json` with metadata (database ID, title, URL, last sync time, entry count), enabling `refreshDatabase()` to work from just a folder path
- **Forward-slash paths** -- core always uses `/` as the path separator; platform adapters resolve to OS-native paths
- **Manual YAML serialization** -- frontmatter is written with hand-rolled code for precise formatting; the `yaml` package is used only for parsing
- **Newer Notion API** -- database entries are queried via `client.dataSources.query()` (not `databases.query()`) to get full property data

## Dependencies

| Package | Version | Used by |
|---------|---------|---------|
| `@notionhq/client` | ^5.3.0 | core |
| `yaml` | ^2.7.0 | core |
| `@napi-rs/keyring` | ^1.1.0 | cli |
| `vitest` | ^3.0.0 | core (dev) |

## Origin

Extracted from [obsidian-notion-database-sync](https://github.com/ran-codes/obsidian-notion-database-sync), an Obsidian plugin. The ADR documenting the extraction is at `.claude/reference/v0.1/initial-ideas.md`.
