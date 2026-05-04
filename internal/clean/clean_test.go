package clean

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ran-codes/notion-sync/internal/sync"
)

func TestStripContent_FrontmatterFileProperty(t *testing.T) {
	in := `---
notion-id: abc
PDF:
  - "https://prod-files-secure.s3.us-west-2.amazonaws.com/bucket/uuid/file.pdf?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=ASIAxx%2F20260428%2Fus-west-2%2Fs3%2Faws4_request&X-Amz-Date=20260428T150234Z&X-Amz-Signature=9af1bc&X-Amz-SignedHeaders=host"
---

body
`
	got, count := stripContent(in)
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if !strings.Contains(got, `"https://prod-files-secure.s3.us-west-2.amazonaws.com/bucket/uuid/file.pdf"`) {
		t.Errorf("stripped URL not found in output:\n%s", got)
	}
	if strings.Contains(got, "X-Amz") {
		t.Errorf("X-Amz should have been stripped:\n%s", got)
	}
}

func TestStripContent_MarkdownImageEmbed(t *testing.T) {
	in := `paragraph

![image](https://prod-files-secure.s3.us-west-2.amazonaws.com/x/y/cat.png?X-Amz-Algorithm=AWS4&X-Amz-Signature=abc&X-Amz-Date=20260101T000000Z)

next
`
	got, count := stripContent(in)
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if !strings.Contains(got, "![image](https://prod-files-secure.s3.us-west-2.amazonaws.com/x/y/cat.png)") {
		t.Errorf("stripped image URL not found:\n%s", got)
	}
}

func TestStripContent_LeavesNonAWSAlone(t *testing.T) {
	in := `[link](https://example.com/file.pdf?foo=bar&baz=qux)
`
	got, count := stripContent(in)
	if count != 0 {
		t.Errorf("count = %d, want 0 (non-AWS URL must not be stripped)", count)
	}
	if got != in {
		t.Errorf("content mutated unexpectedly")
	}
}

func TestStripContent_LeavesAWSWithoutSignatureAlone(t *testing.T) {
	// Public S3 URL without a signature — just a query string, not presigned.
	in := `[link](https://my-bucket.s3.amazonaws.com/path/file.pdf?versionId=abc)
`
	got, count := stripContent(in)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
	if got != in {
		t.Errorf("content mutated unexpectedly")
	}
}

func TestStripContent_MultipleURLsInOneFile(t *testing.T) {
	in := `---
PDF:
  - "https://prod-files-secure.s3.us-west-2.amazonaws.com/a/file.pdf?X-Amz-Signature=aaa&X-Amz-Date=20260101T000000Z"
Thumbnail:
  - "https://prod-files-secure.s3.us-west-2.amazonaws.com/b/img.png?X-Amz-Signature=bbb&X-Amz-Date=20260101T000000Z"
---

![thumb](https://prod-files-secure.s3.us-west-2.amazonaws.com/c/inline.png?X-Amz-Signature=ccc&X-Amz-Date=20260101T000000Z)
`
	_, count := stripContent(in)
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestFolder_DryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "entry.md")
	original := `---
PDF:
  - "https://prod-files-secure.s3.us-west-2.amazonaws.com/x/file.pdf?X-Amz-Signature=abc&X-Amz-Date=20260101T000000Z"
---
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.FilesScanned != 1 || r.FilesChanged != 1 || r.URLsStripped != 1 {
		t.Errorf("got %+v", r)
	}

	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Errorf("dry-run modified the file")
	}
}

func TestFolder_RealRunModifiesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "entry.md")
	original := `---
PDF:
  - "https://prod-files-secure.s3.us-west-2.amazonaws.com/x/file.pdf?X-Amz-Signature=abc&X-Amz-Date=20260101T000000Z"
---
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.FilesScanned != 1 || r.FilesChanged != 1 || r.URLsStripped != 1 {
		t.Errorf("got %+v", r)
	}

	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "X-Amz") {
		t.Errorf("file still contains X-Amz params:\n%s", got)
	}
	if !strings.Contains(string(got), `"https://prod-files-secure.s3.us-west-2.amazonaws.com/x/file.pdf"`) {
		t.Errorf("expected stripped URL in file:\n%s", got)
	}
}

