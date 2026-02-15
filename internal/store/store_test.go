package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenStore_CreatesDB(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	dbPath := filepath.Join(dir, "_notion_sync.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("expected _notion_sync.db to exist")
	}
}

func TestOpenStore_SchemaVersion(t *testing.T) {
	s := setupTestStore(t)

	var version string
	err := s.db.QueryRow("SELECT value FROM _meta WHERE key = 'schema_version'").Scan(&version)
	if err != nil {
		t.Fatalf("query schema_version: %v", err)
	}
	if version != "1" {
		t.Fatalf("expected schema_version '1', got %q", version)
	}
}

func TestOpenStore_WALMode(t *testing.T) {
	s := setupTestStore(t)

	var mode string
	err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("expected journal_mode 'wal', got %q", mode)
	}
}

func TestOpenStore_RecursiveTriggers(t *testing.T) {
	s := setupTestStore(t)

	var enabled int
	err := s.db.QueryRow("PRAGMA recursive_triggers").Scan(&enabled)
	if err != nil {
		t.Fatalf("query recursive_triggers: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("expected recursive_triggers = 1, got %d", enabled)
	}
}

func TestUpsertPage_Insert(t *testing.T) {
	s := setupTestStore(t)

	data := PageData{
		ID:             "page-1",
		Title:          "Test Page",
		URL:            "https://notion.so/test",
		FilePath:       "/output/db/Test Page.md",
		BodyMarkdown:   "# Hello\n\nWorld",
		PropertiesJSON: `{"status":"active"}`,
		CreatedTime:    "2026-01-01T00:00:00Z",
		LastEditedTime: "2026-02-01T00:00:00Z",
		FrozenAt:       "2026-02-15T00:00:00Z",
		DatabaseID:     "db-1",
	}

	if err := s.UpsertPage(data); err != nil {
		t.Fatalf("UpsertPage: %v", err)
	}

	var title, filePath, dbID string
	var deleted int
	err := s.db.QueryRow("SELECT title, file_path, database_id, deleted FROM pages WHERE id = ?", "page-1").
		Scan(&title, &filePath, &dbID, &deleted)
	if err != nil {
		t.Fatalf("query page: %v", err)
	}
	if title != "Test Page" {
		t.Errorf("expected title 'Test Page', got %q", title)
	}
	if filePath != "/output/db/Test Page.md" {
		t.Errorf("expected filePath '/output/db/Test Page.md', got %q", filePath)
	}
	if dbID != "db-1" {
		t.Errorf("expected databaseID 'db-1', got %q", dbID)
	}
	if deleted != 0 {
		t.Errorf("expected deleted 0, got %d", deleted)
	}
}

func TestUpsertPage_Update(t *testing.T) {
	s := setupTestStore(t)

	data := PageData{
		ID: "page-1", Title: "Original", URL: "https://notion.so/test",
		BodyMarkdown: "original content", PropertiesJSON: "{}",
		LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z",
	}
	if err := s.UpsertPage(data); err != nil {
		t.Fatalf("UpsertPage (insert): %v", err)
	}

	data.Title = "Updated"
	data.BodyMarkdown = "updated content"
	data.LastEditedTime = "2026-02-01T00:00:00Z"
	if err := s.UpsertPage(data); err != nil {
		t.Fatalf("UpsertPage (update): %v", err)
	}

	var title, body string
	err := s.db.QueryRow("SELECT title, body_markdown FROM pages WHERE id = ?", "page-1").Scan(&title, &body)
	if err != nil {
		t.Fatalf("query page: %v", err)
	}
	if title != "Updated" {
		t.Errorf("expected title 'Updated', got %q", title)
	}
	if body != "updated content" {
		t.Errorf("expected body 'updated content', got %q", body)
	}
}

func TestUpsertPage_ResetDeletedFlag(t *testing.T) {
	s := setupTestStore(t)

	data := PageData{
		ID: "page-1", Title: "Page", URL: "https://notion.so/test",
		BodyMarkdown: "content", PropertiesJSON: "{}",
		LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z",
	}
	if err := s.UpsertPage(data); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkDeleted("page-1"); err != nil {
		t.Fatal(err)
	}

	// Re-upsert should reset deleted to 0
	if err := s.UpsertPage(data); err != nil {
		t.Fatal(err)
	}

	var deleted int
	s.db.QueryRow("SELECT deleted FROM pages WHERE id = ?", "page-1").Scan(&deleted)
	if deleted != 0 {
		t.Errorf("expected deleted 0 after re-upsert, got %d", deleted)
	}
}

func TestMarkDeleted(t *testing.T) {
	s := setupTestStore(t)

	data := PageData{
		ID: "page-1", Title: "Page", URL: "https://notion.so/test",
		BodyMarkdown: "content", PropertiesJSON: "{}",
		LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z",
	}
	if err := s.UpsertPage(data); err != nil {
		t.Fatal(err)
	}

	if err := s.MarkDeleted("page-1"); err != nil {
		t.Fatalf("MarkDeleted: %v", err)
	}

	var deleted int
	s.db.QueryRow("SELECT deleted FROM pages WHERE id = ?", "page-1").Scan(&deleted)
	if deleted != 1 {
		t.Errorf("expected deleted 1, got %d", deleted)
	}
}

func TestMarkDeleted_NonexistentPage(t *testing.T) {
	s := setupTestStore(t)

	// Should not error — just affects 0 rows
	if err := s.MarkDeleted("nonexistent"); err != nil {
		t.Fatalf("MarkDeleted on nonexistent page should not error: %v", err)
	}
}

func TestFTS_IndexOnInsert(t *testing.T) {
	s := setupTestStore(t)

	data := PageData{
		ID: "page-1", Title: "Quantum Computing", URL: "https://notion.so/test",
		BodyMarkdown: "Exploring quantum entanglement and superposition",
		PropertiesJSON: "{}", LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z",
	}
	if err := s.UpsertPage(data); err != nil {
		t.Fatal(err)
	}

	// Search by title
	var count int
	err := s.db.QueryRow("SELECT count(*) FROM pages_fts WHERE pages_fts MATCH 'quantum'").Scan(&count)
	if err != nil {
		t.Fatalf("FTS query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 FTS match for 'quantum', got %d", count)
	}

	// Search by body
	err = s.db.QueryRow("SELECT count(*) FROM pages_fts WHERE pages_fts MATCH 'entanglement'").Scan(&count)
	if err != nil {
		t.Fatalf("FTS query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 FTS match for 'entanglement', got %d", count)
	}
}

func TestFTS_IndexOnUpdate(t *testing.T) {
	s := setupTestStore(t)

	data := PageData{
		ID: "page-1", Title: "Old Title", URL: "https://notion.so/test",
		BodyMarkdown: "old content about apples",
		PropertiesJSON: "{}", LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z",
	}
	if err := s.UpsertPage(data); err != nil {
		t.Fatal(err)
	}

	// Update with new content
	data.Title = "New Title"
	data.BodyMarkdown = "new content about oranges"
	if err := s.UpsertPage(data); err != nil {
		t.Fatal(err)
	}

	// Old content should NOT be findable
	var count int
	err := s.db.QueryRow("SELECT count(*) FROM pages_fts WHERE pages_fts MATCH 'apples'").Scan(&count)
	if err != nil {
		t.Fatalf("FTS query: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 FTS matches for 'apples' after update, got %d", count)
	}

	// New content should be findable
	err = s.db.QueryRow("SELECT count(*) FROM pages_fts WHERE pages_fts MATCH 'oranges'").Scan(&count)
	if err != nil {
		t.Fatalf("FTS query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 FTS match for 'oranges', got %d", count)
	}
}

func TestFTS_MultiplePages(t *testing.T) {
	s := setupTestStore(t)

	pages := []PageData{
		{ID: "p1", Title: "Go Programming", URL: "u", BodyMarkdown: "concurrency patterns", PropertiesJSON: "{}", LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z"},
		{ID: "p2", Title: "Rust Programming", URL: "u", BodyMarkdown: "memory safety patterns", PropertiesJSON: "{}", LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z"},
		{ID: "p3", Title: "Python Scripting", URL: "u", BodyMarkdown: "dynamic typing", PropertiesJSON: "{}", LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z"},
	}
	for _, p := range pages {
		if err := s.UpsertPage(p); err != nil {
			t.Fatal(err)
		}
	}

	// "patterns" should match 2 pages
	var count int
	s.db.QueryRow("SELECT count(*) FROM pages_fts WHERE pages_fts MATCH 'patterns'").Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 FTS matches for 'patterns', got %d", count)
	}

	// "programming" should match 2 titles
	s.db.QueryRow("SELECT count(*) FROM pages_fts WHERE pages_fts MATCH 'programming'").Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 FTS matches for 'programming', got %d", count)
	}
}

func TestSerializeProperties(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]interface{}
		want  string
	}{
		{"empty", map[string]interface{}{}, "{}"},
		{"string", map[string]interface{}{"key": "value"}, `{"key":"value"}`},
		{"number", map[string]interface{}{"count": float64(42)}, `{"count":42}`},
		{"bool", map[string]interface{}{"done": true}, `{"done":true}`},
		{"null", map[string]interface{}{"empty": nil}, `{"empty":null}`},
		{"array", map[string]interface{}{"tags": []interface{}{"a", "b"}}, `{"tags":["a","b"]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SerializeProperties(tt.input)
			if err != nil {
				t.Fatalf("SerializeProperties: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMultipleDatabases(t *testing.T) {
	s := setupTestStore(t)

	p1 := PageData{
		ID: "p1", Title: "Page from DB-A", URL: "u", BodyMarkdown: "content",
		PropertiesJSON: "{}", LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z",
		DatabaseID: "db-a",
	}
	p2 := PageData{
		ID: "p2", Title: "Page from DB-B", URL: "u", BodyMarkdown: "content",
		PropertiesJSON: "{}", LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z",
		DatabaseID: "db-b",
	}
	if err := s.UpsertPage(p1); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertPage(p2); err != nil {
		t.Fatal(err)
	}

	// Query by database_id
	var count int
	s.db.QueryRow("SELECT count(*) FROM pages WHERE database_id = ?", "db-a").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 page in db-a, got %d", count)
	}

	// Total pages
	s.db.QueryRow("SELECT count(*) FROM pages").Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 total pages, got %d", count)
	}
}

func TestFTS_DeletedPagesStillSearchable(t *testing.T) {
	s := setupTestStore(t)

	data := PageData{
		ID: "p1", Title: "Searchable Page", URL: "u",
		BodyMarkdown: "unique findable content",
		PropertiesJSON: "{}", LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z",
	}
	if err := s.UpsertPage(data); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkDeleted("p1"); err != nil {
		t.Fatal(err)
	}

	// MarkDeleted uses UPDATE, which fires pages_au trigger.
	// Deleted pages remain in FTS — filter at query time with JOIN if needed.
	var count int
	s.db.QueryRow("SELECT count(*) FROM pages_fts WHERE pages_fts MATCH 'findable'").Scan(&count)
	if count != 1 {
		t.Errorf("expected deleted page to still be in FTS index, got %d matches", count)
	}
}

func TestFTS_IntegrityCheck(t *testing.T) {
	s := setupTestStore(t)

	pages := []PageData{
		{ID: "p1", Title: "Alpha", URL: "u", BodyMarkdown: "first", PropertiesJSON: "{}", LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z"},
		{ID: "p2", Title: "Beta", URL: "u", BodyMarkdown: "second", PropertiesJSON: "{}", LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z"},
	}
	for _, p := range pages {
		if err := s.UpsertPage(p); err != nil {
			t.Fatal(err)
		}
	}

	// Update one to exercise DELETE+INSERT trigger path
	pages[0].Title = "Alpha Updated"
	pages[0].BodyMarkdown = "first updated"
	if err := s.UpsertPage(pages[0]); err != nil {
		t.Fatal(err)
	}

	// FTS5 integrity-check verifies index consistency with content table
	_, err := s.db.Exec("INSERT INTO pages_fts(pages_fts) VALUES('integrity-check')")
	if err != nil {
		t.Fatalf("FTS integrity check failed: %v", err)
	}
}

func TestFTS_SurvivesReopen(t *testing.T) {
	dir := t.TempDir()

	s1, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	s1.UpsertPage(PageData{
		ID: "p1", Title: "Persistent Search", URL: "u", BodyMarkdown: "survives reopen",
		PropertiesJSON: "{}", LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z",
	})
	s1.Close()

	s2, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	var count int
	err = s2.db.QueryRow("SELECT count(*) FROM pages_fts WHERE pages_fts MATCH 'survives'").Scan(&count)
	if err != nil {
		t.Fatalf("FTS query after reopen: %v", err)
	}
	if count != 1 {
		t.Errorf("expected FTS to survive reopen, got %d matches", count)
	}
}

func TestUpsertPage_SpecialCharacters(t *testing.T) {
	s := setupTestStore(t)

	data := PageData{
		ID:             "p1",
		Title:          `It's a "test" — with 'quotes' & <tags>`,
		URL:            "https://notion.so/test?foo=bar&baz=1",
		FilePath:       `/output/db/It's a "test".md`,
		BodyMarkdown:   "# Heading\n\n```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n\nUnicode: 日本語 🎉 émojis",
		PropertiesJSON: `{"key":"value with \"quotes\""}`,
		CreatedTime:    "2026-01-01T00:00:00Z",
		LastEditedTime: "2026-02-01T00:00:00Z",
		FrozenAt:       "2026-02-15T00:00:00Z",
		DatabaseID:     "db-1",
	}

	if err := s.UpsertPage(data); err != nil {
		t.Fatalf("UpsertPage with special chars: %v", err)
	}

	var title, body string
	err := s.db.QueryRow("SELECT title, body_markdown FROM pages WHERE id = ?", "p1").Scan(&title, &body)
	if err != nil {
		t.Fatal(err)
	}
	if title != data.Title {
		t.Errorf("title mismatch: got %q", title)
	}
	if body != data.BodyMarkdown {
		t.Errorf("body mismatch: got %q", body)
	}
}

func TestGetPagesByDatabase(t *testing.T) {
	s := setupTestStore(t)

	pages := []PageData{
		{ID: "p1", Title: "A", URL: "u", BodyMarkdown: "c", PropertiesJSON: "{}", LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z", DatabaseID: "db-a"},
		{ID: "p2", Title: "B", URL: "u", BodyMarkdown: "c", PropertiesJSON: "{}", LastEditedTime: "2026-02-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z", DatabaseID: "db-a"},
		{ID: "p3", Title: "C", URL: "u", BodyMarkdown: "c", PropertiesJSON: "{}", LastEditedTime: "2026-03-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z", DatabaseID: "db-b"},
	}
	for _, p := range pages {
		if err := s.UpsertPage(p); err != nil {
			t.Fatal(err)
		}
	}

	// Query db-a: should get 2 pages
	result, err := s.GetPagesByDatabase("db-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 pages in db-a, got %d", len(result))
	}

	// Verify LastEditedTime field is populated
	if result[0].LastEditedTime == "" && result[1].LastEditedTime == "" {
		t.Error("expected LastEditedTime to be populated on returned pages")
	}

	// Mark one deleted — should be excluded
	if err := s.MarkDeleted("p1"); err != nil {
		t.Fatal(err)
	}
	result, err = s.GetPagesByDatabase("db-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 non-deleted page in db-a, got %d", len(result))
	}
	if result[0].ID != "p2" {
		t.Errorf("expected p2, got %s", result[0].ID)
	}
}

