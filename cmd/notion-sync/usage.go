package main

// version is set at build time via -ldflags.
var version = "dev"

var usage = `notion-sync — Sync Notion databases and pages to Markdown (v` + version + `)

Usage:
  notion-sync import <database-or-page-id> [--output <folder>] [--api-key <key>] [--keep-presigned-params]
  notion-sync refresh <folder> [--ids id1,id2] [--force] [--api-key <key>] [--keep-presigned-params]
  notion-sync push <folder> [--force] [--dry-run] [--yes] [--allow-new-options] [--api-key <key>]
  notion-sync list [<output-folder>]
  notion-sync clean <folder> [--dry-run]
  notion-sync agents-md <folder>
  notion-sync config set <key> <value>

Commands:
  import    Import a Notion database or standalone page to local Markdown files
            Automatically detects whether the ID is a database or page.
            --keep-presigned-params  Keep AWS S3 pre-signed query strings on file URLs
                                     (default: stripped to keep diffs stable)
  refresh   Refresh an existing synced database or standalone page (incremental update)
            --ids id1,id2  Refresh only specific pages by ID (databases only)
            --force, -f    Resync all entries, ignoring timestamps
            --keep-presigned-params  Keep AWS S3 pre-signed query strings on file URLs
  push      Push local frontmatter property changes back to Notion (properties only, no body)
            Previews the push queue and prompts y/N before any API write (TTY only).
            Non-interactive runs (CI / pipes) must pass --yes; otherwise the run is cancelled.
            Runs a validation gate before any write: stray .md, malformed YAML, conflicting
            timestamps, unreachable rows, or invalid select/status/multi_select options halt
            the entire run (all-or-nothing).
            --force, -f    Skip the validation gate entirely. Pushes despite conflicts,
                           strays, malformed YAML, unreadable files, or unreachable rows.
                           Use only after manual review — overwrites Notion-side changes.
            --dry-run      Show what would be pushed without writing to Notion (still reads from Notion for conflict detection)
            --yes, -y      Skip the confirmation prompt (required in non-interactive runs)
            --allow-new-options  Let unknown select/multi_select values auto-create options in
                                 Notion's shared schema. Unknown status values still halt (the
                                 API cannot create status options). Default: unknown options halt.
  list      List all synced databases and pages in a folder
  clean     Strip AWS S3 pre-signed query strings from existing .md files in a folder.
            Useful one-time backfill after upgrading. No API calls.
            Also regenerates AGENTS.md if its version stamp is older than this binary.
            --dry-run  Show what would change without writing
  agents-md Regenerate AGENTS.md in a workspace from the running binary.
            Always overwrites any existing AGENTS.md (the command name is the consent).
            No API calls.
  config    Manage configuration (apiKey, defaultOutputFolder)

Examples:
  notion-sync import abc123de-f456-7890-abcd-ef1234567890 --output ./my-notes
  notion-sync refresh ./notion/My\ Database
  notion-sync refresh ./notion/pages/My\ Page_abc12345
  notion-sync refresh ./notion/My\ Database --force
  notion-sync push ./notion/My\ Database
  notion-sync push ./notion/My\ Database --dry-run
  notion-sync clean ./notion --dry-run
  notion-sync list ./notion

API Key Priority:
  1. NOTION_SYNC_API_KEY env var
  2. OS keychain (set via: notion-sync config set apiKey <key>)
  3. Config file fallback (~/.notion-sync.json)
`
