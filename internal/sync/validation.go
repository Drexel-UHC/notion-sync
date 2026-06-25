package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ran-codes/notion-sync/internal/frontmatter"
	"github.com/ran-codes/notion-sync/internal/notion"
)

// Classification is a single file's outcome from the validation pass (DAG n21).
// Halt-class values cause the whole run to abort at n22a; skip/ready proceed.
type Classification int

const (
	ClassSkipAgentsMD      Classification = iota // n21a — generated guide, not a row
	ClassSkipDeleted                             // n21b — already soft-deleted
	ClassReady                                   // n21c — linked, timestamps match
	ClassHaltConflict                            // n21d — Notion edited since last sync
	ClassHaltUnexpected                          // n21e — unlinked, not AGENTS.md
	ClassHaltUnreachable                         // n21f — Notion unreachable during read
	ClassHaltMalformed                           // n21g — YAML frontmatter could not be parsed
	ClassHaltUnreadable                          // n21h — file could not be read from disk
	ClassHaltInvalidOption                       // n21i — select/status/multi_select value not in schema options
)

// IsHalt returns true for any halt-class value.
func (c Classification) IsHalt() bool {
	return c == ClassHaltConflict ||
		c == ClassHaltUnexpected ||
		c == ClassHaltUnreachable ||
		c == ClassHaltMalformed ||
		c == ClassHaltUnreadable ||
		c == ClassHaltInvalidOption
}

// isLocalHalt reports whether a halt class is detectable from disk alone,
// with no Notion API call. Conflict and Unreachable both require a GetPage,
// so the preview (BuildPushQueue) can't predict them — they're excluded.
// This is the filter that turns the single classifier walk's report into
// PushPreview.LocalHalts.
func (c Classification) isLocalHalt() bool {
	return c == ClassHaltUnexpected ||
		c == ClassHaltMalformed ||
		c == ClassHaltUnreadable
}

// FileClassification is one file's row in the validation report.
//
// NotionID is populated whenever the file declares one in frontmatter
// (Ready, ClassHaltConflict, ClassHaltUnreachable, and ClassSkipDeleted
// for soft-deleted rows that still carry their original notion-id). It's
// empty for ClassSkipAgentsMD, ClassHaltUnexpected, ClassHaltMalformed,
// and any ClassSkipDeleted file whose frontmatter never carried notion-id.
type FileClassification struct {
	Path     string
	Class    Classification
	Reason   string // populated for halts; user-facing
	NotionID string

	// fm is the parsed frontmatter, carried so the push loop can build its
	// property payload from a ClassReady row without re-reading and
	// re-parsing the file. Unexported: an internal carry for the single-walk
	// refactor (#80), not part of the report's public contract. Populated for
	// the linked rows (Ready, Conflict, Unreachable, Deleted); nil for
	// AGENTS.md, Unreadable, Malformed, and Unexpected.
	fm map[string]interface{}

	// page is the Notion snapshot the gate fetched while resolving this row,
	// stashed so the push loop's per-cell diff (DAG n31, decision #2) can
	// compare against it without a second fetch. Populated only on the gate
	// path's ClassReady (client != nil, timestamps matched); nil for the
	// network-free preview / --force path and for every non-Ready class.
	page *notion.Page

	// CellDiffs is the per-cell local-vs-Notion divergence on a conflict halt
	// (issue #103, Option A): one entry per schema-backed field whose local
	// value differs from Notion's current value. Computed at classification
	// time from diffRowCells against the page the gate just fetched, so it can
	// never disagree with the gate. Populated only on ClassHaltConflict (and
	// only when the gate supplied a schema); nil for every other class. May be
	// empty on a conflict whose timestamp moved for a non-property reason (page
	// body / unsupported field edit) — that's a real, distinguishable state.
	CellDiffs []CellDiff
}

// ValidationReport is the result of n21+n22 across every file in folderPath.
type ValidationReport struct {
	Files  []FileClassification
	Halted bool // true iff any FileClassification has a halt-class
}

// ValidatePushQueue runs the n21+n22 pass: classify every .md in folderPath
// and flip Halted on any halt-class outcome. Pure read+classify — no writes,
// no UpdatePage calls.
//
// This is a classifier-only function: it assumes folder identity (e.g.
// _database.json presence) has already been validated upstream by the
// caller. PushDatabase does that check before invoking the gate; direct
// callers must do the same or accept that the report says nothing about
// whether the folder is a real synced database.
//
// Thin wrapper over classifyFolder — the single walk that also backs
// BuildPushQueue (preview) and the per-row push queue (#80).
func ValidatePushQueue(client NotionClient, folderPath string) (*ValidationReport, error) {
	return classifyFolder(folderPath, client, nil, false)
}

