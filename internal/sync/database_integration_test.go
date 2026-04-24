package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ran-codes/notion-sync/internal/frontmatter"
	"github.com/ran-codes/notion-sync/internal/notion"
)

// --- Phase 3b: resolveDataSources ---

func TestResolveDataSources_Single(t *testing.T) {
	mock := newMockClient()
	db := &notion.Database{
		ID:          "db-1",
		DataSources: []notion.DataSource{{ID: "ds-1", Type: "default"}},
	}

	sources, err := resolveDataSources(mock, db, "My DB", "/out/My DB")
	if err != nil {
		t.Fatalf("resolveDataSources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].ID != "ds-1" {
		t.Errorf("ID = %s, want ds-1", sources[0].ID)
	}
	if sources[0].Title != "My DB" {
		t.Errorf("Title = %s, want My DB", sources[0].Title)
	}
	if sources[0].FolderPath != "/out/My DB" {
		t.Errorf("FolderPath = %s, want /out/My DB", sources[0].FolderPath)
	}
}

func TestResolveDataSources_Multiple(t *testing.T) {
	mock := newMockClient()
	mock.dataSources["ds-1"] = &notion.DataSourceDetail{
		ID:    "ds-1",
		Title: []notion.RichText{{Type: "text", PlainText: "Projects", Text: &notion.TextContent{Content: "Projects"}}},
	}
	mock.dataSources["ds-2"] = &notion.DataSourceDetail{
		ID:    "ds-2",
		Title: []notion.RichText{{Type: "text", PlainText: "Clients", Text: &notion.TextContent{Content: "Clients"}}},
	}

	db := &notion.Database{
		ID: "db-1",
		DataSources: []notion.DataSource{
			{ID: "ds-1", Type: "default"},
			{ID: "ds-2", Type: "default"},
		},
	}

	sources, err := resolveDataSources(mock, db, "My DB", "/out/My DB")
	if err != nil {
		t.Fatalf("resolveDataSources: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	if sources[0].Title != "Projects" {
		t.Errorf("sources[0].Title = %s, want Projects", sources[0].Title)
	}
	if sources[1].Title != "Clients" {
		t.Errorf("sources[1].Title = %s, want Clients", sources[1].Title)
	}
	// Multi-source should use subfolders
	if !strings.HasSuffix(sources[0].FolderPath, "Projects") {
		t.Errorf("expected subfolder for Projects, got %s", sources[0].FolderPath)
	}
}

func TestResolveDataSources_Zero(t *testing.T) {
	mock := newMockClient()
	db := &notion.Database{
		ID:          "db-1",
		DataSources: []notion.DataSource{},
	}

	_, err := resolveDataSources(mock, db, "My DB", "/out/My DB")
	if err == nil {
		t.Fatal("expected error for zero data sources")
	}
	if !strings.Contains(err.Error(), "no data sources") {
		t.Errorf("error = %v, want 'no data sources'", err)
	}
}

func TestResolveDataSources_EmptyTitleFallback(t *testing.T) {
	mock := newMockClient()
	ds1ID := "abcdef01-2345-6789-abcd-ef0123456789"
	ds2ID := "12345678-abcd-ef01-2345-6789abcdef01"
	mock.dataSources[ds1ID] = &notion.DataSourceDetail{
		ID:    ds1ID,
		Title: []notion.RichText{}, // empty title
	}
	mock.dataSources[ds2ID] = &notion.DataSourceDetail{
		ID:    ds2ID,
		Title: []notion.RichText{{Type: "text", PlainText: "Named", Text: &notion.TextContent{Content: "Named"}}},
	}

	db := &notion.Database{
		ID: "db-1",
		DataSources: []notion.DataSource{
			{ID: ds1ID, Type: "default"},
			{ID: ds2ID, Type: "default"},
		},
	}

	sources, err := resolveDataSources(mock, db, "My DB", "/out/My DB")
	if err != nil {
		t.Fatalf("resolveDataSources: %v", err)
	}
	// Empty title should fallback to "Data Source <first8chars>"
	if !strings.HasPrefix(sources[0].Title, "Data Source ") {
		t.Errorf("expected fallback title, got %s", sources[0].Title)
	}
	if sources[1].Title != "Named" {
		t.Errorf("sources[1].Title = %s, want Named", sources[1].Title)
	}
}

// --- Phase 3c: Deletion detection ---

func TestRefreshDatabase_DeletionDetection(t *testing.T) {
	dir := t.TempDir()
	dbFolder := filepath.Join(dir, "TestDB")
	os.MkdirAll(dbFolder, 0755)

	mock := newMockClient()
	dbID := "db-delete-test"
	dsID := "ds-delete-test"

	mock.databases[dbID] = &notion.Database{
		ID:    dbID,
		URL:   "https://notion.so/" + dbID,
		Title: []notion.RichText{{Type: "text", PlainText: "TestDB", Text: &notion.TextContent{Content: "TestDB"}}},
		DataSources: []notion.DataSource{
			{ID: dsID, Type: "default"},
		},
	}
	mock.dataSources[dsID] = &notion.DataSourceDetail{ID: dsID}

	// Write 3 local .md files
	pages := []struct {
		id, title string
	}{
		{"page-1", "Page One"},
		{"page-2", "Page Two"},
		{"page-3", "Page Three"},
	}
	for _, p := range pages {
		content := "---\nnotion-id: " + p.id + "\nnotion-last-edited: \"2025-01-01T00:00:00Z\"\nnotion-database-id: " + dbID + "\n---\nBody\n"
		os.WriteFile(filepath.Join(dbFolder, p.title+".md"), []byte(content), 0644)
	}

	// Write _database.json
	WriteDatabaseMetadata(dbFolder, &FrozenDatabase{
		DatabaseID:   dbID,
		DataSourceID: dsID,
		Title:        "TestDB",
		FolderPath:   dbFolder,
	})

	// Mock returns only 2 of 3 pages (page-2 was "deleted" in Notion)
	mock.entries[dsID] = []notion.Page{
		testPage("page-1", "Page One", "2025-01-01T00:00:00Z"),
		testPage("page-3", "Page Three", "2025-01-01T00:00:00Z"),
	}
	// Provide blocks for the pages (empty is fine, they'll be skipped anyway)
	mock.blocks["page-1"] = []notion.Block{}
	mock.blocks["page-3"] = []notion.Block{}

	result, err := RefreshDatabase(RefreshOptions{
		Client:     mock,
		FolderPath: dbFolder,
	}, nil)
	if err != nil {
		t.Fatalf("RefreshDatabase: %v", err)
	}

	if result.Deleted != 1 {
		t.Errorf("Deleted = %d, want 1", result.Deleted)
	}

	// Verify the deleted page's .md has notion-deleted: true
	data, err := os.ReadFile(filepath.Join(dbFolder, "Page Two.md"))
	if err != nil {
		t.Fatalf("read deleted file: %v", err)
	}
	fm, err := frontmatter.Parse(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	if fm["notion-deleted"] != true {
		t.Errorf("expected notion-deleted: true, got %v", fm["notion-deleted"])
	}

	// Verify the other pages were skipped (timestamps match)
	if result.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", result.Skipped)
	}
}

// --- Phase 4b: refreshMultiSource aggregation ---

func TestRefreshMultiSource_Aggregation(t *testing.T) {
	dir := t.TempDir()
	dbFolder := filepath.Join(dir, "MultiDB")
	os.MkdirAll(dbFolder, 0755)

	mock := newMockClient()
	dbID := "db-multi"
	dsID1 := "ds-multi-1"
	dsID2 := "ds-multi-2"

	mock.databases[dbID] = &notion.Database{
		ID:    dbID,
		URL:   "https://notion.so/" + dbID,
		Title: []notion.RichText{{Type: "text", PlainText: "MultiDB", Text: &notion.TextContent{Content: "MultiDB"}}},
		DataSources: []notion.DataSource{
			{ID: dsID1, Type: "default"},
			{ID: dsID2, Type: "default"},
		},
	}
	mock.dataSources[dsID1] = &notion.DataSourceDetail{ID: dsID1}
	mock.dataSources[dsID2] = &notion.DataSourceDetail{ID: dsID2}

	// Create subfolder1 with 2 pages
	sub1 := filepath.Join(dbFolder, "Source1")
	os.MkdirAll(sub1, 0755)
	WriteDatabaseMetadata(sub1, &FrozenDatabase{
		DatabaseID:   dbID,
		DataSourceID: dsID1,
		Title:        "Source1",
		FolderPath:   sub1,
	})
	for _, p := range []struct{ id, title string }{{"p1", "Page1"}, {"p2", "Page2"}} {
		content := "---\nnotion-id: " + p.id + "\nnotion-last-edited: \"2025-01-01T00:00:00Z\"\n---\nBody\n"
		os.WriteFile(filepath.Join(sub1, p.title+".md"), []byte(content), 0644)
	}

	// Create subfolder2 with 1 page
	sub2 := filepath.Join(dbFolder, "Source2")
	os.MkdirAll(sub2, 0755)
	WriteDatabaseMetadata(sub2, &FrozenDatabase{
		DatabaseID:   dbID,
		DataSourceID: dsID2,
		Title:        "Source2",
		FolderPath:   sub2,
	})
	content := "---\nnotion-id: p3\nnotion-last-edited: \"2025-01-01T00:00:00Z\"\n---\nBody\n"
	os.WriteFile(filepath.Join(sub2, "Page3.md"), []byte(content), 0644)

	// Top-level metadata (no dataSourceId)
	WriteDatabaseMetadata(dbFolder, &FrozenDatabase{
		DatabaseID: dbID,
		Title:      "MultiDB",
		FolderPath: dbFolder,
	})

	// Mock entries: all pages exist, timestamps match → all skipped
	mock.entries[dsID1] = []notion.Page{
		testPage("p1", "Page1", "2025-01-01T00:00:00Z"),
		testPage("p2", "Page2", "2025-01-01T00:00:00Z"),
	}
	mock.entries[dsID2] = []notion.Page{
		testPage("p3", "Page3", "2025-01-01T00:00:00Z"),
	}

	result, err := RefreshDatabase(RefreshOptions{
		Client:     mock,
		FolderPath: dbFolder,
	}, nil)
	if err != nil {
		t.Fatalf("RefreshDatabase: %v", err)
	}

	// Should aggregate across both subfolders
	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}
	if result.Skipped != 3 {
		t.Errorf("Skipped = %d, want 3", result.Skipped)
	}
}

// --- Phase 5a: Markdown import test ---

func TestFreshImport_Markdown(t *testing.T) {
	dir := t.TempDir()
	mock := newMockClient()
	dbID := "db-md-only"
	dsID := "ds-md-only"

	mock.databases[dbID] = &notion.Database{
		ID:    dbID,
		URL:   "https://notion.so/" + dbID,
		Title: []notion.RichText{{Type: "text", PlainText: "MarkdownDB", Text: &notion.TextContent{Content: "MarkdownDB"}}},
		DataSources: []notion.DataSource{
			{ID: dsID, Type: "default"},
		},
	}
	mock.dataSources[dsID] = &notion.DataSourceDetail{ID: dsID}
	mock.entries[dsID] = []notion.Page{
		testPage("p1", "TestPage", "2025-01-01T00:00:00Z"),
	}
	mock.blocks["p1"] = []notion.Block{
		{Type: "paragraph", Paragraph: &notion.ParagraphBlock{
			RichText: []notion.RichText{{Type: "text", PlainText: "Hello", Text: &notion.TextContent{Content: "Hello"}}},
		}},
	}

	result, err := FreshDatabaseImport(DatabaseImportOptions{
		Client:       mock,
		DatabaseID:   dbID,
		OutputFolder: dir,
	}, nil)
	if err != nil {
		t.Fatalf("FreshDatabaseImport: %v", err)
	}
	if result.Created != 1 {
		t.Errorf("Created = %d, want 1", result.Created)
	}

	// .md file should exist
	mdPath := filepath.Join(dir, "MarkdownDB", "TestPage.md")
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Error("expected .md file to exist")
	}
}

