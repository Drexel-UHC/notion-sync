---
sidebar_position: 5
title: Metadata files
---

# Metadata files

*`_database.json` and `_page.json`.*

These files let `refresh` work from just a folder path — they record what the folder is synced from.

## `_database.json`

One per database folder.

```json
{
  "databaseId": "<uuid>",
  "dataSourceId": "<uuid>",
  "title": "Human Title",
  "url": "https://app.notion.com/p/<database-id-32-hex>",
  "folderPath": "<absolute path>",
  "lastSyncedAt": "<RFC 3339>",
  "entryCount": 42,
  "syncVersion": "v0.5.0"
}
```

For **multi-source databases**, the top-level `_database.json` has no `dataSourceId`; each per-source subfolder has its own with `dataSourceId` set. Top-level `entryCount` is the total across all sources.

## `_page.json`

One per standalone-page folder.

```json
{
  "pageId": "<uuid>",
  "title": "Page Title",
  "url": "https://app.notion.com/p/<page-id-32-hex>",
  "folderPath": "<absolute path>",
  "lastSyncedAt": "<RFC 3339>",
  "syncVersion": "v0.5.0"
}
```
