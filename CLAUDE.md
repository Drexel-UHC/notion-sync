# notion-sync

CLI tool that syncs Notion databases to local Markdown files with YAML frontmatter.

## Two distinct agent docs in this repo — don't confuse them

| File | Audience | Lives at (source of truth) | Lives at (deployed bundle) |
| --- | --- | --- | --- |
| **`CLAUDE.md`** (this file) | Agents working **on** notion-sync's code | repo root — `./CLAUDE.md` | not deployed; dev-only |
| **`AGENTS.md`** (generated) | Agents working **with** the synced output (downstream consumers) | source const in `internal/sync/agents.go` | written to the user's workspace root on every `import` / `refresh` (e.g. `./notion/AGENTS.md`) |

When the user says "the agent docs" or asks about downstream documentation, they almost always mean **`AGENTS.md`**. To change what downstream agents see, edit the `agentsMDContent` const in `internal/sync/agents.go` — it gets emitted by `WriteAgentsMD` (idempotent, never overwrites a user-edited copy).

## Rules

- Be Concise - i like quick responses to iterate quickly. Save long responses for when asked about details or planning.
- Always use github cli for github oeprations; if not isntalled this a critical pblocked and tell em to install it.
- Claude skills are found in .claude/skills
- Tool use
  - **Minimize tool calls.** Use Grep, Read, Glob directly — they're fast and parallel. Never spawn a Task agent (subagent) for simple file reads or searches.
  - **No heavyweight agents for simple operations.** If a skill just needs to read/grep a handful of files, do it inline. If you think a Task agent is needed, ask me first.
- Refer to context7 first if have any questions about claude code, notion mcp or github cli
- **Never commit directly to `main`.** Always work in a branch and ship via `/ship` (PR). No exceptions.
- **Never close/merge without explicit approval**
  - Never write `closes`, `fixes`, or `resolves` in commit messages or PR descriptions — these auto-close issues on merge. Use `ref #N` to reference only.
  - Never merge into `main` by any mechanism (direct push of a merge commit, `gh pr merge`, etc.) without the user explicitly saying to merge.

## Skills

Custom skills live in `.claude/skills/`. Invoke with `/skill-name`.

| Skill | Description |
|-------|-------------|
| `/ship` | Ship code via PR — branching, committing, pushing, and creating/updating PRs |
| `/critical-code-reviewer` | Rigorous adversarial code review — security, slop, edge cases |
| `/grill-me` | Relentless design interview — stress-tests a plan by walking every branch of the decision tree |
| `/grill-with-docs` | Grilling session that sharpens terminology against the domain model and updates CONTEXT.md + ADRs inline |
| `/tdd` | Test-driven development with red-green-refactor loop and TDD philosophy guides |
| `/to-prd` | Synthesizes conversation + codebase into a PRD, written to `src_{title}.md` at repo root |
| `/to-issues` | Breaks a plan/PRD into vertical-slice issue proposals, written to `src_{title}.md` at repo root |
| `/improve-codebase-architecture` | Surfaces deepening opportunities — shallow modules to consolidate for testability and AI-navigability |
| `/test` | Run all tests (unit + system + cleanup) |
| `/test-single-datasource-db` | Integration test against the single data source test database |
| `/test-double-datasource-db` | Integration test against the double data source test database |
| `/test-standalone-page` | Integration test for standalone page import/refresh/list |
| `/release` | Tag and publish a new release interactively |
| `/clean` | Clean up merged branches |

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

| Function                      | Use Case           | Behavior                                                     |
| ----------------------------- | ------------------ | ------------------------------------------------------------ |
| `FreshDatabaseImport()`       | First-time import  | Imports all entries, writes `_database.json`                 |
| `RefreshDatabase()`           | Incremental update | Reads `_database.json`, compares timestamps, skips unchanged |
| `RefreshDatabase(force=true)` | Full resync        | Ignores timestamps, resyncs all entries                      |
| `ListSyncedDatabases()`       | Discovery          | Scans folder for `_database.json` files                      |

### Metadata File

Each synced database folder contains `_database.json`:

```json
{ "databaseId": "...", "title": "...", "url": "...", "folderPath": "...", "lastSyncedAt": "...", "entryCount": N, "syncVersion": "v0.4.0" }
```

### Progress Phases

Progress callback reports: `querying` → `diffing` → `stale-detected` → `importing` → `complete`

---

## Key Code Locations

| To understand...      | Look at...                       |
| --------------------- | -------------------------------- |
| CLI entry point       | `cmd/notion-sync/main.go`        |
| Notion API client     | `internal/notion/client.go`      |
| API response types    | `internal/notion/types.go`       |
| Database sync logic   | `internal/sync/database.go`      |
| Page/entry processing | `internal/sync/page.go`          |
| Block → Markdown      | `internal/markdown/converter.go` |
| Rich text handling    | `internal/markdown/richtext.go`  |
| YAML frontmatter      | `internal/frontmatter/`          |
| Config & keyring      | `internal/config/`               |

---

