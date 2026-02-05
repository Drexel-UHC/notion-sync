# ADR: API Key Storage Security

## 1) Problem

notion-sync requires a Notion API integration token to authenticate against the Notion API. This token grants read access to all pages and databases shared with the integration — potentially an entire workspace. If leaked (via version control, malicious dependencies reading config files, or other processes on the machine), an attacker gains full read access to that Notion content. The token must be stored locally between runs, creating a security surface that needs to be managed.

## 2) Current Behavior

### Origin: Obsidian plugin (`obsidian-notion-database-sync`)

notion-sync was extracted from the [obsidian-notion-database-sync](https://github.com/ran-codes/obsidian-notion-database-sync) Obsidian plugin. That plugin stores the API key as plaintext JSON in `<vault>/.obsidian/plugins/notion-database-sync/data.json` via Obsidian's `loadData()` / `saveData()` plugin API. The only security measure is masking the input field in the settings UI (`inputEl.type = "password"`). The key itself is unencrypted on disk.

This is the standard pattern across the Obsidian plugin ecosystem — Obsidian lacks a built-in secure credential storage API. Multiple Obsidian plugins have received CVEs for this exact issue (plaintext API key storage in `data.json`).

### CLI (`packages/cli`)

The API key is stored in the OS keychain (Windows Credential Manager, macOS Keychain, Linux Secret Service) via `@napi-rs/keyring`. Three input methods exist with this priority: `NOTION_SYNC_API_KEY` env var > OS keychain > config file fallback (with warning). The config file (`~/.notion-sync.json`) is only used as a fallback in headless environments.

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

## 4) Implementation

The CLI uses a three-tier priority cascade matching the `gh` CLI pattern:

| Priority | Source | Use case |
|----------|--------|----------|
| 1 | `NOTION_SYNC_API_KEY` env var | CI/CD, Docker, headless environments |
| 2 | OS keychain via `@napi-rs/keyring` | Normal interactive use |
| 3 | Plaintext config file + warning | Fallback when no keyring is available |

### CLI (`packages/cli`)

- Added `@napi-rs/keyring` dependency for OS keychain access
- `config.ts`: Three keychain helpers (`tryKeychainGet`, `tryKeychainSet`, `tryKeychainDelete`) wrap native calls in try/catch for graceful fallback
- `config.ts`: `migrateApiKeyToKeychain()` — on first run after upgrade, moves the key from `~/.notion-sync.json` to the OS keychain and removes it from the JSON file
- `config.ts`: `loadConfig()` uses priority cascade: env var > keychain > config file (with warning on plaintext fallback)
- `config.ts`: `saveConfig("apiKey", ...)` stores in keychain first, falls back to JSON file with warning if keychain unavailable
- `main.ts`: Calls `migrateApiKeyToKeychain()` at startup before arg parsing

### Files changed

| File | Change |
|------|--------|
| `packages/cli/package.json` | Added `@napi-rs/keyring` dependency |
| `packages/cli/src/config.ts` | Keychain helpers, migration, rewritten load/save |
| `packages/cli/src/main.ts` | Migration call at startup, updated help text |
