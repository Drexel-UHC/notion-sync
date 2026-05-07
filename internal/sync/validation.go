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
)

// IsHalt returns true for the three halt-class values (n21d/n21e/n21f).
func (c Classification) IsHalt() bool {
	return c == ClassHaltConflict || c == ClassHaltUnexpected || c == ClassHaltUnreachable
}

// FileClassification is one file's row in the validation report.
type FileClassification struct {
	Path     string
	Class    Classification
	Reason   string // populated for halts; user-facing
	NotionID string // empty when not applicable (AGENTS.md, unexpected)
}

// ValidationReport is the result of n21+n22 across every file in folderPath.
type ValidationReport struct {
	Files  []FileClassification
	Halted bool // true iff any FileClassification has a halt-class
}

// ValidatePushQueue runs the n21+n22 pass: classify every .md in folderPath,
// then set Halted = true if any classification is a halt class. Pure
// read+classify — no writes, no UpdatePage calls.
func ValidatePushQueue(client NotionClient, folderPath string) (*ValidationReport, error) {
	dirEntries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, fmt.Errorf("read folder: %w", err)
	}

	report := &ValidationReport{}
	for _, entry := range dirEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		fullPath := filepath.Join(folderPath, entry.Name())
		if entry.Name() == "AGENTS.md" {
			report.Files = append(report.Files, FileClassification{
				Path:  fullPath,
				Class: ClassSkipAgentsMD,
			})
			continue
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		fm, _ := frontmatter.Parse(string(content))
		if fm == nil {
			fm = map[string]interface{}{}
		}

		if deleted, _ := fm["notion-deleted"].(bool); deleted {
			notionID, _ := fm["notion-id"].(string)
			report.Files = append(report.Files, FileClassification{
				Path:     fullPath,
				Class:    ClassSkipDeleted,
				NotionID: notionID,
			})
			continue
		}

		notionID, hasID := fm["notion-id"].(string)
		if !hasID || notionID == "" {
			report.Files = append(report.Files, FileClassification{
				Path:   fullPath,
				Class:  ClassHaltUnexpected,
				Reason: "file has no notion-id and is not AGENTS.md — does not belong to this synced folder",
			})
			continue
		}

		localLastEdited, _ := fm["notion-last-edited"].(string)
		page, err := client.GetPage(notionID)
		if err != nil {
			report.Files = append(report.Files, FileClassification{
				Path:     fullPath,
				Class:    ClassHaltUnreachable,
				NotionID: notionID,
				Reason:   fmt.Sprintf("could not read Notion last_edited_time: %v", err),
			})
			continue
		}
		if timestampsEqual(localLastEdited, page.LastEditedTime) {
			report.Files = append(report.Files, FileClassification{
				Path:     fullPath,
				Class:    ClassReady,
				NotionID: notionID,
			})
			continue
		}

		report.Files = append(report.Files, FileClassification{
			Path:     fullPath,
			Class:    ClassHaltConflict,
			NotionID: notionID,
			Reason: fmt.Sprintf(
				"Notion's row has changed since last sync (local %s, Notion %s) — refresh before pushing",
				localLastEdited, page.LastEditedTime,
			),
		})
	}

	// n22 — single-source-of-truth aggregation: Halted is derived from the
	// classifications, not maintained inline at every halt site (where it
	// would be one missed assignment away from a silent gate-bypass).
	for _, f := range report.Files {
		if f.Class.IsHalt() {
			report.Halted = true
			break
		}
	}
	return report, nil
}
