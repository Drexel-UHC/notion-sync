# @notion-sync/core (Legacy TypeScript)

> **Note:** This is the legacy TypeScript implementation. The primary implementation is now in Go at `go/`. This package is kept as a backup/reference.

Platform-agnostic sync engine. All business logic lives here. Zero platform imports (`node:fs`, etc.).

## Quick Reference

```typescript
// First-time import
freshDatabaseImport({ client, fs, fm, databaseId, outputFolder }, onProgress?)

// Incremental update (reads _database.json)
refreshDatabase({ client, fs, fm, folderPath, force? }, onProgress?)

// Discovery
listSyncedDatabases(fs, outputFolder)
readDatabaseMetadata(fs, folderPath)
```

## Build & Test

```sh
npm install
npm run build -w packages/core
npm run test -w packages/core    # 83 tests
```

## Source Files

| File | Purpose |
|------|---------|
| `types.ts` | `FileSystem`, `FrontmatterReader` interfaces, `ProgressPhase`, result types |
| `database-freezer.ts` | `freshDatabaseImport()`, `refreshDatabase()`, `listSyncedDatabases()` |
| `page-freezer.ts` | `freezePage()` — fetch blocks, build frontmatter, write .md file |
| `block-converter.ts` | `convertBlocksToMarkdown()` — 30+ block types to Markdown |
| `notion-client.ts` | `createNotionClient()`, `notionRequest()` (throttle + retry) |
| `frontmatter.ts` | `createFrontmatterReader()` factory |
| `utils.ts` | `sanitizeFileName()`, `joinPath()` |
| `index.ts` | Public exports |

## Key Interfaces

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

## Important Patterns

### Rate Limiting
`notionRequest()` wraps every Notion API call with:
- 340ms minimum between requests (~3 req/s)
- Retry with exponential backoff + jitter on 429, 500, 502, 503, 504

### Notion API
Uses `client.dataSources.query()` (NOT `databases.query()`).

### YAML Serialization
Manual serialization in `page-freezer.ts`. Strings containing `:`, `#`, quotes get double-quoted.

---

## Common Tasks

### Add a new block type
1. Add case to `convertBlocksToMarkdown()` in `block-converter.ts`
2. Add tests in `tests/block-converter.test.ts`
3. `npm test`

### Add a new property type
1. Add case to `mapPropertiesToFrontmatter()` in `page-freezer.ts`
2. Add tests in `tests/page-freezer.test.ts`
3. `npm test`
