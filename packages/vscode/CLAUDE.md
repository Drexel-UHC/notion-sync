# notion-sync-vscode

VS Code extension that wires `vscode.workspace.fs` to core's interfaces.

## Source files

| File | Purpose |
|------|---------|
| `extension.ts` | `activate()` / `deactivate()`. Registers two commands. |
| `commands.ts` | `syncCommand()` — prompts for URL + output folder, detects page vs database, runs with progress. `resyncCommand()` — reads active file frontmatter, re-syncs. |
| `fs-adapter.ts` | `vscodeFs(workspaceRoot: Uri): FileSystem` — all paths are relative to workspace root, resolved via `Uri.joinPath()`. |
| `frontmatter-adapter.ts` | `vscodeFm(fs: FileSystem): FrontmatterReader` — same YAML parsing as CLI, reads via the injected `FileSystem`. |

## Extension manifest

Two commands registered in `package.json` contributes:
- `notionSync.sync` — "Notion Sync: Sync Page or Database"
- `notionSync.resync` — "Notion Sync: Re-sync Current File"

Two settings:
- `notionSync.apiKey` — Notion API token
- `notionSync.defaultOutputFolder` — defaults to `"notion"`

## Build

```sh
npm run build -w packages/vscode    # esbuild, bundles to single dist/extension.js
```

Bundles core + yaml + @notionhq/client into one CJS file. `vscode` is external.

## Key differences from CLI adapter

- Paths are relative to workspace root (not absolute). Core's forward-slash paths map directly to `Uri.joinPath()`.
- `writeFile` auto-creates parent directories by creating the parent URI directory before writing.
- Progress uses `vscode.window.withProgress()` with `ProgressLocation.Notification`.