func TestFolder_AddsMissingTrailingNewline_Md(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "entry.md")
	if err := os.WriteFile(path, []byte("---\nnotion-id: abc\n---\nBody without newline"), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.NewlinesFixed != 1 {
		t.Errorf("NewlinesFixed = %d, want 1", r.NewlinesFixed)
	}

	data, _ := os.ReadFile(path)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Errorf("file still missing trailing newline")
	}
}

func TestFolder_LeavesExistingTrailingNewlineAlone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "entry.md")
	original := "---\nnotion-id: abc\n---\nBody with newline\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.NewlinesFixed != 0 {
		t.Errorf("NewlinesFixed = %d, want 0", r.NewlinesFixed)
	}

	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Errorf("file content changed unexpectedly")
	}
}

func TestFolder_AddsMissingTrailingNewline_Json(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_database.json")
	if err := os.WriteFile(path, []byte(`{"databaseId":"abc"}`), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.NewlinesFixed != 1 {
		t.Errorf("NewlinesFixed = %d, want 1", r.NewlinesFixed)
	}

	data, _ := os.ReadFile(path)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Errorf("json file still missing trailing newline")
	}
}

func TestFolder_DryRunDoesNotFixNewlines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "entry.md")
	original := "---\nnotion-id: abc\n---\nNo newline"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.NewlinesFixed != 1 {
		t.Errorf("dry-run NewlinesFixed = %d, want 1", r.NewlinesFixed)
	}

	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Errorf("dry-run modified file")
	}
}

func TestFolder_SkipsNonMarkdownNonJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "data.csv"), []byte("a,b,c"), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.FilesScanned != 0 || r.FilesChanged != 0 {
		t.Errorf("csv file should not be touched, got %+v", r)
	}
}

func TestFolder_JSONURLsNotStripped(t *testing.T) {
	dir := t.TempDir()
	content := `{"foo": "https://prod-files-secure.s3.us-west-2.amazonaws.com/x?X-Amz-Signature=abc"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "_database.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.URLsStripped != 0 {
		t.Errorf("URLs should not be stripped from JSON files, got URLsStripped=%d", r.URLsStripped)
	}
	if r.FilesChanged != 0 {
		t.Errorf("file already has trailing newline, should be unchanged, got FilesChanged=%d", r.FilesChanged)
	}
}

func TestFolder_StripsFrozenAtLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "entry.md")
	original := `---
notion-id: abc
notion-url: https://notion.so/abc
notion-frozen-at: "2024-01-01T00:00:00Z"
notion-last-edited: "2024-01-02T00:00:00Z"
notion-database-id: db-1
---

body
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.FrozenAtStripped != 1 {
		t.Errorf("FrozenAtStripped = %d, want 1", r.FrozenAtStripped)
	}
	if r.FilesChanged != 1 {
		t.Errorf("FilesChanged = %d, want 1", r.FilesChanged)
	}

	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "notion-frozen-at") {
		t.Errorf("file still contains notion-frozen-at:\n%s", got)
	}
	for _, keep := range []string{"notion-id: abc", "notion-last-edited:", "notion-database-id: db-1", "body"} {
		if !strings.Contains(string(got), keep) {
			t.Errorf("expected %q to remain in file:\n%s", keep, got)
		}
	}
}

func TestFolder_StripsFrozenAtLine_DryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "entry.md")
	original := `---
notion-id: abc
notion-frozen-at: "2024-01-01T00:00:00Z"
notion-last-edited: "2024-01-02T00:00:00Z"
---

body
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.FrozenAtStripped != 1 {
		t.Errorf("dry-run FrozenAtStripped = %d, want 1", r.FrozenAtStripped)
	}

	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Errorf("dry-run mutated file:\n%s", got)
	}
}

func TestFolder_StripsFrozenAtLine_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "entry.md")
	cleanContent := `---
notion-id: abc
notion-last-edited: "2024-01-02T00:00:00Z"
---

