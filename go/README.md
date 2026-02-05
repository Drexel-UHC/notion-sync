# notion-sync (Go)

Go implementation of the Notion database sync CLI.

## Build

```sh
go build ./cmd/notion-sync
```

## Test

```sh
go test ./...
```

## Usage

```sh
# Store API key (saved in OS keychain)
./notion-sync config set apiKey <your-notion-api-key>

# Sync a database
./notion-sync sync https://notion.so/your-database-url --output ./notion

# Refresh (incremental update)
./notion-sync refresh ./notion/MyDatabase

# Force refresh (resync all entries)
./notion-sync refresh ./notion/MyDatabase --force

# List synced databases
./notion-sync list ./notion
```

## Dependencies

- `github.com/zalando/go-keyring` — OS keychain access
- `gopkg.in/yaml.v3` — YAML parsing

No third-party Notion client — uses a thin REST wrapper for full control.

## Documentation

See `CLAUDE.md` for implementation details.
