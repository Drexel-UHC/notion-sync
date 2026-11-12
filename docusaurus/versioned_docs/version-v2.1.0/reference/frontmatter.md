---
sidebar_position: 3
title: Frontmatter
---

# Frontmatter

*Metadata keys and how Notion properties map to YAML.*

Every `.md` file starts with YAML frontmatter. The first block is always notion-sync metadata; everything after is the Notion page's properties.

```yaml
---
notion-id: "<page-uuid>"
notion-url: "https://app.notion.com/p/<page-id-32-hex>"
notion-last-edited: "<RFC 3339 — Notion's last_edited_time>"
notion-database-id: "<database-uuid>"   # only present for database entries
# notion-deleted: true                  # only present if the entry was removed in Notion (soft delete)
# notion-last-pushed: "<RFC 3339>"      # only present after a push — when properties were last written back

# ... all Notion properties below ...
Title Property: "Page Name"
Status: "In Progress"
Tags: [a, b, c]
PDF:
  - "https://prod-files-secure.s3.us-west-2.amazonaws.com/<bucket>/<uuid>/file.pdf"
---
```

## Metadata keys

| Key | Meaning |
| --- | --- |
| `notion-id` | Page UUID — the file's identity |
| `notion-url` | Canonical `app.notion.com/p/...` URL |
| `notion-last-edited` | Notion's `last_edited_time`; refresh skips entries where this matches |
| `notion-database-id` | Parent database (database entries only) |
| `notion-deleted` | `true` once the entry is removed in Notion |
| `notion-last-pushed` | When `push` last wrote this row's properties |
| `notion-create` | Hand-written on a new file: `true` asks `push` to create the row; `pending` means a create is in flight or failed — see [Creating new rows](../guides/push.md#creating-new-rows) |

## Property mapping

| Notion type | Frontmatter value | Pushable? |
| --- | --- | --- |
| `title` | named title-property key (database entries); filename for standalone pages | yes — text only, see [rich-text fidelity](../guides/push.md#rich-text-fidelity) |
| `rich_text` | plain Markdown string | yes — text only, see [rich-text fidelity](../guides/push.md#rich-text-fidelity) |
| `number` | number or `null` | yes |
| `select` | option name or `null` | yes |
| `multi_select` | array of option names | yes |
| `status` | status name or `null` | yes |
| `date` | `start` or `start → end` | yes |
| `checkbox` | `true` or `false` | yes |
| `url` / `email` / `phone_number` | string or `null` | yes |
| `relation` | array of page IDs | yes |
| `people` | array of names (or IDs as fallback) | no — Notion-managed |
| `files` | array of URLs (see [File URLs](file-urls.md)) | no — Notion-managed |
| `created_time` / `last_edited_time` | RFC 3339 timestamp | no — read-only |
| `unique_id` | `PREFIX-N` or `N` | no — read-only |
| `created_by` / `last_edited_by` | user name (or ID as fallback) | no — read-only |

Skipped (not in frontmatter): `formula`, `rollup`, `button`, `verification` — they're computed or non-portable.

## Soft deletes

Pages removed from Notion are **not** deleted from disk on refresh. Instead `notion-deleted: true` is added to the frontmatter. Treat any file with that key as historical. `push` never pushes these files.