body
`
	if err := os.WriteFile(path, []byte(cleanContent), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.FrozenAtStripped != 0 {
		t.Errorf("FrozenAtStripped = %d, want 0 (already clean)", r.FrozenAtStripped)
	}

	got, _ := os.ReadFile(path)
	if string(got) != cleanContent {
		t.Errorf("idempotent run mutated content:\n%s", got)
	}
}

func TestFolder_StripsFrozenAtLine_OnlyInFrontmatter(t *testing.T) {
	// A body line that happens to contain "notion-frozen-at:" must NOT be touched.
	dir := t.TempDir()
	path := filepath.Join(dir, "entry.md")
	original := `---
notion-id: abc
---

This page documents the deprecated notion-frozen-at: "2024-01-01" key.
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), `deprecated notion-frozen-at: "2024-01-01"`) {
		t.Errorf("body line was modified — should only strip from frontmatter:\n%s", got)
	}
}

func TestFolder_CanonicalizesNotionURLInFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "entry.md")
	original := `---
notion-id: 1234567890abcdef1234567890abcdef
notion-url: "https://www.notion.so/Title-1234567890abcdef1234567890abcdef"
notion-last-edited: "2024-01-02T00:00:00Z"
---

body
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.URLsCanonicalized != 1 {
		t.Errorf("URLsCanonicalized = %d, want 1", r.URLsCanonicalized)
	}
	if r.FilesChanged != 1 {
		t.Errorf("FilesChanged = %d, want 1", r.FilesChanged)
	}

	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "https://app.notion.com/p/1234567890abcdef1234567890abcdef") {
		t.Errorf("file missing canonical URL:\n%s", got)
	}
	if strings.Contains(string(got), "https://www.notion.so/") {
		t.Errorf("file still contains legacy notion.so URL:\n%s", got)
	}
}

func TestFolder_CanonicalizesURLInDatabaseJSON(t *testing.T) {
	prev := sync.Version
	sync.Version = "v0.99.0-test"
	t.Cleanup(func() { sync.Version = prev })

	dir := t.TempDir()
	dbJSON := `{
  "databaseId": "db-1",
  "title": "Test",
  "url": "https://www.notion.so/Title-1234567890abcdef1234567890abcdef",
  "folderPath": "/tmp",
  "lastSyncedAt": "2024-01-01T00:00:00Z",
  "entryCount": 1,
  "syncVersion": "v0.5.0"
}
`
	dbPath := filepath.Join(dir, "_database.json")
	if err := os.WriteFile(dbPath, []byte(dbJSON), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.URLsCanonicalized != 1 {
		t.Errorf("URLsCanonicalized = %d, want 1", r.URLsCanonicalized)
	}

	got, _ := os.ReadFile(dbPath)
	if !strings.Contains(string(got), `"url": "https://app.notion.com/p/1234567890abcdef1234567890abcdef"`) {
		t.Errorf("_database.json missing canonical url:\n%s", got)
	}
	if strings.Contains(string(got), "https://www.notion.so/") {
		t.Errorf("_database.json still contains legacy notion.so URL:\n%s", got)
	}
}

func TestFolder_BumpsMetadata_OnURLOnlyCanonicalization(t *testing.T) {
	// Folder where the ONLY thing wrong is a non-canonical URL in _database.json.
	// No .md changes, no presigned URLs, no frozen-at lines. The clean step
	// must still mark the folder dirty and bump syncVersion + canonicalize URL
	// in _database.json — otherwise legacy URLs in metadata files would silently
	// survive every clean run.
	prev := sync.Version
	sync.Version = "v0.99.0-test"
	t.Cleanup(func() { sync.Version = prev })

	dir := t.TempDir()
	dbJSON := `{
  "databaseId": "db-1",
  "title": "Test",
  "url": "https://www.notion.so/Title-1234567890abcdef1234567890abcdef",
  "folderPath": "/tmp",
  "lastSyncedAt": "2024-01-01T00:00:00Z",
  "entryCount": 1,
  "syncVersion": "v0.5.0"
}
`
	dbPath := filepath.Join(dir, "_database.json")
	if err := os.WriteFile(dbPath, []byte(dbJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cleanMd := `---
notion-id: 1234567890abcdef1234567890abcdef
notion-url: "https://app.notion.com/p/1234567890abcdef1234567890abcdef"
notion-last-edited: "2024-01-02T00:00:00Z"
---

