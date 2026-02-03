# notion-sync (CLI)

Command-line tool for syncing Notion pages and databases to local Markdown files. Thin wrapper that connects `node:fs` to the `@notion-sync/core` engine.

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
node packages/cli/dist/main.js sync <url-or-id> --api-key <key>
```

### Global install (after npm publish)

```sh
npm install -g notion-sync
notion-sync sync <url-or-id> --api-key <key>
```

## Commands

### `sync` -- Sync a page or database

```sh
notion-sync sync <url-or-id> [--output <folder>] [--api-key <key>]
```

- `<url-or-id>` -- a Notion page/database URL, a 32-char hex ID, or a UUID with dashes
- `--output`, `-o` -- output folder (default: `./notion` from config)
- `--api-key` -- Notion API token (overrides config and env var)

**Single page:** Writes one `.md` file to the output folder.

**Database:** Creates a subfolder named after the database, then writes one `.md` file per entry. On re-sync, entries removed from Notion get `notion-deleted: true` in their frontmatter.

### `resync` -- Re-sync a previously synced file

```sh
notion-sync resync <path> [--api-key <key>]
```

- `<path>` -- path to a `.md` file with `notion-id` in its frontmatter
- If the file has a `notion-database-id`, the entire database is re-synced
- Otherwise just the single page is re-synced

### `config set` -- Save configuration

```sh
notion-sync config set <key> <value>
```

Keys:
- `apiKey` -- your Notion integration token
- `defaultOutputFolder` -- default output path (default: `./notion`)

## Configuration

Configuration is loaded in this priority order:

1. CLI flags (`--api-key`, `--output`)
2. Environment variable `NOTION_SYNC_API_KEY`
3. Config file

The config file location:
- `$XDG_CONFIG_HOME/notion-sync/config.json` (if `XDG_CONFIG_HOME` is set)
- `~/.notion-sync.json` (default)

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error (invalid ID, missing arg, API failure) |
| 2 | Authentication error (no API key) |

## Source files

| File | Purpose |
|------|---------|
| `main.ts` | Entry point. Parses args via `node:util parseArgs`. Routes to `sync`, `resync`, or `config` commands. |
| `fs-adapter.ts` | `nodeFs: FileSystem` implementation using `node:fs/promises`. Resolves forward-slash paths from core via `path.resolve()`. |
| `frontmatter-adapter.ts` | Creates a `FrontmatterReader` via core's `createFrontmatterReader()` using the node filesystem. |
| `config.ts` | Reads/writes config file. Env var `NOTION_SYNC_API_KEY` overrides file values. |

## Development

```sh
# Build
npm run build -w packages/cli

# Run (after building core too)
node packages/cli/dist/main.js sync <url> --api-key <key> --output ./test-out
```

The CLI has no tests of its own -- it's a thin adapter layer. Core logic is tested via `npm test` in the core package.

## Publishing to npm

```sh
# Build everything
npm run build

# Publish (from repo root)
npm publish -w packages/cli
```

The `bin` field in `package.json` maps the `notion-sync` command to `./dist/main.js`, which has a `#!/usr/bin/env node` shebang.
