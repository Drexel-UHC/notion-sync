package sync

import (
	"encoding/json"
	"strings"
	"testing"
)

// Phase 4 (DAG n41) — the run summary is the single agent-readable sink every
// terminal path drains into. These tests pin the status rules (clean / partial
// / halted / cancelled) and the JSON shape from dag-v1.4.0.mmd. Status rules
// (DAG header): clean = no failed AND no halted; partial = failed, no halted;
// halted = any halt (validation or auth); cancelled = user declined.

// Tracer: a run where every queued row pushed and nothing failed or halted is
// "clean", and the pushed rows carry their file + the fields that were sent.
func TestRunSummary_n41_CleanStatusWhenAllPushed(t *testing.T) {
	result := &PushResult{
		PushedRows: []PushedEntry{
			{File: "alpha.md", Fields: []string{"title", "Status"}},
		},
	}

	s := result.Summary()

	if s.Status != "clean" {
		t.Errorf("status = %q, want clean", s.Status)
	}
	if len(s.Pushed) != 1 || s.Pushed[0].File != "alpha.md" {
		t.Fatalf("pushed = %+v, want one entry for alpha.md", s.Pushed)
	}
	if got := s.Pushed[0].Fields; len(got) != 2 || got[0] != "title" || got[1] != "Status" {
		t.Errorf("pushed fields = %v, want [title Status]", got)
	}
	// JSON marshals cleanly (sanity that the contract serializes at all).
	if _, err := json.Marshal(s); err != nil {
		t.Fatalf("marshal: %v", err)
	}
}

// A run with at least one continue-class failure and no halt is "partial", and
// each failure carries file + reason + fix for the agent to act on.
func TestRunSummary_n41_PartialStatusWhenAnyFailed(t *testing.T) {
	result := &PushResult{
		PushedRows: []PushedEntry{{File: "ok.md", Fields: []string{"title"}}},
		FailedRows: []FailedEntry{
			{File: "bad.md", Reason: "Notion rejected the title", Fix: "shorten it and re-push"},
		},
	}

	s := result.Summary()

	if s.Status != "partial" {
		t.Errorf("status = %q, want partial", s.Status)
	}
	if len(s.Failed) != 1 {
		t.Fatalf("failed = %+v, want one entry", s.Failed)
	}
	f := s.Failed[0]
	if f.File != "bad.md" || f.Reason == "" || f.Fix == "" {
		t.Errorf("failed entry = %+v, want file+reason+fix populated", f)
	}
}

// A validation-gate halt makes the run "halted", and every halt-class file maps
// to a halted entry tagged phase "validation" with a basename, reason, and fix.
func TestRunSummary_n41_HaltedStatusFromValidation(t *testing.T) {
	result := &PushResult{
		Halted: true,
		Halts: []FileClassification{
			{Path: "/abs/folder/stray.md", Class: ClassHaltUnexpected, Reason: "no notion-id"},
			{Path: "/abs/folder/conflict.md", Class: ClassHaltConflict, Reason: "Notion moved ahead"},
		},
	}

	s := result.Summary()

	if s.Status != "halted" {
		t.Errorf("status = %q, want halted", s.Status)
	}
	if len(s.Halted) != 2 {
		t.Fatalf("halted = %+v, want 2 entries", s.Halted)
	}
	for _, h := range s.Halted {
		if h.Phase != "validation" {
			t.Errorf("phase = %q, want validation, entry %+v", h.Phase, h)
		}
		if h.File == "" || h.Reason == "" || h.Fix == "" {
			t.Errorf("halted entry missing file/reason/fix: %+v", h)
		}
	}
	// Basename only — no leaking the user's absolute paths into the contract.
	if s.Halted[0].File != "stray.md" {
		t.Errorf("file = %q, want basename stray.md", s.Halted[0].File)
	}
}

