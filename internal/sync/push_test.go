package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ran-codes/notion-sync/internal/notion"
)

// --- buildPropertyPayload tests ---

func TestBuildPropertyPayload_SkipsNotionKeys(t *testing.T) {
	fm := map[string]interface{}{
		"notion-id":          "abc",
		"notion-url":         "https://notion.so/abc",
		"notion-last-edited": "2024-01-01T00:00:00Z",
		"notion-database-id": "db1",
		"notion-deleted":     false,
		"notion-last-pushed": "2024-01-01T00:00:00Z",
	}
	schema := map[string]notion.DatabaseProperty{}
	got, _ := buildPropertyPayload(fm, schema)
	if len(got) != 0 {
		t.Errorf("expected empty payload, got %v", got)
	}
}

func TestBuildPropertyPayload_SkipsReadOnlyTypes(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{
		"Assignee":    {Type: "people"},
		"Created":     {Type: "created_time"},
		"LastEdited":  {Type: "last_edited_time"},
		"CreatedBy":   {Type: "created_by"},
		"EditedBy":    {Type: "last_edited_by"},
		"Formula":     {Type: "formula"},
		"Rollup":      {Type: "rollup"},
		"UniqueID":    {Type: "unique_id"},
		"Attachments": {Type: "files"},
	}
	fm := map[string]interface{}{
		"Assignee": "Alice", "Created": "2024-01-01",
		"LastEdited": "2024-01-01", "CreatedBy": "Alice",
		"EditedBy": "Bob", "Formula": "calc",
		"Rollup": "sum", "UniqueID": "ID-1",
		"Attachments": []interface{}{"url"},
	}
	got, _ := buildPropertyPayload(fm, schema)
	if len(got) != 0 {
		t.Errorf("expected empty payload for read-only types, got %v", got)
	}
}

func TestBuildPropertyPayload_SkipsUnknownProperties(t *testing.T) {
	fm := map[string]interface{}{"SomeProp": "value"}
	got, _ := buildPropertyPayload(fm, map[string]notion.DatabaseProperty{})
	if len(got) != 0 {
		t.Errorf("expected empty payload for unknown property, got %v", got)
	}
}

func TestBuildPropertyPayload_RichTextTooLong(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{"Notes": {Type: "rich_text"}}
	fm := map[string]interface{}{"Notes": strings.Repeat("x", 2001)}
	got, errs := buildPropertyPayload(fm, schema)
	if len(got) != 0 {
		t.Error("expected property to be excluded from payload")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 validation error, got %d", len(errs))
	}
	if !strings.Contains(errs[0], "2000-char limit") {
		t.Errorf("unexpected error message: %s", errs[0])
	}
}

func TestBuildPropertyPayload_IncludesTitle(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{
		"Metric Name": {Type: "title"},
	}
	fm := map[string]interface{}{
		"Metric Name": "Prevalence Estimate",
	}
	got, errs := buildPropertyPayload(fm, schema)
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	payload, ok := got["Metric Name"]
	if !ok {
		t.Fatalf("expected 'Metric Name' in payload, got %v", got)
	}
	tt := payload.(map[string]interface{})["title"].([]interface{})
	text := tt[0].(map[string]interface{})["text"].(map[string]interface{})["content"]
	if text != "Prevalence Estimate" {
		t.Errorf("expected 'Prevalence Estimate', got %v", text)
	}
}

func TestBuildPropertyPayload_TitleTooLong(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{"Metric Name": {Type: "title"}}
	fm := map[string]interface{}{"Metric Name": strings.Repeat("x", 2001)}
	got, errs := buildPropertyPayload(fm, schema)
	if len(got) != 0 {
		t.Error("expected over-limit title to be excluded from payload")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 validation error, got %d", len(errs))
	}
	if !strings.Contains(errs[0], "2000-char limit") {
		t.Errorf("unexpected error message: %s", errs[0])
	}
}

