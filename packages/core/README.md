# @notion-sync/core

Platform-agnostic sync engine. Contains all business logic for fetching Notion databases and writing Markdown files. Has zero platform imports -- no `node:fs`, no `vscode`.

## What it does

1. Connects to the Notion API and fetches database entries
2. Converts 30+ Notion block types to Markdown
3. Maps Notion properties (text, number, select, date, etc.) to YAML frontmatter
4. Writes `.md` files with frontmatter + Markdown body
5. Tracks changes via `last_edited_time` for incremental sync
6. Marks deleted database entries with `notion-deleted: true` in frontmatter

## Public API

### Orchestration Functions

```typescript
// First-time import -- processes all entries
freshDatabaseImport(options: DatabaseImportOptions, onProgress?: ProgressCallback): Promise<DatabaseFreezeResult>

// Incremental update -- only processes changed entries (reads metadata from _database.json)
refreshDatabase(options: RefreshOptions, onProgress?: ProgressCallback): Promise<DatabaseFreezeResult>

// List all synced databases in a folder
listSyncedDatabases(fs: FileSystem, outputFolder: string): Promise<FrozenDatabase[]>

// Read database metadata from a folder
readDatabaseMetadata(fs: FileSystem, folderPath: string): Promise<FrozenDatabase | null>
```

### Types

```typescript
interface DatabaseImportOptions {
  client: Client;           // Notion SDK client
  fs: FileSystem;           // Platform filesystem adapter
  fm: FrontmatterReader;    // Frontmatter parser
  databaseId: string;       // Notion database ID
  outputFolder: string;     // Where to write files
}

interface RefreshOptions {
  client: Client;           // Notion SDK client
  fs: FileSystem;           // Platform filesystem adapter
  fm: FrontmatterReader;    // Frontmatter parser
  folderPath: string;       // Path to synced database folder
}

interface FrozenDatabase {
  databaseId: string;
  title: string;
  folderPath: string;
  lastSyncedAt: string;     // ISO timestamp
  entryCount: number;
}

type ProgressPhase =
  | { phase: "querying" }
  | { phase: "diffing"; total: number }
  | { phase: "stale-detected"; stale: number; total: number }
  | { phase: "importing"; current: number; total: number; title: string }
  | { phase: "complete" };
```

### Metadata file

Each synced database folder contains a `_database.json` file storing metadata:

```json
{
  "databaseId": "abc123...",
  "title": "My Database",
  "folderPath": "notion/My Database",
  "lastSyncedAt": "2024-01-15T10:00:00.000Z",
  "entryCount": 42
}
```

This enables `refreshDatabase()` to work from just a folder path without needing external state.

## Source files

| File | Purpose |
|------|---------|
| `types.ts` | Core interfaces: `FileSystem`, `FrontmatterReader`, `ProgressPhase`, and result types. |
| `notion-client.ts` | `createNotionClient()`, `notionRequest()` with rate limiting + retry, `normalizeNotionId()`. |
| `database-freezer.ts` | `freshDatabaseImport()` and `refreshDatabase()` orchestration functions. |
| `page-freezer.ts` | `freezePage()` (internal) -- fetches blocks, builds frontmatter, writes file. |
| `block-converter.ts` | `convertBlocksToMarkdown()` -- handles 30+ block types. |
| `frontmatter.ts` | `createFrontmatterReader()` factory. |
| `utils.ts` | `sanitizeFileName()`, `joinPath()`. |
| `index.ts` | Public exports. |

## Key patterns

### Dependency injection

Core never touches the filesystem directly:

```typescript
interface FileSystem {
  readFile(path: string): Promise<string>;
  writeFile(path: string, content: string): Promise<void>;
  fileExists(path: string): Promise<boolean>;
  mkdir(path: string, recursive?: boolean): Promise<void>;
  listMarkdownFiles(dir: string): Promise<string[]>;
  listDirectories(dir: string): Promise<string[]>;
}

interface FrontmatterReader {
  readFrontmatter(filePath: string): Promise<Record<string, unknown> | null>;
}
```

### Rate limiting and retry

`notionRequest()` enforces 340ms between API calls (~3 req/s) and retries on 429, 500, 502, 503, 504 with exponential backoff + jitter.

### Incremental sync

`refreshDatabase()` compares `notion-last-edited` in local frontmatter against the entry's `last_edited_time`. Unchanged entries are skipped entirely.

---

## Development

```sh
npm run build -w packages/core   # Build
npm run test -w packages/core    # Test (83 tests)
```

### Adding a new block type

1. Add case to `convertBlocksToMarkdown()` in `block-converter.ts`
2. Add tests in `tests/block-converter.test.ts`
3. Run `npm test`

### Adding a new property type

1. Add case to `mapPropertiesToFrontmatter()` in `page-freezer.ts`
2. Add tests in `tests/page-freezer.test.ts`
3. Run `npm test`
