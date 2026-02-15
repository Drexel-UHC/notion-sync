## Architecture: SyncSession Orchestrator

Unified architecture to support three high-value features identified from the Obsidian Importer comparison:

- **Feature 5:** Relation → wiki link resolution
- **Feature 6:** Child page recursive import
- **Feature 7:** Related database auto-import

All three share the same root problem: the current architecture operates on **one database in isolation** with no cross-database awareness.

---

### Current Architecture (single-database silo)

```
CLI → FreshDatabaseImport(one DB) → FreezePage(one page) → .md file
                                          ↑ no knowledge of other DBs
                                          ↑ no global page registry
                                          ↑ relations = raw IDs, never resolved
                                          ↑ child_page = [[title]], not imported
```

**Key limitations:**
- `FreshDatabaseImport` takes ONE database ID, imports its entries, returns
- `FreezePage` writes ONE page to ONE folder, no awareness of other databases
- Relations store raw page IDs (`relation: [id1, id2]`) with no resolution
- Child pages rendered as `[[title]]` but not recursively imported
- No global page ID → file path mapping exists

---

### Proposed Architecture: SyncSession

```
CLI → SyncSession
        ├── PageRegistry: map[notionID] → filePath  (global, cross-database)
        ├── DatabaseQueue: []databaseID             (grows during import)
        ├── ImportedDBs: set[databaseID]            (prevents re-import loops)
        │
        ├── ImportDatabase(DB-A)
        │     ├── FreezePage(page1)
        │     │     ├── writes .md, registers in PageRegistry
        │     │     ├── discovers relation to DB-B → queues DB-B    (feature 7)
        │     │     └── discovers child_page → imports recursively  (feature 6)
        │     ├── FreezePage(page2) ...
        │     └── returns DiscoveredWork{newDBs: [DB-B], childPages: [...]}
        │
        ├── ImportDatabase(DB-B)  ← auto-queued from DB-A's relations
        │     └── registers all pages in PageRegistry
        │
        └── PostProcess()  ← after ALL databases imported               (feature 5)
              ├── scan all .md files across all database folders
              ├── replace [[notion-id: xxx]] → [[Actual Page Name]]
              └── replace relation IDs in frontmatter → [[wiki links]]
```

---

### New Components

#### 1. SyncSession (new: `internal/sync/session.go`)

Top-level orchestrator. Holds shared state across multiple database imports.

```go
type SyncSession struct {
    Client        *notion.Client
    OutputFolder  string
    PageRegistry  map[string]string    // notionID → filePath (global)
    DatabaseQueue []string             // database IDs to import
    ImportedDBs   map[string]bool      // already-imported DB IDs (loop prevention)
    OnProgress    ProgressCallback
}

// Run processes the initial database + any discovered databases
func (s *SyncSession) Run(initialDBID string) (*SessionResult, error)
```

**Loop logic:**
1. Queue initial database
2. While queue is not empty:
   - Pop next database ID
   - Skip if already in `ImportedDBs`
   - Call `ImportDatabase()` — returns `DiscoveredWork`
   - Append newly discovered DB IDs to queue
3. Run `PostProcess()` across all output folders

**Loop safety:** Track `ImportedDBs` set + cap max iterations (e.g., 20 databases) to prevent runaway chains.

#### 2. PageRegistry (part of SyncSession)

Simple `map[string]string` mapping Notion page IDs to local file paths. Populated by `FreezePage` during import.

Used by `PostProcess()` to resolve:
- `[[notion-id: xxx]]` in markdown body → `[[Page Name]]`
- Relation property IDs in frontmatter → `[[Page Name]]`
- Database mention IDs → `[[Database Folder/Page Name]]`

#### 3. DiscoveredWork (new return type)

```go
type DiscoveredWork struct {
    RelatedDatabaseIDs []string  // from relation properties pointing to other DBs
    ChildPages         []string  // standalone pages discovered inside blocks
}
```

Returned by `ImportDatabase()` so the session can queue additional work without the import function calling back into the session (keeps functions pure).

#### 4. PostProcess (new: `internal/sync/postprocess.go`)

Runs after all databases are imported. Two passes:

