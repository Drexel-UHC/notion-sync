# notion-sync

CLI tool that syncs Notion databases to local Markdown files with YAML frontmatter.

## Quick Start (for agents)

```sh
go build ./cmd/notion-sync    # build binary
go test ./...                 # run tests
```

## Repo Layout

```
cmd/notion-sync/main.go       # CLI entry point, flag parsing, commands
internal/
├── notion/
│   ├── client.go             # HTTP client, throttle, retry logic
│   ├── client_test.go        # NormalizeNotionID tests
│   └── types.go              # All Notion API response structs
├── sync/
│   ├── database.go           # FreshDatabaseImport, RefreshDatabase
│   ├── page.go               # FreezePage, property mapping
│   ├── metadata.go           # _database.json read/write
│   └── types.go              # FrozenDatabase, result types, progress
├── markdown/
│   ├── converter.go          # 30+ block types → Markdown
│   ├── converter_test.go     # Block conversion tests
│   └── richtext.go           # Rich text annotations
├── frontmatter/
│   ├── parser.go             # Parse YAML from .md files
│   ├── writer.go             # Manual YAML serialization
│   └── frontmatter_test.go
├── config/
│   ├── config.go             # Config file, env vars, key priority
│   └── keyring.go            # go-keyring wrapper
└── util/
    ├── path.go               # SanitizeFileName, JoinPath
    └── path_test.go
```

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

## Key Code Locations

| To understand... | Look at... |
|------------------|------------|
| CLI entry point | `cmd/notion-sync/main.go` |
| Notion API client | `internal/notion/client.go` |
| API response types | `internal/notion/types.go` |
| Database sync logic | `internal/sync/database.go` |
| Page/entry processing | `internal/sync/page.go` |
| Block → Markdown | `internal/markdown/converter.go` |
| Rich text handling | `internal/markdown/richtext.go` |
| YAML frontmatter | `internal/frontmatter/` |
| Config & keyring | `internal/config/` |

---

## Key Design Decisions

- **Database-only import** — no individual page syncing
- **Metadata file** — `_database.json` in each folder stores databaseId, title, url, last sync time
- **Force refresh** — `--force` flag bypasses timestamp checks (useful when database schema changes)
- **Notion dataSources API** — uses `/data_sources/{id}/query` (not `/databases/{id}/query`)
- **Manual YAML serialization** — `yaml` package used only for *parsing*; writing is manual for precise formatting
- **Soft deletes** — removed entries get `notion-deleted: true` in frontmatter
- **Incremental sync** — compares `notion-last-edited` timestamps
- **No third-party Notion client** — thin REST wrapper for full control over rate limiting

---

## Notion Client (`internal/notion/`)

### Rate Limiting

```go
const minRequestIntervalMs = 340  // ~3 requests per second
```

Mutex-protected `lastRequestTime` ensures minimum interval between requests.

### Retry Logic

- Max 5 retries
- Retryable status codes: 429, 500, 502, 503, 504
- Exponential backoff: `2^attempt` seconds
- ±25% jitter to avoid thundering herd
- Respects `Retry-After` header on 429
- Max backoff capped at 30 seconds

### Key Client Functions

| Function | Purpose |
|----------|---------|
| `NewClient(apiKey)` | Create client |
| `GetDatabase(id)` | Fetch database metadata |
| `QueryAllEntries(dataSourceID)` | Paginated query for all entries |
| `FetchAllBlocks(pageID)` | Paginated fetch of all blocks |
| `NormalizeNotionID(input)` | Convert URL/hex/UUID to standard UUID format |

### API Endpoints Used

```
GET  /databases/{id}
GET  /data_sources/{id}
POST /data_sources/{id}/query
GET  /pages/{id}
GET  /blocks/{id}/children
```

---

## Sync Logic (`internal/sync/`)

### FreshDatabaseImport

1. Fetch database metadata
2. Get dataSourceID from `database.DataSources[0].ID`
3. Query all entries via dataSource
4. For each entry: `FreezePage()`
5. Write `_database.json`

### RefreshDatabase

1. Read `_database.json` to get databaseID
2. Fetch database metadata
3. Query all entries
4. Scan local `.md` files for `notion-id` and `notion-last-edited`
5. Skip entries where timestamps match (unless `force=true`)
6. For stale entries: `FreezePage()`
7. Mark deleted entries with `notion-deleted: true`
8. Update `_database.json`

### FreezePage

1. Fetch page (or use pre-fetched)
2. Extract title from `title` property
3. Check if file exists and timestamps match → skip
4. Fetch all blocks
5. Convert blocks to Markdown
6. Build frontmatter with properties
7. Write `.md` file

---

## Markdown Conversion (`internal/markdown/`)

### Supported Block Types

| Type | Output |
|------|--------|
| `paragraph` | Text |
| `heading_1/2/3` | `#/##/###` (toggleable → callout) |
| `bulleted_list_item` | `- item` |
| `numbered_list_item` | `1. item` (auto-numbered) |
| `to_do` | `- [ ]` or `- [x]` |
| `code` | Fenced code block |
| `quote` | `> quote` |
| `callout` | `> [!type]` (emoji → type mapping) |
| `equation` | `$$...$$` |
| `divider` | `---` |
| `toggle` | `> [!note]+ ...` |
| `child_page` | `[[title]]` |
| `child_database` | HTML comment |
| `image` | `![alt](url)` |
| `video/audio/file/pdf` | `[caption](url)` or URL |
| `bookmark/embed` | `[caption](url)` or URL |
| `link_to_page` | `[[notion-id: ...]]` |
| `synced_block` | Fetches and converts children |
| `table` | Markdown table |
| `column_list` | Columns separated by `---` |

