package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanLocalFiles(t *testing.T) {
	dir := t.TempDir()

	// Write a markdown file with frontmatter
	content := "---\nnotion-id: abc-123\nnotion-last-edited: \"2025-01-01T00:00:00Z\"\n---\nHello world\n"
	os.WriteFile(filepath.Join(dir, "test.md"), []byte(content), 0644)

	// Write a file without frontmatter (should be skipped)
	os.WriteFile(filepath.Join(dir, "plain.md"), []byte("no frontmatter"), 0644)

	// Write a non-md file (should be skipped)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not markdown"), 0644)

	files, err := scanLocalFiles(dir)
	if err != nil {
		t.Fatalf("scanLocalFiles: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}

	info, ok := files["abc-123"]
	if !ok {
		t.Fatal("missing entry for abc-123")
	}
	if info.lastEdited != "2025-01-01T00:00:00Z" {
		t.Errorf("lastEdited = %q, want %q", info.lastEdited, "2025-01-01T00:00:00Z")
	}
	if filepath.Base(info.filePath) != "test.md" {
		t.Errorf("filePath = %q, want test.md", info.filePath)
	}
}

func TestScanLocalFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	files, err := scanLocalFiles(dir)
	if err != nil {
		t.Fatalf("scanLocalFiles: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty map, got %d entries", len(files))
	}
}

func TestScanLocalFiles_NonexistentDir(t *testing.T) {
	files, err := scanLocalFiles(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty map, got %d entries", len(files))
	}
}

func TestScanLocalFiles_MissingNotionID(t *testing.T) {
	dir := t.TempDir()

	// File with frontmatter but no notion-id
	content := "---\ntitle: Test\n---\nBody\n"
	os.WriteFile(filepath.Join(dir, "no-id.md"), []byte(content), 0644)

	files, err := scanLocalFiles(dir)
	if err != nil {
		t.Fatalf("scanLocalFiles: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty map for file without notion-id, got %d", len(files))
	}
}

func TestMarkAsDeleted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	content := "---\nnotion-id: abc-123\nnotion-last-edited: \"2025-01-01T00:00:00Z\"\n---\nBody content\n"
	os.WriteFile(path, []byte(content), 0644)

	if err := markAsDeleted(path); err != nil {
		t.Fatalf("markAsDeleted: %v", err)
	}

	data, _ := os.ReadFile(path)
	got := string(data)

	if !contains(got, "notion-deleted: true") {
		t.Errorf("expected notion-deleted: true in output, got:\n%s", got)
	}
	if !contains(got, "notion-id: abc-123") {
		t.Error("original frontmatter should be preserved")
	}
}

func TestMarkAsDeleted_AlreadyMarked(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	content := "---\nnotion-id: abc-123\nnotion-deleted: true\n---\nBody\n"
	os.WriteFile(path, []byte(content), 0644)

	if err := markAsDeleted(path); err != nil {
		t.Fatalf("markAsDeleted: %v", err)
	}

	data, _ := os.ReadFile(path)
	// Should not add a second notion-deleted
	if string(data) != content {
		t.Errorf("file should not be modified when already deleted, got:\n%s", string(data))
	}
}

func TestMarkAsDeleted_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	content := "Just some content with no frontmatter\n"
	os.WriteFile(path, []byte(content), 0644)

	if err := markAsDeleted(path); err != nil {
		t.Fatalf("markAsDeleted: %v", err)
	}

	data, _ := os.ReadFile(path)
	got := string(data)

	if !contains(got, "---\nnotion-deleted: true\n---\n") {
		t.Errorf("expected new frontmatter to be added, got:\n%s", got)
	}
	if !contains(got, "Just some content") {
		t.Error("original content should be preserved")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
