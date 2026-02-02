# ADR: Portability Initiative — Monorepo with CLI and VS Code Extension

**Status:** Proposed
**Date:** 2026-02-02
**Reference implementation:** https://github.com/ran-codes/obsidian-notion-database-sync

---

## Context

The Obsidian plugin "Notion Database Sync" (reference repo above) syncs Notion pages and databases into local Markdown files with YAML frontmatter. It supports incremental sync, deletion tracking, 30+ Notion block types, and database property mapping.

The plugin is tightly coupled to Obsidian's runtime (vault API, modal UI, requestUrl, metadata cache). This ADR proposes extracting the core logic into a platform-agnostic package and shipping it as both a standalone CLI tool and a VS Code extension, in a single monorepo.

### Market gap

Existing tools in this space are one-shot exporters, not incremental sync tools:

- **notion-to-md** (npm) — library for block conversion only. No sync, no frontmatter, no database property mapping.
- **notion-exporter** (npm) — uses Notion's internal export API (ZIP downloads), not the official REST API.
- **notion2md** (Python) — basic one-shot export.
- **@1984vc/notion_mdx** (npm) — database to MDX export. Closest competitor but no incremental sync or deletion tracking.

No existing VS Code extension does Notion database-to-Markdown sync. The closest is `munch-group/notion-interface` which does bidirectional page editing, a different use case.

**This tool's differentiators (must be preserved in all targets):**

1. Incremental sync via `last_edited_time` comparison
2. Database property-to-YAML frontmatter mapping (15+ property types)
3. Deletion tracking (marks removed entries, does not delete local files)
4. Rate limiting with exponential backoff, jitter, and Retry-After support
5. 30+ Notion block type conversion to Markdown

---

## Decision

Create a new monorepo `notion-freeze` with three packages sharing one core library:

```
notion-freeze/
├── packages/
│   ├── core/              # Platform-agnostic sync engine
│   │   ├── src/
│   │   │   ├── notion-client.ts
│   │   │   ├── block-converter.ts
│   │   │   ├── page-freezer.ts
│   │   │   ├── database-freezer.ts
│   │   │   ├── types.ts
│   │   │   └── index.ts
│   │   ├── package.json
│   │   └── tsconfig.json
│   ├── cli/               # CLI wrapper (Bun-compiled binary)
│   │   ├── src/
│   │   │   ├── main.ts
│   │   │   ├── config.ts
│   │   │   └── progress.ts
│   │   ├── package.json
│   │   └── tsconfig.json
│   └── vscode/            # VS Code extension wrapper
│       ├── src/
│       │   ├── extension.ts
│       │   ├── commands.ts
│       │   ├── settings.ts
│       │   └── progress.ts
│       ├── package.json
│       └── tsconfig.json
├── package.json           # Workspace root
├── tsconfig.base.json     # Shared TS config
└── README.md
```

Use **npm workspaces** for package management. TypeScript throughout.

---

## Package Details

### 1. `@notion-freeze/core`

The core package contains ALL business logic. It has zero platform dependencies — no `obsidian`, no `vscode`, no `node:fs` direct usage. Platform-specific behavior is injected via interfaces.

#### FileSystem interface

The reference implementation uses `app.vault` (Obsidian) for all file I/O. The core package replaces this with an injected interface:

```typescript
export interface FileSystem {
  readFile(path: string): Promise<string>;
  writeFile(path: string, content: string): Promise<void>;
  fileExists(path: string): Promise<boolean>;
  mkdir(path: string, recursive?: boolean): Promise<void>;
  listFiles(dir: string): Promise<string[]>;
}
```

Each platform provides its own implementation:

- **CLI:** Uses `node:fs/promises`
- **VS Code:** Uses `vscode.workspace.fs`

#### FrontmatterReader interface

The reference implementation uses `app.metadataCache.getFileCache()` to read existing frontmatter from files (used for skip detection and deletion tracking). The core package abstracts this:

```typescript
export interface FrontmatterReader {
  readFrontmatter(filePath: string): Promise<Record<string, unknown> | null>;
}
```

- **CLI:** Parse YAML between `---` markers from raw file content
- **VS Code:** Same parsing approach, or use a lightweight YAML parser

