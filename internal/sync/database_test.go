package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ran-codes/notion-sync/internal/notion"
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

func TestTimestampsEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"identical strings", "2025-01-15T10:30:00Z", "2025-01-15T10:30:00Z", true},
		{".000Z vs Z", "2025-01-15T10:30:00.000Z", "2025-01-15T10:30:00Z", true},
		{"different times", "2025-01-15T10:30:00Z", "2025-01-15T11:00:00Z", false},
		{"both empty", "", "", true},
		{"one empty", "2025-01-15T10:30:00Z", "", false},
		{"unparseable a", "not-a-time", "2025-01-15T10:30:00Z", false},
		{"unparseable b", "2025-01-15T10:30:00Z", "not-a-time", false},
		{"both unparseable", "nope", "nada", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := timestampsEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("timestampsEqual(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestScanLocalFiles_MillisecondTimestamp(t *testing.T) {
	dir := t.TempDir()

	// Write a file with .000Z timestamp (what yaml.v3 might produce after normalization)
	content := "---\nnotion-id: ms-page\nnotion-last-edited: 2025-06-01T12:00:00.000Z\n---\nBody\n"
	os.WriteFile(filepath.Join(dir, "page.md"), []byte(content), 0644)

	files, err := scanLocalFiles(dir)
	if err != nil {
		t.Fatalf("scanLocalFiles: %v", err)
	}

	info, ok := files["ms-page"]
	if !ok {
		t.Fatal("missing entry for ms-page")
	}

	// After normalization, .000Z should become Z
	want := "2025-06-01T12:00:00Z"
	if info.lastEdited != want {
		t.Errorf("lastEdited = %q, want %q", info.lastEdited, want)
	}
}

func TestFindSubSourceFolders_Empty(t *testing.T) {
	dir := t.TempDir()
	folders := findSubSourceFolders(dir)
	if len(folders) != 0 {
		t.Errorf("expected 0 folders, got %d", len(folders))
	}
}

func TestFindSubSourceFolders_WithDataSourceID(t *testing.T) {
	dir := t.TempDir()

	// Subfolder with dataSourceId in metadata
	sub := filepath.Join(dir, "Projects")
	os.MkdirAll(sub, 0755)
	WriteDatabaseMetadata(sub, &FrozenDatabase{
		DatabaseID:   "db-1",
		DataSourceID: "ds-1",
		Title:        "Projects",
		FolderPath:   sub,
	})

	folders := findSubSourceFolders(dir)
	if len(folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(folders))
	}
	if folders[0] != sub {
		t.Errorf("expected %s, got %s", sub, folders[0])
	}
}

func TestFindSubSourceFolders_WithoutDataSourceID(t *testing.T) {
	dir := t.TempDir()

	// Subfolder with metadata but no dataSourceId
	sub := filepath.Join(dir, "Notes")
	os.MkdirAll(sub, 0755)
	WriteDatabaseMetadata(sub, &FrozenDatabase{
		DatabaseID: "db-1",
		Title:      "Notes",
		FolderPath: sub,
	})

	folders := findSubSourceFolders(dir)
	if len(folders) != 0 {
		t.Errorf("expected 0 folders (no dataSourceId), got %d", len(folders))
	}
}

func TestFindSubSourceFolders_Mixed(t *testing.T) {
	dir := t.TempDir()

	// Sub with dataSourceId
	sub1 := filepath.Join(dir, "Source1")
	os.MkdirAll(sub1, 0755)
	WriteDatabaseMetadata(sub1, &FrozenDatabase{
		DatabaseID:   "db-1",
		DataSourceID: "ds-1",
		Title:        "Source1",
		FolderPath:   sub1,
	})

	// Sub without dataSourceId
	sub2 := filepath.Join(dir, "Source2")
	os.MkdirAll(sub2, 0755)
	WriteDatabaseMetadata(sub2, &FrozenDatabase{
		DatabaseID: "db-1",
		Title:      "Source2",
		FolderPath: sub2,
	})

	// Regular file (not a directory)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("hi"), 0644)

	// Sub with no metadata at all
	os.MkdirAll(filepath.Join(dir, "Empty"), 0755)

	folders := findSubSourceFolders(dir)
	if len(folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(folders))
	}
	if folders[0] != sub1 {
		t.Errorf("expected %s, got %s", sub1, folders[0])
	}
}

func TestBuildFileNameMap_NoDuplicates(t *testing.T) {
	entries := []notion.Page{
		testPage("aaa-111", "Alpha", "2025-01-01T00:00:00Z"),
		testPage("bbb-222", "Beta", "2025-01-01T00:00:00Z"),
		testPage("ccc-333", "Gamma", "2025-01-01T00:00:00Z"),
	}
	m := buildFileNameMap(entries)
	if len(m) != 0 {
		t.Errorf("expected empty map for unique titles, got %d entries: %v", len(m), m)
	}
}

func TestBuildFileNameMap_TwoDuplicates(t *testing.T) {
	entries := []notion.Page{
		testPage("aaa-111", "Hello", "2025-01-01T00:00:00Z"),
		testPage("bbb-222", "Hello", "2025-01-01T00:00:00Z"),
		testPage("ccc-333", "World", "2025-01-01T00:00:00Z"),
	}
	m := buildFileNameMap(entries)

	// Both "Hello" pages should be in the map
	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(m), m)
	}
	if m["aaa-111"] != "Hello-aaa111" {
		t.Errorf("aaa-111 = %q, want Hello-aaa111", m["aaa-111"])
	}
	if m["bbb-222"] != "Hello-bbb222" {
		t.Errorf("bbb-222 = %q, want Hello-bbb222", m["bbb-222"])
	}
	// "World" should NOT be in the map
	if _, ok := m["ccc-333"]; ok {
		t.Error("ccc-333 (unique title) should not be in the map")
	}
}

func TestBuildFileNameMap_ThreeDuplicates(t *testing.T) {
	entries := []notion.Page{
		testPage("a1a1-b2b2", "Dup", "2025-01-01T00:00:00Z"),
		testPage("c3c3-d4d4", "Dup", "2025-01-01T00:00:00Z"),
		testPage("e5e5-f6f6", "Dup", "2025-01-01T00:00:00Z"),
	}
	m := buildFileNameMap(entries)
	if len(m) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(m), m)
	}
	for _, entry := range entries {
		cleanID := strings.ReplaceAll(entry.ID, "-", "")
		want := "Dup-" + cleanID
		if m[entry.ID] != want {
			t.Errorf("%s = %q, want %q", entry.ID, m[entry.ID], want)
		}
	}
}

func TestBuildFileNameMap_UntitledCollision(t *testing.T) {
	// Two pages with empty titles → both become "Untitled"
	p1 := testPage("id-1", "", "2025-01-01T00:00:00Z")
	p1.Properties = map[string]notion.Property{
		"Name": {Type: "title", Title: []notion.RichText{}},
	}
	p2 := testPage("id-2", "", "2025-01-01T00:00:00Z")
	p2.Properties = map[string]notion.Property{
		"Name": {Type: "title", Title: []notion.RichText{}},
	}
	entries := []notion.Page{p1, p2}
	m := buildFileNameMap(entries)
	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(m), m)
	}
	if m["id-1"] != "Untitled-id1" {
		t.Errorf("id-1 = %q, want Untitled-id1", m["id-1"])
	}
	if m["id-2"] != "Untitled-id2" {
		t.Errorf("id-2 = %q, want Untitled-id2", m["id-2"])
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