**Pass 1: Frontmatter relations**
- For each `.md` file, parse frontmatter
- Find relation arrays (lists of IDs)
- Replace IDs with `[[Page Name]]` using PageRegistry
- Rewrite file

**Pass 2: Body links**
- Find `[[notion-id: xxx]]` patterns in markdown body
- Replace with `[[Page Name]]` using PageRegistry
- Leave unresolved IDs as-is (target page may not be in scope)

---

### Changes to Existing Components

| Component | Change | Details |
|---|---|---|
| `FreezePage` | Accept `*PageRegistry` param | Register written pages; optionally discover child pages |
| `FreezePage` | Recurse into `child_page` blocks | When registry provided, import child pages into subfolders and register them |
| `FreshDatabaseImport` | Return `DiscoveredWork` | Inspect relation properties to find target database IDs |
| `FreshDatabaseImport` | Accept `*PageRegistry` param | Pass through to `FreezePage` |
| `RefreshDatabase` | Same changes as `FreshDatabaseImport` | Registry + discovered work support |
| `_database.json` | Add `relatedDatabases` field | Track cross-database relationships for refresh awareness |
| CLI (`main.go`) | New `--deep` flag (or default) | Triggers SyncSession mode vs single-DB mode |

---

### Design Decisions

#### Option A (recommended): Functions return discovered work, session drives the loop

```
Session.Run()
  └── loop:
        result, discovered = ImportDatabase(dbID, registry)
        queue.append(discovered.RelatedDatabaseIDs)
```

- Functions stay pure — no callback into session
- Easy to test: `ImportDatabase` returns predictable output
- Session owns the loop, can apply policies (max depth, skip list, etc.)

#### Option B (rejected): Functions accept session and call back into it

```
ImportDatabase(session)
  └── FreezePage(session)
        └── session.QueueDatabase(newDBID)  // callback
```

- Circular dependency risk
- Harder to test — need to mock session
- Less control over ordering

---

### Folder Structure Output

```
output/
├── Database A/
│   ├── _database.json
│   ├── Page 1.md
│   ├── Page 2.md
│   └── Child Page/           ← child page with its own children gets a subfolder
│       ├── Child Page.md
│       └── Grandchild.md
├── Database B/               ← auto-imported via relation discovery
│   ├── _database.json
│   ├── Entry 1.md
│   └── Entry 2.md
```

---

### Feature Mapping

| Feature | Component | How it works |
|---|---|---|
| **Relation → wiki link** | PostProcess | After all DBs imported, scan files, replace IDs with `[[Name]]` via PageRegistry |
| **Child page recursive import** | FreezePage | When `child_page` block found, recursively call FreezePage, create subfolder if needed |
| **Related DB auto-import** | SyncSession loop | `ImportDatabase` returns discovered DB IDs from relation properties, session queues them |

---

### Migration Path

This is **additive** — existing single-DB import/refresh still works unchanged:

1. `notion-sync import <db-id>` — works as today (single DB, no session)
2. `notion-sync import <db-id> --deep` — uses SyncSession, follows relations
3. `notion-sync refresh <folder>` — works as today
4. `notion-sync refresh <folder> --deep` — uses SyncSession, refreshes related DBs too

The `--deep` flag (or a future default) opts into the new behavior. No breaking changes.

---

### Estimated Effort

| Component | Effort | Notes |
|---|---|---|
| SyncSession + loop | Medium | New file, ~150 lines |
| PageRegistry | Easy | Just a map, threaded through existing functions |
| DiscoveredWork extraction | Medium | Parse relation properties to find target DB IDs (need API call to get relation config) |
| Child page recursion in FreezePage | Medium | Subfolder logic, recursive calls, registry updates |
| PostProcess (link resolution) | Medium | File scanning, regex replace, frontmatter rewrite |
| CLI flag + wiring | Easy | Flag parsing, session creation |
| **Total** | ~1-2 weeks | Incremental delivery possible per feature |

---

### Incremental Delivery Order

1. **PageRegistry + PostProcess** (feature 5 standalone) — most value, least risk. Can work with manually imported DBs.
2. **Child page recursion** (feature 6) — extends FreezePage, uses registry.
3. **SyncSession + auto-import** (feature 7) — ties it all together with the orchestration loop.
