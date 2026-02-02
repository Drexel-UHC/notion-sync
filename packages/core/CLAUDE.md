# @notion-sync/core

Platform-agnostic sync engine. Contains all business logic. Zero platform imports.

## Source files

| File | Lines | Purpose |
|------|-------|---------|
| `types.ts` | ~60 | `FileSystem`, `FrontmatterReader` interfaces, `FreezeOptions`, result types |
| `notion-client.ts` | ~120 | `createNotionClient()`, `notionRequest()` (throttle + retry + backoff), `normalizeNotionId()`, `detectNotionObject()` |
| `block-converter.ts` | ~420 | `convertBlocksToMarkdown()` — 30+ Notion block types to Markdown. Pure logic, no side effects. |
| `page-freezer.ts` | ~190 | `freezePage()` — fetch page, build frontmatter, write Markdown. Maps 16 property types. |
| `database-freezer.ts` | ~140 | `freezeDatabase()` — paginate entries, freeze each page, track deletions. |
| `index.ts` | barrel | Re-exports public API |

## Testing

```sh
npm run test    # vitest run
```

76 tests across 4 files. Tests mock `FileSystem`, `FrontmatterReader`, and Notion `Client`. The `notionRequest` mock skips throttle/retry.

Block-converter tests are the highest value — 45 tests covering all supported block types and rich text formatting.

## Important patterns

- `notionRequest()` wraps every Notion API call with rate limiting (340ms between requests) and retry with exponential backoff + jitter. Module-level `lastRequestTime` state is intentional.
- `freezePage()` takes a `FreezeOptions` object with `client`, `fs`, `fm`, `outputFolder`, `notionId`, and optional `databaseId`.
- `freezeDatabase()` uses `client.dataSources.query()` (NOT `databases.query()`). This is a newer Notion API that returns full property data for entries.
- YAML serialization in `page-freezer.ts` is manual (`buildFileContent`, `formatYamlEntry`, `yamlEscapeString`). Strings containing `:`, `#`, quotes, or looking like booleans/numbers get double-quoted.
- `scanLocalFiles()` in database-freezer returns `Map<notionId, filePath>` by reading frontmatter from each `.md` file in the database folder.
- `markAsDeleted()` inserts `notion-deleted: true` into existing frontmatter via string manipulation (not re-serialization).
