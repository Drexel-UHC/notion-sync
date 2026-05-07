package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ran-codes/notion-sync/internal/notion"
)

// n21a — AGENTS.md is the generated downstream-agent guide; never a Notion
// row. Validation must classify it as skip (not halt-unexpected) so its
// presence in a synced folder doesn't abort the run.
func TestValidate_n21a_AgentsMDClassifiedAsSkip(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Agent guide\n"), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	report, err := ValidatePushQueue(client, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Files) != 1 {
		t.Fatalf("expected 1 classified file, got %d", len(report.Files))
	}
	got := report.Files[0]
	if filepath.Base(got.Path) != "AGENTS.md" {
		t.Errorf("expected AGENTS.md, got %s", got.Path)
	}
	if got.Class != ClassSkipAgentsMD {
		t.Errorf("expected ClassSkipAgentsMD, got %v", got.Class)
	}
	if report.Halted {
		t.Error("AGENTS.md alone must not halt the run")
	}
}

// n21b — `notion-deleted: true` marks a soft-deleted row. Push must skip,
// not halt: the user has already accepted the row is gone, and the next
// refresh will reconcile.
func TestValidate_n21b_DeletedClassifiedAsSkip(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	md := "---\nnotion-id: page-001\nnotion-deleted: true\nnotion-last-edited: 2024-01-01T00:00:00Z\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "page-001.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	report, err := ValidatePushQueue(client, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Files) != 1 {
		t.Fatalf("expected 1 classified file, got %d", len(report.Files))
	}
	if report.Files[0].Class != ClassSkipDeleted {
		t.Errorf("expected ClassSkipDeleted, got %v", report.Files[0].Class)
	}
	if report.Halted {
		t.Error("deleted row must not halt the run")
	}
}

