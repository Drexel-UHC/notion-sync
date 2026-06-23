package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ran-codes/notion-sync/internal/notion"
)

// selectProp builds a select/multi_select/status schema property with the given
// option names. propType picks which option config field is populated.
func optionProp(propType string, names ...string) notion.DatabaseProperty {
	opts := make([]notion.SelectValue, 0, len(names))
	for _, n := range names {
		opts = append(opts, notion.SelectValue{Name: n})
	}
	cfg := &notion.SelectConfig{Options: opts}
	p := notion.DatabaseProperty{Type: propType}
	switch propType {
	case "select":
		p.Select = cfg
	case "multi_select":
		p.MultiSelect = cfg
	case "status":
		p.Status = cfg
	}
	return p
}

// --- validateRowOptions unit tests (issue #90) ---

func TestValidateRowOptions_ValidSelectPasses(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{
		"Status": optionProp("select", "To Do", "Doing", "Done"),
	}
	fm := map[string]interface{}{"Status": "Done"}
	if reason, ok := validateRowOptions(fm, schema, false); !ok {
		t.Errorf("expected valid select to pass, got reason %q", reason)
	}
}

func TestValidateRowOptions_InvalidSelectHaltsWithClearReason(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{
		"Status": optionProp("select", "To Do", "Doing", "Done"),
	}
	fm := map[string]interface{}{"Status": "Doen"}
	reason, ok := validateRowOptions(fm, schema, false)
	if ok {
		t.Fatal("expected typo'd select value to fail validation")
	}
	// Acceptance criterion: exact reason shape.
	want := `"Doen" is not a valid option for "Status" (allowed: To Do, Doing, Done)`
	if reason != want {
		t.Errorf("reason mismatch:\n got: %s\nwant: %s", reason, want)
	}
}

func TestValidateRowOptions_ValidStatusPasses(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{
		"Stage": optionProp("status", "Backlog", "In Progress", "Shipped"),
	}
	fm := map[string]interface{}{"Stage": "In Progress"}
	if reason, ok := validateRowOptions(fm, schema, false); !ok {
		t.Errorf("expected valid status to pass, got reason %q", reason)
	}
}

func TestValidateRowOptions_InvalidStatusHalts(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{
		"Stage": optionProp("status", "Backlog", "In Progress", "Shipped"),
	}
	fm := map[string]interface{}{"Stage": "Shppd"}
	if _, ok := validateRowOptions(fm, schema, false); ok {
		t.Error("expected unknown status value to fail validation")
	}
}

// status has no opt-in: even with --allow-new-options, an unknown status value
// halts (Notion's API cannot create status options).
func TestValidateRowOptions_AllowNewOptionsStillHaltsStatus(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{
		"Stage": optionProp("status", "Backlog", "Shipped"),
	}
	fm := map[string]interface{}{"Stage": "Brand New"}
	if _, ok := validateRowOptions(fm, schema, true); ok {
		t.Error("unknown status must halt even with allowNewOptions=true")
	}
}

// select/multi_select opt-in: --allow-new-options lets an unknown value through
// (Notion auto-creates the option on push).
func TestValidateRowOptions_AllowNewOptionsPassesSelectAndMultiSelect(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{
		"Priority": optionProp("select", "Low", "High"),
		"Tags":     optionProp("multi_select", "urgent"),
	}
	fm := map[string]interface{}{
		"Priority": "Critical",                         // not in options
		"Tags":     []interface{}{"urgent", "new-tag"}, // new-tag not in options
	}
	if reason, ok := validateRowOptions(fm, schema, true); !ok {
		t.Errorf("allowNewOptions must pass unknown select/multi_select, got reason %q", reason)
	}
}

func TestValidateRowOptions_ValidMultiSelectPasses(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{
		"Tags": optionProp("multi_select", "a", "b", "c"),
	}
	fm := map[string]interface{}{"Tags": []interface{}{"a", "c"}}
	if reason, ok := validateRowOptions(fm, schema, false); !ok {
		t.Errorf("expected valid multi_select to pass, got reason %q", reason)
	}
}

