package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadDatabaseMetadata(t *testing.T) {
	dir := t.TempDir()

	meta := &FrozenDatabase{
		DatabaseID:   "abc-123",
		Title:        "Test DB",
		URL:          "https://notion.so/test",
		FolderPath:   dir,
		LastSyncedAt: "2025-01-01T00:00:00Z",
		EntryCount:   42,
	}

	if err := WriteDatabaseMetadata(dir, meta); err != nil {
		t.Fatalf("WriteDatabaseMetadata: %v", err)
	}

	got, err := ReadDatabaseMetadata(dir)
	if err != nil {
		t.Fatalf("ReadDatabaseMetadata: %v", err)
	}
	if got == nil {
		t.Fatal("ReadDatabaseMetadata returned nil")
	}

	if got.DatabaseID != meta.DatabaseID {
		t.Errorf("DatabaseID = %q, want %q", got.DatabaseID, meta.DatabaseID)
	}
	if got.Title != meta.Title {
		t.Errorf("Title = %q, want %q", got.Title, meta.Title)
	}
	if got.URL != meta.URL {
		t.Errorf("URL = %q, want %q", got.URL, meta.URL)
	}
	if got.EntryCount != meta.EntryCount {
		t.Errorf("EntryCount = %d, want %d", got.EntryCount, meta.EntryCount)
	}
	if got.LastSyncedAt != meta.LastSyncedAt {
		t.Errorf("LastSyncedAt = %q, want %q", got.LastSyncedAt, meta.LastSyncedAt)
	}
}

func TestReadDatabaseMetadata_NoFile(t *testing.T) {
	dir := t.TempDir()

	got, err := ReadDatabaseMetadata(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestReadDatabaseMetadata_NonexistentDir(t *testing.T) {
	got, err := ReadDatabaseMetadata(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestListSyncedDatabases(t *testing.T) {
	root := t.TempDir()

	// Create two database folders
	for _, name := range []string{"db-one", "db-two"} {
		dir := filepath.Join(root, name)
		os.MkdirAll(dir, 0755)
		WriteDatabaseMetadata(dir, &FrozenDatabase{
			DatabaseID: name,
			Title:      name,
			EntryCount: 1,
		})
	}

	// Create a folder without metadata (should be skipped)
	os.MkdirAll(filepath.Join(root, "not-a-db"), 0755)

	databases, err := ListSyncedDatabases(root)
	if err != nil {
		t.Fatalf("ListSyncedDatabases: %v", err)
	}
	if len(databases) != 2 {
		t.Fatalf("got %d databases, want 2", len(databases))
	}

	ids := map[string]bool{}
	for _, db := range databases {
		ids[db.DatabaseID] = true
	}
	if !ids["db-one"] || !ids["db-two"] {
		t.Errorf("missing expected databases: got %v", ids)
	}
}

func TestWriteDatabaseMetadata_TrailingNewline(t *testing.T) {
	dir := t.TempDir()
	meta := &FrozenDatabase{DatabaseID: "abc", Title: "T", EntryCount: 1}
	if err := WriteDatabaseMetadata(dir, meta); err != nil {
		t.Fatalf("WriteDatabaseMetadata: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, DatabaseMetadataFile))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Errorf("file does not end with newline; last byte = %q", data[len(data)-1])
	}
}

func TestWriteDatabaseMetadata_CanonicalizesURL(t *testing.T) {
	dir := t.TempDir()
	meta := &FrozenDatabase{
		DatabaseID: "abc",
		Title:      "T",
		URL:        "https://www.notion.so/Title-1234567890abcdef1234567890abcdef",
		EntryCount: 1,
	}
	if err := WriteDatabaseMetadata(dir, meta); err != nil {
		t.Fatalf("WriteDatabaseMetadata: %v", err)
	}
	got, err := ReadDatabaseMetadata(dir)
	if err != nil {
		t.Fatalf("ReadDatabaseMetadata: %v", err)
	}
	want := "https://app.notion.com/p/1234567890abcdef1234567890abcdef"
	if got.URL != want {
		t.Errorf("URL = %q, want %q", got.URL, want)
	}
}

func TestWritePageMetadata_CanonicalizesURL(t *testing.T) {
	dir := t.TempDir()
	meta := &FrozenPage{
		PageID: "abc",
		Title:  "T",
		URL:    "https://www.notion.so/Title-1234567890abcdef1234567890abcdef",
	}
	if err := WritePageMetadata(dir, meta); err != nil {
		t.Fatalf("WritePageMetadata: %v", err)
	}
	got, err := ReadPageMetadata(dir)
	if err != nil {
		t.Fatalf("ReadPageMetadata: %v", err)
	}
	want := "https://app.notion.com/p/1234567890abcdef1234567890abcdef"
	if got.URL != want {
		t.Errorf("URL = %q, want %q", got.URL, want)
	}
}

func TestWriteDatabaseMetadata_NormalizesFolderPathSeparator(t *testing.T) {
	dir := t.TempDir()
	meta := &FrozenDatabase{
		DatabaseID: "abc",
		Title:      "T",
		FolderPath: `_etl\notion-sync\v1\Foo`,
		EntryCount: 1,
	}
	if err := WriteDatabaseMetadata(dir, meta); err != nil {
		t.Fatalf("WriteDatabaseMetadata: %v", err)
	}
	got, err := ReadDatabaseMetadata(dir)
	if err != nil {
		t.Fatalf("ReadDatabaseMetadata: %v", err)
	}
	want := "_etl/notion-sync/v1/Foo"
	if got.FolderPath != want {
		t.Errorf("FolderPath = %q, want %q", got.FolderPath, want)
	}
}

func TestWritePageMetadata_NormalizesFolderPathSeparator(t *testing.T) {
	dir := t.TempDir()
	meta := &FrozenPage{
		PageID:     "abc",
		Title:      "T",
		FolderPath: `pages\MyPage_abc`,
	}
	if err := WritePageMetadata(dir, meta); err != nil {
		t.Fatalf("WritePageMetadata: %v", err)
	}
	got, err := ReadPageMetadata(dir)
	if err != nil {
		t.Fatalf("ReadPageMetadata: %v", err)
	}
	want := "pages/MyPage_abc"
	if got.FolderPath != want {
		t.Errorf("FolderPath = %q, want %q", got.FolderPath, want)
	}
}

func TestWritePageMetadata_TrailingNewline(t *testing.T) {
	dir := t.TempDir()
	meta := &FrozenPage{PageID: "abc", Title: "T"}
	if err := WritePageMetadata(dir, meta); err != nil {
		t.Fatalf("WritePageMetadata: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, PageMetadataFile))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Errorf("file does not end with newline; last byte = %q", data[len(data)-1])
	}
}

func TestListSyncedDatabases_NonexistentDir(t *testing.T) {
	databases, err := ListSyncedDatabases(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(databases) != 0 {
		t.Fatalf("expected empty slice, got %d", len(databases))
	}
}
