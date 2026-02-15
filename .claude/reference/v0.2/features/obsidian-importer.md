## Initial Comparison

Feature comparison between notion-sync and [Obsidian Importer](https://github.com/obsidianmd/obsidian-importer) (API-based Notion importer).

### They have, we don't

| Feature | Obsidian Importer | Import Difficulty | Refresh Difficulty |
|---|---|---|---|
| **Attachment downloading** (images, files, PDFs saved locally) | Downloads all files to vault | Medium | Medium |
| **Related database auto-import** (follow relations -> import linked DBs) | Up to 10 rounds of chasing relations | Hard | Hard |
| **Relation -> wiki link resolution** (replace IDs with `[[Page Name]]`) | Post-processing pass after all DBs imported | Medium | Medium |
| **Page mention -> wiki link resolution** | Placeholder `[[NOTION_PAGE:id]]` -> resolved | Medium | Medium |
| **Synced block deduplication** (shared file, embedded everywhere) | Extracts to separate `.md`, uses `![[embed]]` | Hard | Hard |
| **Child page recursive import** (pages inside pages -> nested folders) | Recursively imports, creates folder structure | Medium | Hard |
| **Child database import** (inline DBs imported as sibling folders) | Imports with `.base` file | Medium | Hard |
| **`.base` file generation** (Obsidian Bases/Dataview table view) | Schema, column order, filter rules | Medium | Low (schema rarely changes) |
| **Formula conversion** (Notion formulas -> Obsidian Bases syntax) | Hybrid/static strategies | Hard | N/A |
| **Cover image download** | Downloaded, stored as frontmatter property | Easy | Easy |
| **Unique ID property** | `prefix-number` format | Easy | Easy |
| **Created_by / Last_edited_by** | Fetches user names | Easy | Easy |
| **Verification property** | State string | Easy | Easy |
| **Rollup property** (via `.base` file) | Computed in Bases | Hard | N/A |
| **YouTube/embed detection** (special embed syntax) | `![](url)` for YouTube, skip download | Easy | Easy |
| **File timestamp preservation** (ctime/mtime from Notion) | Sets OS file timestamps | Easy | Easy |
| **Page/DB selection UI** (tree picker for what to import) | Interactive tree with select all | N/A (CLI) | N/A |
| **Incremental import skip** (skip already-imported pages) | Checks `notion-id` in frontmatter | Already have (refresh) | Already have |

### We have, they don't

| Feature | notion-sync | Notes |
|---|---|---|
| **True incremental refresh** (timestamp diffing) | Compares `notion-last-edited`, only re-syncs stale | They re-import everything or skip entirely |
| **Soft deletes** (`notion-deleted: true`) | Marks removed entries, preserves files | They don't track deletions |
| **Selective `--ids` refresh** | Refresh specific pages by ID | No equivalent |
| **OS keychain API key storage** | Windows/macOS/Linux native keychain | They use Obsidian's settings |
| **`_database.json` metadata** | Tracks sync state, entry count, timestamps | They rely on frontmatter only |
| **Rate limiting (340ms throttle)** | Built-in mutex-protected throttle | They have retry but no proactive throttle |
| **Data source API with fallback** | Uses newer API, falls back to classic | They use data sources only |
| **CLI-native** (no Obsidian dependency) | Works standalone, CI-friendly | Theirs is an Obsidian plugin |

### Both have (parity)

| Feature | Notes |
|---|---|
| 30+ block types | Near-identical coverage |
| Rich text annotations (bold, italic, code, strikethrough, underline, highlight) | Same output |
| Callout emoji mapping | Similar mappings |
| Toggle -> callout conversion | Same approach |
| Table support | Both generate markdown tables |
| Column list handling | Both flatten columns |
| Equation (block + inline) | Same `$$`/`$` syntax |
| All standard property types (number, select, multi_select, status, date, checkbox, url, email, phone, people, files, relations, rich_text) | Minor format differences |
| Manual YAML serialization | Both do custom serialization |
| Retry with exponential backoff | Similar logic |

### Priority recommendations

**High value, lower effort:**
1. Attachment downloading - most requested missing feature
2. Unique ID / created_by / last_edited_by properties - trivial additions
3. Cover image support - easy win
4. File timestamp preservation - easy win

**High value, higher effort:**
5. Relation -> wiki link resolution (requires multi-DB awareness)
6. Child page recursive import (changes folder structure assumptions)
7. Related database auto-import (biggest architectural change)
