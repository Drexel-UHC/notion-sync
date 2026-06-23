package main

import (
	"fmt"

	"github.com/ran-codes/notion-sync/internal/sync"
)

func formatPushProgress(p sync.ProgressPhase, dbTitle string) string {
	switch p.Phase {
	case sync.PhasePushScanning:
		return "Scanning local files..."
	case sync.PhasePushing:
		title := dbTitle
		if title == "" {
			title = p.Title
		}
		return fmt.Sprintf("Pushing \"%s\"... %d/%d", title, p.Current, p.Total)
	case sync.PhaseComplete:
		return "Done"
	default:
		return ""
	}
}

func formatProgress(p sync.ProgressPhase, dbTitle string) string {
	switch p.Phase {
	case sync.PhaseQuerying:
		return "Querying database entries..."
	case sync.PhaseDiffing:
		return fmt.Sprintf("Comparing %d entries with local files...", p.Total)
	case sync.PhaseStaleDetected:
		return fmt.Sprintf("Found %d entries to sync (%d total)", p.Stale, p.Total)
	case sync.PhaseImporting:
		title := dbTitle
		if title == "" {
			title = p.Title
		}
		return fmt.Sprintf("Importing \"%s\"... %d/%d", title, p.Current, p.Total)
	case sync.PhaseComplete:
		return "Done"
	default:
		return ""
	}
}
