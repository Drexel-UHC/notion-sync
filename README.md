# notion-sync

CLI tool to sync Notion databases to local Markdown files with YAML frontmatter.

Given a Notion database URL, notion-sync fetches all entries via the Notion API and writes them to `.md` files on disk. Each file gets YAML frontmatter containing the Notion ID, URL, edit timestamp, and all property values. On subsequent runs it compares `last_edited_time` and only re-syncs entries that changed.

## Install (Recommended)

Download a pre-built binary for your platform:

```sh
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/ran-codes/notion-sync/main/scripts/install.sh | bash

# Or download manually from GitHub Releases:
# https://github.com/ran-codes/notion-sync/releases
```

Then configure and use:

```sh
# Store your API key (saved in OS keychain)
notion-sync config set apiKey <your-notion-api-key>

# Sync a database
notion-sync sync https://notion.so/your-database-url --output ./notion

# Refresh (incremental update)
notion-sync refresh ./notion/MyDatabase

# Force refresh (resync all entries)
notion-sync refresh ./notion/MyDatabase --force

# List synced databases
notion-sync list ./notion
```

## Build from Source

### Go (Recommended)

```sh
cd go
go build -o notion-sync ./cmd/notion-sync
./notion-sync --help
```

### TypeScript (Legacy)

```sh
npm install
npm run build
node packages/cli/dist/main.js --help
```

## Prerequisites

- A **Notion integration** with access to the databases you want to sync

### Creating a Notion integration

1. Go to [notion.so/my-integrations](https://www.notion.so/my-integrations)
2. Click "New integration"
3. Give it a name (e.g. "notion-sync") and select a workspace
4. Copy the **Internal Integration Secret** (starts with `ntn_`)
5. In Notion, open the database you want to sync
6. Click the `...` menu > "Connections" > add your integration

## Commands

```sh
notion-sync sync <database-url> [--output <folder>] [--api-key <key>]
notion-sync refresh <folder> [--force] [--api-key <key>]
notion-sync list [<folder>]
notion-sync config set <key> <value>
```

| Command | Description |
|---------|-------------|
| `sync` | First-time import of a Notion database |
| `refresh` | Incremental update (only changed entries) |
| `refresh --force` | Full resync ignoring timestamps |
| `list` | Show all synced databases in a folder |
| `config set apiKey <key>` | Store API key in OS keychain |

## Architecture

```
go/                          # Go implementation (recommended)
├── cmd/notion-sync/         # CLI entry point
└── internal/
    ├── notion/              # API client (rate limit, retry)
    ├── sync/                # Core sync logic
    ├── markdown/            # Block → Markdown conversion
    ├── frontmatter/         # YAML parse/write
    └── config/              # Keyring + config file

packages/                    # TypeScript implementation (legacy)
├── core/                    # Platform-agnostic sync engine
└── cli/                     # Node.js CLI adapter
```

## Development

### Go

```sh
cd go
go test ./...                # Run tests
go build ./cmd/notion-sync   # Build binary
```

### TypeScript

```sh
npm install
npm run build                # Build all packages
npm run test                 # Run core unit tests (83 tests)
```

## Documentation

- [go/CLAUDE.md](go/CLAUDE.md) -- Go implementation details, how to add block/property types
- [packages/core/CLAUDE.md](packages/core/CLAUDE.md) -- Legacy TypeScript sync engine
- [packages/cli/CLAUDE.md](packages/cli/CLAUDE.md) -- Legacy TypeScript CLI

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

### Go

| Package | Used for |
|---------|----------|
| `github.com/zalando/go-keyring` | OS keychain access |
| `gopkg.in/yaml.v3` | YAML parsing |

No third-party Notion client — uses a thin REST wrapper for full control over rate limiting.

### TypeScript (Legacy)

| Package | Version | Used by |
|---------|---------|---------|
| `@notionhq/client` | ^5.3.0 | core |
| `yaml` | ^2.7.0 | core |
| `@napi-rs/keyring` | ^1.1.0 | cli |
| `vitest` | ^3.0.0 | core (dev) |

## Origin

Extracted from [obsidian-notion-database-sync](https://github.com/ran-codes/obsidian-notion-database-sync), an Obsidian plugin.
