package sync

import (
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
	mapPropertiesToFrontmatter(props, fm)

	// Title should be skipped (used as filename)
	if _, ok := fm["Name"]; ok {
		t.Error("title property should not be in frontmatter")
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
	mapPropertiesToFrontmatter(props, fm)

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
	mapPropertiesToFrontmatter(props, fm)

	people, ok := fm["Reviewers"].([]interface{})
	if !ok {
		t.Fatalf("Reviewers: not a []interface{}")
	}
	if len(people) != 1 || people[0] != "user-id-1" {
		t.Errorf("Reviewers = %v, want [user-id-1]", people)
	}
}

func TestMapPropertiesToFrontmatter_UnknownTypeSkipped(t *testing.T) {
	props := map[string]notion.Property{
		"Formula": {Type: "formula"},
		"Rollup":  {Type: "rollup"},
	}

	fm := map[string]interface{}{}
	mapPropertiesToFrontmatter(props, fm)

	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter for unknown types, got %v", fm)
	}
}