// #103 Option A: a conflict halt entry carries the per-cell diff so an agent gets
// the same local-vs-Notion evidence the human view prints; non-conflict halts have
// cells == [] (never null) so an agent can index without a presence check.
func TestRunSummary_103_ConflictHaltCarriesCellDiff(t *testing.T) {
	result := &PushResult{
		Halted: true,
		Halts: []FileClassification{
			{
				Path:  "/abs/folder/row.md",
				Class: ClassHaltConflict,
				Reason: "Notion's row has changed since last sync",
				CellDiffs: []CellDiff{
					{Field: "Score", Local: "555", Notion: "400"},
				},
			},
			{Path: "/abs/folder/stray.md", Class: ClassHaltUnexpected, Reason: "no notion-id"},
		},
	}

	s := result.Summary()
	if len(s.Halted) != 2 {
		t.Fatalf("halted = %+v, want 2 entries", s.Halted)
	}
	conflict := s.Halted[0]
	if len(conflict.Cells) != 1 || conflict.Cells[0] != (CellDiff{Field: "Score", Local: "555", Notion: "400"}) {
		t.Errorf("conflict cells = %+v, want one Score diff", conflict.Cells)
	}
	// Non-conflict halt: cells present and empty (marshals [], not null).
	stray := s.Halted[1]
	if stray.Cells == nil {
		t.Errorf("non-conflict halt cells must be [] not nil, got %+v", stray)
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"cells":[{"field":"Score","local":"555","notion":"400"}]`) {
		t.Errorf("expected conflict cells in JSON, got:\n%s", b)
	}
	if !strings.Contains(string(b), `"cells":[]`) {
		t.Errorf("expected empty cells array for non-conflict halt, got:\n%s", b)
	}
}

// A run-wide auth failure (401/403, DAG n34h) is "halted" with a single halted
// entry tagged phase "auth" carrying the auth reason.
func TestRunSummary_n41_HaltedStatusFromAuth(t *testing.T) {
	result := &PushResult{
		Pushed:     1,
		AuthHalted: true,
		AuthError:  "authentication failed (token invalid) — check the API key has write access",
	}

	s := result.Summary()

	if s.Status != "halted" {
		t.Errorf("status = %q, want halted", s.Status)
	}
	if len(s.Halted) != 1 {
		t.Fatalf("halted = %+v, want one auth entry", s.Halted)
	}
	h := s.Halted[0]
	if h.Phase != "auth" {
		t.Errorf("phase = %q, want auth", h.Phase)
	}
	if h.Reason == "" || h.Fix == "" {
		t.Errorf("auth halted entry missing reason/fix: %+v", h)
	}
}

// A run the user declined at the confirmation gate is "cancelled" with every
// array empty — nothing was inspected or pushed.
func TestRunSummary_n41_CancelledStatus(t *testing.T) {
	s := (&PushResult{Cancelled: true}).Summary()

	if s.Status != "cancelled" {
		t.Errorf("status = %q, want cancelled", s.Status)
	}
	if len(s.Pushed) != 0 || len(s.SkippedNoOp) != 0 || len(s.SkippedNonRow) != 0 ||
		len(s.Failed) != 0 || len(s.Halted) != 0 {
		t.Errorf("cancelled summary should have all-empty arrays, got %+v", s)
	}
}

// When a run both fails some rows AND halts, "halted" wins — the halt is the
// headline the agent must act on first (DAG: halted = any halt, regardless of
// failures).
func TestRunSummary_n41_HaltedTakesPrecedenceOverFailed(t *testing.T) {
	result := &PushResult{
		FailedRows: []FailedEntry{{File: "bad.md", Reason: "x", Fix: "y"}},
		Halted:     true,
		Halts:      []FileClassification{{Path: "conflict.md", Class: ClassHaltConflict, Reason: "moved"}},
	}

	if s := result.Summary(); s.Status != "halted" {
		t.Errorf("status = %q, want halted (precedence over partial)", s.Status)
	}
}

// skippedNoOp (no-op rows) and skippedNonRow (AGENTS.md / deleted) map straight
// through with their respective shapes.
func TestRunSummary_n41_SkippedNoOpAndNonRowPopulated(t *testing.T) {
	result := &PushResult{
		SkippedNoOpFiles: []string{"unchanged.md"},
		SkippedNonRow: []SkippedNonRowEntry{
			{File: "AGENTS.md", Reason: "AGENTS.md"},
			{File: "gone.md", Reason: "notion-deleted"},
		},
	}

	s := result.Summary()

	if s.Status != "clean" {
		t.Errorf("status = %q, want clean (skips are not failures)", s.Status)
	}
	if len(s.SkippedNoOp) != 1 || s.SkippedNoOp[0] != "unchanged.md" {
		t.Errorf("skippedNoOp = %v, want [unchanged.md]", s.SkippedNoOp)
	}
	if len(s.SkippedNonRow) != 2 {
		t.Fatalf("skippedNonRow = %+v, want 2 entries", s.SkippedNonRow)
	}
	if s.SkippedNonRow[0].Reason != "AGENTS.md" || s.SkippedNonRow[1].Reason != "notion-deleted" {
		t.Errorf("skippedNonRow reasons = %+v, want AGENTS.md + notion-deleted", s.SkippedNonRow)
	}
}

// Agent-critical: empty result serializes arrays as [] not null, so consumers
// can index without a nil check.
func TestRunSummary_n41_EmptyArraysSerializeAsBracketsNotNull(t *testing.T) {
	b, err := json.Marshal((&PushResult{}).Summary())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	for _, key := range []string{
		`"pushed":[]`, `"skippedNoOp":[]`, `"skippedNonRow":[]`, `"failed":[]`, `"halted":[]`,
	} {
		if !strings.Contains(got, key) {
			t.Errorf("expected %s in JSON, got:\n%s", key, got)
		}
	}
}
