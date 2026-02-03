# ADR: Database Refresh Workflow Optimization

## 1) Problem

When `freezeDatabase()` syncs a Notion database, it calls `dataSources.query()` to fetch all entries (which include `last_edited_time` and all properties), but then discards that data and only uses the entry IDs to drive the processing loop. For each entry, `freezePage()` makes a separate `pages.retrieve()` API call to re-fetch the same page data.

This means for a 200-row database with only 5 changed rows, the current code makes 200 redundant `pages.retrieve()` calls. The `last_edited_time` comparison that determines whether a page actually needs updating only happens *after* that redundant fetch.

## 2) Proposed Changes

- **Pass the already-fetched page object** from `dataSources.query()` into `freezePage()`, eliminating the per-entry `pages.retrieve()` call entirely.
- **Pre-filter entries in `freezeDatabase()`** by comparing each entry's `last_edited_time` against the `notion-last-edited` value stored in local frontmatter. Only entries that are new or changed proceed to block fetching and file writing.
- **Expand `scanLocalFiles()`** to return `lastEdited` alongside `filePath` so the comparison can happen before the processing loop.
- **Add `page?: PageObjectResponse`** to `FreezeOptions` so `freezePage()` can accept a pre-fetched page while remaining backwards-compatible for standalone callers.

## 3) Implementation Notes

### Files changed

| File | Change |
|------|--------|
| `packages/core/src/types.ts` | Added `page?: PageObjectResponse` to `FreezeOptions` |
| `packages/core/src/page-freezer.ts` | Uses `options.page` when provided, falls back to `pages.retrieve()` |
| `packages/core/src/database-freezer.ts` | `scanLocalFiles` returns `LocalFileInfo` with `lastEdited`, entries pre-filtered before loop, `page` passed to `freezePage`, deletion uses `allEntryIds` |
| `packages/core/tests/database-freezer.test.ts` | `makeEntry` includes `last_edited_time`, added skip test and pre-fetched page test, updated frontmatter mocks |
| `packages/core/tests/page-freezer.test.ts` | Added test verifying `pages.retrieve()` is skipped when `page` is provided |

### Key decisions

- The `last_edited_time` check inside `freezePage()` is kept as a safety net for standalone callers (not called via `freezeDatabase()`).
- Deletion tracking uses `allEntryIds` (all entries from the query, not just the filtered ones) to avoid incorrectly marking unchanged entries as deleted.
- `scanLocalFiles` returns a `LocalFileInfo` object (`{ filePath, lastEdited? }`) instead of a plain string, which is a minor interface change contained within `database-freezer.ts`.