#### HTTP fetch

The reference implementation uses Obsidian's `requestUrl()` because Electron sandboxes native `fetch`. The core package uses standard `fetch` (available in Node 18+, Bun, and VS Code's Node runtime). No abstraction needed — just remove the `requestUrl` adapter.

The `createNotionClient()` function simplifies to:

```typescript
export function createNotionClient(apiKey: string): Client {
  return new Client({ auth: apiKey });
  // No custom fetch override needed outside Obsidian
}
```

#### What moves to core (from reference repo)

| Reference file            | Core file             | Changes needed                                                                                                                                                                                                                                                                                                                                                                                                            |
| ------------------------- | --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `src/types.ts`            | `types.ts`            | Remove `Client` import from Notion types. Add `FileSystem` and `FrontmatterReader` interfaces. Remove Obsidian-specific `FreezeOptions.client` — restructure so client is internal.                                                                                                                                                                                                                                       |
| `src/notion-client.ts`    | `notion-client.ts`    | Remove `requestUrl` import and custom fetch adapter. Keep `notionRequest()` (throttle + retry), `normalizeNotionId()`, `detectNotionObject()` unchanged.                                                                                                                                                                                                                                                                  |
| `src/block-converter.ts`  | `block-converter.ts`  | **No changes.** Already platform-agnostic. Only depends on `@notionhq/client` types and `notionRequest()`.                                                                                                                                                                                                                                                                                                                |
| `src/page-freezer.ts`     | `page-freezer.ts`     | Replace `app: App` parameter with `fs: FileSystem` and `fm: FrontmatterReader`. Replace `app.vault.getAbstractFileByPath()` → `fs.fileExists()`. Replace `app.metadataCache.getFileCache()` → `fm.readFrontmatter()`. Replace `app.vault.create/modify()` → `fs.writeFile()`. Replace `normalizePath()` → `path.posix.normalize()` or platform join. Remove `ensureFolder` using Obsidian vault → `fs.mkdir(path, true)`. |
| `src/database-freezer.ts` | `database-freezer.ts` | Same substitutions as page-freezer. Replace `scanLocalFiles()` to use `fs.listFiles()` + `fm.readFrontmatter()` instead of `TFolder.children` + `metadataCache`. Remove `.base` file generation entirely (Obsidian-specific).                                                                                                                                                                                             |

#### What does NOT move to core

- `src/main.ts` — Obsidian plugin lifecycle (onload, commands, ribbon icon)
- `src/settings.ts` — Obsidian settings UI tab
- `src/freeze-modal.ts` — Obsidian modal dialog
- `.base` file generation — Obsidian Bases feature, irrelevant outside Obsidian

#### Core public API

```typescript
// packages/core/src/index.ts

export {
  // Notion client
  createNotionClient,
  notionRequest,
  normalizeNotionId,
  detectNotionObject,
} from './notion-client';

export {
  // Page sync
  freezePage,
} from './page-freezer';

export {
  // Database sync
  freezeDatabase,
  type ProgressCallback,
} from './database-freezer';

export {
  // Block conversion (exposed for custom use)
  convertBlocksToMarkdown,
  convertRichText,
  fetchAllChildren,
} from './block-converter';

export type {
  FileSystem,
  FrontmatterReader,
  FreezeFrontmatter,
  FreezeOptions,
  PageFreezeResult,
  DatabaseFreezeResult,
  DetectionResult,
} from './types';
```

#### Restructured FreezeOptions

The reference `FreezeOptions` takes a `Client` instance. In the core package, restructure so the API key is passed and client creation is internal:

```typescript
export interface FreezeOptions {
  apiKey: string;
  notionId: string;
  outputFolder: string;
  fs: FileSystem;
  fm: FrontmatterReader;
  databaseId?: string;
}
```

Or alternatively, keep `Client` in options but have the consumer create it via `createNotionClient(apiKey)`. Either approach works — the key point is that `App` is removed.

#### Dependencies

```json
{
  "name": "@notion-freeze/core",
  "dependencies": {
    "@notionhq/client": "^5.3.0"
  },
  "devDependencies": {
    "typescript": "^5.5.0"
  }
}
```

Zero platform dependencies. Only `@notionhq/client`.

