## SQLite + SyncSession Architecture Synergy Analysis

Evaluates how the two planned architecture upgrades fit together.

**Source plans:**
- `obsidian-importer-arch.md` — SyncSession orchestrator (relation resolution, child page recursion, related DB auto-import)
- `sql-lite-from-msgvault.md` — SQLite queryable output alongside markdown

---

### Key Finding: SQLite IS the PageRegistry

The SyncSession proposes an in-memory `PageRegistry map[string]string` (notionID → filePath). The SQLite `pages` table already has `id`, `title`, `database_id`. Adding a `file_path` column makes it the same thing — but persistent across sessions.

Using SQLite as the registry instead of an in-memory map:
- **Persists across sessions** — refresh can resolve links from previously-imported DBs without re-importing them
- **Cross-database queries are native** — `SELECT title, file_path FROM pages WHERE id IN (?, ?, ?)`
- **Relation discovery via SQL** — `SELECT DISTINCT value FROM json_each(properties_json, '$.relations')` finds all target DBs
- **PostProcess becomes SQL** — lookup is a query, not a file scan

---

### Synergies

| Area | How SQLite helps SyncSession |
|---|---|
| **PageRegistry** | `pages` table replaces in-memory map. Persistent, queryable, cross-session. |
| **PostProcess link resolution** | Instead of scanning all .md files to find IDs, query `SELECT id, title, file_path FROM pages`. |
| **Relation discovery** | `properties_json` column lets you query relation targets: find which DB IDs are referenced but not yet imported. |
| **Deletion tracking** | `deleted` column already exists. Cross-database delete detection becomes a query. |
| **Refresh intelligence** | Session can check "have I seen this page before in ANY database?" via a single query. |

---

### Conflicts / Friction

#### 1. DB location: per-database vs per-workspace (CRITICAL)

**Current SQLite plan:** `_notion_sync.db` inside each database folder (isolated, can't see other DBs).

**SyncSession needs:** global registry across all databases — DB must live at the output root.

```
output/
├── _notion_sync.db        <- workspace-level, sees all databases
├── Database A/
│   └── ...
├── Database B/
│   └── ...
```

**Resolution:** Move DB to parent output folder from day 1. The `database_id` column already partitions data per database.

#### 2. Same function signatures touched twice

Both plans modify `FreezePageOptions` — SQLite adds `*store.Store`, SyncSession adds `*PageRegistry`. Building together means one refactor pass with `*store.Store` serving both purposes.

#### 3. Implementation order matters

If SQLite ships per-database first, then SyncSession refactors to per-workspace, that's wasted migration. Design workspace-level from the start.

---

### Recommendation: Build together

SQLite store = workspace-level from day 1, doubling as the PageRegistry.

#### Additional schema for SyncSession support

```sql
-- Add to pages table
file_path TEXT

-- Track imported databases (supplements _database.json)
CREATE TABLE IF NOT EXISTS databases (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    url TEXT,
    folder_path TEXT NOT NULL,
    last_synced_at TEXT,
    entry_count INTEGER DEFAULT 0
);

-- Relation tracking for discovery + resolution
CREATE TABLE IF NOT EXISTS relations (
    source_page_id TEXT NOT NULL,
    target_page_id TEXT NOT NULL,
    property_name TEXT NOT NULL,
    PRIMARY KEY (source_page_id, target_page_id, property_name)
);
```

#### Revised component flow

```
CLI -> SyncSession
        ├── store = OpenStore(outputFolder)  <- workspace-level DB
        ├── ImportDatabase(DB-A, store)
        │     ├── FreezePage(page, store)
        │     │     ├── writes .md
        │     │     ├── store.UpsertPage(...)
        │     │     └── store.UpsertRelations(...)
        │     └── returns discovered DB IDs from store.GetUnimportedRelationTargets()
        ├── ImportDatabase(DB-B, store)  <- auto-queued
        └── PostProcess(store)
              ├── targets = store.GetAllPages()
              ├── resolve [[notion-id: X]] -> [[title]]
              └── rewrite .md files
```

#### What changes in the SQLite plan

| Item | Change |
|---|---|
| DB location | Per-database -> per-workspace (output root) |
| `pages` table | Add `file_path TEXT` column |
| New: `databases` table | Replaces `_database.json` eventually (keep both during transition) |
| New: `relations` table | Enables relation discovery without re-parsing frontmatter |
| `OpenStore` | Called by SyncSession, not by individual import functions |

Everything else (schema, FTS, triggers, UpsertPage, MarkDeleted, OutputMode, error handling) stays the same.

---

### Incremental delivery order

1. **SQLite store** (workspace-level) — foundation, can ship standalone
2. **PageRegistry via SQLite** — `file_path` column + query helpers
3. **PostProcess** (relation -> wiki link resolution)
4. **Child page recursion** (extends FreezePage)
5. **SyncSession + auto-import** (relations table + discovery queries + orchestration loop)

Steps 1-2 can ship as v0.2.0. Steps 3-5 build on top incrementally.