func TestFreshImport_DuplicateTitles(t *testing.T) {
	dir := t.TempDir()
	mock := newMockClient()
	dbID := "db-dup"
	dsID := "ds-dup"

	mock.databases[dbID] = &notion.Database{
		ID:    dbID,
		URL:   "https://notion.so/" + dbID,
		Title: []notion.RichText{{Type: "text", PlainText: "DupDB", Text: &notion.TextContent{Content: "DupDB"}}},
		DataSources: []notion.DataSource{
			{ID: dsID, Type: "default"},
		},
	}
	mock.dataSources[dsID] = &notion.DataSourceDetail{ID: dsID}
	mock.entries[dsID] = []notion.Page{
		testPage("aaaa1111-2222-3333-4444-555566667777", "Same Name", "2025-01-01T00:00:00Z"),
		testPage("bbbb1111-2222-3333-4444-555566667777", "Same Name", "2025-01-01T00:00:00Z"),
		testPage("cccc1111-2222-3333-4444-555566667777", "Unique", "2025-01-01T00:00:00Z"),
	}
	for _, e := range mock.entries[dsID] {
		mock.blocks[e.ID] = []notion.Block{
			{Type: "paragraph", Paragraph: &notion.ParagraphBlock{
				RichText: []notion.RichText{{Type: "text", PlainText: "Body", Text: &notion.TextContent{Content: "Body"}}},
			}},
		}
	}

	result, err := FreshDatabaseImport(DatabaseImportOptions{
		Client:       mock,
		DatabaseID:   dbID,
		OutputFolder: dir,
	}, nil)
	if err != nil {
		t.Fatalf("FreshDatabaseImport: %v", err)
	}
	if result.Created != 3 {
		t.Errorf("Created = %d, want 3", result.Created)
	}

	dbFolder := filepath.Join(dir, "DupDB")

	// "Same Name.md" should NOT exist (both pages need disambiguation)
	if _, err := os.Stat(filepath.Join(dbFolder, "Same Name.md")); !os.IsNotExist(err) {
		t.Error("expected 'Same Name.md' to NOT exist (should be disambiguated)")
	}

	// Both disambiguated files should exist
	file1 := filepath.Join(dbFolder, "Same Name-aaaa111122223333444455556666777.md")
	file2 := filepath.Join(dbFolder, "Same Name-bbbb111122223333444455556666777.md")

	// Use glob since the exact dash-removal may vary
	matches, _ := filepath.Glob(filepath.Join(dbFolder, "Same Name-*.md"))
	if len(matches) != 2 {
		t.Fatalf("expected 2 'Same Name-*.md' files, got %d: %v", len(matches), matches)
	}

	// Verify both have different notion-ids
	ids := map[string]bool{}
	for _, f := range matches {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		fm, err := frontmatter.Parse(string(data))
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		id, ok := fm["notion-id"].(string)
		if !ok || id == "" {
			t.Errorf("missing notion-id in %s", f)
		}
		ids[id] = true
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 unique notion-ids, got %d", len(ids))
	}

	// "Unique.md" should exist with clean name
	if _, err := os.Stat(filepath.Join(dbFolder, "Unique.md")); os.IsNotExist(err) {
		t.Error("expected 'Unique.md' to exist (unique title, no disambiguation)")
	}

	_ = file1
	_ = file2
}