// classifyFolder is the single classifier walk behind every push folder scan
// (#80): the validation gate, the confirmation preview, and the per-row push
// queue all derive from one report instead of three near-duplicate walks that
// drifted apart on AGENTS.md casing, IO-error policy, and deleted ordering.
//
// It reads each .md once and applies the shared local filter: AGENTS.md skip,
// IO error → Unreadable, parse error → Malformed, notion-deleted → Deleted,
// missing notion-id → Unexpected. For a locally-clean linked row the behavior
// forks on client:
//
//   - client == nil — the network-free view. The row is marked ClassReady
//     without contacting Notion. BuildPushQueue uses this for its preview, and
//     PushDatabase uses it under --force (which deliberately skips the gate).
//   - client != nil — the gate. A GetPage resolves the row into Ready (matched
//     timestamps), Conflict (Notion moved ahead), or Unreachable (GetPage
//     failed).
//
// When schema is non-nil (the gate path) each linked row's select/status/
// multi_select values are checked against the schema's allowed options before
// the conflict GetPage — an unknown value halts the run (ClassHaltInvalidOption)
// before any write, preventing typo-driven schema pollution and mid-run 422s
// (issue #90). Network-free callers (preview, --force) pass a nil schema and
// skip the check. allowNewOptions relaxes select/multi_select (Notion will
// auto-create the option on push); status always validates (the API can't
// create status options).
//
// Halted flips on any halt-class outcome, in lockstep with the appends, so it
// can never drift out of sync with the underlying classifications.
func classifyFolder(folderPath string, client NotionClient, schema map[string]notion.DatabaseProperty, allowNewOptions bool) (*ValidationReport, error) {
	dirEntries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, fmt.Errorf("read folder: %w", err)
	}

	report := &ValidationReport{}
	add := func(fc FileClassification) {
		report.Files = append(report.Files, fc)
		if fc.Class.IsHalt() {
			report.Halted = true
		}
	}

	for _, entry := range dirEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		fullPath := filepath.Join(folderPath, entry.Name())
		// Case-insensitive: defensive on Windows / default-config macOS,
		// where agents.md or Agents.md could land here. Misclassifying
		// the generated agents guide as a stray would halt the run for
		// the user's filesystem casing alone.
		if strings.EqualFold(entry.Name(), "AGENTS.md") {
			add(FileClassification{
				Path:  fullPath,
				Class: ClassSkipAgentsMD,
			})
			continue
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			// IO errors (permission denied, transient FS hiccup) classify as
			// halt rather than abort the whole pass — surfaces the file in
			// the halt list with a fixable reason instead of dropping the
			// entire validation report on the floor.
			add(FileClassification{
				Path:   fullPath,
				Class:  ClassHaltUnreadable,
				Reason: fmt.Sprintf("could not read file: %v", err),
			})
			continue
		}
		fm, parseErr := frontmatter.Parse(string(content))
		if parseErr != nil {
			add(FileClassification{
				Path:   fullPath,
				Class:  ClassHaltMalformed,
				Reason: fmt.Sprintf("could not parse YAML frontmatter: %v", parseErr),
			})
			continue
		}
		if fm == nil {
			fm = map[string]interface{}{}
		}

		if deleted, _ := fm["notion-deleted"].(bool); deleted {
			notionID, _ := fm["notion-id"].(string)
			add(FileClassification{
				Path:     fullPath,
				Class:    ClassSkipDeleted,
				NotionID: notionID,
				fm:       fm,
			})
			continue
		}

		notionID, hasID := fm["notion-id"].(string)
		if !hasID || notionID == "" {
			add(FileClassification{
				Path:   fullPath,
				Class:  ClassHaltUnexpected,
				Reason: "file has no notion-id and is not AGENTS.md — does not belong to this synced folder",
			})
			continue
		}

		// Schema-based option validation (issue #90). An unknown select/
		// multi_select value would silently auto-create a junk option in the
		// shared schema; an unknown status value would 422 mid-run. Catch both
		// here — local string compare against the fetched schema, before the
		// conflict GetPage — so the whole run halts before any write. Only the
		// gate supplies a schema; preview / --force pass nil and skip it.
		if schema != nil {
			if reason, ok := validateRowOptions(fm, schema, allowNewOptions); !ok {
				add(FileClassification{
					Path:     fullPath,
					Class:    ClassHaltInvalidOption,
					NotionID: notionID,
					fm:       fm,
					Reason:   reason,
				})
				continue
			}
		}

		// Linked, not deleted, parseable. Network-free callers (preview,
		// --force) stop here: the row is ready as far as disk can tell. The
		// gate will resolve conflicts when a client is supplied.
		if client == nil {
			add(FileClassification{
				Path:     fullPath,
				Class:    ClassReady,
				NotionID: notionID,
				fm:       fm,
			})
			continue
		}

		localLastEdited, _ := fm["notion-last-edited"].(string)
		page, err := client.GetPage(notionID)
		if err != nil {
			add(FileClassification{
				Path:     fullPath,
				Class:    ClassHaltUnreachable,
				NotionID: notionID,
				fm:       fm,
				Reason:   fmt.Sprintf("could not read Notion last_edited_time: %v", err),
			})
			continue
		}
		if timestampsEqual(localLastEdited, page.LastEditedTime) {
			add(FileClassification{
				Path:     fullPath,
				Class:    ClassReady,
				NotionID: notionID,
				fm:       fm,
				page:     page,
			})
			continue
		}

		add(FileClassification{
			Path:      fullPath,
			Class:     ClassHaltConflict,
			NotionID:  notionID,
			fm:        fm,
			page:      page,
			CellDiffs: diffRowCells(fm, page, schema),
			Reason: fmt.Sprintf(
				"Notion's row has changed since last sync (local %s, Notion %s) — refresh before pushing",
				localLastEdited, page.LastEditedTime,
			),
		})
	}
	return report, nil
}