---

### 2. `@notion-freeze/cli`

Thin wrapper that provides a terminal interface and wires `node:fs` to the core's `FileSystem` interface.

#### Commands

```
notion-freeze sync <notion-url-or-id> [--output <folder>] [--api-key <key>]
notion-freeze resync <folder-or-file>
notion-freeze config set api-key <key>
notion-freeze config set output-folder <path>
```

#### Config file

Store settings in `~/.notion-freeze.json` (or `$XDG_CONFIG_HOME/notion-freeze/config.json`):

```json
{
  "apiKey": "ntn_...",
  "defaultOutputFolder": "./notion"
}
```

Also support `NOTION_FREEZE_API_KEY` environment variable (takes precedence over config file). This enables CI/CD and scripting use cases without persisting keys to disk.

#### FileSystem implementation (CLI)

```typescript
import { mkdir, readFile, writeFile, stat, readdir } from 'node:fs/promises';
import { join } from 'node:path';

const nodeFs: FileSystem = {
  readFile: (p) => readFile(p, 'utf-8'),
  writeFile: async (p, content) => {
    await mkdir(dirname(p), { recursive: true });
    await writeFile(p, content, 'utf-8');
  },
  fileExists: async (p) =>
    stat(p)
      .then(() => true)
      .catch(() => false),
  mkdir: (p, recursive) => mkdir(p, { recursive }).then(() => {}),
  listFiles: async (dir) => {
    const entries = await readdir(dir);
    return entries.filter((e) => e.endsWith('.md'));
  },
};
```

#### FrontmatterReader implementation (CLI)

Parse YAML from raw file content. No external YAML parser needed for the simple key-value frontmatter this tool produces:

```typescript
const nodeFm: FrontmatterReader = {
  readFrontmatter: async (filePath) => {
    try {
      const content = await readFile(filePath, 'utf-8');
      if (!content.startsWith('---\n')) return null;
      const endIdx = content.indexOf('\n---', 3);
      if (endIdx === -1) return null;
      const yamlBlock = content.slice(4, endIdx);
      // Simple line-by-line parser for flat YAML
      // (sufficient for the frontmatter this tool generates)
      return parseSimpleYaml(yamlBlock);
    } catch {
      return null;
    }
  },
};
```

#### Progress output

Use simple terminal progress:

```
Syncing "Project Tracker"... 12/48 entries
```

No dependency on a progress bar library needed. Use `process.stdout.write("\r...")` for in-place updates.

#### Distribution

**Primary:** `bun build --compile` to produce standalone binaries for:

- `notion-freeze-linux-x64`
- `notion-freeze-linux-arm64`
- `notion-freeze-darwin-x64`
- `notion-freeze-darwin-arm64`
- `notion-freeze-windows-x64.exe`

**Secondary:** Also publish to npm as `notion-freeze` for users who have Node/Bun:

```
npx notion-freeze sync <url>
```

**Binary size:** Expect ~50-90MB (Bun runtime bundled). Acceptable — Quarto ships at ~120-200MB, GitHub CLI at ~17MB (Go). If size becomes a concern, a Go rewrite of just the CLI is an option later.

#### Dependencies

```json
{
  "name": "notion-freeze",
  "bin": { "notion-freeze": "./dist/main.js" },
  "dependencies": {
    "@notion-freeze/core": "workspace:*"
  }
}
```

No argument parsing library needed for this simple command set. Use `process.argv` directly or a lightweight parser like `parseArgs` (built into Node 18.3+).

---

### 3. `@notion-freeze/vscode`

VS Code extension that provides a GUI for the same sync operations.

#### Why VS Code extension works

- VS Code ships its own embedded Node.js runtime — users do NOT need Node installed
- Distribution via VS Code Marketplace — same install experience as the Obsidian plugin
- TypeScript codebase, same language as core — no rewrite needed
- API surface maps closely to Obsidian's: commands, settings, file system, notifications

#### Obsidian → VS Code API mapping