func TestRefresh_DuplicateTitleRename(t *testing.T) {
	dir := t.TempDir()
	dbFolder := filepath.Join(dir, "TestDB")
	os.MkdirAll(dbFolder, 0755)

	mock := newMockClient()
	dbID := "db-dup-rename"
	dsID := "ds-dup-rename"

	mock.databases[dbID] = &notion.Database{
		ID:    dbID,
		URL:   "https://notion.so/" + dbID,
		Title: []notion.RichText{{Type: "text", PlainText: "TestDB", Text: &notion.TextContent{Content: "TestDB"}}},
		DataSources: []notion.DataSource{
			{ID: dsID, Type: "default"},
		},
	}
	mock.dataSources[dsID] = &notion.DataSourceDetail{ID: dsID}

	pageAID := "aaaa0000-1111-2222-3333-444455556666"
	pageBID := "bbbb0000-1111-2222-3333-444455556666"

	// Pre-existing local file: PageA.md with notion-id page-a
	oldContent := "---\nnotion-id: " + pageAID + "\nnotion-last-edited: \"2025-01-01T00:00:00Z\"\nnotion-database-id: " + dbID + "\n---\nOld body\n"
	os.WriteFile(filepath.Join(dbFolder, "PageA.md"), []byte(oldContent), 0644)

	// Write _database.json
	WriteDatabaseMetadata(dbFolder, &FrozenDatabase{
		DatabaseID:   dbID,
		DataSourceID: dsID,
		Title:        "TestDB",
		FolderPath:   dbFolder,
	})

	// Now mock returns 2 entries both titled "PageA" — triggers disambiguation
	mock.entries[dsID] = []notion.Page{
		testPage(pageAID, "PageA", "2025-01-02T00:00:00Z"), // newer timestamp → will be processed
		testPage(pageBID, "PageA", "2025-01-01T00:00:00Z"),
	}
	mock.blocks[pageAID] = []notion.Block{
		{Type: "paragraph", Paragraph: &notion.ParagraphBlock{
			RichText: []notion.RichText{{Type: "text", PlainText: "Body A", Text: &notion.TextContent{Content: "Body A"}}},
		}},
	}
	mock.blocks[pageBID] = []notion.Block{
		{Type: "paragraph", Paragraph: &notion.ParagraphBlock{
			RichText: []notion.RichText{{Type: "text", PlainText: "Body B", Text: &notion.TextContent{Content: "Body B"}}},
		}},
	}

	result, err := RefreshDatabase(RefreshOptions{
		Client:     mock,
		FolderPath: dbFolder,
		Force:      true,
	}, nil)
	if err != nil {
		t.Fatalf("RefreshDatabase: %v", err)
	}

	if result.Updated+result.Created != 2 {
		t.Errorf("Updated+Created = %d, want 2", result.Updated+result.Created)
	}

	// Old "PageA.md" should be gone
	if _, err := os.Stat(filepath.Join(dbFolder, "PageA.md")); !os.IsNotExist(err) {
		t.Error("expected old 'PageA.md' to be removed")
	}

	// Two disambiguated files should exist
	matches, _ := filepath.Glob(filepath.Join(dbFolder, "PageA-*.md"))
	if len(matches) != 2 {
		t.Fatalf("expected 2 'PageA-*.md' files, got %d: %v", len(matches), matches)
	}
}