func TestBuildPropertyValue_RichText(t *testing.T) {
	got := buildPropertyValue("rich_text", "hello world")
	rt := got.(map[string]interface{})["rich_text"].([]interface{})
	if len(rt) != 1 {
		t.Fatalf("expected 1 rich_text item, got %d", len(rt))
	}
	text := rt[0].(map[string]interface{})["text"].(map[string]interface{})["content"]
	if text != "hello world" {
		t.Errorf("expected 'hello world', got %v", text)
	}
}

func TestBuildPropertyValue_Title(t *testing.T) {
	got := buildPropertyValue("title", "Prevalence Estimate")
	tt := got.(map[string]interface{})["title"].([]interface{})
	if len(tt) != 1 {
		t.Fatalf("expected 1 title item, got %d", len(tt))
	}
	text := tt[0].(map[string]interface{})["text"].(map[string]interface{})["content"]
	if text != "Prevalence Estimate" {
		t.Errorf("expected 'Prevalence Estimate', got %v", text)
	}
}

// Empty string in the title frontmatter key is intentionally pushed as a
// single empty rich-text item, mirroring rich_text behavior. Locks in this
// choice so it doesn't silently drift to {"title": []} (Notion's "clear").
func TestBuildPropertyValue_TitleEmptyString(t *testing.T) {
	got := buildPropertyValue("title", "")
	tt := got.(map[string]interface{})["title"].([]interface{})
	if len(tt) != 1 {
		t.Fatalf("expected 1 title item for empty string, got %d", len(tt))
	}
	text := tt[0].(map[string]interface{})["text"].(map[string]interface{})["content"]
	if text != "" {
		t.Errorf("expected empty content, got %q", text)
	}
}

// Nil flows through coerceString → "" and is pushed as an empty title item,
// same as rich_text. Locks in the behavior.
func TestBuildPropertyPayload_TitleNilCoercesToEmpty(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{"Metric Name": {Type: "title"}}
	fm := map[string]interface{}{"Metric Name": nil}
	got, errs := buildPropertyPayload(fm, schema)
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	payload, ok := got["Metric Name"]
	if !ok {
		t.Fatalf("expected 'Metric Name' in payload (nil → empty), got %v", got)
	}
	tt := payload.(map[string]interface{})["title"].([]interface{})
	text := tt[0].(map[string]interface{})["text"].(map[string]interface{})["content"]
	if text != "" {
		t.Errorf("expected empty content for nil title, got %q", text)
	}
}

// Known limitation: imported titles encode formatting (bold, links, mentions)
// as literal markdown via ConvertRichText. Push sends that string as plain
// text content with no parsing — so a roundtripped title loses its formatting
// and gains visible asterisks/brackets in Notion. This test pins the current
// behavior so a regression (or a future fix) is loud.
func TestBuildPropertyValue_TitleMarkdownIsLiteral(t *testing.T) {
	in := "**Bold** with [link](https://example.com) and [[notion-id: abc]]"
	got := buildPropertyValue("title", in)
	tt := got.(map[string]interface{})["title"].([]interface{})
	if len(tt) != 1 {
		t.Fatalf("expected 1 title item, got %d", len(tt))
	}
	text := tt[0].(map[string]interface{})["text"].(map[string]interface{})["content"]
	if text != in {
		t.Errorf("expected markdown to be sent verbatim as plain text\n  want: %q\n   got: %q", in, text)
	}
}

func TestBuildPropertyValue_Number(t *testing.T) {
	got := buildPropertyValue("number", float64(42)).(map[string]interface{})
	if got["number"] != float64(42) {
		t.Errorf("expected 42, got %v", got["number"])
	}

	got = buildPropertyValue("number", nil).(map[string]interface{})
	if got["number"] != nil {
		t.Errorf("expected nil, got %v", got["number"])
	}
}

func TestBuildPropertyValue_Select(t *testing.T) {
	got := buildPropertyValue("select", "Option A").(map[string]interface{})
	sel := got["select"].(map[string]interface{})
	if sel["name"] != "Option A" {
		t.Errorf("expected 'Option A', got %v", sel["name"])
	}

	got = buildPropertyValue("select", nil).(map[string]interface{})
	if got["select"] != nil {
		t.Errorf("expected nil select, got %v", got["select"])
	}
}

