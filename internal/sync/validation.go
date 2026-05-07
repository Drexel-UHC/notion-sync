package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ran-codes/notion-sync/internal/frontmatter"
)

// Classification is a single file's outcome from the validation pass (DAG n21).
// Halt-class values cause the whole run to abort at n22a; skip/ready proceed.
type Classification int

const (
	ClassSkipAgentsMD    Classification = iota // n21a — generated guide, not a row
	ClassSkipDeleted                           // n21b — already soft-deleted
	ClassReady                                 // n21c — linked, timestamps match
	ClassHaltConflict                          // n21d — Notion edited since last sync
	ClassHaltUnexpected                        // n21e — unlinked, not AGENTS.md
	ClassHaltUnreachable                       // n21f — Notion unreachable during read
	ClassHaltMalformed                         // n21g — YAML frontmatter could not be parsed
)

// IsHalt returns true for the four halt-class values.
func (c Classification) IsHalt() bool {
	return c == ClassHaltConflict ||
		c == ClassHaltUnexpected ||
		c == ClassHaltUnreachable ||
		c == ClassHaltMalformed
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
func ValidatePushQueue(client NotionClient, folderPath string) (*ValidationReport, error) {
	dirEntries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, fmt.Errorf("read folder: %w", err)
	}

	report := &ValidationReport{}
	// add appends a classification and flips Halted in lockstep, so Halted
	// can never drift out of sync with the underlying classifications. The
	// rule is centralized here (not at every call site) — one bug-fix
	// surface, no missed assignments.
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
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
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

		localLastEdited, _ := fm["notion-last-edited"].(string)
		page, err := client.GetPage(notionID)
		if err != nil {
			add(FileClassification{
				Path:     fullPath,
				Class:    ClassHaltUnreachable,
				NotionID: notionID,
				Reason:   fmt.Sprintf("could not read Notion last_edited_time: %v", err),
			})
			continue
		}
		if timestampsEqual(localLastEdited, page.LastEditedTime) {
			add(FileClassification{
				Path:     fullPath,
				Class:    ClassReady,
				NotionID: notionID,
			})
			continue
		}

		add(FileClassification{
			Path:     fullPath,
			Class:    ClassHaltConflict,
			NotionID: notionID,
			Reason: fmt.Sprintf(
				"Notion's row has changed since last sync (local %s, Notion %s) — refresh before pushing",
				localLastEdited, page.LastEditedTime,
			),
		})
	}
	return report, nil
}

// scanLocalHalts walks folderPath and returns the halt-class files that
// can be detected without any Notion API call: ClassHaltUnexpected (stray
// .md missing notion-id) and ClassHaltMalformed (corrupt YAML frontmatter).
//
// Used by BuildPushQueue so the phase-1 confirmation preview can warn the
// user about locally-detectable halts before they consent. Without this,
// the user confirms a queue, validation halts on a stray they were never
// shown, and they get the fix-loop UX phase 2's halt enumerator was meant
// to avoid. Network-dependent halts (Conflict, Unreachable) still only
// surface at the validation gate — by definition we can't predict them
// from disk alone.
func scanLocalHalts(folderPath string) ([]FileClassification, error) {
	dirEntries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, fmt.Errorf("read folder: %w", err)
	}
	var halts []FileClassification
	for _, entry := range dirEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if strings.EqualFold(entry.Name(), "AGENTS.md") {
			continue
		}
		fullPath := filepath.Join(folderPath, entry.Name())
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		fm, parseErr := frontmatter.Parse(string(content))
		if parseErr != nil {
			halts = append(halts, FileClassification{
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
			continue
		}
		notionID, hasID := fm["notion-id"].(string)
		if !hasID || notionID == "" {
			halts = append(halts, FileClassification{
				Path:   fullPath,
				Class:  ClassHaltUnexpected,
				Reason: "file has no notion-id and is not AGENTS.md — does not belong to this synced folder",
			})
		}
	}
	return halts, nil
}
