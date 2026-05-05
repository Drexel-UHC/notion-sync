package main

// CLI-level e2e coverage for the `clean` command.
//
// `clean` exists to absorb one-time migrations after a binary upgrade. The
// fixture below is a synthetic workspace that hits every migration the
// `internal/clean` package doc lists. The test asserts both stdout summary
// counts and post-state file contents, then re-runs `clean` to verify
// idempotency.
//
// Migrations exercised by the seed:
//
//	| # | Migration                                       | Fixture                          |
//	|---|-------------------------------------------------|----------------------------------|
//	| 1 | Strip S3 presigned query strings from .md       | entry.md frontmatter (PDF field) |
//	| 2 | Remove `notion-frozen-at:` from frontmatter     | entry.md                         |
//	| 3 | Canonicalize `notion-url:` in .md               | entry.md                         |
//	| 4 | Canonicalize `"url"` in metadata JSON           | _database.json + _page.json      |
//	| 5 | Normalize `folderPath` separator (\ → /)        | _database.json + _page.json      |
//	| 6 | Append trailing newline on .md/.json            | pages/.../My Page.md             |
//	| 7 | Bump syncVersion in _database.json              | My Database/_database.json       |
//	| 8 | Bump syncVersion in _page.json                  | pages/.../_page.json             |
//	| 9 | Regenerate stale AGENTS.md                      | workspace AGENTS.md (v0.0.1)     |
//
// API-drift caveat: this seed encodes notion-sync's *historical* output
// shapes — frozen-in-time legacy artifacts. It does NOT call the Notion API.
// If the importer's output contract changes (frontmatter keys, _database.json
// shape, AGENTS.md template, URL format), this fixture may need regeneration
// so the test stays representative of real workspaces. See issue #66.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testBinaryPath is the absolute path to a notion-sync binary built once in
// TestMain and reused across every test invocation in this package's e2e
// tests. Building once instead of `go run`-ing per call keeps a 4-invocation
// test from paying 4× the toolchain build cost.
var testBinaryPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "notion-sync-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "test bin temp dir:", err)
		os.Exit(2)
	}
	bin := "notion-sync-e2e"
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	testBinaryPath = filepath.Join(dir, bin)
	if out, err := exec.Command("go", "build", "-o", testBinaryPath, ".").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "test bin build failed: %v\n%s\n", err, out)
		os.RemoveAll(dir)
		os.Exit(2)
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

const (
	legacyEntryID     = "1234567890abcdef1234567890abcdef"
	legacyEntryURL    = "https://www.notion.so/Title-1234567890abcdef1234567890abcdef"
	canonicalEntryURL = "https://app.notion.com/p/1234567890abcdef1234567890abcdef"

	legacyDBURL    = "https://www.notion.so/My-Database-abcdefabcdefabcdefabcdefabcdefab"
	canonicalDBURL = "https://app.notion.com/p/abcdefabcdefabcdefabcdefabcdefab"

	legacyPageURL    = "https://www.notion.so/My-Page-fedcba9876543210fedcba9876543210"
	canonicalPageURL = "https://app.notion.com/p/fedcba9876543210fedcba9876543210"

	presignedURL    = "https://prod-files-secure.s3.us-west-2.amazonaws.com/x/y/file.pdf?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=abc&X-Amz-Date=20260101T000000Z"
	strippedFileURL = "https://prod-files-secure.s3.us-west-2.amazonaws.com/x/y/file.pdf"

	staleAgentsMD = "<!-- notion-sync-version: v0.0.1 -->\n# stale\n"
)

type seedPaths struct {
	workspace, entryMD, dbJSON, agentsMD, pageMD, pageJSON string
}

