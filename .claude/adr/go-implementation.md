# ADR: Go Implementation of notion-sync CLI

## Status
Proposed

## Context
The current notion-sync CLI is implemented in TypeScript (Node.js). For better distribution as a standalone binary without requiring Node.js, we're rewriting in Go.

**Why Go over alternatives:**
- Native binaries (~10MB) vs Bun-compiled (~50-80MB)
- Battle-tested cross-compilation (`GOOS`/`GOARCH`)
- `go-keyring` library provides clean OS keyring access (no native module bundling issues like `@napi-rs/keyring` with Bun)
- Widely used for CLI tools (gh, docker, terraform)

## Decision
Rewrite the CLI in Go while keeping the TypeScript packages as backup.

## Implementation Gaps & Notes

### Gap 1: Notion API Client
**Problem:** No official Go client for Notion API.

**Solution:** Build thin REST wrapper covering only needed endpoints:
- `GET /v1/databases/{id}` - Retrieve database metadata
- `POST /v1/dataSources/{id}/query` - Query entries (pagination)
- `GET /v1/blocks/{id}/children` - Fetch child blocks

**Notes:**
- Define Go structs for API responses (database, page, block types)
- Consider code-generating types from Notion's OpenAPI spec if available
- The `dataSources.query` endpoint is newer - verify Go struct matches response shape

### Gap 2: Block Type Coverage
**Problem:** 30+ Notion block types need conversion to Markdown.

**Implementation notes:**
- Start with common types: paragraph, headings, lists, code, quote
- Add complex types incrementally: table, column_list, synced_block
- Port TypeScript tests case-by-case to validate parity
- Unknown block types: log warning, return empty string (match TS behavior)

**Block type priority:**
1. Text: paragraph, heading_1/2/3, bulleted_list_item, numbered_list_item, to_do
2. Formatting: code, quote, callout, divider, equation
3. Media: image, video, file, bookmark, embed
4. Complex: table, column_list, toggle, synced_block
5. Links: child_page, child_database, link_to_page

### Gap 3: YAML Serialization Parity
**Problem:** Must produce identical YAML frontmatter to TypeScript version.

**Why not use yaml.Marshal:**
- TypeScript uses manual serialization for control over quoting
- Need exact parity for users switching between versions

**Quoting rules to implement:**
- Strings containing `:`, `#`, `"`, `'`, `\n`
- Strings starting with `-`, `[`, `{`, `>`, `|`, `*`, `&`, `!`, `%`, `@`, `` ` ``
- Strings that parse as bool: `true`, `false`, `yes`, `no`, `on`, `off`
- Strings that parse as numbers: `123`, `1.5`, `.inf`, `.nan`
- Empty strings

**Test approach:** Generate frontmatter in both TS and Go for same input, diff output.

### Gap 4: Rate Limiting State
**Problem:** TypeScript uses module-level `lastRequestTime`. Go needs thread-safe approach.

**Solution:**
```go
type Client struct {
    mu              sync.Mutex
    lastRequestTime time.Time
    // ...
}

func (c *Client) throttle() {
    c.mu.Lock()
    defer c.mu.Unlock()
    elapsed := time.Since(c.lastRequestTime)
    if elapsed < 340*time.Millisecond {
        time.Sleep(340*time.Millisecond - elapsed)
    }
    c.lastRequestTime = time.Now()
}
```

### Gap 5: Progress Callback Pattern
**Problem:** TypeScript uses callback function. Go idioms differ.

**Options:**
1. Callback function (match TS) - `func(ProgressPhase)`
2. Channel - `chan ProgressPhase`
3. Interface - `ProgressReporter` with methods

**Decision:** Use callback function for simplest port. Can refactor to channels later if needed.

### Gap 6: Keyring on Linux
**Problem:** `go-keyring` requires `libsecret` (GNOME) or `kwallet` (KDE).

**Mitigation:**
- Document dependency in README
- Graceful fallback to config file with warning (match current TS behavior)
- Test on Ubuntu with GNOME, verify KDE works

### Gap 7: Windows Path Handling
**Problem:** Core uses forward slashes internally. Windows uses backslashes.

**Solution:**
- Internal: Always use `/` (match TypeScript)
- File I/O: Use `filepath.FromSlash()` when calling OS functions
- User display: Convert to OS-native for output

### Gap 8: Testing Without Notion API
**Problem:** Can't call real Notion API in tests.

**Solution:**
- Mock HTTP server using `httptest.NewServer()`
- Fixture files with sample API responses
- Port existing TypeScript test mocks

## File System Interface

Match TypeScript interface for testability:

```go
type FileSystem interface {
    ReadFile(path string) (string, error)
    WriteFile(path string, content string) error
    FileExists(path string) (bool, error)
    Mkdir(path string, recursive bool) error
    ListMarkdownFiles(dir string) ([]string, error)
    ListDirectories(dir string) ([]string, error)
}

// Real implementation
type OSFileSystem struct{}

// Test implementation
type MemoryFileSystem struct {
    files map[string]string
}
```

## Migration Checklist

- [ ] Verify `dataSources.query` response shape with real API call
- [ ] Test keyring on all 3 platforms before release
- [ ] Compare YAML output for 10+ real database entries
- [ ] Benchmark: Go vs TypeScript sync time for same database
- [ ] Document any behavioral differences in CHANGELOG

## Consequences

**Positive:**
- Single binary distribution, no runtime dependencies
- Smaller download (~10MB vs npm install)
- Faster startup time
- Cleaner cross-platform keyring integration

**Negative:**
- Maintaining two codebases temporarily
- Go learning curve if unfamiliar
- Need to keep block type support in sync if Notion adds new types

**Neutral:**
- TypeScript version remains as reference/backup
- Can deprecate TypeScript version once Go is stable