| Obsidian API                          | VS Code API                                                                    | Notes                                     |
| ------------------------------------- | ------------------------------------------------------------------------------ | ----------------------------------------- |
| `Plugin.addCommand()`                 | `contributes.commands` in `package.json` + `vscode.commands.registerCommand()` | Near-identical pattern                    |
| `new Notice(msg)`                     | `vscode.window.showInformationMessage(msg)`                                    | For toasts                                |
| `new Notice(msg, 0)` (persistent)     | `vscode.window.withProgress()`                                                 | For long-running operations with progress |
| `Modal`                               | `vscode.window.showInputBox()` + `vscode.window.showQuickPick()`               | Multi-step input via sequential prompts   |
| `PluginSettingTab` / `Setting`        | `contributes.configuration` in `package.json`                                  | Declarative settings in VS Code           |
| `this.loadData()` / `this.saveData()` | `vscode.workspace.getConfiguration("notionFreeze")`                            | Built-in settings storage                 |
| `app.vault.create()` / `modify()`     | `vscode.workspace.fs.writeFile()`                                              | Both async                                |
| `app.vault.getAbstractFileByPath()`   | `vscode.workspace.fs.stat()`                                                   | Check existence                           |
| `app.vault.read()`                    | `vscode.workspace.fs.readFile()`                                               | Read content                              |
| `app.metadataCache.getFileCache()`    | Custom frontmatter parser (same as CLI)                                        | No built-in YAML cache in VS Code         |
| `addRibbonIcon()`                     | Activity bar icon or status bar item                                           | Optional                                  |
| `normalizePath()`                     | `vscode.Uri.joinPath()`                                                        | URI-based paths in VS Code                |

#### Extension manifest (package.json contributes)

```json
{
  "contributes": {
    "commands": [
      {
        "command": "notionFreeze.sync",
        "title": "Notion Freeze: Sync Page or Database"
      },
      {
        "command": "notionFreeze.resync",
        "title": "Notion Freeze: Re-sync Current File"
      }
    ],
    "configuration": {
      "title": "Notion Freeze",
      "properties": {
        "notionFreeze.apiKey": {
          "type": "string",
          "default": "",
          "description": "Notion API integration token"
        },
        "notionFreeze.defaultOutputFolder": {
          "type": "string",
          "default": "notion",
          "description": "Default folder for synced Markdown files (relative to workspace root)"
        }
      }
    }
  }
}
```

#### FileSystem implementation (VS Code)

```typescript
import * as vscode from 'vscode';

function vscodeFs(workspaceRoot: vscode.Uri): FileSystem {
  return {
    readFile: async (p) => {
      const uri = vscode.Uri.joinPath(workspaceRoot, p);
      const bytes = await vscode.workspace.fs.readFile(uri);
      return Buffer.from(bytes).toString('utf-8');
    },
    writeFile: async (p, content) => {
      const uri = vscode.Uri.joinPath(workspaceRoot, p);
      await vscode.workspace.fs.writeFile(uri, Buffer.from(content, 'utf-8'));
    },
    fileExists: async (p) => {
      const uri = vscode.Uri.joinPath(workspaceRoot, p);
      try {
        await vscode.workspace.fs.stat(uri);
        return true;
      } catch {
        return false;
      }
    },
    mkdir: async (p) => {
      const uri = vscode.Uri.joinPath(workspaceRoot, p);
      await vscode.workspace.fs.createDirectory(uri);
    },
    listFiles: async (dir) => {
      const uri = vscode.Uri.joinPath(workspaceRoot, dir);
      const entries = await vscode.workspace.fs.readDirectory(uri);
      return entries
        .filter(
          ([name, type]) =>
            type === vscode.FileType.File && name.endsWith('.md'),
        )
        .map(([name]) => name);
    },
  };
}
```

#### Sync command flow (VS Code)

1. User triggers `Notion Freeze: Sync Page or Database` from Command Palette
2. `vscode.window.showInputBox()` prompts for Notion URL or ID
3. `vscode.window.showInputBox()` prompts for output folder (pre-filled with default)
4. `vscode.window.withProgress()` wraps the sync operation with a progress bar
5. Core `detectNotionObject()` determines page vs database
6. Core `freezePage()` or `freezeDatabase()` runs with VS Code filesystem adapter
7. `vscode.window.showInformationMessage()` displays results

#### Re-sync command flow (VS Code)