## Key Design Decisions

- **Database-only import** — no individual page syncing
- **Metadata file** — `_database.json` in each folder stores databaseId, title, url, last sync time
- **Force refresh** — `--force` flag bypasses timestamp checks (useful when database schema changes)
- **Notion dataSources API** — uses `/data_sources/{id}/query` (not `/databases/{id}/query`)
- **Manual YAML serialization** — `yaml` package used only for _parsing_; writing is manual for precise formatting
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

| Function                        | Purpose                                      |
| ------------------------------- | -------------------------------------------- |
| `NewClient(apiKey)`             | Create client                                |
| `GetDatabase(id)`               | Fetch database metadata                      |
| `QueryAllEntries(dataSourceID)` | Paginated query for all entries              |
| `FetchAllBlocks(pageID)`        | Paginated fetch of all blocks                |
| `NormalizeNotionID(input)`      | Convert URL/hex/UUID to standard UUID format |

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

| Type                   | Output                             |
| ---------------------- | ---------------------------------- |
| `paragraph`            | Text                               |
| `heading_1/2/3`        | `#/##/###` (toggleable → callout)  |
| `bulleted_list_item`   | `- item`                           |
| `numbered_list_item`   | `1. item` (auto-numbered)          |
| `to_do`                | `- [ ]` or `- [x]`                 |
| `code`                 | Fenced code block                  |
| `quote`                | `> quote`                          |
| `callout`              | `> [!type]` (emoji → type mapping) |
| `equation`             | `$$...$$`                          |
| `divider`              | `---`                              |
| `toggle`               | `> [!note]+ ...`                   |
| `child_page`           | `[[title]]`                        |
| `child_database`       | HTML comment                       |
| `image`                | `![alt](url)`                      |
| `video/audio/file/pdf` | `[caption](url)` or URL            |
| `bookmark/embed`       | `[caption](url)` or URL            |
| `link_to_page`         | `[[notion-id: ...]]`               |
| `synced_block`         | Fetches and converts children      |
| `table`                | Markdown table                     |
| `column_list`          | Columns separated by `---`         |

### Rich Text Annotations

| Annotation       | Markdown       |
| ---------------- | -------------- |
| Bold             | `**text**`     |
| Italic           | `*text*`       |
| Code             | `` `text` ``   |
| Strikethrough    | `~~text~~`     |
| Underline        | `<u>text</u>`  |
| Background color | `==text==`     |
| Link             | `[text](url)`  |
| Equation         | `$expression$` |

### Mention Types

| Type         | Output                   |
| ------------ | ------------------------ |
| Page         | `[[notion-id: id]]`      |
| Database     | `[[notion-id: id]]`      |
| Date         | `start` or `start → end` |
| User         | `@name`                  |
| Link preview | `[text](url)`            |

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

| Notion Type                     | Frontmatter Value                      |
| ------------------------------- | -------------------------------------- |
| `title`                         | (used as filename, not in frontmatter) |
| `rich_text`                     | Converted to plain Markdown            |
| `number`                        | Number or `null`                       |
| `select`                        | Option name or `null`                  |
| `multi_select`                  | Array of option names                  |
| `status`                        | Status name or `null`                  |
| `date`                          | `start` or `start → end`               |
| `checkbox`                      | `true` or `false`                      |
| `url/email/phone_number`        | String or `null`                       |
| `relation`                      | Array of page IDs                      |
| `people`                        | Array of names (or IDs)                |
| `files`                         | Array of URLs                          |
| `created_time/last_edited_time` | ISO timestamp                          |

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

| Package                         | Used for           |
| ------------------------------- | ------------------ |
| `github.com/zalando/go-keyring` | OS keychain access |
| `gopkg.in/yaml.v3`              | YAML parsing only  |

---

## Testing

Three tiers — unit/integration run offline, system tests hit real Notion API.

```sh
# Unit + integration (mock client, no API needed)
go test ./...

# System tests (real Notion API, require API key)
/test-single-datasource-db
/test-double-datasource-db

# Everything together
/test
```

For detailed coverage map and architecture, see `.claude/reference/testing/README.md`.

### Key interfaces for testing

- `sync.NotionClient` — interface in `sync/client.go`, mocked in `sync/mock_client_test.go`
- `markdown.BlockFetcher` — interface in `markdown/converter.go`, mocked in `converter_test.go`

### Test Databases

| Name | Database ID | Skill |
| ---- | ----------- | ----- |
| Single data source (complex) | `2fe57008-e885-8003-b1f3-cc05981dc6b0` | `/test-single-datasource-db` |
| Double data source | `c9aa5ab2-b470-429c-ba9c-86c853782bb2` | `/test-double-datasource-db` |

---

## Release

Push a git tag (`v1.0.0`) to trigger GitHub Actions release workflow. Builds binaries for:

- Windows amd64
- macOS amd64, arm64
- Linux amd64, arm64

Install script: `curl -fsSL https://raw.githubusercontent.com/ran-codes/notion-sync/main/scripts/install.sh | bash`
