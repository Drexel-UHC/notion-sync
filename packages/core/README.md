# @notion-sync/core

Platform-agnostic sync engine. Contains all business logic for fetching Notion content and writing Markdown files. Has zero platform imports -- no `node:fs`, no `vscode`.

## What it does

1. Connects to the Notion API and fetches pages or database entries
2. Converts 30+ Notion block types to Markdown
3. Maps Notion properties (text, number, select, date, etc.) to YAML frontmatter
4. Writes `.md` files with frontmatter + Markdown body
5. Tracks changes via `last_edited_time` for incremental sync
6. Marks deleted database entries with `notion-deleted: true` in frontmatter

## Source files

| File | Purpose |
|------|---------|
| `types.ts` | Core interfaces: `FileSystem`, `FrontmatterReader`, `FreezeOptions`, and result types. This is the contract that platform adapters implement. |
| `notion-client.ts` | `createNotionClient()` creates a Notion SDK client. `notionRequest()` wraps every API call with rate limiting (340ms between requests) and retry with exponential backoff + jitter. `normalizeNotionId()` parses URLs and hex strings into UUIDs. `detectNotionObject()` determines if an ID is a page or database. |
| `block-converter.ts` | `convertBlocksToMarkdown()` converts Notion blocks to Markdown. Handles paragraphs, headings, lists, to-dos, toggles, code blocks, images, tables, callouts, quotes, dividers, embeds, and more. `convertRichText()` handles bold, italic, strikethrough, code, links, and colors. |
| `page-freezer.ts` | `freezePage()` fetches a single page, builds YAML frontmatter (including all database properties if it's a database entry), converts blocks to Markdown, and writes the file. Returns `created`, `updated`, or `skipped`. |
| `database-freezer.ts` | `freezeDatabase()` queries all entries in a database (paginated via `dataSources.query()`), freezes each page, and marks locally-tracked pages that no longer appear in the query as deleted. |
| `frontmatter.ts` | `createFrontmatterReader()` builds a `FrontmatterReader` from any `FileSystem`. Shared by both CLI and VS Code adapters. |
| `utils.ts` | `sanitizeFileName()` strips invalid filename characters. `joinPath()` joins path segments with `/`. |
| `index.ts` | Barrel file re-exporting the public API. |

## Key patterns

### Dependency injection

Core never touches the filesystem directly. Platform-specific behavior comes in through two interfaces:

```typescript
interface FileSystem {
  readFile(path: string): Promise<string>;
  writeFile(path: string, content: string): Promise<void>;
  fileExists(path: string): Promise<boolean>;
  mkdir(path: string, recursive?: boolean): Promise<void>;
  listMarkdownFiles(dir: string): Promise<string[]>;
}

interface FrontmatterReader {
  readFrontmatter(filePath: string): Promise<Record<string, unknown> | null>;
}
```

The CLI implements these with `node:fs/promises`. VS Code implements them with `vscode.workspace.fs`. Tests use in-memory mocks.

### Rate limiting and retry

`notionRequest()` enforces a 340ms minimum interval between API calls (~3 req/s) and retries on 429, 500, 502, 503, 504 with exponential backoff. Jitter of +/-25% prevents thundering herd. Maximum 5 retries, max 30s delay.

### Incremental sync

`freezePage()` reads the existing file's frontmatter and compares `notion-last-edited` to the page's `last_edited_time`. If they match, it returns `skipped` without fetching blocks.

### Deletion tracking

`freezeDatabase()` scans local `.md` files, reads their `notion-id` from frontmatter, and compares against the set of IDs returned by the database query. Missing IDs get `notion-deleted: true` injected into their frontmatter.

## Development

```sh
# Build
npm run build -w packages/core

# Test
npm run test -w packages/core

# Build + test
npm run build -w packages/core && npm run test -w packages/core
```

After editing core, rebuild before testing CLI or VS Code:

```sh
npm run build:core
```

### Adding a new block type

1. Add a case to `convertBlocksToMarkdown()` in `block-converter.ts`
2. Add test cases in `packages/core/tests/block-converter.test.ts`
3. Run `npm test` to verify

### Adding a new property type

1. Add a case to `mapPropertiesToFrontmatter()` in `page-freezer.ts`
2. Add test cases in `packages/core/tests/page-freezer.test.ts`
3. Run `npm test` to verify
