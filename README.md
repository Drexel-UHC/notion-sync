# notion-sync

CLI tool to sync Notion databases to local Markdown files with YAML frontmatter.

Given a Notion database ID, notion-sync fetches all entries via the Notion API and writes them to `.md` files on disk. Each file gets YAML frontmatter containing the Notion ID, URL, edit timestamp, and all property values. On subsequent runs it compares `last_edited_time` and only re-syncs entries that changed.

## Install

### Install script (macOS / Linux)

```sh
curl -fsSL https://raw.githubusercontent.com/ran-codes/notion-sync/main/scripts/install.sh | bash
```

### Scoop (Windows)

```powershell
## Install Scoop if you don't have it: https://scoop.sh
# irm get.scoop.sh | iex
scoop bucket add notion-sync https://github.com/ran-codes/notion-sync
scoop install notion-sync
```

### Manual download

Download the binary for your platform from [GitHub Releases](https://github.com/ran-codes/notion-sync/releases), rename it to `notion-sync` (or `notion-sync.exe` on Windows), and add it to your PATH.

## Update

### Install script (macOS / Linux)

Re-run the install script — it always fetches the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/ran-codes/notion-sync/main/scripts/install.sh | bash
```

### Scoop (Windows)

```powershell
scoop update
scoop update notion-sync
```

### Manual

Download the latest binary from [GitHub Releases](https://github.com/ran-codes/notion-sync/releases) and replace the existing one.

## Usage

```sh
# Store your API key (saved in OS keychain)
notion-sync config set apiKey <your-notion-api-key>

# Import databases into an output folder
notion-sync import <database-id-A> --output ./my-notes
notion-sync import <database-id-B> --output ./my-notes

# Refresh (incremental — only changed entries)
notion-sync refresh ./my-notes/Database\ A

# Force refresh (resync everything)
notion-sync refresh ./my-notes/Database\ A --force

# List synced databases
notion-sync list ./my-notes
```

The `--output` folder is a **workspace**. Each database gets a subfolder, and all databases in a workspace share a single SQLite store:

```
my-notes/                        ← workspace (--output target)
├── _notion_sync.db              ← shared SQLite store (FTS5 search, all pages)
├── Database A/
│   ├── _database.json
│   ├── Page One.md
│   └── Page Two.md
└── Database B/
    ├── _database.json
    ├── Entry Alpha.md
    └── Entry Beta.md
```

By default both `.md` files and SQLite are written. Use `--output-mode` to control this:

```sh
notion-sync import <id> --output ./notes --output-mode markdown  # .md only
notion-sync import <id> --output ./notes --output-mode sqlite    # SQLite only
notion-sync import <id> --output ./notes --output-mode both      # default
```

You can also set the default in config: `notion-sync config set outputMode sqlite`

## Prerequisites

- A **Notion integration** with access to the databases you want to sync

### Creating a Notion integration

1. Go to [notion.so/my-integrations](https://www.notion.so/my-integrations)
2. Click "New integration"
3. Give it a name (e.g. "notion-sync") and select a workspace
4. Copy the **Internal Integration Secret** (starts with `ntn_`)
5. In Notion, open the database you want to sync
6. Click the `...` menu > "Connections" > add your integration

### Finding your database ID

Open the database as a **full page** in Notion. The URL will look like:

```
https://www.notion.so/yourworkspace/abc123def4567890abcdef1234567890?v=...
```

The database ID is the 32-character hex string after your workspace name — in this example, `abc123def4567890abcdef1234567890`. You can pass it with or without dashes; notion-sync accepts both formats as well as the full URL.

## Commands

```sh
notion-sync import <database-id> [--out <folder>] [--api-key <key>]
notion-sync refresh <folder> [--force] [--api-key <key>]
notion-sync list [<folder>]
notion-sync config set <key> <value>
```

| Command                   | Description                               |
| ------------------------- | ----------------------------------------- |
| `import`                  | First-time import of a Notion database    |
| `refresh`                 | Incremental update (only changed entries) |
| `refresh --force`         | Full resync ignoring timestamps           |
| `list`                    | Show all synced databases in a folder     |
| `config set apiKey <key>` | Store API key in OS keychain              |

## Architecture

```
cmd/notion-sync/         # CLI entry point
internal/
├── notion/              # API client (rate limit, retry)
├── sync/                # Core sync logic
├── markdown/            # Block → Markdown conversion
├── frontmatter/         # YAML parse/write
└── config/              # Keyring + config file
```

## Development

```sh
go build ./cmd/notion-sync   # Build binary
```

### Testing

```sh
# Unit + integration tests (mock client, no API needed)
go test ./...

# System tests (hit real Notion API, require API key)
/test-single-datasource-db        # single data source lifecycle
/test-double-datasource-db        # multi-source layout + edge cases

# Everything together (unit → single → double, sequential)
/test
```

## Documentation

See [CLAUDE.md](CLAUDE.md) for implementation details, how to add block/property types.

## Key design decisions

- **Incremental sync** -- compares `last_edited_time` from frontmatter and skips unchanged entries
- **Force refresh** -- `--force` flag bypasses timestamp checks to resync all entries (useful when database schema changes)
- **Soft deletes** -- entries removed from a Notion database get `notion-deleted: true` in their frontmatter rather than being deleted from disk
- **Two orchestration functions** -- `freshDatabaseImport()` for first-time imports, `refreshDatabase()` for incremental updates with diff-based optimization
- **Database metadata file** -- each synced database folder contains `_database.json` with metadata (database ID, title, URL, last sync time, entry count), enabling `refreshDatabase()` to work from just a folder path
- **Manual YAML serialization** -- frontmatter is written with hand-rolled code for precise formatting; the `yaml` package is used only for parsing
- **Newer Notion API** -- database entries are queried via `client.dataSources.query()` (not `databases.query()`) to get full property data

## Dependencies

| Package                         | Used for                       |
| ------------------------------- | ------------------------------ |
| `github.com/zalando/go-keyring` | OS keychain access             |
| `gopkg.in/yaml.v3`              | YAML parsing                   |
| `modernc.org/sqlite`            | Pure-Go SQLite (FTS5, no CGO)  |

No third-party Notion client — uses a thin REST wrapper for full control over rate limiting.

## Origin

Extracted from [obsidian-notion-database-sync](https://github.com/ran-codes/obsidian-notion-database-sync), an Obsidian plugin.
