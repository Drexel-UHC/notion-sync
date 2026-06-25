package sync

import (
	_ "embed"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// agentsMDVersionPattern matches the version-stamp HTML comment emitted at the
// top of AGENTS.md. The captured group is the version string (e.g. "v1.2.0").
var agentsMDVersionPattern = regexp.MustCompile(`<!--\s*notion-sync-version:\s*(\S.*?)\s*-->`)

// ParseAgentsMDVersion extracts the notion-sync version stamped into an
// AGENTS.md file's content, or "" if the stamp is missing or empty.
func ParseAgentsMDVersion(content string) string {
	m := agentsMDVersionPattern.FindStringSubmatch(content)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// agentsMDTemplate is the AGENTS.md body written to the workspace root so that
// any downstream LLM/agent that lands in a notion-sync output folder
// understands the layout, conventions, and gotchas of the synced data. It is
// embedded from agents.md.tmpl (edit that file, not a Go string) and contains a
// single {{VERSION}} placeholder filled in by renderAgentsMD.
//
// AGENTS.md is the cross-vendor convention (Cursor, OpenAI's agents.md spec,
// and others) for provider-neutral agent instructions. Claude Code reads it
// alongside CLAUDE.md.
//
//go:embed agents.md.tmpl
var agentsMDTemplate string

// renderAgentsMD returns the AGENTS.md content with the current binary's
// Version interpolated into the version stamp. If Version is unset (e.g. in
// tests that don't wire it), the stamp value is empty.
func renderAgentsMD() string {
	return strings.Replace(agentsMDTemplate, "{{VERSION}}", Version, 1)
}

// EnsureAgentsMDCurrent keeps the workspace-root AGENTS.md in lock-step with the
// running binary. AGENTS.md is a generated, tool-owned file — not a
// user-editable one — so the rules are:
//
//   - stamp != Version → force-overwrite (the doc upgrades on binary upgrade;
//     any local edits are intentionally discarded)
//   - stamp == Version → leave alone (already current)
//   - missing          → write
//
// Returns true if a write happened (or, in dryRun, would have happened).
//
// If Version is unset (build-time -ldflags not wired) this is a no-op — we have
// nothing meaningful to stamp or compare against. The fire-and-forget auto-path
// (syncAgentsMD) layers a dev-build first-write on top of this for import.
func EnsureAgentsMDCurrent(workspacePath string, dryRun bool) (bool, error) {
	if Version == "" {
		return false, nil
	}
	dest := filepath.Join(workspacePath, "AGENTS.md")
	existing, err := os.ReadFile(dest)
	if err == nil {
		if ParseAgentsMDVersion(string(existing)) == Version {
			return false, nil
		}
		// stamp drift → fall through to (over)write
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if dryRun {
		return true, nil
	}
	if err := os.WriteFile(dest, []byte(renderAgentsMD()), 0644); err != nil {
		return false, err
	}
	return true, nil
}

// syncAgentsMD is the fire-and-forget auto-path used by import and refresh:
//   - missing            → write (even in a dev build with an empty stamp, so a
//     first import always emits the doc)
//   - present + drift    → force-upgrade via EnsureAgentsMDCurrent
//   - present, Version="" → leave alone (dev build can't tell current from stale)
//
// Callers log (not fail) on error — a doc-write hiccup must never abort a sync.
func syncAgentsMD(workspacePath string) error {
	dest := filepath.Join(workspacePath, "AGENTS.md")
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(dest, []byte(renderAgentsMD()), 0644)
		}
		return err
	}
	_, err := EnsureAgentsMDCurrent(workspacePath, false)
	return err
}

// RegenerateAgentsMD writes AGENTS.md to the workspace root, overwriting any
// existing file. Used by `notion-sync agents-md` for explicit user-driven
// refreshes — the command name is the consent.
func RegenerateAgentsMD(workspacePath string) error {
	dest := filepath.Join(workspacePath, "AGENTS.md")
	return os.WriteFile(dest, []byte(renderAgentsMD()), 0644)
}
