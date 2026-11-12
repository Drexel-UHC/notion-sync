# Commands

> *Every command and flag.*

```sh
notion-sync import <database-or-page-id> [--output <folder>] [--api-key <key>] [--keep-presigned-params]
notion-sync refresh <folder> [--ids id1,id2] [--force] [--api-key <key>] [--keep-presigned-params]
notion-sync push <folder> [--force] [--dry-run] [--yes] [--allow-new-options] [--api-key <key>]
notion-sync list [<output-folder>]
notion-sync clean <folder> [--dry-run]
notion-sync agents-md <folder>
notion-sync config set <key> <value>
```

Exit code is `0` on success, non-zero on error. `push` has its own exit-code rules — see [Push](../guides/push.md#run-summary).

## import

Import a Notion database or standalone page into a workspace. Detects automatically whether the ID is a database or a page. Accepts a 32-char hex ID, a dashed UUID, or a full Notion URL.

| Flag | Meaning |
| --- | --- |
| `--output`, `--out`, `-o` | Workspace folder (default from `config`, else `./notion`) |
| `--api-key` | Notion API key, overrides keychain/env |
| `--keep-presigned-params` | Keep AWS S3 pre-signed query strings on file URLs (default: stripped — see [File URLs](file-urls.md)) |

## refresh

Incrementally update an existing synced database or standalone page folder.

| Flag | Meaning |
| --- | --- |
| `--force`, `-f` | Resync every entry, ignoring timestamps (useful after a schema change) |
| `--ids id1,id2` | Resync only these page IDs (databases only) |
| `--api-key` | Notion API key |
| `--keep-presigned-params` | Keep S3 pre-signed query strings |

Semantics:

- Default refresh is incremental: entries whose `notion-last-edited` matches the local copy are skipped.
- Entries removed from Notion are **not** deleted from disk — they get `notion-deleted: true` (see [soft deletes](frontmatter.md#soft-deletes)).
- `refresh` also regenerates `AGENTS.md` when its version stamp is older than the running binary.

## push

Write local frontmatter property changes back to Notion, and create new rows from files marked `notion-create: true`. Properties only — page body is never touched. Guarded by a confirmation gate and an all-or-nothing validation gate.

| Flag | Meaning |
| --- | --- |
| `--dry-run` | Classify and diff with no writes (still reads Notion) |
| `--yes`, `-y` | Consent token for the confirmation gate; required non-interactively |
| `--force`, `-f` | Skip the whole validation gate — blind overwrite |
| `--allow-new-options` | Let unknown `select` / `multi_select` values create new options |
| `--api-key` | Notion API key |

Full guide: [Push](../guides/push.md).

## list

List every synced database and standalone page under a folder (scans for `_database.json` / `_page.json`).

## clean

Offline, in-place cleanups — **no API calls**. Run once after upgrading.

- strips Notion S3 pre-signed query strings from file URLs in `.md` content
- removes the deprecated `notion-frozen-at` frontmatter line
- canonicalizes legacy `www.notion.so/Title-{id}` URLs to `app.notion.com/p/{id}` in frontmatter and metadata JSON
- ensures trailing newlines on `.md` / `.json` files
- re-stamps `_database.json` / `_page.json` with the current `syncVersion` for any folder it modified
- regenerates `AGENTS.md` when its version stamp is older than the running binary

| Flag | Meaning |
| --- | --- |
| `--dry-run` | Show what would change without writing |

## agents-md

Regenerate `AGENTS.md` in a workspace from the running binary. **Always overwrites** the existing copy — the command name is the consent. No API calls. Use `clean` instead if you only want a rewrite on version drift.

## config

```sh
notion-sync config set apiKey <key>                 # stored in OS keychain
notion-sync config set defaultOutputFolder ./notion
```

Config file: `$XDG_CONFIG_HOME/notion-sync/config.json` or `~/.notion-sync.json`. The API key is not written to the file when a keychain is available. Key lookup order is in [Notion setup](../getting-started/notion-setup.md#store-the-api-key).
