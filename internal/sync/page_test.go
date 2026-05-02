package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ran-codes/notion-sync/internal/notion"
)

func TestGetPageTitle(t *testing.T) {
	tests := []struct {
		name  string
		page  notion.Page
		want  string
	}{
		{
			name: "extracts title from title property",
			page: notion.Page{
				Properties: map[string]notion.Property{
					"Name": {
						Type: "title",
						Title: []notion.RichText{
							{Type: "text", PlainText: "Hello World", Text: &notion.TextContent{Content: "Hello World"}},
						},
					},
				},
			},
			want: "Hello World",
		},
		{
			name: "returns Untitled when no title property",
			page: notion.Page{
				Properties: map[string]notion.Property{
					"Status": {Type: "select"},
				},
			},
			want: "Untitled",
		},
		{
			name: "returns Untitled when title is empty",
			page: notion.Page{
				Properties: map[string]notion.Property{
					"Name": {
						Type:  "title",
						Title: []notion.RichText{},
					},
				},
			},
			want: "Untitled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPageTitle(&tt.page)
			if got != tt.want {
				t.Errorf("getPageTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMapPropertiesToFrontmatter(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	floatPtr := func(f float64) *float64 { return &f }

	props := map[string]notion.Property{
		"Name": {
			Type: "title",
			Title: []notion.RichText{
				{Type: "text", PlainText: "Test", Text: &notion.TextContent{Content: "Test"}},
			},
		},
		"Description": {
			Type: "rich_text",
			RichText: []notion.RichText{
				{Type: "text", PlainText: "A description", Text: &notion.TextContent{Content: "A description"}},
			},
		},
		"Count": {
			Type:   "number",
			Number: floatPtr(42),
		},
		"EmptyNumber": {
			Type:   "number",
			Number: nil,
		},
		"Category": {
			Type:   "select",
			Select: &notion.SelectValue{Name: "Engineering"},
		},
		"EmptySelect": {
			Type:   "select",
			Select: nil,
		},
		"Tags": {
			Type:        "multi_select",
			MultiSelect: []notion.SelectValue{{Name: "go"}, {Name: "cli"}},
		},
		"EmptyTags": {
			Type:        "multi_select",
			MultiSelect: []notion.SelectValue{},
		},
		"Status": {
			Type:   "status",
			Status: &notion.SelectValue{Name: "In Progress"},
		},
		"Done": {
			Type:     "checkbox",
			Checkbox: true,
		},
		"NotDone": {
			Type:     "checkbox",
			Checkbox: false,
		},
		"Website": {
			Type: "url",
			URL:  strPtr("https://example.com"),
		},
		"EmptyURL": {
			Type: "url",
			URL:  nil,
		},
		"Email": {
			Type:  "email",
			Email: strPtr("test@example.com"),
		},
		"Phone": {
			Type:        "phone_number",
			PhoneNumber: strPtr("+1234567890"),
		},
		"DueDate": {
			Type: "date",
			Date: &notion.DateValue{Start: "2025-01-15"},
		},
		"DateRange": {
			Type: "date",
			Date: &notion.DateValue{Start: "2025-01-01", End: strPtr("2025-01-31")},
		},
		"EmptyDate": {
			Type: "date",
			Date: nil,
		},
		"Related": {
			Type:     "relation",
			Relation: []notion.Relation{{ID: "abc-123"}, {ID: "def-456"}},
		},
		"EmptyRelation": {
			Type:     "relation",
			Relation: []notion.Relation{},
		},
		"Assignee": {
			Type:   "people",
			People: []notion.Person{{ID: "p1", Name: strPtr("Alice")}},
		},
		"CreatedAt": {
			Type:        "created_time",
			CreatedTime: "2025-01-01T00:00:00Z",
		},
		"EditedAt": {
			Type:           "last_edited_time",
			LastEditedTime: "2025-06-01T12:00:00Z",
		},
	}

	fm := map[string]interface{}{}
	mapPropertiesToFrontmatter(props, fm, false)

	// Title should be included in frontmatter
	if fm["Name"] != "Test" {
		t.Errorf("Name = %v, want \"Test\"", fm["Name"])
	}

	assertFM := func(key string, want interface{}) {
		t.Helper()
		got, ok := fm[key]
		if !ok {
			t.Errorf("missing key %q", key)
			return
		}
		// Handle slice comparisons
		switch w := want.(type) {
		case []interface{}:
			g, ok := got.([]interface{})
			if !ok {
				t.Errorf("%s: not a []interface{}, got %T", key, got)
				return
			}
			if len(g) != len(w) {
				t.Errorf("%s: len = %d, want %d", key, len(g), len(w))
				return
			}
			for i := range w {
				if g[i] != w[i] {
					t.Errorf("%s[%d] = %v, want %v", key, i, g[i], w[i])
				}
			}
		default:
			if got != want {
				t.Errorf("%s = %v (%T), want %v (%T)", key, got, got, want, want)
			}
		}
	}

	assertNil := func(key string) {
		t.Helper()
		got, ok := fm[key]
		if !ok {
			t.Errorf("missing key %q", key)
			return
		}
		if got != nil {
			t.Errorf("%s = %v, want nil", key, got)
		}
	}

	assertFM("Description", "A description")
	assertFM("Count", float64(42))
	assertNil("EmptyNumber")
	assertFM("Category", "Engineering")
	assertNil("EmptySelect")
	assertFM("Tags", []interface{}{"go", "cli"})
	assertFM("EmptyTags", []interface{}{})
	assertFM("Status", "In Progress")
	assertFM("Done", true)
	assertFM("NotDone", false)
	assertFM("Website", "https://example.com")
	assertNil("EmptyURL")
	assertFM("Email", "test@example.com")
	assertFM("Phone", "+1234567890")
	assertFM("DueDate", "2025-01-15")
	assertFM("DateRange", "2025-01-01 → 2025-01-31")
	assertNil("EmptyDate")
	assertFM("Related", []interface{}{"abc-123", "def-456"})
	assertFM("EmptyRelation", []interface{}{})
	assertFM("Assignee", []interface{}{"Alice"})
	assertFM("CreatedAt", "2025-01-01T00:00:00Z")
	assertFM("EditedAt", "2025-06-01T12:00:00Z")
}

func TestMapPropertiesToFrontmatter_Files(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	_ = strPtr // suppress unused if needed

	props := map[string]notion.Property{
		"Attachments": {
			Type: "files",
			Files: []notion.File{
				{Name: "doc.pdf", Type: "file", File: &notion.FileURL{URL: "https://s3.example.com/doc.pdf"}},
				{Name: "logo.png", Type: "external", External: &notion.ExternalURL{URL: "https://example.com/logo.png"}},
			},
		},
		"EmptyFiles": {
			Type:  "files",
			Files: []notion.File{},
		},
	}

	fm := map[string]interface{}{}
	mapPropertiesToFrontmatter(props, fm, false)

	// Check files
	files, ok := fm["Attachments"].([]interface{})
	if !ok {
		t.Fatalf("Attachments: not a []interface{}, got %T", fm["Attachments"])
	}
	if len(files) != 2 {
		t.Fatalf("Attachments: len = %d, want 2", len(files))
	}
	if files[0] != "https://s3.example.com/doc.pdf" {
		t.Errorf("Attachments[0] = %v", files[0])
	}
	if files[1] != "https://example.com/logo.png" {
		t.Errorf("Attachments[1] = %v", files[1])
	}

	// Empty files
	emptyFiles, ok := fm["EmptyFiles"].([]interface{})
	if !ok {
		t.Fatalf("EmptyFiles: not a []interface{}, got %T", fm["EmptyFiles"])
	}
	if len(emptyFiles) != 0 {
		t.Errorf("EmptyFiles: len = %d, want 0", len(emptyFiles))
	}
}

func TestMapPropertiesToFrontmatter_Files_StripPresigned(t *testing.T) {
	presigned := "https://prod-files-secure.s3.us-west-2.amazonaws.com/abc/uuid/file.pdf?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Date=20260428T150234Z&X-Amz-Signature=9af1bc"
	external := "https://my-cdn.example.org/logo.png"

	props := map[string]notion.Property{
		"PDF": {
			Type: "files",
			Files: []notion.File{
				{Name: "file.pdf", Type: "file", File: &notion.FileURL{URL: presigned}},
				{Name: "logo.png", Type: "external", External: &notion.ExternalURL{URL: external}},
			},
		},
	}

	// Strip=true: presigned URL is reduced to its stable path; external URL untouched.
	fm := map[string]interface{}{}
	mapPropertiesToFrontmatter(props, fm, true)

	files, ok := fm["PDF"].([]interface{})
	if !ok || len(files) != 2 {
		t.Fatalf("PDF: unexpected value: %#v", fm["PDF"])
	}
	wantStripped := "https://prod-files-secure.s3.us-west-2.amazonaws.com/abc/uuid/file.pdf"
	if files[0] != wantStripped {
		t.Errorf("PDF[0] = %v, want %v", files[0], wantStripped)
	}
	if files[1] != external {
		t.Errorf("PDF[1] = %v, want %v (external URL must not be modified)", files[1], external)
	}

	// Strip=false: opt-out preserves the full presigned URL.
	fm2 := map[string]interface{}{}
	mapPropertiesToFrontmatter(props, fm2, false)
	files2 := fm2["PDF"].([]interface{})
	if files2[0] != presigned {
		t.Errorf("with strip=false, PDF[0] should be unchanged, got %v", files2[0])
	}
}

func TestMapPropertiesToFrontmatter_PeopleFallbackToID(t *testing.T) {
	props := map[string]notion.Property{
		"Reviewers": {
			Type: "people",
			People: []notion.Person{
				{ID: "user-id-1", Name: nil},
			},
		},
	}

	fm := map[string]interface{}{}
	mapPropertiesToFrontmatter(props, fm, false)

	people, ok := fm["Reviewers"].([]interface{})
	if !ok {
		t.Fatalf("Reviewers: not a []interface{}")
	}
	if len(people) != 1 || people[0] != "user-id-1" {
		t.Errorf("Reviewers = %v, want [user-id-1]", people)
	}
}

func TestMapPropertiesToFrontmatter_UniqueID(t *testing.T) {
	props := map[string]notion.Property{
		"ID": {
			Type:     "unique_id",
			UniqueID: &notion.UniqueIDValue{Prefix: "TASK", Number: 42},
		},
		"NoPrefixID": {
			Type:     "unique_id",
			UniqueID: &notion.UniqueIDValue{Prefix: "", Number: 7},
		},
		"NilID": {
			Type:     "unique_id",
			UniqueID: nil,
		},
	}

	fm := map[string]interface{}{}
	mapPropertiesToFrontmatter(props, fm, false)

	if fm["ID"] != "TASK-42" {
		t.Errorf("ID = %v, want TASK-42", fm["ID"])
	}
	if fm["NoPrefixID"] != "7" {
		t.Errorf("NoPrefixID = %v, want 7", fm["NoPrefixID"])
	}
	if fm["NilID"] != nil {
		t.Errorf("NilID = %v, want nil", fm["NilID"])
	}
}

func TestMapPropertiesToFrontmatter_CreatedBy(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	props := map[string]notion.Property{
		"Creator": {
			Type:      "created_by",
			CreatedBy: &notion.Person{ID: "user-1", Name: strPtr("Alice")},
		},
		"CreatorNoName": {
			Type:      "created_by",
			CreatedBy: &notion.Person{ID: "user-2", Name: nil},
		},
		"NilCreator": {
			Type:      "created_by",
			CreatedBy: nil,
		},
	}

	fm := map[string]interface{}{}
	mapPropertiesToFrontmatter(props, fm, false)

	if fm["Creator"] != "Alice" {
		t.Errorf("Creator = %v, want Alice", fm["Creator"])
	}
	if fm["CreatorNoName"] != "user-2" {
		t.Errorf("CreatorNoName = %v, want user-2", fm["CreatorNoName"])
	}
	if fm["NilCreator"] != nil {
		t.Errorf("NilCreator = %v, want nil", fm["NilCreator"])
	}
}

func TestMapPropertiesToFrontmatter_LastEditedBy(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	props := map[string]notion.Property{
		"Editor": {
			Type:         "last_edited_by",
			LastEditedBy: &notion.Person{ID: "user-1", Name: strPtr("Bob")},
		},
		"EditorNoName": {
			Type:         "last_edited_by",
			LastEditedBy: &notion.Person{ID: "user-3", Name: nil},
		},
		"NilEditor": {
			Type:         "last_edited_by",
			LastEditedBy: nil,
		},
	}

	fm := map[string]interface{}{}
	mapPropertiesToFrontmatter(props, fm, false)

	if fm["Editor"] != "Bob" {
		t.Errorf("Editor = %v, want Bob", fm["Editor"])
	}
	if fm["EditorNoName"] != "user-3" {
		t.Errorf("EditorNoName = %v, want user-3", fm["EditorNoName"])
	}
	if fm["NilEditor"] != nil {
		t.Errorf("NilEditor = %v, want nil", fm["NilEditor"])
	}
}

func TestMapPropertiesToFrontmatter_UnknownTypeSkipped(t *testing.T) {
	props := map[string]notion.Property{
		"Formula": {Type: "formula"},
		"Rollup":  {Type: "rollup"},
	}

	fm := map[string]interface{}{}
	mapPropertiesToFrontmatter(props, fm, false)

	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter for unknown types, got %v", fm)
	}
}

func TestFreezePage_DoesNotEmitFrozenAt(t *testing.T) {
	// notion-frozen-at was removed because it added per-file diff churn on every
	// refresh without driving any logic. The database-level lastSyncedAt in
	// _database.json is the authoritative "when was this synced" record.
	dir := t.TempDir()
	page := testPage("page-id-frozen", "Frozen Test", "2025-01-01T00:00:00Z")
	client := newMockClient()
	client.pages["page-id-frozen"] = &page
	client.blocks["page-id-frozen"] = []notion.Block{}

	_, err := FreezePage(FreezePageOptions{
		Client:       client,
		NotionID:     "page-id-frozen",
		OutputFolder: dir,
		DatabaseID:   "db-1",
		Page:         &page,
	})
	if err != nil {
		t.Fatalf("FreezePage: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "page-id-frozen.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(data), "notion-frozen-at") {
		t.Errorf("freshly-written file should not contain notion-frozen-at:\n%s", data)
	}
}

func TestFreezePage_CanonicalizesNotionURLInFrontmatter(t *testing.T) {
	dir := t.TempDir()
	page := testPage("1234567890abcdef1234567890abcdef", "Url Test", "2025-01-01T00:00:00Z")
	page.URL = "https://www.notion.so/Url-Test-1234567890abcdef1234567890abcdef"
	client := newMockClient()
	client.pages[page.ID] = &page
	client.blocks[page.ID] = []notion.Block{}

	_, err := FreezePage(FreezePageOptions{
		Client:       client,
		NotionID:     page.ID,
		OutputFolder: dir,
		DatabaseID:   "db-1",
		Page:         &page,
	})
	if err != nil {
		t.Fatalf("FreezePage: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, page.ID+".md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "https://app.notion.com/p/1234567890abcdef1234567890abcdef"
	if !strings.Contains(string(data), want) {
		t.Errorf("frontmatter missing canonical URL %q:\n%s", want, data)
	}
	if strings.Contains(string(data), "https://www.notion.so/") {
		t.Errorf("frontmatter still contains legacy notion.so URL:\n%s", data)
	}
}

func TestFreezePage_TrailingNewline(t *testing.T) {
	dir := t.TempDir()
	page := testPage("page-id-001", "My Page", "2025-01-01T00:00:00Z")
	client := newMockClient()
	client.pages["page-id-001"] = &page
	// paragraph block so md body ends without \n (joins produce "Hello world" with no newline)
	client.blocks["page-id-001"] = []notion.Block{
		{
			ID:   "block-1",
			Type: "paragraph",
			Paragraph: &notion.ParagraphBlock{
				RichText: []notion.RichText{
					{Type: "text", PlainText: "Hello world", Text: &notion.TextContent{Content: "Hello world"}},
				},
			},
		},
	}

	_, err := FreezePage(FreezePageOptions{
		Client:       client,
		NotionID:     "page-id-001",
		OutputFolder: dir,
		DatabaseID:   "db-123",
		Page:         &page,
	})
	if err != nil {
		t.Fatalf("FreezePage: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "page-id-001.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Errorf("md file does not end with newline; last byte = %q", data[len(data)-1])
	}
}
