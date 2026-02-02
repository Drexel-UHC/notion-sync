# notion-sync CLI

Thin CLI wrapper that wires `node:fs` to core's `FileSystem` and `FrontmatterReader` interfaces.

## Source files

| File | Purpose |
|------|---------|
| `main.ts` | Entry point. Parses args via `node:util parseArgs`. Commands: `sync`, `resync`, `config set`. |
| `fs-adapter.ts` | `nodeFs: FileSystem` — uses `node:fs/promises`. Resolves forward-slash paths from core via `path.resolve()`. |
| `frontmatter-adapter.ts` | `nodeFm: FrontmatterReader` — reads file, extracts YAML between `---` markers, parses with `yaml` package. |
| `config.ts` | Read/write `~/.notion-sync.json` (or `$XDG_CONFIG_HOME/notion-sync/config.json`). Env var `NOTION_SYNC_API_KEY` overrides file. |

## CLI usage

```sh
notion-sync sync <url-or-id> [--output <folder>] [--api-key <key>]
notion-sync resync <path> [--api-key <key>]
notion-sync config set <key> <value>
```

Exit codes: 0 success, 1 general error, 2 auth error.

## Build

```sh
npm run build -w packages/cli    # tsc
```

The `bin` field points to `./dist/main.js` with a `#!/usr/bin/env node` shebang.