func TestGetPageLastEdited(t *testing.T) {
	s := setupTestStore(t)

	if err := s.UpsertPage(PageData{
		ID: "p1", Title: "A", URL: "u", BodyMarkdown: "c",
		PropertiesJSON: "{}", LastEditedTime: "2026-02-15T10:30:00Z", FrozenAt: "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	// Existing page
	got := s.GetPageLastEdited("p1")
	if got != "2026-02-15T10:30:00Z" {
		t.Errorf("expected '2026-02-15T10:30:00Z', got %q", got)
	}

	// Nonexistent page
	got = s.GetPageLastEdited("nonexistent")
	if got != "" {
		t.Errorf("expected empty string for nonexistent page, got %q", got)
	}

	// After update, should return new timestamp
	if err := s.UpsertPage(PageData{
		ID: "p1", Title: "A", URL: "u", BodyMarkdown: "c",
		PropertiesJSON: "{}", LastEditedTime: "2026-02-16T12:00:00Z", FrozenAt: "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	got = s.GetPageLastEdited("p1")
	if got != "2026-02-16T12:00:00Z" {
		t.Errorf("expected updated timestamp '2026-02-16T12:00:00Z', got %q", got)
	}

	// Deleted page should return empty
	if err := s.MarkDeleted("p1"); err != nil {
		t.Fatal(err)
	}
	got = s.GetPageLastEdited("p1")
	if got != "" {
		t.Errorf("expected empty string for deleted page, got %q", got)
	}
}

func TestGetPagesByDatabase_Empty(t *testing.T) {
	s := setupTestStore(t)

	result, err := s.GetPagesByDatabase("nonexistent-db")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 pages for nonexistent db, got %d", len(result))
	}
}

func TestOpenStore_Idempotent(t *testing.T) {
	dir := t.TempDir()

	// Open and close twice — should not error or corrupt
	s1, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	s1.UpsertPage(PageData{
		ID: "p1", Title: "Page", URL: "u", BodyMarkdown: "c",
		PropertiesJSON: "{}", LastEditedTime: "2026-01-01T00:00:00Z", FrozenAt: "2026-01-01T00:00:00Z",
	})
	s1.Close()

	s2, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("second OpenStore: %v", err)
	}
	defer s2.Close()

	var title string
	err = s2.db.QueryRow("SELECT title FROM pages WHERE id = ?", "p1").Scan(&title)
	if err == sql.ErrNoRows {
		t.Fatal("expected page to persist across store reopens")
	}
	if err != nil {
		t.Fatal(err)
	}
	if title != "Page" {
		t.Errorf("expected title 'Page', got %q", title)
	}
}