body
`
	if err := os.WriteFile(filepath.Join(dir, "entry.md"), []byte(cleanMd), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.URLsCanonicalized != 1 {
		t.Errorf("URLsCanonicalized = %d, want 1", r.URLsCanonicalized)
	}
	if r.MetadataBumped != 1 {
		t.Errorf("MetadataBumped = %d, want 1 (URL-only change must trigger bump)", r.MetadataBumped)
	}

	got, _ := os.ReadFile(dbPath)
	if !strings.Contains(string(got), `"url": "https://app.notion.com/p/1234567890abcdef1234567890abcdef"`) {
		t.Errorf("_database.json url not canonicalized:\n%s", got)
	}
	if !strings.Contains(string(got), `"syncVersion": "v0.99.0-test"`) {
		t.Errorf("syncVersion not bumped:\n%s", got)
	}
}

func TestFolder_CanonicalizeURL_DryRun(t *testing.T) {
	prev := sync.Version
	sync.Version = "v0.99.0-test"
	t.Cleanup(func() { sync.Version = prev })

	dir := t.TempDir()
	mdPath := filepath.Join(dir, "entry.md")
	mdOriginal := `---
notion-id: 1234567890abcdef1234567890abcdef
notion-url: "https://www.notion.so/Title-1234567890abcdef1234567890abcdef"
notion-last-edited: "2024-01-02T00:00:00Z"
---

body
`
	if err := os.WriteFile(mdPath, []byte(mdOriginal), 0644); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "_database.json")
	dbOriginal := `{
  "databaseId": "db-1",
  "url": "https://www.notion.so/Title-1234567890abcdef1234567890abcdef",
  "syncVersion": "v0.5.0"
}
`
	if err := os.WriteFile(dbPath, []byte(dbOriginal), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.URLsCanonicalized != 2 {
		t.Errorf("dry-run URLsCanonicalized = %d, want 2 (1 .md + 1 .json)", r.URLsCanonicalized)
	}

	gotMd, _ := os.ReadFile(mdPath)
	if string(gotMd) != mdOriginal {
		t.Errorf("dry-run mutated .md:\n%s", gotMd)
	}
	gotDb, _ := os.ReadFile(dbPath)
	if string(gotDb) != dbOriginal {
		t.Errorf("dry-run mutated _database.json:\n%s", gotDb)
	}
}

func TestFolder_CanonicalizeURL_OnlyInFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "entry.md")
	original := `---
notion-id: 1234567890abcdef1234567890abcdef
notion-last-edited: "2024-01-02T00:00:00Z"
---

This page documents the legacy notion-url: "https://www.notion.so/Title-1234567890abcdef1234567890abcdef" key from before normalization.
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.URLsCanonicalized != 0 {
		t.Errorf("URLsCanonicalized = %d, want 0 (body should not be touched)", r.URLsCanonicalized)
	}

	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), `legacy notion-url: "https://www.notion.so/Title-1234567890abcdef1234567890abcdef" key`) {
		t.Errorf("body line was modified — should only canonicalize in frontmatter:\n%s", got)
	}
}

func TestFolder_CanonicalizeURL_Idempotent(t *testing.T) {
	prev := sync.Version
	sync.Version = "v0.99.0-test"
	t.Cleanup(func() { sync.Version = prev })

	dir := t.TempDir()
	mdPath := filepath.Join(dir, "entry.md")
	mdContent := `---
notion-id: 1234567890abcdef1234567890abcdef
notion-url: "https://app.notion.com/p/1234567890abcdef1234567890abcdef"
notion-last-edited: "2024-01-02T00:00:00Z"
---

body
`
	if err := os.WriteFile(mdPath, []byte(mdContent), 0644); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "_database.json")
	dbContent := `{
  "databaseId": "db-1",
  "title": "Test",
  "url": "https://app.notion.com/p/1234567890abcdef1234567890abcdef",
  "folderPath": "/tmp",
  "lastSyncedAt": "2024-01-01T00:00:00Z",
  "entryCount": 1,
  "syncVersion": "v0.99.0-test"
}
`
	if err := os.WriteFile(dbPath, []byte(dbContent), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.URLsCanonicalized != 0 {
		t.Errorf("URLsCanonicalized = %d, want 0 (already canonical)", r.URLsCanonicalized)
	}
	if r.FilesChanged != 0 {
		t.Errorf("FilesChanged = %d, want 0", r.FilesChanged)
	}
	if r.MetadataBumped != 0 {
		t.Errorf("MetadataBumped = %d, want 0 (no changes ⇒ no bump)", r.MetadataBumped)
	}

	gotMd, _ := os.ReadFile(mdPath)
	if string(gotMd) != mdContent {
		t.Errorf(".md was mutated:\n%s", gotMd)
	}
	gotDb, _ := os.ReadFile(dbPath)
	if string(gotDb) != dbContent {
		t.Errorf("_database.json was mutated:\n%s", gotDb)
	}
}