// n21e — a .md without `notion-id` and not named AGENTS.md is unexpected:
// it doesn't belong to the synced database. Push must HALT the run rather
// than silently ignoring (could be a misplaced file the user thinks is
// being synced) or pushing (no row to push to).
func TestValidate_n21e_StrayMDClassifiedAsHaltUnexpected(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	if err := os.WriteFile(filepath.Join(dir, "stray.md"), []byte("---\ntitle: stray\n---\nbody\n"), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	report, err := ValidatePushQueue(client, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Files) != 1 {
		t.Fatalf("expected 1 classified file, got %d", len(report.Files))
	}
	if report.Files[0].Class != ClassHaltUnexpected {
		t.Errorf("expected ClassHaltUnexpected, got %v", report.Files[0].Class)
	}
	if report.Files[0].Reason == "" {
		t.Error("halt classifications must populate Reason for the user")
	}
	if !report.Halted {
		t.Error("a halt-class file must flip Halted=true")
	}
}

// n21c — file linked to Notion (notion-id present) and Notion's current
// last_edited_time matches the local notion-last-edited stamp. Safe to push.
func TestValidate_n21c_MatchedTimestampsClassifiedAsReady(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	md := "---\nnotion-id: page-001\nnotion-last-edited: 2024-01-01T00:00:00Z\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "page-001.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	client.pages["page-001"] = &notion.Page{ID: "page-001", LastEditedTime: "2024-01-01T00:00:00Z"}

	report, err := ValidatePushQueue(client, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Files) != 1 {
		t.Fatalf("expected 1 classified file, got %d", len(report.Files))
	}
	if report.Files[0].Class != ClassReady {
		t.Errorf("expected ClassReady, got %v", report.Files[0].Class)
	}
	if report.Files[0].NotionID != "page-001" {
		t.Errorf("expected NotionID=page-001, got %q", report.Files[0].NotionID)
	}
	if report.Halted {
		t.Error("a clean file must not halt the run")
	}
}

// n21d — file linked, but Notion's last_edited_time has advanced past the
// local stamp. Pushing would clobber whatever changed in Notion since the
// last sync. HALT (not skip) so the user reconciles before any writes.
func TestValidate_n21d_MismatchedTimestampsClassifiedAsHaltConflict(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	md := "---\nnotion-id: page-001\nnotion-last-edited: 2024-01-01T00:00:00Z\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "page-001.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	client := newMockClient()
	// Notion's row has advanced past the local stamp.
	client.pages["page-001"] = &notion.Page{ID: "page-001", LastEditedTime: "2024-06-01T00:00:00Z"}

	report, err := ValidatePushQueue(client, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Files) != 1 {
		t.Fatalf("expected 1 classified file, got %d", len(report.Files))
	}
	got := report.Files[0]
	if got.Class != ClassHaltConflict {
		t.Errorf("expected ClassHaltConflict, got %v", got.Class)
	}
	if got.Reason == "" {
		t.Error("conflict halt must populate Reason")
	}
	if got.NotionID != "page-001" {
		t.Errorf("expected NotionID=page-001, got %q", got.NotionID)
	}
	if !report.Halted {
		t.Error("a conflict must flip Halted=true")
	}
}

// n21f — GetPage failed during the validation read (network error, 5xx,
// timeout). The row cannot be safely classified, so HALT rather than
// optimistically push (could clobber unread Notion changes) or skip
// (would silently drop the row). Per backlog/network-blip-validation-halt.md.
func TestValidate_n21f_GetPageErrorClassifiedAsHaltUnreachable(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	md := "---\nnotion-id: page-missing\nnotion-last-edited: 2024-01-01T00:00:00Z\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "page-missing.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock client returns "page not found" for unknown IDs — same code path
	// as a network/5xx failure for our purposes (any GetPage error halts).
	client := newMockClient()

	report, err := ValidatePushQueue(client, dir)
	if err != nil {
		t.Fatalf("validation must surface GetPage failures via classification, not err: %v", err)
	}

	if len(report.Files) != 1 {
		t.Fatalf("expected 1 classified file, got %d", len(report.Files))
	}
	got := report.Files[0]
	if got.Class != ClassHaltUnreachable {
		t.Errorf("expected ClassHaltUnreachable, got %v", got.Class)
	}
	if got.Reason == "" {
		t.Error("unreachable halt must populate Reason")
	}
	if got.NotionID != "page-missing" {
		t.Errorf("expected NotionID=page-missing, got %q", got.NotionID)
	}
	if !report.Halted {
		t.Error("an unreachable file must flip Halted=true")
	}
}

// n22 / n22a — every halt across every file must be enumerated in one pass.
// Surfacing only the first halt would force the user into a fix-one-then-rerun
// loop; surfacing all of them lets them fix the whole batch at once. This
// also confirms that ready/skip files coexist with halt files in the report
// (the gate is aggregate, not per-file fail-fast).
func TestValidate_n22a_AnyHaltMakesReportHalted_AllHaltsEnumerated(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	files := map[string]string{
		"AGENTS.md":      "# guide\n",
		"ready.md":       "---\nnotion-id: page-ready\nnotion-last-edited: 2024-01-01T00:00:00Z\n---\n",
		"deleted.md":     "---\nnotion-id: page-deleted\nnotion-deleted: true\n---\n",
		"conflict.md":    "---\nnotion-id: page-conflict\nnotion-last-edited: 2024-01-01T00:00:00Z\n---\n",
		"unexpected.md":  "---\ntitle: stray\n---\n",
		"unreachable.md": "---\nnotion-id: page-missing\nnotion-last-edited: 2024-01-01T00:00:00Z\n---\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	client := newMockClient()
	client.pages["page-ready"] = &notion.Page{ID: "page-ready", LastEditedTime: "2024-01-01T00:00:00Z"}
	client.pages["page-conflict"] = &notion.Page{ID: "page-conflict", LastEditedTime: "2024-06-01T00:00:00Z"}
	// page-missing is intentionally absent → GetPage errors → unreachable halt.

	report, err := ValidatePushQueue(client, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Files) != 6 {
		t.Fatalf("expected 6 classifications, got %d", len(report.Files))
	}
	if !report.Halted {
		t.Fatal("any halt-class file must flip Halted=true")
	}

	got := map[string]Classification{}
	for _, f := range report.Files {
		got[filepath.Base(f.Path)] = f.Class
	}
	want := map[string]Classification{
		"AGENTS.md":      ClassSkipAgentsMD,
		"deleted.md":     ClassSkipDeleted,
		"ready.md":       ClassReady,
		"conflict.md":    ClassHaltConflict,
		"unexpected.md":  ClassHaltUnexpected,
		"unreachable.md": ClassHaltUnreachable,
	}
	for name, wantClass := range want {
		if got[name] != wantClass {
			t.Errorf("%s: got class %v, want %v", name, got[name], wantClass)
		}
	}
}

// n22b — every file is skip or ready, zero halts → gate clears, Halted=false.
// This is the only path that lets the run proceed to phase 3 (push).
func TestValidate_n22b_NoHaltsMakesReportNotHalted(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")

	files := map[string]string{
		"AGENTS.md":  "# guide\n",
		"deleted.md": "---\nnotion-id: page-deleted\nnotion-deleted: true\n---\n",
		"ready-1.md": "---\nnotion-id: page-1\nnotion-last-edited: 2024-01-01T00:00:00Z\n---\n",
		"ready-2.md": "---\nnotion-id: page-2\nnotion-last-edited: 2024-01-01T00:00:00Z\n---\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	client := newMockClient()
	client.pages["page-1"] = &notion.Page{ID: "page-1", LastEditedTime: "2024-01-01T00:00:00Z"}
	client.pages["page-2"] = &notion.Page{ID: "page-2", LastEditedTime: "2024-01-01T00:00:00Z"}

	report, err := ValidatePushQueue(client, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Halted {
		t.Fatal("zero halts must leave Halted=false (gate clears, run proceeds)")
	}
	if len(report.Files) != 4 {
		t.Errorf("expected 4 classifications, got %d", len(report.Files))
	}
	for _, f := range report.Files {
		if f.Class == ClassHaltConflict || f.Class == ClassHaltUnexpected || f.Class == ClassHaltUnreachable {
			t.Errorf("%s: unexpected halt class %v", f.Path, f.Class)
		}
	}
}
