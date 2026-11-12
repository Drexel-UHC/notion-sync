# Quickstart

> *Import, refresh, list.*

```sh
# Store your API key (saved in OS keychain)
notion-sync config set apiKey <your-notion-api-key>

# Import databases into an output folder (the workspace)
notion-sync import <database-id-A> --output ./my-notes
notion-sync import <database-id-B> --output ./my-notes

# Import a standalone page
notion-sync import <page-url-or-id> --output ./my-notes

# Refresh (incremental — only changed entries)
notion-sync refresh "./my-notes/Database A"

# Force refresh (resync everything)
notion-sync refresh "./my-notes/Database A" --force

# List synced databases and pages
notion-sync list ./my-notes
```

The `--output` folder is a **workspace**: each database gets a subfolder, standalone pages go under `pages/`, and notion-sync writes an `AGENTS.md` at the root for LLM tools. See [Workspace layout](../reference/workspace-layout.md).

```
my-notes/                        ← workspace (--output target)
├── AGENTS.md
├── Database A/
│   ├── _database.json
│   └── <notion-id>.md
└── Database B/
    ├── _database.json
    └── <notion-id>.md
```

Next: edit properties locally and send them back with [Push](../guides/push.md).