func TestFolder_CanonicalizesURLInPageJSON(t *testing.T) {
	prev := sync.Version
	sync.Version = "v0.99.0-test"
	t.Cleanup(func() { sync.Version = prev })

	root := t.TempDir()
	pageDir := filepath.Join(root, "pages", "MyPage_abc12345")
	if err := os.MkdirAll(pageDir, 0755); err != nil {
		t.Fatal(err)
	}
	pageJSON := `{
  "pageId": "page-1",
  "title": "MyPage",
  "url": "https://www.notion.so/Title-1234567890abcdef1234567890abcdef",
  "folderPath": "/tmp",
  "lastSyncedAt": "2024-01-01T00:00:00Z",
  "syncVersion": "v0.5.0"
}
`
	pagePath := filepath.Join(pageDir, "_page.json")
	if err := os.WriteFile(pagePath, []byte(pageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.URLsCanonicalized != 1 {
		t.Errorf("URLsCanonicalized = %d, want 1", r.URLsCanonicalized)
	}

	got, _ := os.ReadFile(pagePath)
	if !strings.Contains(string(got), `"url": "https://app.notion.com/p/1234567890abcdef1234567890abcdef"`) {
		t.Errorf("_page.json missing canonical url:\n%s", got)
	}
}

func TestFolder_BumpsSyncVersionInDirtyDatabaseFolder(t *testing.T) {
	prev := sync.Version
	sync.Version = "v0.99.0-test"
	t.Cleanup(func() { sync.Version = prev })

	dir := t.TempDir()

	dbJSON := `{
  "databaseId": "db-1",
  "title": "Test",
  "url": "https://notion.so/db-1",
  "folderPath": "/tmp",
  "lastSyncedAt": "2024-01-01T00:00:00Z",
  "entryCount": 1,
  "syncVersion": "v0.5.0"
}
`
	if err := os.WriteFile(filepath.Join(dir, "_database.json"), []byte(dbJSON), 0644); err != nil {
		t.Fatal(err)
	}

	md := `---
notion-id: abc
notion-frozen-at: "2024-01-01T00:00:00Z"
notion-last-edited: "2024-01-02T00:00:00Z"
---

body
`
	if err := os.WriteFile(filepath.Join(dir, "abc.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.MetadataBumped != 1 {
		t.Errorf("MetadataBumped = %d, want 1", r.MetadataBumped)
	}

	got, err := os.ReadFile(filepath.Join(dir, "_database.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"syncVersion": "v0.99.0-test"`) {
		t.Errorf("syncVersion not bumped:\n%s", got)
	}
	if !strings.Contains(string(got), `"databaseId": "db-1"`) {
		t.Errorf("databaseId field lost:\n%s", got)
	}
	if !strings.Contains(string(got), `"entryCount": 1`) {
		t.Errorf("entryCount field lost:\n%s", got)
	}
}

func TestFolder_SkipsMetadataBumpWhenVersionUnset(t *testing.T) {
	// If sync.Version is empty (misconfigured caller), bumping would rewrite
	// the file without a stamp while still claiming MetadataBumped. Skip both.
	prev := sync.Version
	sync.Version = ""
	t.Cleanup(func() { sync.Version = prev })

	dir := t.TempDir()

	dbJSON := `{
  "databaseId": "db-1",
  "title": "Test",
  "url": "https://notion.so/db-1",
  "folderPath": "/tmp",
  "lastSyncedAt": "2024-01-01T00:00:00Z",
  "entryCount": 1,
  "syncVersion": "v0.5.0"
}
`
	dbPath := filepath.Join(dir, "_database.json")
	if err := os.WriteFile(dbPath, []byte(dbJSON), 0644); err != nil {
		t.Fatal(err)
	}

	md := `---
notion-id: abc
notion-frozen-at: "2024-01-01T00:00:00Z"
---

body
`
	if err := os.WriteFile(filepath.Join(dir, "abc.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.MetadataBumped != 0 {
		t.Errorf("MetadataBumped = %d, want 0 when sync.Version is empty", r.MetadataBumped)
	}

	got, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != dbJSON {
		t.Errorf("_database.json was rewritten despite empty sync.Version:\n%s", got)
	}

	// Dry-run path should also stay silent.
	r2, err := Folder(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if r2.MetadataBumped != 0 {
		t.Errorf("dry-run MetadataBumped = %d, want 0 when sync.Version is empty", r2.MetadataBumped)
	}
}

func TestFolder_LeavesCleanFolderMetadataUntouched(t *testing.T) {
	prev := sync.Version
	sync.Version = "v0.99.0-test"
	t.Cleanup(func() { sync.Version = prev })

	dir := t.TempDir()

	dbJSON := `{
  "databaseId": "db-1",
  "title": "Test",
  "url": "https://notion.so/db-1",
  "folderPath": "/tmp",
  "lastSyncedAt": "2024-01-01T00:00:00Z",
  "entryCount": 1,
  "syncVersion": "v0.5.0"
}
`
	dbPath := filepath.Join(dir, "_database.json")
	if err := os.WriteFile(dbPath, []byte(dbJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// .md file with no notion-frozen-at, no presigned URLs, trailing newline already present.
	md := `---
notion-id: abc
notion-last-edited: "2024-01-02T00:00:00Z"
---

body
`
	if err := os.WriteFile(filepath.Join(dir, "abc.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.FilesChanged != 0 {
		t.Errorf("FilesChanged = %d, want 0 (folder was already clean)", r.FilesChanged)
	}
	if r.MetadataBumped != 0 {
		t.Errorf("MetadataBumped = %d, want 0 (no .md changes ⇒ no metadata bump)", r.MetadataBumped)
	}

	got, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != dbJSON {
		t.Errorf("_database.json was rewritten despite folder being clean:\n%s", got)
	}
}

func TestFolder_BumpsSyncVersionInDirtyStandalonePageFolder(t *testing.T) {
	prev := sync.Version
	sync.Version = "v0.99.0-test"
	t.Cleanup(func() { sync.Version = prev })

	root := t.TempDir()
	pageDir := filepath.Join(root, "pages", "MyPage_abc12345")
	if err := os.MkdirAll(pageDir, 0755); err != nil {
		t.Fatal(err)
	}

	pageJSON := `{
  "pageId": "page-1",
  "title": "MyPage",
  "url": "https://notion.so/page-1",
  "folderPath": "/tmp",
  "lastSyncedAt": "2024-01-01T00:00:00Z",
  "syncVersion": "v0.5.0"
}
`
	if err := os.WriteFile(filepath.Join(pageDir, "_page.json"), []byte(pageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	md := `---
notion-id: page-1
notion-frozen-at: "2024-01-01T00:00:00Z"
---

body
`
	if err := os.WriteFile(filepath.Join(pageDir, "MyPage.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.MetadataBumped != 1 {
		t.Errorf("MetadataBumped = %d, want 1", r.MetadataBumped)
	}

	got, err := os.ReadFile(filepath.Join(pageDir, "_page.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"syncVersion": "v0.99.0-test"`) {
		t.Errorf("syncVersion not bumped in _page.json:\n%s", got)
	}
	if !strings.Contains(string(got), `"pageId": "page-1"`) {
		t.Errorf("pageId field lost:\n%s", got)
	}
}

func TestFolder_SurfacesCorruptMetadataReadError(t *testing.T) {
	// A corrupt _database.json (exists but malformed) used to be silently
	// swallowed by bumpFolderMetadata, leaving the user with no signal that
	// their workspace is broken. The clean walk must now surface the read
	// error instead of returning success.
	prev := sync.Version
	sync.Version = "v0.99.0-test"
	t.Cleanup(func() { sync.Version = prev })

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "_database.json"), []byte("{ this is not valid json"), 0644); err != nil {
		t.Fatal(err)
	}
	// A .md file with notion-frozen-at so the folder gets marked dirty and
	// bumpFolderMetadata is reached.
	md := `---
notion-id: abc
notion-frozen-at: "2024-01-01T00:00:00Z"
---

body
`
	if err := os.WriteFile(filepath.Join(dir, "abc.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Folder(dir, false)
	if err == nil {
		t.Fatal("expected error from corrupt _database.json, got nil")
	}
	if !strings.Contains(err.Error(), "_database.json") {
		t.Errorf("error should name the broken file, got: %v", err)
	}
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("error should include the folder path, got: %v", err)
	}
}

func TestCountNonCanonicalFolderPathInJSON(t *testing.T) {
	// `\\` in a Go raw string is two literal backslash characters, which is
	// exactly how JSON encodes a single backslash on disk.
	tests := []struct {
		name string
		json string
		want int
	}{
		{
			name: "windows separator",
			json: `{"folderPath": "_etl\\notion-sync\\v1\\Foo"}`,
			want: 1,
		},
		{
			name: "forward slash",
			json: `{"folderPath": "_etl/notion-sync/v1/Foo"}`,
			want: 0,
		},
		{
			name: "missing field",
			json: `{"databaseId": "abc"}`,
			want: 0,
		},
		{
			name: "empty value",
			json: `{"folderPath": ""}`,
			want: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := countNonCanonicalFolderPathInJSON(tc.json)
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFolder_NormalizesFolderPathInDatabaseJSON(t *testing.T) {
	prev := sync.Version
	sync.Version = "v0.99.0-test"
	t.Cleanup(func() { sync.Version = prev })

	dir := t.TempDir()
	dbJSON := `{
  "databaseId": "db-1",
  "title": "Test",
  "url": "https://app.notion.com/p/1234567890abcdef1234567890abcdef",
  "folderPath": "_etl\\notion-sync\\v1\\Foo",
  "lastSyncedAt": "2024-01-01T00:00:00Z",
  "entryCount": 1,
  "syncVersion": "v0.5.0"
}
`
	dbPath := filepath.Join(dir, "_database.json")
	if err := os.WriteFile(dbPath, []byte(dbJSON), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.FolderPathsNormalized != 1 {
		t.Errorf("FolderPathsNormalized = %d, want 1", r.FolderPathsNormalized)
	}
	if r.MetadataBumped != 1 {
		t.Errorf("MetadataBumped = %d, want 1 (folderPath-only change must trigger bump)", r.MetadataBumped)
	}

	got, _ := os.ReadFile(dbPath)
	if !strings.Contains(string(got), `"folderPath": "_etl/notion-sync/v1/Foo"`) {
		t.Errorf("_database.json folderPath not normalized:\n%s", got)
	}
	if strings.Contains(string(got), `\\`) {
		t.Errorf("_database.json still contains backslashes:\n%s", got)
	}
}

func TestFolder_NormalizesFolderPathInPageJSON(t *testing.T) {
	prev := sync.Version
	sync.Version = "v0.99.0-test"
	t.Cleanup(func() { sync.Version = prev })

	root := t.TempDir()
	pageDir := filepath.Join(root, "pages", "MyPage_abc12345")
	if err := os.MkdirAll(pageDir, 0755); err != nil {
		t.Fatal(err)
	}
	pageJSON := `{
  "pageId": "page-1",
  "title": "MyPage",
  "url": "https://app.notion.com/p/1234567890abcdef1234567890abcdef",
  "folderPath": "pages\\MyPage_abc12345",
  "lastSyncedAt": "2024-01-01T00:00:00Z",
  "syncVersion": "v0.5.0"
}
`
	pagePath := filepath.Join(pageDir, "_page.json")
	if err := os.WriteFile(pagePath, []byte(pageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.FolderPathsNormalized != 1 {
		t.Errorf("FolderPathsNormalized = %d, want 1", r.FolderPathsNormalized)
	}

	got, _ := os.ReadFile(pagePath)
	if !strings.Contains(string(got), `"folderPath": "pages/MyPage_abc12345"`) {
		t.Errorf("_page.json folderPath not normalized:\n%s", got)
	}
}

func TestFolder_NormalizeFolderPath_DryRun(t *testing.T) {
	prev := sync.Version
	sync.Version = "v0.99.0-test"
	t.Cleanup(func() { sync.Version = prev })

	dir := t.TempDir()
	dbOriginal := `{
  "databaseId": "db-1",
  "url": "https://app.notion.com/p/1234567890abcdef1234567890abcdef",
  "folderPath": "_etl\\notion-sync\\v1\\Foo",
  "syncVersion": "v0.5.0"
}
`
	dbPath := filepath.Join(dir, "_database.json")
	if err := os.WriteFile(dbPath, []byte(dbOriginal), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.FolderPathsNormalized != 1 {
		t.Errorf("dry-run FolderPathsNormalized = %d, want 1", r.FolderPathsNormalized)
	}

	got, _ := os.ReadFile(dbPath)
	if string(got) != dbOriginal {
		t.Errorf("dry-run mutated _database.json:\n%s", got)
	}
}

func TestFolder_NormalizeFolderPath_Idempotent(t *testing.T) {
	prev := sync.Version
	sync.Version = "v0.99.0-test"
	t.Cleanup(func() { sync.Version = prev })

	dir := t.TempDir()
	dbContent := `{
  "databaseId": "db-1",
  "title": "Test",
  "url": "https://app.notion.com/p/1234567890abcdef1234567890abcdef",
  "folderPath": "_etl/notion-sync/v1/Foo",
  "lastSyncedAt": "2024-01-01T00:00:00Z",
  "entryCount": 1,
  "syncVersion": "v0.99.0-test"
}
`
	dbPath := filepath.Join(dir, "_database.json")
	if err := os.WriteFile(dbPath, []byte(dbContent), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.FolderPathsNormalized != 0 {
		t.Errorf("FolderPathsNormalized = %d, want 0 (already canonical)", r.FolderPathsNormalized)
	}
	if r.MetadataBumped != 0 {
		t.Errorf("MetadataBumped = %d, want 0 (no changes ⇒ no bump)", r.MetadataBumped)
	}

	got, _ := os.ReadFile(dbPath)
	if string(got) != dbContent {
		t.Errorf("_database.json was mutated:\n%s", got)
	}
}

func TestFolder_RegeneratesStaleAgentsMD(t *testing.T) {
	prev := sync.Version
	sync.Version = "v9.9.9-test"
	t.Cleanup(func() { sync.Version = prev })

	dir := t.TempDir()
	stale := "<!-- notion-sync-version: v0.0.1 -->\n# stale agents doc\n"
	dest := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(dest, []byte(stale), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.AgentsMDWritten != 1 {
		t.Errorf("AgentsMDWritten = %d, want 1", r.AgentsMDWritten)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "<!-- notion-sync-version: v9.9.9-test -->") {
		t.Errorf("AGENTS.md not regenerated:\n%s", got)
	}
}

func TestFolder_LeavesCurrentAgentsMDAlone(t *testing.T) {
	prev := sync.Version
	sync.Version = "v9.9.9-test"
	t.Cleanup(func() { sync.Version = prev })

	dir := t.TempDir()
	current := "<!-- notion-sync-version: v9.9.9-test -->\n# current agents doc, possibly user-edited\n"
	dest := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(dest, []byte(current), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.AgentsMDWritten != 0 {
		t.Errorf("AgentsMDWritten = %d, want 0 (stamp matches)", r.AgentsMDWritten)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != current {
		t.Errorf("AGENTS.md was overwritten despite matching stamp")
	}
}

func TestFolder_RecursesIntoSubfolders(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{filepath.Join(dir, "a.md"), filepath.Join(sub, "b.md")} {
		if err := os.WriteFile(p,
			[]byte(`![x](https://prod-files-secure.s3.us-west-2.amazonaws.com/p/file.png?X-Amz-Signature=xyz)`),
			0644); err != nil {
			t.Fatal(err)
		}
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.FilesScanned != 2 || r.FilesChanged != 2 {
		t.Errorf("expected 2/2, got %+v", r)
	}
}