1. User triggers `Notion Freeze: Re-sync Current File` from Command Palette
2. Extension reads the active editor's file content
3. Parses frontmatter for `notion-id` and `notion-database-id`
4. If `notion-database-id` exists: re-syncs entire database
5. Otherwise: re-syncs single page
6. Progress and results shown via VS Code notifications

#### Distribution

Publish to VS Code Marketplace as `notion-freeze`. Use `@vscode/vsce` for packaging.

The extension bundles `@notion-freeze/core` (including `@notionhq/client`) via esbuild into a single `extension.js` file — same bundling approach as the Obsidian plugin.

#### Dependencies

```json
{
  "name": "notion-freeze-vscode",
  "dependencies": {
    "@notion-freeze/core": "workspace:*"
  },
  "devDependencies": {
    "@types/vscode": "^1.85.0",
    "esbuild": "^0.25.0",
    "typescript": "^5.5.0"
  }
}
```

---

## Reference Implementation: Coupling Analysis

Precise mapping of every Obsidian-coupled line in the reference repo, for the implementing agent to know exactly what to change.

### `src/notion-client.ts` — 2 coupling points

| Line  | Obsidian API                              | Replacement                                                          |
| ----- | ----------------------------------------- | -------------------------------------------------------------------- |
| L2    | `import { requestUrl } from "obsidian"`   | Remove import                                                        |
| L8-23 | Custom `fetch` adapter using `requestUrl` | Remove entirely — `new Client({ auth: apiKey })` uses native `fetch` |

Everything else in this file (`notionRequest`, `normalizeNotionId`, `detectNotionObject`, throttle/retry logic) is platform-agnostic and moves unchanged.

### `src/block-converter.ts` — 0 coupling points

Entirely platform-agnostic. Only imports from `@notionhq/client` types and `./notion-client`. Moves to core unchanged.

### `src/page-freezer.ts` — 8 coupling points

| Line   | Obsidian API                                           | Replacement                                       |
| ------ | ------------------------------------------------------ | ------------------------------------------------- |
| L6     | `import { App, normalizePath, TFile } from "obsidian"` | Remove                                            |
| L11    | `app: App` parameter                                   | `fs: FileSystem, fm: FrontmatterReader`           |
| L24    | `normalizePath(...)`                                   | `path.posix.join(outputFolder, safeName + ".md")` |
| L27    | `app.vault.getAbstractFileByPath(filePath)`            | `fs.fileExists(filePath)`                         |
| L29    | `app.metadataCache.getFileCache(existingFile)`         | `fm.readFrontmatter(filePath)`                    |
| L62-63 | `app.vault.modify(existingFile, content)`              | `fs.writeFile(filePath, content)`                 |
| L65    | `ensureFolder(app, outputFolder)`                      | `fs.mkdir(outputFolder, true)`                    |
| L66    | `app.vault.create(filePath, content)`                  | `fs.writeFile(filePath, content)`                 |

### `src/database-freezer.ts` — 10 coupling points

| Line     | Obsidian API                                                      | Replacement                                              |
| -------- | ----------------------------------------------------------------- | -------------------------------------------------------- |
| L9       | `import { App, normalizePath, TFile, TFolder } from "obsidian"`   | Remove                                                   |
| L17      | `app: App` parameter                                              | `fs: FileSystem, fm: FrontmatterReader`                  |
| L31      | `normalizePath(...)`                                              | `path.posix.join(...)`                                   |
| L47      | `ensureFolderExists(app, folderPath)`                             | `fs.mkdir(folderPath, true)`                             |
| L50      | `generateBaseFile(app, ...)`                                      | Remove entirely (Obsidian-specific)                      |
| L56      | `scanLocalFiles(app, folderPath)`                                 | Rewrite to use `fs.listFiles()` + `fm.readFrontmatter()` |
| L77      | `freezePage(app, ...)`                                            | `freezePage(fs, fm, ...)`                                |
| L104     | `markAsDeleted(app, file)`                                        | Rewrite to use `fs.readFile()` + `fs.writeFile()`        |
| L149-167 | `scanLocalFiles()` body using `TFolder`, `TFile`, `metadataCache` | Rewrite with `fs` and `fm` interfaces                    |
| L191-233 | `generateBaseFile()`                                              | Remove entirely                                          |