### Rich Text Annotations

| Annotation | Markdown |
|------------|----------|
| Bold | `**text**` |
| Italic | `*text*` |
| Code | `` `text` `` |
| Strikethrough | `~~text~~` |
| Underline | `<u>text</u>` |
| Background color | `==text==` |
| Link | `[text](url)` |
| Equation | `$expression$` |

### Mention Types

| Type | Output |
|------|--------|
| Page | `[[notion-id: id]]` |
| Database | `[[notion-id: id]]` |
| Date | `start` or `start → end` |
| User | `@name` |
| Link preview | `[text](url)` |

---

## Frontmatter (`internal/frontmatter/`)

### Parsing

Uses `gopkg.in/yaml.v3` to parse existing frontmatter from `.md` files.

### Writing

**Manual serialization** (no `yaml.Marshal`) for precise control:

- Keys with `:` or space are quoted
- Strings are quoted if they contain: `:`, `#`, `'`, `"`, `\n`, or start with space/dash/bracket
- Strings matching `true`, `false`, `null`, or all-digits are quoted
- Arrays use `- item` format
- Empty arrays use `[]`

---

## Property Mapping (`internal/sync/page.go`)

| Notion Type | Frontmatter Value |
|-------------|-------------------|
| `title` | (used as filename, not in frontmatter) |
| `rich_text` | Converted to plain Markdown |
| `number` | Number or `null` |
| `select` | Option name or `null` |
| `multi_select` | Array of option names |
| `status` | Status name or `null` |
| `date` | `start` or `start → end` |
| `checkbox` | `true` or `false` |
| `url/email/phone_number` | String or `null` |
| `relation` | Array of page IDs |
| `people` | Array of names (or IDs) |
| `files` | Array of URLs |
| `created_time/last_edited_time` | ISO timestamp |

Skipped: `formula`, `rollup`, `button`, `unique_id`, `verification`

---

## Config (`internal/config/`)

### API Key Priority

1. `--api-key` flag
2. `NOTION_SYNC_API_KEY` env var
3. OS keychain (via go-keyring)
4. Config file (`~/.notion-sync.json`) with warning

### Keyring

```go
const keyringService = "notion-sync"
const keyringAccount = "api-key"
```

Uses `github.com/zalando/go-keyring`:
- Windows: Credential Manager
- macOS: Keychain
- Linux: Secret Service (GNOME Keyring, KWallet)

### Config File

Location: `$XDG_CONFIG_HOME/notion-sync/config.json` or `~/.notion-sync.json`

```json
{
  "defaultOutputFolder": "./notion"
}
```

API key is NOT stored in config file when keyring is available.

---

## CLI Commands

```sh
notion-sync import <database-id> [--output <folder>] [--api-key <key>]
notion-sync refresh <folder> [--force/-f] [--api-key <key>]
notion-sync list [<folder>]
notion-sync config set <key> <value>
```

Exit codes: `0` success, `1` error

---

## Common Tasks

### Add a new Notion block type

1. Add struct field to `Block` in `internal/notion/types.go` if needed
2. Add case to `convertBlock()` in `internal/markdown/converter.go`
3. Add test case in `internal/markdown/converter_test.go`
4. Run `go test ./internal/markdown/`

### Add a new property type

1. Add struct field to `Property` in `internal/notion/types.go` if needed
2. Add case to `mapPropertiesToFrontmatter()` in `internal/sync/page.go`
3. Run `go test ./...`

### Add a new CLI flag

1. Add `fs.Type()` call in the relevant `run*` function in `main.go`
2. Pass value to the appropriate sync function
3. Update usage text

### Modify progress reporting

1. Update `ProgressPhase` struct in `internal/sync/types.go`
2. Update phase emissions in `internal/sync/database.go`
3. Update `formatProgress()` in `cmd/notion-sync/main.go`

---

## Dependencies

| Package | Used for |
|---------|----------|
| `github.com/zalando/go-keyring` | OS keychain access |
| `gopkg.in/yaml.v3` | YAML parsing only |

---

## Testing

```sh
go test ./...                           # all tests
go test ./internal/markdown/            # just converter tests
go test -v ./internal/notion/           # verbose client tests
go test -run TestConvertBlocksToMarkdown ./internal/markdown/  # specific test
```

Tests don't require Notion API access — they test pure conversion logic.

### Test Databases

| Name | Database ID | Reference |
|------|-------------|-----------|
| Complex (Property & Block Coverage) | `2fe57008-e885-8003-b1f3-cc05981dc6b0` | `.claude/reference/v0.1/test-databases/complex/` |

---

## Release

Push a git tag (`v1.0.0`) to trigger GitHub Actions release workflow. Builds binaries for:
- Windows amd64
- macOS amd64, arm64
- Linux amd64, arm64

Install script: `curl -fsSL https://raw.githubusercontent.com/ran-codes/notion-sync/main/scripts/install.sh | bash`
