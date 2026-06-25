package sync

import (
	"path/filepath"
	"strings"
)

// RunSummary is the agent-readable structured output emitted at the end of a
// push run (DAG n41). It is the single sink every terminal path drains into:
// agents parse it to decide their next action. The JSON shape is the contract
// pinned in dag-v1.4.0.mmd. Every slice is non-nil so it marshals as [] rather
// than null — agents can index without a presence check.
type RunSummary struct {
	Status        string               `json:"status"` // clean | partial | halted | cancelled
	Pushed        []PushedEntry        `json:"pushed"`
	SkippedNoOp   []string             `json:"skippedNoOp"`
	SkippedNonRow []SkippedNonRowEntry `json:"skippedNonRow"`
	Failed        []FailedEntry        `json:"failed"`
	Halted        []HaltedEntry        `json:"halted"`
}

// PushedEntry is one row that was written to Notion, plus the fields sent.
type PushedEntry struct {
	File   string   `json:"file"`
	Fields []string `json:"fields"`
}

// SkippedNonRowEntry is a file the push intentionally ignored because it isn't a
// synced row. Reason is "AGENTS.md" or "notion-deleted".
type SkippedNonRowEntry struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// FailedEntry is a per-row failure that did not halt the run (continue-class).
type FailedEntry struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
	Fix    string `json:"fix"`
}

// HaltedEntry is a halt that aborted (validation) or stopped (auth) the run.
// Phase is "validation" or "auth". Cells carries the per-cell local-vs-Notion
// diff on a conflict halt (issue #103, Option A) so an agent gets the same
// evidence as the human view; it is [] (never null) for non-conflict halts.
type HaltedEntry struct {
	File   string     `json:"file"`
	Reason string     `json:"reason"`
	Fix    string     `json:"fix"`
	Phase  string     `json:"phase"`
	Cells  []CellDiff `json:"cells"`
}

// Summary maps the accumulated PushResult into the RunSummary contract,
// computing status per the DAG header rules. Slices are initialized empty so
// the JSON never emits null arrays.
func (r *PushResult) Summary() RunSummary {
	s := RunSummary{
		Status:        r.status(),
		Pushed:        append([]PushedEntry{}, r.PushedRows...),
		SkippedNoOp:   append([]string{}, r.SkippedNoOpFiles...),
		SkippedNonRow: append([]SkippedNonRowEntry{}, r.SkippedNonRow...),
		Failed:        append([]FailedEntry{}, r.FailedRows...),
		Halted:        []HaltedEntry{},
	}
	// Validation-gate halts (DAG n22a): one entry per halt-class file, tagged
	// phase "validation". Basename only — the contract never leaks abs paths.
	for _, h := range r.Halts {
		s.Halted = append(s.Halted, HaltedEntry{
			File:   filepath.Base(h.Path),
			Reason: h.Reason,
			Fix:    haltFix(h.Class),
			Phase:  "validation",
			// Conflict halts carry the per-cell diff (#103); other classes
			// have nil CellDiffs, folded to [] so the array never marshals null.
			Cells: append([]CellDiff{}, h.CellDiffs...),
		})
	}
	// Auth halt (DAG n34h): run-wide, so no single file owns it (File empty).
	// AuthError packs reason and fix as "reason — fix"; split so the agent gets
	// the actionable half separately.
	if r.AuthHalted {
		reason, fix := splitReasonFix(r.AuthError)
		s.Halted = append(s.Halted, HaltedEntry{
			Reason: reason,
			Fix:    fix,
			Phase:  "auth",
			Cells:  []CellDiff{},
		})
	}
	return s
}

// splitReasonFix splits a "reason — fix" message into its two halves. Falls
// back to (whole, generic) when there's no em-dash separator.
func splitReasonFix(msg string) (reason, fix string) {
	if i := strings.Index(msg, " — "); i != -1 {
		return msg[:i], msg[i+len(" — "):]
	}
	return msg, "fix the issue, then re-run"
}

// haltFix returns the one-line remediation for a halt class — the actionable
// half of a halted entry, kept distinct from the diagnostic Reason so an agent
// can surface "what to do" without parsing prose.
func haltFix(c Classification) string {
	switch c {
	case ClassHaltConflict:
		return "run `notion-sync refresh` to pull Notion's version, reconcile, then push again"
	case ClassHaltUnexpected:
		return "remove the file or add a valid notion-id, then re-run"
	case ClassHaltUnreachable:
		return "check network / Notion availability, then re-run"
	case ClassHaltMalformed:
		return "fix the YAML frontmatter, then re-run"
	case ClassHaltUnreadable:
		return "fix the file's permissions, then re-run"
	case ClassHaltInvalidOption:
		return "use an existing option (or pass --allow-new-options for select/multi_select), then re-run"
	default:
		return "fix the issue, then re-run"
	}
}

// status applies the DAG status rules. Order matters: cancelled and halted are
// checked before partial so a halted-and-failed run reports "halted".
func (r *PushResult) status() string {
	switch {
	case r.Cancelled:
		return "cancelled"
	case len(r.Halts) > 0 || r.AuthHalted:
		return "halted"
	case len(r.FailedRows) > 0:
		return "partial"
	default:
		return "clean"
	}
}