func TestBuildPropertyValue_MultiSelect(t *testing.T) {
	got := buildPropertyValue("multi_select", []interface{}{"A", "B"}).(map[string]interface{})
	items := got["multi_select"].([]interface{})
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].(map[string]interface{})["name"] != "A" {
		t.Errorf("expected 'A', got %v", items[0])
	}
}

func TestBuildPropertyValue_Checkbox(t *testing.T) {
	got := buildPropertyValue("checkbox", true).(map[string]interface{})
	if got["checkbox"] != true {
		t.Errorf("expected true, got %v", got["checkbox"])
	}
}

func TestBuildPropertyValue_Date(t *testing.T) {
	got := buildPropertyValue("date", "2024-01-15").(map[string]interface{})
	d := got["date"].(map[string]interface{})
	if d["start"] != "2024-01-15" {
		t.Errorf("expected '2024-01-15', got %v", d["start"])
	}
	if _, hasEnd := d["end"]; hasEnd {
		t.Error("expected no end for single date")
	}
}

func TestBuildPropertyValue_DateRange(t *testing.T) {
	got := buildPropertyValue("date", "2024-01-15 → 2024-01-20").(map[string]interface{})
	d := got["date"].(map[string]interface{})
	if d["start"] != "2024-01-15" {
		t.Errorf("expected '2024-01-15', got %v", d["start"])
	}
	if d["end"] != "2024-01-20" {
		t.Errorf("expected '2024-01-20', got %v", d["end"])
	}
}

func TestBuildPropertyValue_Relation(t *testing.T) {
	got := buildPropertyValue("relation", []interface{}{"id1", "id2"}).(map[string]interface{})
	rels := got["relation"].([]interface{})
	if len(rels) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(rels))
	}
	if rels[0].(map[string]interface{})["id"] != "id1" {
		t.Errorf("expected 'id1', got %v", rels[0])
	}
}

// --- PushDatabase integration tests ---