// writeSyntheticWorkspace lays out the fixture documented in the file header.
// Returns absolute paths to every file the test asserts against.
func writeSyntheticWorkspace(t *testing.T) seedPaths {
	t.Helper()
	p := seedPaths{workspace: t.TempDir()}

	// --- Database folder: covers migrations #1, #2, #3, #4, #5, #7
	dbDir := filepath.Join(p.workspace, "My Database")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatal(err)
	}
	p.entryMD = filepath.Join(dbDir, legacyEntryID+".md")
	entry := "---\n" +
		`notion-id: "` + legacyEntryID + `"` + "\n" +
		`notion-url: "` + legacyEntryURL + `"` + "\n" +
		`notion-frozen-at: "2026-01-01T00:00:00Z"` + "\n" +
		"PDF:\n" +
		`  - "` + presignedURL + `"` + "\n" +
		"---\n\nbody\n"
	if err := os.WriteFile(p.entryMD, []byte(entry), 0644); err != nil {
		t.Fatal(err)
	}
	p.dbJSON = filepath.Join(dbDir, "_database.json")
	dbJSON := `{
  "databaseId": "abcdefabcdefabcdefabcdefabcdefab",
  "title": "My Database",
  "url": "` + legacyDBURL + `",
  "folderPath": "test-output\\My Database",
  "lastSyncedAt": "2026-01-01T00:00:00Z",
  "entryCount": 1,
  "syncVersion": "v0.0.1"
}
`
	if err := os.WriteFile(p.dbJSON, []byte(dbJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// --- Standalone page folder: covers migrations #4, #5, #6, #8
	pageDir := filepath.Join(p.workspace, "pages", "My Page_fedcba98")
	if err := os.MkdirAll(pageDir, 0755); err != nil {
		t.Fatal(err)
	}
	p.pageMD = filepath.Join(pageDir, "My Page.md")
	// No trailing newline — exercises migration #6.
	pageMD := "---\n" +
		`notion-id: "fedcba9876543210fedcba9876543210"` + "\n" +
		"---\n\nstandalone body"
	if err := os.WriteFile(p.pageMD, []byte(pageMD), 0644); err != nil {
		t.Fatal(err)
	}
	p.pageJSON = filepath.Join(pageDir, "_page.json")
	pageJSON := `{
  "pageId": "fedcba9876543210fedcba9876543210",
  "title": "My Page",
  "url": "` + legacyPageURL + `",
  "folderPath": "test-output\\pages\\My Page_fedcba98",
  "lastSyncedAt": "2026-01-01T00:00:00Z",
  "syncVersion": "v0.0.1"
}
`
	if err := os.WriteFile(p.pageJSON, []byte(pageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// --- Workspace root: covers migration #9
	p.agentsMD = filepath.Join(p.workspace, "AGENTS.md")
	if err := os.WriteFile(p.agentsMD, []byte(staleAgentsMD), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCLI_Clean_AppliesAllMigrations(t *testing.T) {
	p := writeSyntheticWorkspace(t)

	originalEntry := readFile(t, p.entryMD)
	originalDB := readFile(t, p.dbJSON)
	originalPageMD := readFile(t, p.pageMD)
	originalPageJSON := readFile(t, p.pageJSON)
	originalAgents := readFile(t, p.agentsMD)

	// Counts the real run and the dry-run share. Derived from the seed:
	//   1 S3 URL  · 3 notion URLs (1 .md + 2 .json) · 2 backslash folderPaths
	//   1 frozen-at line · 1 file missing trailing newline
	//   2 metadata folders bumped (My Database + pages/My Page_fedcba98)
	// Anchor each count with the literal punctuation that immediately precedes
	// it in the summary line:
	//   "Modified: N files (N URLs stripped, N URLs canonicalized, ...)"
	// Without the leading "(" / ", " anchors, "1 URLs stripped" would also
	// match "11 URLs stripped" and any future double-digit counts.
	expectedCounts := []string{
		"(1 URLs stripped",
		", 3 URLs canonicalized",
		", 2 folderPaths normalized",
		", 1 trailing newlines added",
		", 1 notion-frozen-at lines stripped",
	}

	// Phase 1: dry-run reports counts but does not mutate disk.
	out := mustRunClean(t, p.workspace, true)
	expectContains(t, out, append(expectedCounts,
		"Would stamp syncVersion in: 2 folder(s)",
		"Would regenerate AGENTS.md",
	))
	expectUnchanged(t, p.entryMD, originalEntry, "entry.md")
	expectUnchanged(t, p.dbJSON, originalDB, "_database.json")
	expectUnchanged(t, p.pageMD, originalPageMD, "page.md")
	expectUnchanged(t, p.pageJSON, originalPageJSON, "_page.json")
	expectUnchanged(t, p.agentsMD, originalAgents, "AGENTS.md")

	// Phase 2: real run applies every migration.
	out = mustRunClean(t, p.workspace, false)
	expectContains(t, out, append(expectedCounts,
		"Stamped syncVersion in: 2 folder(s)",
		"Regenerated AGENTS.md",
	))

	// Migration #1, #2, #3 — entry.md
	entryAfter := readFile(t, p.entryMD)
	if !strings.Contains(entryAfter, strippedFileURL) {
		t.Errorf("[#1] presigned URL not stripped:\n%s", entryAfter)
	}
	if strings.Contains(entryAfter, "X-Amz-Signature") {
		t.Errorf("[#1] X-Amz-Signature still present:\n%s", entryAfter)
	}
	if strings.Contains(entryAfter, "notion-frozen-at") {
		t.Errorf("[#2] notion-frozen-at line still present:\n%s", entryAfter)
	}
	if !strings.Contains(entryAfter, canonicalEntryURL) {
		t.Errorf("[#3] notion-url not canonicalized:\n%s", entryAfter)
	}
	if strings.Contains(entryAfter, legacyEntryURL) {
		t.Errorf("[#3] legacy notion-url still present:\n%s", entryAfter)
	}

	// Migration #4, #5, #7 — _database.json
	expectMetadataMigrated(t, p.dbJSON, expectedMeta{
		URL:          canonicalDBURL,
		ID:           "abcdefabcdefabcdefabcdefabcdefab",
		Title:        "My Database",
		LastSyncedAt: "2026-01-01T00:00:00Z",
		EntryCount:   1,
	}, "[db]")

	// Migration #4, #5, #8 — _page.json
	expectMetadataMigrated(t, p.pageJSON, expectedMeta{
		URL:          canonicalPageURL,
		ID:           "fedcba9876543210fedcba9876543210",
		Title:        "My Page",
		LastSyncedAt: "2026-01-01T00:00:00Z",
	}, "[page]")

	// Migration #6 — page.md trailing newline added
	pageAfter := readFile(t, p.pageMD)
	if !strings.HasSuffix(pageAfter, "\n") {
		t.Errorf("[#6] page.md missing trailing newline:\n%q", pageAfter)
	}

	// Migration #9 — AGENTS.md regenerated with current binary's version stamp.
	// The TestMain build runs `go build` without -ldflags, so `version` stays at
	// its source default ("dev"). Anchor on that exact stamp so a regression
	// that writes the wrong version (e.g., a hardcoded constant) fails loudly.
	agentsAfter := readFile(t, p.agentsMD)
	if !strings.Contains(agentsAfter, "<!-- notion-sync-version: dev -->") {
		t.Errorf("[#9] AGENTS.md missing expected version stamp '<!-- notion-sync-version: dev -->':\n%s", agentsAfter)
	}

	// Phase 3: idempotency — second run reports zero work for every counter.
	out = mustRunClean(t, p.workspace, false)
	for _, want := range []string{
		"Modified: 0 files",
		"(0 URLs stripped",
		", 0 URLs canonicalized",
		", 0 folderPaths normalized",
		", 0 trailing newlines added",
		", 0 notion-frozen-at lines stripped",
		"Stamped syncVersion in: 0 folder(s)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("idempotent run missing %q\n--- output ---\n%s", want, out)
		}
	}
	if strings.Contains(out, "Regenerated AGENTS.md") {
		t.Errorf("AGENTS.md should not regenerate on second run, got:\n%s", out)
	}
}

func TestCLI_Clean_MissingFolderArg_ExitOne(t *testing.T) {
	out, err := exec.Command(testBinaryPath, "clean").CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit when folder arg is missing\n--- output ---\n%s", out)
	}
	// Anchor on the actual error string from runClean so this test fails
	// loudly if `clean` ever exits non-zero for an unrelated reason (build
	// regression, panic, different validation path).
	if !strings.Contains(string(out), "missing folder path") {
		t.Errorf("expected 'missing folder path' in stderr, got:\n%s", out)
	}
}

func mustRunClean(t *testing.T, workspace string, dryRun bool) string {
	t.Helper()
	args := []string{"clean", workspace}
	if dryRun {
		args = append(args, "--dry-run")
	}
	out, err := exec.Command(testBinaryPath, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("clean failed (dryRun=%v): %v\n%s", dryRun, err, out)
	}
	return string(out)
}

func expectContains(t *testing.T, out string, needles []string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(out, n) {
			t.Errorf("output missing %q\n--- output ---\n%s", n, out)
		}
	}
}

func expectUnchanged(t *testing.T, path, original, label string) {
	t.Helper()
	if readFile(t, path) != original {
		t.Errorf("dry-run mutated %s", label)
	}
}

// expectedMeta captures both the migrated values (URL) and the round-trip
// values that must survive the syncVersion bump. EntryCount is page-vs-db
// asymmetric: _database.json carries it, _page.json does not — a zero value
// is treated as "not part of this metadata type, skip the check."
type expectedMeta struct {
	URL          string
	ID           string // databaseId or pageId
	Title        string
	LastSyncedAt string
	EntryCount   int // 0 = skip (page metadata has no entryCount)
}

// expectMetadataMigrated asserts a metadata JSON file (_database.json or
// _page.json) had its url canonicalized, folderPath de-backslashed, and
// syncVersion bumped off the stale "v0.0.1" seed value. It also asserts that
// the rest of the struct round-tripped unchanged through the bump — the bump
// goes through Write{Database,Page}Metadata, so a regression that drops a
// field (entryCount, lastSyncedAt, title, id) would be invisible if we only
// asserted on the migrated fields.
func expectMetadataMigrated(t *testing.T, path string, want expectedMeta, tag string) {
	t.Helper()
	var meta struct {
		DatabaseID   string `json:"databaseId"`
		PageID       string `json:"pageId"`
		Title        string `json:"title"`
		URL          string `json:"url"`
		FolderPath   string `json:"folderPath"`
		LastSyncedAt string `json:"lastSyncedAt"`
		EntryCount   int    `json:"entryCount"`
		SyncVersion  string `json:"syncVersion"`
	}
	if err := json.Unmarshal([]byte(readFile(t, path)), &meta); err != nil {
		t.Fatalf("%s unmarshal: %v", tag, err)
	}

	// Migrations.
	if meta.URL != want.URL {
		t.Errorf("%s [#4] url: got %q, want %q", tag, meta.URL, want.URL)
	}
	if strings.Contains(meta.FolderPath, `\`) {
		t.Errorf("%s [#5] folderPath still contains backslash: %q", tag, meta.FolderPath)
	}
	if !strings.Contains(meta.FolderPath, "/") {
		t.Errorf("%s [#5] folderPath should contain forward slash: %q", tag, meta.FolderPath)
	}
	if meta.SyncVersion == "" || meta.SyncVersion == "v0.0.1" {
		t.Errorf("%s [#7/#8] syncVersion not bumped: got %q", tag, meta.SyncVersion)
	}

	// Preservation: fields that must round-trip unchanged through the bump.
	id := meta.DatabaseID
	if id == "" {
		id = meta.PageID
	}
	if id != want.ID {
		t.Errorf("%s preservation: id got %q, want %q", tag, id, want.ID)
	}
	if meta.Title != want.Title {
		t.Errorf("%s preservation: title got %q, want %q", tag, meta.Title, want.Title)
	}
	if meta.LastSyncedAt != want.LastSyncedAt {
		t.Errorf("%s preservation: lastSyncedAt got %q, want %q", tag, meta.LastSyncedAt, want.LastSyncedAt)
	}
	if want.EntryCount > 0 && meta.EntryCount != want.EntryCount {
		t.Errorf("%s preservation: entryCount got %d, want %d", tag, meta.EntryCount, want.EntryCount)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
