---
slug: /
sidebar_position: 1
title: Introduction
---

# notion-sync

*Sync Notion databases and pages to local Markdown files with YAML frontmatter.*

Given a Notion database ID (or a standalone page URL), notion-sync fetches every entry through the Notion API and writes one `.md` file per page. Each file carries YAML frontmatter with the Notion ID, URL, last-edited timestamp, and every property value. Later runs compare timestamps and re-sync only what changed. `push` sends property edits back to Notion.

## Who these docs are for

- **People** using the CLI — start with [Install](getting-started/install.md) and [Quickstart](getting-started/quickstart.md).
- **Agents** (LLM tools) reading or editing a synced workspace — see [For agents](agents.md). Every page here is also served as plain Markdown, indexed in [`llms.txt`](pathname:///notion-sync/next/llms.txt).

## At a glance

| Command | What it does |
| --- | --- |
| `import` | First-time import of a database or standalone page |
| `refresh` | Incremental update — only changed entries |
| `push` | Write local property edits back to Notion (guarded) |
| `list` | Show synced databases and pages in a folder |
| `clean` | Offline cleanups after an upgrade |
| `agents-md` | Regenerate the workspace `AGENTS.md` |
| `config` | Store the API key and defaults |

Full details: [Commands](reference/commands.md).
