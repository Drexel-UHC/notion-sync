# msgvault

https://github.com/wesm/msgvault

## Feature Comparisons

| Feature | msgvault | notion-sync |
|---------|----------|-------------|
| **SQLite database** | Local SQLite store for all data | Flat `.md` files only |
| **Full-text search** | FTS5 with query syntax (`from:`, date ranges) | None — rely on external tools |
| **TUI (terminal UI)** | Interactive browsing via `tui` command | None |
| **MCP server** | Built-in `mcp` command for AI assistant access | None |
| **Analytics / Parquet** | DuckDB-powered aggregate queries over Parquet cache | None |
| **Multi-account** | Multiple accounts in one DB | Single API key at a time |
| **Resumable sync** | Checkpoint-based resume on interruption | No resume — restarts from scratch |
| **`search` command** | CLI search with `--json` output | None |
| **`stats` command** | Archive statistics | None |
| **`verify` command** | Integrity validation against source | None |
| **Export format** | `.eml` export | Markdown only |
| **Attachment dedup** | SHA-256 content-addressed storage | No attachment handling |
| **Date filtering on sync** | `--after`/`--before` flags | No date filtering |
| **`list-*` commands** | `list-senders`, `list-domains`, `list-labels` | `list` only shows synced DBs |
| **Config file format** | TOML with structured sections | Minimal JSON config |

### Most Impactful Gaps

1. **MCP server** — let AI assistants query synced Notion content directly
2. **Full-text search** — CLI search across all synced markdown files
3. **Resumable sync** — checkpoint support for large databases
4. **Stats command** — quick overview of what's synced
5. **TUI** — interactive browsing of synced content

> Note: msgvault is a Gmail archiver, so some features (attachments, multi-account, `.eml` export) are domain-specific. The transferable ideas are: SQLite backing store, FTS, MCP, TUI, and resumable sync.

## Evaluation

### Core Insight

msgvault's features (search, stats, MCP, TUI) all stem from one architectural choice: **SQLite as the primary store**. Without a queryable database, these features are just fancy grep. SQLite is the foundation — everything else is incremental.

### Approach: Hybrid Output (Additive, Not Replacing)

Keep markdown files as-is. Add SQLite alongside them. Default is both, configurable per-database.

```
notion/Project Tracker/
├── _database.json          # existing metadata
├── _notion_sync.db         # NEW — SQLite with FTS
├── Auth system redesign.md
├── API rate limiter.md
└── ...
```

Config option:

```toml
[sync]
output = "both"  # "both" (default) | "markdown" | "sqlite"
```

- **both** — markdown for reading/editing + SQLite for querying
- **markdown** — current behavior, nothing changes
- **sqlite** — power users who only care about search/MCP/analytics

### SQLite Schema Strategy

Notion databases have dynamic, user-defined properties — can't hardcode columns. Use a flat approach:

- `pages` table: `id, title, body_text, properties_json, last_edited, created_at`
- FTS5 virtual table on `title` + `body_text`
- Query properties via `json_extract()` in SQLite

### Implementation Scope

**New code:**
- `internal/store/` package — schema init, insert/update, FTS setup
- SQLite driver dep: `modernc.org/sqlite` (pure Go, no CGO)

**Modified code:**
- `database.go` — open/close DB alongside folder, write after each page
- `page.go` — insert row after writing `.md`
- `config.go` — output mode setting
- `main.go` — pass config through

Estimated effort: ~2-3 days focused work. Not a rewrite — just a second output channel on the existing sync flow.

## Roadmap

SQLite is the foundation. Each subsequent feature is incremental once the DB exists.

| Phase | Feature | Depends On | Effort |
|-------|---------|------------|--------|
| 1 | **SQLite output** — write to DB alongside markdown during sync | — | Medium |
| 2 | **`search` command** — FTS5 query across synced content | Phase 1 | Small |
| 3 | **`stats` command** — SQL aggregates (entry counts, last sync, staleness) | Phase 1 | Small |
| 4 | **MCP server** — expose search/stats queries to AI assistants | Phase 2 | Medium |
| 5 | **Resumable sync** — checkpoint state in SQLite | Phase 1 | Medium |
| 6 | **`verify` command** — compare DB records against Notion API | Phase 1 | Small |
| 7 | **TUI** — bubbletea interactive browser over SQLite queries | Phase 2 | Large |
