# notion-sync CLI (Legacy TypeScript)

> **Note:** This is the legacy TypeScript implementation. The primary implementation is now in Go at `go/`. This package is kept as a backup/reference.

Thin CLI wrapper. Wires `node:fs` to core's `FileSystem` interface.

## Build & Run

```sh
npm install
npm run build -w packages/cli
node packages/cli/dist/main.js --help
```

## Commands

```sh
notion-sync sync <database-url-or-id> [--output <folder>] [--api-key <key>]
notion-sync refresh <database-folder> [--force] [--api-key <key>]
notion-sync list [<output-folder>]
notion-sync config set <key> <value>
```

| Command | Purpose |
|---------|---------|
| `sync` | First-time import of a Notion database |
| `refresh` | Incremental update (only changed entries) |
| `refresh --force` | Full resync ignoring timestamps |
| `list` | Show all synced databases in a folder |
| `config set apiKey <key>` | Store API key in OS keychain |

Exit codes: `0` success, `1` general error, `2` auth error

## Source Files

| File | Purpose |
|------|---------|
| `main.ts` | Entry point, arg parsing (`node:util parseArgs`), command routing |
| `fs-adapter.ts` | `nodeFs: FileSystem` using `node:fs/promises` |
| `frontmatter-adapter.ts` | `nodeFm: FrontmatterReader` using `yaml` package |
| `config.ts` | API key storage (OS keychain via `@napi-rs/keyring`), config file I/O |

## API Key Priority

1. `--api-key` CLI flag
2. `NOTION_SYNC_API_KEY` env var
3. OS keychain (Windows Credential Manager / macOS Keychain / Linux Secret Service)
4. Config file fallback (`~/.notion-sync.json`) with warning