func TestValidateRowOptions_InvalidMultiSelectMemberHalts(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{
		"Tags": optionProp("multi_select", "a", "b"),
	}
	fm := map[string]interface{}{"Tags": []interface{}{"a", "z"}}
	reason, ok := validateRowOptions(fm, schema, false)
	if ok {
		t.Fatal("expected unknown multi_select member to fail validation")
	}
	if !strings.Contains(reason, `"z"`) || !strings.Contains(reason, "Tags") {
		t.Errorf("reason should name the bad value and property, got %q", reason)
	}
}

// Empty/nil scalar means "clear the property" — never an invalid option.
func TestValidateRowOptions_EmptyAndNilAreClears(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{
		"Status": optionProp("select", "Done"),
		"Stage":  optionProp("status", "Shipped"),
	}
	for _, val := range []interface{}{"", nil} {
		fm := map[string]interface{}{"Status": val, "Stage": val}
		if reason, ok := validateRowOptions(fm, schema, false); !ok {
			t.Errorf("clearing value %v must pass, got reason %q", val, reason)
		}
	}
}

// A schema property the row doesn't set is never validated.
func TestValidateRowOptions_AbsentPropertyIgnored(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{
		"Status": optionProp("select", "Done"),
	}
	fm := map[string]interface{}{"OtherField": "whatever"}
	if reason, ok := validateRowOptions(fm, schema, false); !ok {
		t.Errorf("absent option property must not be validated, got reason %q", reason)
	}
}

// Multiple violations in one row are collected and joined deterministically
// (sorted) so the user fixes the whole row in one pass — no within-file fix loop.
func TestValidateRowOptions_CollectsAllViolationsSorted(t *testing.T) {
	schema := map[string]notion.DatabaseProperty{
		"Status":   optionProp("select", "Done"),
		"Priority": optionProp("select", "High"),
	}
	fm := map[string]interface{}{"Status": "Doen", "Priority": "Hihg"}
	reason, ok := validateRowOptions(fm, schema, false)
	if ok {
		t.Fatal("expected two violations to fail validation")
	}
	if !strings.Contains(reason, "Priority") || !strings.Contains(reason, "Status") {
		t.Errorf("expected both violations in reason, got %q", reason)
	}
	if !strings.Contains(reason, "; ") {
		t.Errorf("expected violations joined by '; ', got %q", reason)
	}
	// Sorted: the Priority violation (`"Hihg"...`) sorts before Status (`"Doen"`...)?
	// Sort is lexicographic on the full violation string; pin determinism by
	// running twice and comparing.
	reason2, _ := validateRowOptions(fm, schema, false)
	if reason != reason2 {
		t.Errorf("violation order must be deterministic: %q vs %q", reason, reason2)
	}
}

// classifyFolder integration: a row with an invalid option under the gate
// (schema supplied) classifies as ClassHaltInvalidOption and flips Halted —
// without ever calling GetPage (local check runs before the conflict probe).
func TestClassifyFolder_InvalidOptionHaltsBeforeConflictProbe(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	md := "---\nnotion-id: page-001\nnotion-last-edited: 2024-01-01T00:00:00Z\nStatus: Doen\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "page-001.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock has no page-001 → a GetPage would classify Unreachable. We expect
	// InvalidOption instead, proving the option check short-circuits before the
	// network probe.
	client := newMockClient()
	schema := map[string]notion.DatabaseProperty{
		"Status": optionProp("select", "To Do", "Doing", "Done"),
	}

	report, err := classifyFolder(dir, client, schema, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Files) != 1 {
		t.Fatalf("expected 1 classification, got %d", len(report.Files))
	}
	got := report.Files[0]
	if got.Class != ClassHaltInvalidOption {
		t.Errorf("expected ClassHaltInvalidOption, got %v", got.Class)
	}
	if !report.Halted {
		t.Error("invalid option must flip Halted=true")
	}
	if got.Reason == "" {
		t.Error("invalid-option halt must populate Reason")
	}
}
