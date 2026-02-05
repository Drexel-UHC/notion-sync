# notion-sync CLI (Legacy TypeScript)

> **Note:** This is the legacy TypeScript implementation. The primary implementation is now in Go at `go/`. This package is kept as a backup/reference.

Command-line tool for syncing Notion databases to local Markdown files. Thin wrapper that connects `node:fs` to the `@notion-sync/core` engine.

## Install

### From source (development)

```sh
git clone https://github.com/ran-codes/notion-sync.git
cd notion-sync
npm install
npm run build
```

Then run directly:

```sh
node packages/cli/dist/main.js sync <database-url-or-id> --api-key <key>
```

### Global install (after npm publish)

```sh
npm install -g notion-sync
notion-sync sync <database-url-or-id> --api-key <key>
```

## Commands

### `sync` -- Import a database

```sh
notion-sync sync <database-url-or-id> [--output <folder>] [--api-key <key>]
```

- `<database-url-or-id>` -- a Notion database URL, 32-char hex ID, or UUID with dashes
- `--output`, `-o` -- output folder (default: `./notion` from config)
- `--api-key` -- Notion API token (overrides config and env var)

Creates a subfolder named after the database, then writes one `.md` file per entry with YAML frontmatter containing all properties.

### `refresh` -- Incremental update

```sh
notion-sync refresh <database-folder> [--force] [--api-key <key>]
```

- `<database-folder>` -- path to a synced database folder (contains `_database.json`)
- `--force`, `-f` -- resync all entries, ignoring timestamps (useful when database schema changes)

Reads database metadata from `_database.json` and refreshes the database. Only re-syncs entries where `last_edited_time` changed (unless `--force` is used). Marks entries removed from Notion with `notion-deleted: true`.

### `list` -- List synced databases

```sh
notion-sync list [<output-folder>]
```

- `<output-folder>` -- folder to scan for synced databases (default: `./notion`)

Scans for `_database.json` files in subdirectories and lists all synced databases with their metadata.

### `config set` -- Save configuration

```sh
notion-sync config set <key> <value>
```

Keys:
- `apiKey` -- your Notion integration token (stored in OS keychain)
- `defaultOutputFolder` -- default output path (default: `./notion`)

## Configuration

### API key storage

API keys are stored in the OS keychain (Windows Credential Manager, macOS Keychain, Linux Secret Service).

Priority (highest to lowest):

1. `--api-key` CLI flag
2. `NOTION_SYNC_API_KEY` environment variable
3. OS keychain (via `@napi-rs/keyring`)
4. Config file (plaintext fallback, with warning)

### Config file

Non-sensitive settings stored at:
- `$XDG_CONFIG_HOME/notion-sync/config.json` (if set)
- `~/.notion-sync.json` (default)

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error (invalid ID, missing arg, API failure) |
| 2 | Authentication error (no API key) |

---

## Testing the CLI

### Prerequisites

1. A Notion integration token (starts with `ntn_`) -- see [Creating a Notion integration](../../README.md#creating-a-notion-integration)
2. A Notion **database** shared with your integration

### Test 1: Fresh database import

```sh
# Build
npm run build

# Store your API key
node packages/cli/dist/main.js config set apiKey ntn_YOUR_TOKEN

# Sync a database
node packages/cli/dist/main.js sync <your-notion-database-url> --output ./test-out

# Expected output:
# Syncing database...
# Found X entries to sync (X total)
# Syncing "Database Name"... X/X
# Done: "Database Name"
#   Total:   X
#   Created: X
#   Updated: 0
#   Skipped: 0
```

Check the output:

```sh
ls ./test-out/
# Should show a folder named after your database

ls "./test-out/Your Database Name/"
# Should show .md files for each entry
```

### Test 2: List synced databases

```sh
# List databases synced to the output folder
node packages/cli/dist/main.js list ./test-out

# Expected output:
# Synced databases in ./test-out:
#
#   Your Database Name
#     Folder:      ./test-out/Your Database Name
#     Database ID: abc123...
#     Entries:     X
#     Last synced: 2024-01-15T10:00:00.000Z
```

### Test 3: Incremental refresh

```sh
# Make a change in Notion (edit an entry or add a new one)

# Refresh using the database folder path
node packages/cli/dist/main.js refresh "./test-out/Your Database Name"

# Expected output:
# Refreshing database...
# Comparing X entries with local files...
# Found Y entries to sync (X total)    # Y should be small (only changed)
# Done: "Database Name"
#   Total:   X
#   Created: 0
#   Updated: Y
#   Skipped: Z
#   Deleted: 0
```

### Test 4: Force refresh

```sh
# Add a new property column in Notion (existing entries won't have timestamps updated)

# Force refresh to resync all entries with the new property
node packages/cli/dist/main.js refresh "./test-out/Your Database Name" --force

# Expected output:
# Force refreshing database (ignoring timestamps)...
# All entries will show as Updated (none skipped)
```

### Test 5: Deletion tracking

```sh
# In Notion, delete an entry from your database

# Refresh
node packages/cli/dist/main.js refresh "./test-out/Your Database Name"

# Expected: Deleted: 1 in the output

# Check the deleted file
cat "./test-out/Your Database Name/Deleted Entry.md"
# Should contain "notion-deleted: true" in frontmatter
```

### Cleanup

```sh
rm -rf ./test-out
```

---

## Source files

| File | Purpose |
|------|---------|
| `main.ts` | Entry point. Parses args, routes to `sync`, `refresh`, or `config`. |
| `fs-adapter.ts` | `FileSystem` implementation using `node:fs/promises`. |
| `frontmatter-adapter.ts` | Creates `FrontmatterReader` via core. |
| `config.ts` | API key storage (keychain + fallback), config file I/O. |
