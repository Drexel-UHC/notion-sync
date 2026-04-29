package clean

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestFolder_SkipsNonMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "_database.json"),
		[]byte(`{"foo": "https://prod-files-secure.s3.us-west-2.amazonaws.com/x?X-Amz-Signature=abc"}`),
		0644); err != nil {
		t.Fatal(err)
	}

	r, err := Folder(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.FilesScanned != 0 {
		t.Errorf("non-md file should not be scanned, got FilesScanned=%d", r.FilesScanned)
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
