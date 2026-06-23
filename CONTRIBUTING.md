# Contributing

How to set up a **fresh clone** for development. This is a pure Go module — no node/pnpm.

> `scripts/install.sh` / `install.ps1` are **end-user binary installers**, not dev setup. Don't run them to develop.

## Prerequisites

- **Go 1.24+** — `go.mod` pins `go 1.24.0`
- **GitHub CLI (`gh`)** — required for all GitHub ops (see [CLAUDE.md](CLAUDE.md))

### Install Go (Windows)

```powershell
winget install --id GoLang.Go -e    # or: scoop install go (no admin, user-dir install)
```

Then **restart the shell** so the updated `PATH` is picked up.

## Build + test

Dependencies (`zalando/go-keyring`, `yaml.v3`, `x/term`) are fetched automatically by Go modules on first build — nothing to install manually.

```sh
go build ./cmd/notion-sync   # build the CLI
go test ./...                # unit + integration (mock client, no API key needed)
```

## System tests

Hit the real Notion API — require an API key (see [CLAUDE.md](CLAUDE.md) for key setup).

```
/test-single-datasource-db
/test-double-datasource-db
```
