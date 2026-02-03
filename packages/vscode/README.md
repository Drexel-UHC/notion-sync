# Notion Sync (VS Code Extension)

VS Code extension for syncing Notion pages and databases to Markdown files in your workspace. Thin wrapper that connects `vscode.workspace.fs` to the `@notion-sync/core` engine.

## Commands

Open the Command Palette (`Ctrl+Shift+P` / `Cmd+Shift+P`) and run:

- **Notion Sync: Sync Page or Database** -- prompts for a Notion URL and output folder, then syncs
- **Notion Sync: Re-sync Current File** -- reads `notion-id` from the active file's frontmatter and re-syncs it (or the whole database if it's a database entry)

## Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `notionSync.apiKey` | `""` | Notion API integration token |
| `notionSync.defaultOutputFolder` | `"notion"` | Default folder for synced files (relative to workspace root) |

Set these in VS Code settings (`Ctrl+,` / `Cmd+,`) or in `.vscode/settings.json`:

```json
{
  "notionSync.apiKey": "ntn_...",
  "notionSync.defaultOutputFolder": "notion"
}
```

## Source files

| File | Purpose |
|------|---------|
| `extension.ts` | `activate()` and `deactivate()`. Registers the two commands. |
| `commands.ts` | `syncCommand()` prompts for URL + output folder, detects page vs database, runs with progress notification. `resyncCommand()` reads frontmatter from the active file and re-syncs. |
| `fs-adapter.ts` | `vscodeFs(workspaceRoot): FileSystem` implementation using `vscode.workspace.fs`. All paths are relative to the workspace root, resolved via `Uri.joinPath()`. |
| `frontmatter-adapter.ts` | Creates a `FrontmatterReader` via core's `createFrontmatterReader()` using the VS Code filesystem. |

## Development

### Running in development

1. Open this repo in VS Code
2. Press `F5` to launch the Extension Development Host
3. In the new window, set `notionSync.apiKey` in settings
4. Run "Notion Sync: Sync Page or Database" from the Command Palette

### Building

```sh
npm run build -w packages/vscode
```

This runs esbuild to bundle `src/extension.ts` (plus core, yaml, and @notionhq/client) into a single CJS file at `dist/extension.js`. The `vscode` module is marked as external.

### Key differences from the CLI adapter

- **Relative paths** -- all paths are relative to the workspace root (not absolute). Core's forward-slash paths map directly to `Uri.joinPath()`.
- **Auto-creates directories** -- `writeFile` creates the parent directory before writing.
- **Progress UI** -- sync progress is shown via `vscode.window.withProgress()` as a notification.

## Publishing

### Package the extension

```sh
cd packages/vscode
npx @vscode/vsce package
```

This produces a `.vsix` file you can install locally or distribute.

### Publish to the VS Code Marketplace

```sh
cd packages/vscode
npx @vscode/vsce publish
```

You'll need a [Personal Access Token](https://code.visualstudio.com/api/working-with-extensions/publishing-extension#get-a-personal-access-token) from Azure DevOps and a publisher ID configured in `package.json`.