func TestPushDatabase_PushesProperties(t *testing.T) {
	dir := t.TempDir()

	// Write _database.json
	writeDatabaseMeta(t, dir, "db-001")

	// Write a .md file
	md := "---\n" +
		"notion-id: page-001\n" +
		"notion-url: https://notion.so/page-001\n" +
		"notion-last-edited: 2024-01-01T00:00:00Z\n" +
		"notion-database-id: db-001\n" +
		"Status: In Progress\n" +
		"Priority: 1\n" +
		"---\n# Content\n"
	if err := os.WriteFile(filepath.Join(dir, "page-001.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	client.databases["db-001"] = &notion.Database{
		ID:    "db-001",
		Title: []notion.RichText{{PlainText: "Test DB"}},
		Properties: map[string]notion.DatabaseProperty{
			"Status":   {Type: "select"},
			"Priority": {Type: "number"},
		},
	}
	client.pages["page-001"] = &notion.Page{
		ID:             "page-001",
		LastEditedTime: "2024-01-01T00:00:00Z",
	}

	result, err := PushDatabase(PushOptions{
		Client:     client,
		FolderPath: dir,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Pushed != 1 {
		t.Errorf("expected 1 pushed, got %d", result.Pushed)
	}
	if result.Conflicts != 0 {
		t.Errorf("expected 0 conflicts, got %d", result.Conflicts)
	}
	if len(client.updateRequests) != 1 {
		t.Fatalf("expected 1 UpdatePage call, got %d", len(client.updateRequests))
	}

	req := client.updateRequests[0]
	if req.PageID != "page-001" {
		t.Errorf("expected page-001, got %s", req.PageID)
	}
	if _, ok := req.Properties["Status"]; !ok {
		t.Error("expected Status in payload")
	}
	if _, ok := req.Properties["Priority"]; !ok {
		t.Error("expected Priority in payload")
	}
}

func TestPushDatabase_PushesTitleRename(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	// Local edit: typo correction in the title-typed property.
	md := "---\n" +
		"notion-id: page-001\n" +
		"notion-last-edited: 2024-01-01T00:00:00Z\n" +
		"notion-database-id: db-001\n" +
		"Metric Name: Prevalence Estimate for Obese\n" +
		"---\n# Body\n"
	if err := os.WriteFile(filepath.Join(dir, "page-001.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	client.databases["db-001"] = &notion.Database{
		ID: "db-001",
		Properties: map[string]notion.DatabaseProperty{
			"Metric Name": {Type: "title"},
		},
	}
	client.pages["page-001"] = &notion.Page{
		ID:             "page-001",
		LastEditedTime: "2024-01-01T00:00:00Z",
	}

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pushed != 1 {
		t.Fatalf("expected 1 pushed, got %d", result.Pushed)
	}
	if len(client.updateRequests) != 1 {
		t.Fatalf("expected 1 UpdatePage call, got %d", len(client.updateRequests))
	}

	req := client.updateRequests[0]
	payload, ok := req.Properties["Metric Name"]
	if !ok {
		t.Fatalf("expected 'Metric Name' in UpdatePage payload, got %v", req.Properties)
	}
	tt := payload.(map[string]interface{})["title"].([]interface{})
	text := tt[0].(map[string]interface{})["text"].(map[string]interface{})["content"]
	if text != "Prevalence Estimate for Obese" {
		t.Errorf("expected title 'Prevalence Estimate for Obese', got %v", text)
	}
}

// Force bypasses the conflict check and still pushes the renamed title —
// confirms title push composes correctly with --force when Notion is "ahead".
func TestPushDatabase_ForcePushesTitleRenameOverConflict(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	md := "---\n" +
		"notion-id: page-001\n" +
		"notion-last-edited: 2024-01-01T00:00:00Z\n" +
		"notion-database-id: db-001\n" +
		"Metric Name: Renamed Locally\n" +
		"---\n# Body\n"
	if err := os.WriteFile(filepath.Join(dir, "page-001.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	client.databases["db-001"] = &notion.Database{
		ID: "db-001",
		Properties: map[string]notion.DatabaseProperty{
			"Metric Name": {Type: "title"},
		},
	}
	// Notion's timestamp is newer than local — would normally be a conflict.
	client.pages["page-001"] = &notion.Page{
		ID:             "page-001",
		LastEditedTime: "2024-06-01T00:00:00Z",
	}

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir, Force: true}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pushed != 1 {
		t.Fatalf("expected 1 pushed under --force, got %d (conflicts=%d, errors=%v)", result.Pushed, result.Conflicts, result.Errors)
	}
	if result.Conflicts != 0 {
		t.Errorf("expected 0 conflicts under --force, got %d", result.Conflicts)
	}
	if len(client.updateRequests) != 1 {
		t.Fatalf("expected 1 UpdatePage call, got %d", len(client.updateRequests))
	}
	payload, ok := client.updateRequests[0].Properties["Metric Name"]
	if !ok {
		t.Fatalf("expected 'Metric Name' in payload, got %v", client.updateRequests[0].Properties)
	}
	text := payload.(map[string]interface{})["title"].([]interface{})[0].(map[string]interface{})["text"].(map[string]interface{})["content"]
	if text != "Renamed Locally" {
		t.Errorf("expected 'Renamed Locally', got %v", text)
	}
}

func TestPushDatabase_DetectsConflict(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	md := "---\n" +
		"notion-id: page-001\n" +
		"notion-last-edited: 2024-01-01T00:00:00Z\n" +
		"Status: Done\n" +
		"---\n"
	if err := os.WriteFile(filepath.Join(dir, "page-001.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	client.databases["db-001"] = &notion.Database{
		ID: "db-001",
		Properties: map[string]notion.DatabaseProperty{
			"Status": {Type: "select"},
		},
	}
	// Notion has a newer timestamp
	client.pages["page-001"] = &notion.Page{
		ID:             "page-001",
		LastEditedTime: "2024-06-01T00:00:00Z",
	}

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.Conflicts != 1 {
		t.Errorf("expected 1 conflict, got %d", result.Conflicts)
	}
	if len(client.updateRequests) != 0 {
		t.Error("expected no UpdatePage calls on conflict")
	}
}

func TestPushDatabase_ForceSkipsConflictCheck(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	md := "---\n" +
		"notion-id: page-001\n" +
		"notion-last-edited: 2024-01-01T00:00:00Z\n" +
		"Status: Done\n" +
		"---\n"
	if err := os.WriteFile(filepath.Join(dir, "page-001.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	client.databases["db-001"] = &notion.Database{
		ID: "db-001",
		Properties: map[string]notion.DatabaseProperty{
			"Status": {Type: "select"},
		},
	}
	client.pages["page-001"] = &notion.Page{
		ID:             "page-001",
		LastEditedTime: "2024-06-01T00:00:00Z",
	}

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir, Force: true}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.Pushed != 1 {
		t.Errorf("expected 1 pushed, got %d", result.Pushed)
	}
	if len(client.updateRequests) != 1 {
		t.Errorf("expected 1 UpdatePage call, got %d", len(client.updateRequests))
	}
}

func TestPushDatabase_DryRunNoWrite(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	md := "---\n" +
		"notion-id: page-001\n" +
		"notion-last-edited: 2024-01-01T00:00:00Z\n" +
		"Status: Done\n" +
		"---\n"
	filePath := filepath.Join(dir, "page-001.md")
	if err := os.WriteFile(filePath, []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	client.databases["db-001"] = &notion.Database{
		ID: "db-001",
		Properties: map[string]notion.DatabaseProperty{
			"Status": {Type: "select"},
		},
	}
	client.pages["page-001"] = &notion.Page{
		ID:             "page-001",
		LastEditedTime: "2024-01-01T00:00:00Z",
	}

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir, DryRun: true}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.Pushed != 1 {
		t.Errorf("expected 1 in dry-run pushed count, got %d", result.Pushed)
	}
	if len(client.updateRequests) != 0 {
		t.Error("expected no UpdatePage calls in dry-run mode")
	}

	// File should be unchanged (no notion-last-pushed added)
	got, _ := os.ReadFile(filePath)
	if strings.Contains(string(got), "notion-last-pushed") {
		t.Error("dry-run should not modify files")
	}
}

func TestPushDatabase_SkipsDeletedEntries(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	md := "---\n" +
		"notion-id: page-001\n" +
		"notion-last-edited: 2024-01-01T00:00:00Z\n" +
		"notion-deleted: true\n" +
		"Status: Done\n" +
		"---\n"
	if err := os.WriteFile(filepath.Join(dir, "page-001.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	client.databases["db-001"] = &notion.Database{
		ID:         "db-001",
		Properties: map[string]notion.DatabaseProperty{"Status": {Type: "select"}},
	}

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.Total != 0 {
		t.Errorf("expected 0 total (deleted skipped), got %d", result.Total)
	}
}

func TestPushDatabase_WritesLastPushed(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	md := "---\n" +
		"notion-id: page-001\n" +
		"notion-last-edited: 2024-01-01T00:00:00Z\n" +
		"Status: Done\n" +
		"---\n"
	filePath := filepath.Join(dir, "page-001.md")
	if err := os.WriteFile(filePath, []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	client.databases["db-001"] = &notion.Database{
		ID:         "db-001",
		Properties: map[string]notion.DatabaseProperty{"Status": {Type: "select"}},
	}
	client.pages["page-001"] = &notion.Page{
		ID:             "page-001",
		LastEditedTime: "2024-01-01T00:00:00Z",
	}

	if _, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(filePath)
	if !strings.Contains(string(got), "notion-last-pushed:") {
		t.Error("expected notion-last-pushed to be written to file after push")
	}
}

func TestUpdateAfterPush_DoesNotCorruptValueContainingKey(t *testing.T) {
	dir := t.TempDir()
	// A property value contains the substring "notion-last-edited:" — the function
	// must not match inside this value when updating the actual key.
	md := "---\n" +
		"notion-id: page-001\n" +
		"Notes: \"see notion-last-edited: for tracking\"\n" +
		"notion-last-edited: 2024-01-01T00:00:00Z\n" +
		"---\n"
	filePath := filepath.Join(dir, "page.md")
	if err := os.WriteFile(filePath, []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	if err := updateAfterPush(filePath, "2024-06-01T00:00:00Z", "2024-06-01T01:00:00Z"); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(filePath)
	s := string(got)

	if !strings.Contains(s, "Notes: \"see notion-last-edited: for tracking\"") {
		t.Error("Notes value was corrupted")
	}
	if !strings.Contains(s, "notion-last-edited: 2024-06-01T00:00:00Z") {
		t.Error("notion-last-edited was not updated")
	}
	if !strings.Contains(s, "notion-last-pushed: 2024-06-01T01:00:00Z") {
		t.Error("notion-last-pushed was not written")
	}
}

// writeDatabaseMeta writes a minimal _database.json for tests.
func writeDatabaseMeta(t *testing.T, dir, dbID string) {
	t.Helper()
	meta := &FrozenDatabase{DatabaseID: dbID, Title: "Test DB"}
	if err := WriteDatabaseMetadata(dir, meta); err != nil {
		t.Fatal(err)
	}
}

// writeDatabaseMetaWithDataSource writes a _database.json that includes a
// dataSourceId, simulating metadata produced by post-multi-data-source imports.
func writeDatabaseMetaWithDataSource(t *testing.T, dir, dbID, dsID string) {
	t.Helper()
	meta := &FrozenDatabase{DatabaseID: dbID, DataSourceID: dsID, Title: "Test DB"}
	if err := WriteDatabaseMetadata(dir, meta); err != nil {
		t.Fatal(err)
	}
}

// When _database.json has a dataSourceId, push must fetch the schema from
// /data_sources/{id} (not /databases/{id}). This is the production path for
// every Notion DB imported under the multi-data-source API. The mock's
// `databases` map is intentionally left empty to prove the schema is sourced
// from `dataSources`.
func TestPushDatabase_FetchesSchemaFromDataSource(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMetaWithDataSource(t, dir, "db-001", "ds-001")

	md := "---\n" +
		"notion-id: page-001\n" +
		"notion-last-edited: 2024-01-01T00:00:00Z\n" +
		"notion-database-id: db-001\n" +
		"Status: In Progress\n" +
		"---\n# Content\n"
	if err := os.WriteFile(filepath.Join(dir, "page-001.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	client.dataSources["ds-001"] = &notion.DataSourceDetail{
		ID: "ds-001",
		Properties: map[string]notion.DatabaseProperty{
			"Status": {Type: "select"},
		},
	}
	client.pages["page-001"] = &notion.Page{
		ID:             "page-001",
		LastEditedTime: "2024-01-01T00:00:00Z",
	}

	result, err := PushDatabase(PushOptions{
		Client:     client,
		FolderPath: dir,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pushed != 1 {
		t.Errorf("expected 1 pushed, got %d", result.Pushed)
	}
	if len(client.updateRequests) != 1 {
		t.Fatalf("expected 1 UpdatePage call, got %d", len(client.updateRequests))
	}
	if _, ok := client.updateRequests[0].Properties["Status"]; !ok {
		t.Error("expected Status in payload (proving schema was loaded from data source)")
	}
}

// When the data source returns no properties, push must fail with a clear
// error rather than silently producing an empty payload. This locks in the
// failure mode that motivated the GetDataSource fix in the first place.
func TestPushDatabase_DataSourceWithEmptySchemaErrors(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMetaWithDataSource(t, dir, "db-001", "ds-001")

	md := "---\n" +
		"notion-id: page-001\n" +
		"notion-last-edited: 2024-01-01T00:00:00Z\n" +
		"---\n"
	if err := os.WriteFile(filepath.Join(dir, "page-001.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	client.dataSources["ds-001"] = &notion.DataSourceDetail{ID: "ds-001"} // no Properties

	_, err := PushDatabase(PushOptions{
		Client:     client,
		FolderPath: dir,
	}, nil)
	if err == nil {
		t.Fatal("expected error when data source has empty schema, got nil")
	}
	if !strings.Contains(err.Error(), "no property schema") {
		t.Errorf("expected error to mention missing schema, got: %v", err)
	}
}