// validateRowOptions checks every select / status / multi_select value in a
// row's frontmatter against the schema's allowed options (issue #90). It
// returns ok=false plus a user-facing reason listing all violations in the row
// (sorted for deterministic output) when any value is unknown.
//
// The select/status asymmetry mirrors Notion's API: select and multi_select
// options can be auto-created on write, so allowNewOptions lets a genuinely-new
// category through; status options cannot be created via API, so an unknown
// status always halts regardless of the flag.
//
// Empty/nil scalar values mean "clear the property" and are never invalid.
func validateRowOptions(fm map[string]interface{}, schema map[string]notion.DatabaseProperty, allowNewOptions bool) (string, bool) {
	var violations []string
	for key, prop := range schema {
		val, present := fm[key]
		if !present {
			continue
		}
		switch prop.Type {
		case "select":
			if allowNewOptions || prop.Select == nil {
				continue
			}
			name := coerceString(val)
			if name == "" {
				continue
			}
			if !optionAllowed(prop.Select.Options, name) {
				violations = append(violations, invalidOptionReason(key, name, prop.Select.Options))
			}
		case "status":
			// No opt-in: Notion's API cannot create status options.
			if prop.Status == nil {
				continue
			}
			name := coerceString(val)
			if name == "" {
				continue
			}
			if !optionAllowed(prop.Status.Options, name) {
				violations = append(violations, invalidOptionReason(key, name, prop.Status.Options))
			}
		case "multi_select":
			if allowNewOptions || prop.MultiSelect == nil {
				continue
			}
			for _, name := range coerceStringSlice(val) {
				if name == "" {
					continue
				}
				if !optionAllowed(prop.MultiSelect.Options, name) {
					violations = append(violations, invalidOptionReason(key, name, prop.MultiSelect.Options))
				}
			}
		}
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		return strings.Join(violations, "; "), false
	}
	return "", true
}

// optionAllowed reports whether name exactly matches one of the schema options.
// Match is case-sensitive: Notion treats "Done" and "done" as distinct options.
func optionAllowed(options []notion.SelectValue, name string) bool {
	for _, o := range options {
		if o.Name == name {
			return true
		}
	}
	return false
}

// invalidOptionReason formats the halt reason for an unknown option value,
// listing the allowed options in schema (Notion display) order.
func invalidOptionReason(prop, value string, options []notion.SelectValue) string {
	names := make([]string, 0, len(options))
	for _, o := range options {
		names = append(names, o.Name)
	}
	return fmt.Sprintf("%q is not a valid option for %q (allowed: %s)", value, prop, strings.Join(names, ", "))
}
