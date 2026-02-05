# notion-sync

CLI tool that syncs Notion databases to local Markdown files with YAML frontmatter.

## Quick Start (for agents)

```sh
cd go
go build ./cmd/notion-sync    # build binary
go test ./...                 # run tests
```

## Repo Layout

```
go/                           # Go implementation (primary)
├── cmd/notion-sync/          # CLI entry point
└── internal/
    ├── notion/               # API client
    ├── sync/                 # Core sync logic
    ├── markdown/             # Block → Markdown
    ├── frontmatter/          # YAML handling
    └── config/               # Keyring + config

packages/                     # TypeScript (legacy backup)
├── core/                     # Sync engine
└── cli/                      # Node CLI
```

**Go docs:** `go/CLAUDE.md` — implementation details, how to add block/property types

**Legacy TypeScript docs:** `packages/core/CLAUDE.md`, `packages/cli/CLAUDE.md`

---

## Architecture

### Data Flow

```
Notion Database
       ↓
FreshDatabaseImport() or RefreshDatabase()
       ↓
   FreezePage() (per entry)
       ↓
   .md files with YAML frontmatter
```

### Key Functions

| Function | Use Case | Behavior |
|----------|----------|----------|
| `FreshDatabaseImport()` | First-time import | Imports all entries, writes `_database.json` |
| `RefreshDatabase()` | Incremental update | Reads `_database.json`, compares timestamps, skips unchanged |
| `RefreshDatabase(force=true)` | Full resync | Ignores timestamps, resyncs all entries |
| `ListSyncedDatabases()` | Discovery | Scans folder for `_database.json` files |

### Metadata File

Each synced database folder contains `_database.json`:
```json
{ "databaseId": "...", "title": "...", "url": "...", "folderPath": "...", "lastSyncedAt": "...", "entryCount": N }
```

### Progress Phases

Progress callback reports: `querying` → `diffing` → `stale-detected` → `importing` → `complete`

---

## Key Code Locations (Go)

| To understand... | Look at... |
|------------------|------------|
| CLI entry point | `go/cmd/notion-sync/main.go` |
| Notion API client | `go/internal/notion/client.go` |
| API response types | `go/internal/notion/types.go` |
| Database sync logic | `go/internal/sync/database.go` |
| Page/entry processing | `go/internal/sync/page.go` |
| Block → Markdown | `go/internal/markdown/converter.go` |
| Rich text handling | `go/internal/markdown/richtext.go` |
| YAML frontmatter | `go/internal/frontmatter/` |
| Config & keyring | `go/internal/config/` |

---

## Key Design Decisions

- **Database-only sync** — no individual page syncing
- **Metadata file** — `_database.json` in each folder stores databaseId, title, url, last sync time
- **Force refresh** — `--force` flag bypasses timestamp checks (useful when database schema changes)
- **Notion dataSources API** — uses `/data_sources/{id}/query` (not `/databases/{id}/query`)
- **Manual YAML serialization** — `yaml` package used only for *parsing*; writing is manual for precise formatting
- **Soft deletes** — removed entries get `notion-deleted: true` in frontmatter
- **Incremental sync** — compares `notion-last-edited` timestamps
- **No third-party Notion client** — thin REST wrapper for full control over rate limiting

---

## Common Tasks

### Add a new Notion block type

1. Add case to `convertBlock()` in `go/internal/markdown/converter.go`
2. Add tests in `go/internal/markdown/converter_test.go`
3. Run `go test ./...`

### Add a new property type

1. Add case to `mapPropertiesToFrontmatter()` in `go/internal/sync/page.go`
2. Run `go test ./...`

### Modify progress reporting

1. Update `ProgressPhase` struct in `go/internal/sync/types.go`
2. Update phase emissions in `go/internal/sync/database.go`
3. Update `formatProgress()` in `go/cmd/notion-sync/main.go`

---

## Dependencies (Go)

| Package | Used for |
|---------|----------|
| `github.com/zalando/go-keyring` | OS keychain access |
| `gopkg.in/yaml.v3` | YAML parsing only |

---

## Release

Push a git tag (`v1.0.0`) to trigger GitHub Actions release workflow. Builds binaries for:
- Windows amd64
- macOS amd64, arm64
- Linux amd64, arm64

Install script: `curl -fsSL https://raw.githubusercontent.com/ran-codes/notion-sync/main/scripts/install.sh | bash`
