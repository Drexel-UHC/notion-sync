---
sidebar_position: 1
title: Install
---

# Install

*Install, update, or download notion-sync.*

## Install

### Install script (macOS / Linux)

```sh
curl -fsSL https://raw.githubusercontent.com/ran-codes/notion-sync/main/scripts/install.sh | bash
```

### Scoop (Windows)

```powershell
# Install Scoop if you don't have it: https://scoop.sh
# irm get.scoop.sh | iex
scoop bucket add notion-sync https://github.com/ran-codes/notion-sync
scoop install notion-sync
```

### Manual download

Download the binary for your platform from [GitHub Releases](https://github.com/Drexel-UHC/notion-sync/releases), rename it to `notion-sync` (or `notion-sync.exe` on Windows), and add it to your `PATH`.

Binaries are published for Windows amd64, macOS amd64/arm64, and Linux amd64/arm64.

## Update

### Install script (macOS / Linux)

Re-run the install script — it always fetches the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/ran-codes/notion-sync/main/scripts/install.sh | bash
```

### Scoop (Windows)

```powershell
scoop update
scoop update notion-sync
```

### Manual

Download the latest binary from [GitHub Releases](https://github.com/Drexel-UHC/notion-sync/releases) and replace the existing one.

After upgrading, run [`clean`](../reference/commands.md#clean) once on existing workspaces to apply format cleanups without any API calls.
