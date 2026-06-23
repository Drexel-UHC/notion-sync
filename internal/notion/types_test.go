package notion

import (
	"encoding/json"
	"testing"
)

// Issue #90 prerequisite: DatabaseProperty must parse the allowed-option lists
// that Notion returns on GET /data_sources/{id} for select / multi_select /
// status properties. This pins the JSON contract (field names + nesting) the
// validation gate relies on to compare pushed values against the schema.
func TestDatabaseProperty_ParsesSelectStatusMultiSelectOptions(t *testing.T) {
	// Trimmed shape of a real /data_sources/{id} properties payload.
	raw := `{
		"Status": {
			"id": "abc",
			"name": "Status",
			"type": "status",
			"status": {
				"options": [
					{"id": "1", "name": "To Do", "color": "gray"},
					{"id": "2", "name": "Doing", "color": "blue"},
					{"id": "3", "name": "Done", "color": "green"}
				]
			}
		},
		"Priority": {
			"id": "def",
			"name": "Priority",
			"type": "select",
			"select": {
				"options": [
					{"id": "4", "name": "Low"},
					{"id": "5", "name": "High"}
				]
			}
		},
		"Tags": {
			"id": "ghi",
			"name": "Tags",
			"type": "multi_select",
			"multi_select": {
				"options": [
					{"id": "6", "name": "urgent"},
					{"id": "7", "name": "backlog"}
				]
			}
		},
		"Notes": {
			"id": "jkl",
			"name": "Notes",
			"type": "rich_text"
		}
	}`

	var props map[string]DatabaseProperty
	if err := json.Unmarshal([]byte(raw), &props); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	status := props["Status"]
	if status.Status == nil {
		t.Fatal("Status.Status (option config) was not parsed")
	}
	gotStatus := optionNames(status.Status.Options)
	wantStatus := []string{"To Do", "Doing", "Done"}
	if !equalSlice(gotStatus, wantStatus) {
		t.Errorf("status options: got %v, want %v", gotStatus, wantStatus)
	}

	sel := props["Priority"]
	if sel.Select == nil {
		t.Fatal("Priority.Select (option config) was not parsed")
	}
	if got := optionNames(sel.Select.Options); !equalSlice(got, []string{"Low", "High"}) {
		t.Errorf("select options: got %v", got)
	}

	multi := props["Tags"]
	if multi.MultiSelect == nil {
		t.Fatal("Tags.MultiSelect (option config) was not parsed")
	}
	if got := optionNames(multi.MultiSelect.Options); !equalSlice(got, []string{"urgent", "backlog"}) {
		t.Errorf("multi_select options: got %v", got)
	}

	// A non-option property carries nil configs (no spurious empty structs).
	notes := props["Notes"]
	if notes.Select != nil || notes.MultiSelect != nil || notes.Status != nil {
		t.Errorf("rich_text property should have nil option configs, got %+v", notes)
	}
}

func optionNames(options []SelectValue) []string {
	names := make([]string, 0, len(options))
	for _, o := range options {
		names = append(names, o.Name)
	}
	return names
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
