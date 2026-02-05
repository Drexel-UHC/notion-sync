# ADR: API Key Storage Security

## 1) Problem

notion-sync requires a Notion API integration token to authenticate against the Notion API. This token grants read access to all pages and databases shared with the integration — potentially an entire workspace. If leaked (via version control, malicious dependencies reading config files, or other processes on the machine), an attacker gains full read access to that Notion content. The token must be stored locally between runs, creating a security surface that needs to be managed in both the CLI and VS Code extension.

## 2) Current Behavior

### Origin: Obsidian plugin (`obsidian-notion-database-sync`)

notion-sync was extracted from the [obsidian-notion-database-sync](https://github.com/ran-codes/obsidian-notion-database-sync) Obsidian plugin. That plugin stores the API key as plaintext JSON in `<vault>/.obsidian/plugins/notion-database-sync/data.json` via Obsidian's `loadData()` / `saveData()` plugin API. The only security measure is masking the input field in the settings UI (`inputEl.type = "password"`). The key itself is unencrypted on disk.

This is the standard pattern across the Obsidian plugin ecosystem — Obsidian lacks a built-in secure credential storage API. Multiple Obsidian plugins have received CVEs for this exact issue (plaintext API key storage in `data.json`).

### CLI (`packages/cli`)

The API key is stored as plaintext JSON in `~/.notion-sync.json` (or `$XDG_CONFIG_HOME/notion-sync/config.json`). Three input methods exist with this priority: `NOTION_SYNC_API_KEY` env var > config file > `--api-key` flag. None are encrypted. The config file is readable by any process running as the current user.

### VS Code extension (`packages/vscode`)

The API key is stored in VS Code's `settings.json` via `vscode.workspace.getConfiguration("notionSync").get("apiKey")`. This is actively problematic because:
- Every other installed extension can read it via the same configuration API
- `settings.json` is frequently committed to dotfiles repos or synced via Settings Sync
- The file is plaintext on disk

### Version control implications

| Platform | Storage location | In the repo? | Git risk |
|----------|-----------------|--------------|----------|
| **Obsidian plugin** | `<vault>/.obsidian/plugins/notion-database-sync/data.json` | Yes — inside the vault | High. If the vault is a git repo (common with Obsidian), the key is committed unless `.obsidian/plugins/*/data.json` is in `.gitignore` |
| **CLI** | `~/.notion-sync.json` or `$XDG_CONFIG_HOME/notion-sync/config.json` | No — system-level user home directory | Low. The file lives outside any project repo. Not at risk of accidental commit |
| **VS Code extension** | `settings.json` (user-level: `%APPDATA%/Code/User/settings.json` on Windows, `~/.config/Code/User/settings.json` on Linux) | No — system-level VS Code config | Medium. Not in the project repo, but often shared via dotfiles repos or synced via Settings Sync to Microsoft's servers |
| **VS Code extension** | `settings.json` (workspace-level: `<project>/.vscode/settings.json`) | Yes — inside the project | High. If the user sets `apiKey` at workspace scope, it lands in `.vscode/settings.json` which is routinely committed |

## 3) Research: Best Practices

### CLI tools

Most developer CLIs (Stripe, Vercel, Railway, Netlify) store tokens in plaintext config files and rely on filesystem permissions. The notable exception is **GitHub CLI (`gh`)**, which since v2.26.0 uses the OS keychain by default:

- **macOS:** Keychain
- **Windows:** Credential Manager
- **Linux:** Secret Service API (gnome-keyring, kwallet)

The recommended Node.js library for OS keychain access is **`@napi-rs/keyring`** (~77K weekly downloads). It is a Rust-based replacement for the deprecated `keytar` (archived Dec 2022). Unlike keytar, it does not require `libsecret` on Linux. The Azure SDK adopted it as their keytar replacement.

```js
import { Entry } from '@napi-rs/keyring'
const entry = new Entry('notion-sync', 'api-key')
entry.setPassword(key)   // store
entry.getPassword()      // retrieve
```

**Limitation:** OS keychains do not work in headless environments (CI/CD, Docker, SSH without D-Bus). A fallback strategy is required.

### VS Code extensions

VS Code provides a built-in **`SecretStorage`** API (`context.secrets`) specifically for this purpose. It uses Electron's `safeStorage` backed by the OS keychain (Credential Manager on Windows, Keychain on macOS, Secret Service on Linux). Secrets are encrypted at rest in a SQLite database in VS Code's user data directory.

Popular extensions that use SecretStorage: GitHub Copilot (for BYOK keys), GitLab Workflow (migrated from globalState), Kilo Code (API keys in SecretStorage, non-sensitive config in JSON).

**Gotchas:**
- Global scope only (no per-workspace variant) — acceptable for a single API key
- Not synced across machines by design — users re-enter on each device
- On Linux without a keyring daemon, VS Code silently falls back to weak obfuscation (platform-level issue)
- Not isolated between extensions (a malicious extension could read another extension's secrets — VS Code platform limitation documented by Cycode)

## 4) Recommendations

### CLI

Adopt a three-tier priority cascade matching the `gh` CLI pattern:

| Priority | Source | Use case |
|----------|--------|----------|
| 1 | `NOTION_SYNC_API_KEY` env var | CI/CD, Docker, headless environments |
| 2 | OS keychain via `@napi-rs/keyring` | Normal interactive use |
| 3 | Plaintext config file + warning | Fallback when no keyring is available |

Implementation notes:
- Add `@napi-rs/keyring` as a dependency of `packages/cli`
- Update `notion-sync config set apiKey <key>` to write to keychain instead of JSON file
- On first run after upgrade, auto-migrate: read key from `~/.notion-sync.json`, store in keychain, remove from JSON, log a message
- If keychain is unavailable (headless), fall back to the existing JSON file with a one-time warning: `"Warning: No OS keychain found. API key stored in plaintext at ~/.notion-sync.json"`
- `@napi-rs/keyring` is a native addon (compiled Rust), which adds build complexity — acceptable tradeoff given the security improvement

### VS Code extension

Migrate from `settings.json` to `SecretStorage`:

| Step | Change |
|------|--------|
| 1 | Add a `notionSync.setApiKey` command using `showInputBox` with `password: true` |
| 2 | Store key via `context.secrets.store("notionSync.apiKey", key)` |
| 3 | On activation, auto-migrate: if `settings.json` has `apiKey`, move it to SecretStorage and clear the setting |
| 4 | Mark the `notionSync.apiKey` setting in `package.json` as deprecated via `deprecationMessage` |
| 5 | Pass `ExtensionContext` through to command handlers so they can access `context.secrets` |

Implementation notes:
- Keep `notionSync.defaultOutputFolder` in `settings.json` — it is non-sensitive
- The migration should be silent except for a one-time info message: `"Notion Sync: API key migrated to secure storage."`
- Error message on missing key should direct users to the new command: `Run "Notion Sync: Set API Key" from the Command Palette`

## 5) Implementation Done

Both recommendations from section 4 have been implemented.

### CLI (`packages/cli`)

- Added `@napi-rs/keyring` dependency for OS keychain access
- `config.ts`: Three keychain helpers (`tryKeychainGet`, `tryKeychainSet`, `tryKeychainDelete`) wrap native calls in try/catch for graceful fallback
- `config.ts`: `migrateApiKeyToKeychain()` — on first run after upgrade, moves the key from `~/.notion-sync.json` to the OS keychain and removes it from the JSON file
- `config.ts`: `loadConfig()` uses priority cascade: env var > keychain > config file (with warning on plaintext fallback)
- `config.ts`: `saveConfig("apiKey", ...)` stores in keychain first, falls back to JSON file with warning if keychain unavailable
- `main.ts`: Calls `migrateApiKeyToKeychain()` at startup before arg parsing

### VS Code extension (`packages/vscode`)

- `package.json`: Added `notionSync.setApiKey` command, marked `notionSync.apiKey` setting with `deprecationMessage`
- `commands.ts`: `getApiKey()` is now async and reads from `context.secrets` (SecretStorage)
- `commands.ts`: `setApiKeyCommand()` — input box with `password: true`, stores via SecretStorage
- `commands.ts`: `migrateApiKey()` — checks all settings scopes (global, workspace, workspaceFolder), moves key to SecretStorage, clears from all scopes
- `extension.ts`: Passes `ExtensionContext` to all command handlers, calls `migrateApiKey()` fire-and-forget on activation

### Files changed

| File | Change |
|------|--------|
| `packages/cli/package.json` | Added `@napi-rs/keyring` dependency |
| `packages/cli/src/config.ts` | Keychain helpers, migration, rewritten load/save |
| `packages/cli/src/main.ts` | Migration call at startup, updated help text |
| `packages/vscode/package.json` | New `setApiKey` command, deprecated `apiKey` setting |
| `packages/vscode/src/commands.ts` | SecretStorage-based getApiKey, setApiKey command, migration |
| `packages/vscode/src/extension.ts` | Context passing, new command registration, migration on activate |
