---
sidebar_position: 2
title: Notion setup
---

# Notion setup

*Create an integration, share databases with it, and find their IDs.*

## Create a Notion integration

1. Go to [notion.so/my-integrations](https://www.notion.so/my-integrations).
2. Click **New integration**.
3. Give it a name (e.g. "notion-sync") and select a workspace.
4. Copy the **Internal Integration Secret** (starts with `ntn_`).
5. In Notion, open the database you want to sync.
6. Click the `...` menu → **Connections** → add your integration.

The integration can only see databases and pages explicitly connected to it.

## Store the API key

```sh
notion-sync config set apiKey <your-notion-api-key>
```

The key is saved in your OS keychain (Windows Credential Manager, macOS Keychain, or Linux Secret Service).

notion-sync looks for the key in this order:

1. `--api-key` flag
2. `NOTION_SYNC_API_KEY` environment variable
3. OS keychain
4. Config file fallback (`~/.notion-sync.json`) — prints a warning

## Find a database ID

Open the database as a **full page** in Notion. The URL looks like:

```
https://www.notion.so/yourworkspace/abc123def4567890abcdef1234567890?v=...
```

The database ID is the 32-character hex string after the workspace name — here `abc123def4567890abcdef1234567890`. notion-sync accepts it with or without dashes, or the full URL.

Standalone pages work the same way: pass the page's URL or ID to `import` and notion-sync detects that it's a page.