### `src/types.ts` — 1 coupling point

| Line | Obsidian API                                | Replacement                                                                |
| ---- | ------------------------------------------- | -------------------------------------------------------------------------- |
| L1   | `import { Client } from "@notionhq/client"` | Keep, but add `FileSystem` and `FrontmatterReader` interfaces to this file |

### Files that do NOT port (Obsidian-only)

- `src/main.ts` — Plugin class, command registration, ribbon icon, lifecycle
- `src/settings.ts` — Obsidian PluginSettingTab UI
- `src/freeze-modal.ts` — Obsidian Modal dialog

---

## Workspace Configuration

### Root `package.json`

```json
{
  "name": "notion-freeze",
  "private": true,
  "workspaces": ["packages/core", "packages/cli", "packages/vscode"],
  "scripts": {
    "build": "npm run build --workspaces",
    "build:core": "npm run build -w packages/core",
    "build:cli": "npm run build -w packages/cli",
    "build:vscode": "npm run build -w packages/vscode",
    "compile:cli": "npm run compile -w packages/cli"
  }
}
```

### Root `tsconfig.base.json`

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "outDir": "dist"
  }
}
```

Each package extends this with its own `tsconfig.json`.

---

## Build and Distribution

### Core package

```bash
tsc -p packages/core/tsconfig.json
```

Outputs to `packages/core/dist/`. Consumed by CLI and VS Code as a workspace dependency.

### CLI

```bash
# Development
tsc -p packages/cli/tsconfig.json

# Production binary (standalone, no Node required)
bun build packages/cli/src/main.ts --compile --outfile dist/notion-freeze
```

Cross-compile for all targets:

```bash
bun build --compile --target=bun-linux-x64 --outfile dist/notion-freeze-linux-x64
bun build --compile --target=bun-linux-arm64 --outfile dist/notion-freeze-linux-arm64
bun build --compile --target=bun-darwin-x64 --outfile dist/notion-freeze-darwin-x64
bun build --compile --target=bun-darwin-arm64 --outfile dist/notion-freeze-darwin-arm64
bun build --compile --target=bun-windows-x64 --outfile dist/notion-freeze-windows-x64.exe
```

### VS Code extension

```bash
# Bundle with esbuild (same pattern as the Obsidian plugin)
esbuild packages/vscode/src/extension.ts \
  --bundle --outfile=packages/vscode/dist/extension.js \
  --format=cjs --platform=node \
  --external:vscode

# Package for marketplace
cd packages/vscode && vsce package
```

---

## Implementation Order

An agent building this from scratch should follow this sequence:

1. **Scaffold the monorepo** — root `package.json` with workspaces, `tsconfig.base.json`
2. **Build `@notion-freeze/core`** — extract and adapt from reference repo:
   - Copy `block-converter.ts` unchanged
   - Copy `notion-client.ts`, remove `requestUrl` adapter
   - Copy `page-freezer.ts`, replace `App` with `FileSystem`/`FrontmatterReader`
   - Copy `database-freezer.ts`, same replacements, remove `.base` generation
   - Define interfaces in `types.ts`
   - Create `index.ts` barrel export
3. **Build `notion-freeze` CLI** — wire `node:fs` to core interfaces, add arg parsing and config
4. **Build `notion-freeze-vscode`** — wire `vscode.workspace.fs` to core interfaces, register commands and settings
5. **Test** — sync a known Notion database via both CLI and VS Code extension, verify identical Markdown output
6. **Set up Bun compile** for CLI binary distribution
7. **Set up `vsce package`** for VS Code Marketplace

---

## What This Document Is For

This ADR is designed to be handed to an agent (e.g., Claude Code) in a fresh, empty repository. The agent should:

1. Read this document
2. Clone or reference the source at https://github.com/ran-codes/obsidian-notion-database-sync to understand the existing implementation
3. Build the monorepo from scratch following the architecture above
4. The reference repo's `src/` files contain the exact logic to extract — the coupling analysis tables above identify every line that needs to change

The core logic is ~420 lines of block conversion, ~80 lines of Notion client/retry, ~160 lines of page freezing, and ~170 lines of database freezing. The total portable codebase is under 900 lines of TypeScript.
