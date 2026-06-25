package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAgentsMD_DocumentsRichTextPushLimitations locks the Gap 1 agreement (#95):
// the distributed AGENTS.md must warn downstream agents that foreground color,
// @user mention identity, and inline equations do not round-trip on push (#100), so
// the "Pushable? yes" rows aren't read as full-fidelity.
func TestAgentsMD_DocumentsRichTextPushLimitations(t *testing.T) {
	content := renderAgentsMD()
	for _, want := range []string{
		"Rich-text fidelity on push",
		"Foreground (text) color",
		"mention identity",
		"never be restored on push",
		"Inline equation",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("AGENTS.md missing rich-text push limitation note: %q", want)
		}
	}
}

// TestAgentsMD_HasDoNotEditBanner pins the DO-NOT-EDIT banner (#87, Bucket 2):
// AGENTS.md is tool-owned and force-overwritten on version-stamp drift, so the
// emitted file must announce that up front or the overwrite is a surprise. A
// future template edit that drops the banner should turn this test red.
func TestAgentsMD_HasDoNotEditBanner(t *testing.T) {
	content := renderAgentsMD()
	for _, want := range []string{
		"DO NOT EDIT",
		"regenerated on every import/refresh",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("AGENTS.md missing DO-NOT-EDIT banner marker: %q", want)
		}
	}
}

func TestEnsureAgentsMDCurrent_RewritesIfStaleStamp(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "AGENTS.md")

	prev := Version
	Version = "v2.0.0"
	defer func() { Version = prev }()

	stale := "<!-- notion-sync-version: v1.0.0 -->\n# stale content\n"
	if err := os.WriteFile(dest, []byte(stale), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	written, err := EnsureAgentsMDCurrent(tmp, false)
	if err != nil {
		t.Fatalf("EnsureAgentsMDCurrent: %v", err)
	}
	if !written {
		t.Errorf("written = false, want true")
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(got), "<!-- notion-sync-version: v2.0.0 -->") {
		t.Errorf("AGENTS.md was not rewritten with new stamp:\n%s", got)
	}
	if strings.Contains(string(got), "# stale content") {
		t.Errorf("AGENTS.md still contains stale content:\n%s", got)
	}
}

func TestEnsureAgentsMDCurrent_LeavesAloneIfCurrent(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "AGENTS.md")

	prev := Version
	Version = "v3.0.0"
	defer func() { Version = prev }()

	current := "<!-- notion-sync-version: v3.0.0 -->\n# user-edited but stamp matches\n"
	if err := os.WriteFile(dest, []byte(current), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	written, err := EnsureAgentsMDCurrent(tmp, false)
	if err != nil {
		t.Fatalf("EnsureAgentsMDCurrent: %v", err)
	}
	if written {
		t.Errorf("written = true, want false (stamp matches current)")
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != current {
		t.Errorf("AGENTS.md was overwritten despite matching stamp")
	}
}

func TestEnsureAgentsMDCurrent_DryRunMissing(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "AGENTS.md")

	prev := Version
	Version = "v4.0.0"
	defer func() { Version = prev }()

	written, err := EnsureAgentsMDCurrent(tmp, true)
	if err != nil {
		t.Fatalf("EnsureAgentsMDCurrent: %v", err)
	}
	if !written {
		t.Errorf("written = false, want true (dry-run should still report)")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote a file (or stat err = %v)", err)
	}
}

func TestEnsureAgentsMDCurrent_DryRunStale(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "AGENTS.md")

	prev := Version
	Version = "v4.0.0"
	defer func() { Version = prev }()

	stale := "<!-- notion-sync-version: v1.0.0 -->\n# stale content\n"
	if err := os.WriteFile(dest, []byte(stale), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	written, err := EnsureAgentsMDCurrent(tmp, true)
	if err != nil {
		t.Fatalf("EnsureAgentsMDCurrent: %v", err)
	}
	if !written {
		t.Errorf("written = false, want true (dry-run with stale stamp should report)")
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != stale {
		t.Errorf("dry-run modified file on disk:\nwant: %q\ngot:  %q", stale, got)
	}
}

func TestEnsureAgentsMDCurrent_DryRunCurrent(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "AGENTS.md")

	prev := Version
	Version = "v4.0.0"
	defer func() { Version = prev }()

	current := "<!-- notion-sync-version: v4.0.0 -->\n# current content\n"
	if err := os.WriteFile(dest, []byte(current), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	written, err := EnsureAgentsMDCurrent(tmp, true)
	if err != nil {
		t.Fatalf("EnsureAgentsMDCurrent: %v", err)
	}
	if written {
		t.Errorf("written = true, want false (dry-run with current stamp must not report a write)")
	}
}

func TestEnsureAgentsMDCurrent_NoVersion(t *testing.T) {
	tmp := t.TempDir()

	prev := Version
	Version = ""
	defer func() { Version = prev }()

	written, err := EnsureAgentsMDCurrent(tmp, false)
	if err != nil {
		t.Fatalf("EnsureAgentsMDCurrent: %v", err)
	}
	if written {
		t.Errorf("written = true, want false (Version unset must skip)")
	}
	if _, err := os.Stat(filepath.Join(tmp, "AGENTS.md")); !os.IsNotExist(err) {
		t.Errorf("AGENTS.md was written despite Version unset")
	}
}

// syncAgentsMD is the import/refresh auto-path. A *missing* AGENTS.md is always
// created — even in a dev build (Version unset) — so a first import emits the doc.
func TestSyncAgentsMD_WritesIfMissingEvenWithoutVersion(t *testing.T) {
	tmp := t.TempDir()

	prev := Version
	Version = ""
	defer func() { Version = prev }()

	if err := syncAgentsMD(tmp); err != nil {
		t.Fatalf("syncAgentsMD: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "AGENTS.md")); err != nil {
		t.Errorf("AGENTS.md was not created on first import: %v", err)
	}
}

// syncAgentsMD force-upgrades an existing, stale-stamped AGENTS.md (tool-owned
// file — local edits are intentionally discarded on version drift).
func TestSyncAgentsMD_ForceUpgradesStale(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "AGENTS.md")

	prev := Version
	Version = "v2.0.0"
	defer func() { Version = prev }()

	stale := "<!-- notion-sync-version: v1.0.0 -->\n# stale + hand-edited\n"
	if err := os.WriteFile(dest, []byte(stale), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := syncAgentsMD(tmp); err != nil {
		t.Fatalf("syncAgentsMD: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(got), "<!-- notion-sync-version: v2.0.0 -->") {
		t.Errorf("stale AGENTS.md was not force-upgraded:\n%s", got)
	}
	if strings.Contains(string(got), "stale + hand-edited") {
		t.Errorf("force-upgrade kept stale content:\n%s", got)
	}
}

// syncAgentsMD leaves a current-stamped file alone (no needless rewrite).
func TestSyncAgentsMD_LeavesCurrentAlone(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "AGENTS.md")

	prev := Version
	Version = "v3.0.0"
	defer func() { Version = prev }()

	current := "<!-- notion-sync-version: v3.0.0 -->\n# whatever\n"
	if err := os.WriteFile(dest, []byte(current), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := syncAgentsMD(tmp); err != nil {
		t.Fatalf("syncAgentsMD: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != current {
		t.Errorf("current AGENTS.md was needlessly rewritten")
	}
}

func TestEnsureAgentsMDCurrent_WritesIfMissing(t *testing.T) {
	tmp := t.TempDir()

	prev := Version
	Version = "v1.0.0"
	defer func() { Version = prev }()

	written, err := EnsureAgentsMDCurrent(tmp, false)
	if err != nil {
		t.Fatalf("EnsureAgentsMDCurrent: %v", err)
	}
	if !written {
		t.Errorf("written = false, want true")
	}

	got, err := os.ReadFile(filepath.Join(tmp, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not written: %v", err)
	}
	if !strings.Contains(string(got), "<!-- notion-sync-version: v1.0.0 -->") {
		t.Errorf("missing version stamp:\n%s", got)
	}
}

func TestRegenerateAgentsMD_OverwritesExisting(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "AGENTS.md")

	prev := Version
	Version = "vREGEN"
	defer func() { Version = prev }()

	if err := os.WriteFile(dest, []byte("# old hand-edited content\n"), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := RegenerateAgentsMD(tmp); err != nil {
		t.Fatalf("RegenerateAgentsMD: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(got), "<!-- notion-sync-version: vREGEN -->") {
		t.Errorf("AGENTS.md was not regenerated:\n%s", got)
	}
}

func TestParseAgentsMDVersion_Stamped(t *testing.T) {
	in := "<!-- notion-sync-version: v1.2.3 -->\n# notion-sync workspace\n"
	if got := ParseAgentsMDVersion(in); got != "v1.2.3" {
		t.Errorf("ParseAgentsMDVersion = %q, want v1.2.3", got)
	}
}

func TestParseAgentsMDVersion_Missing(t *testing.T) {
	in := "# notion-sync workspace\n\nno stamp here.\n"
	if got := ParseAgentsMDVersion(in); got != "" {
		t.Errorf("ParseAgentsMDVersion = %q, want empty", got)
	}
}

func TestParseAgentsMDVersion_EmptyValue(t *testing.T) {
	// Stamp with empty version (e.g. binary built without -ldflags).
	in := "<!-- notion-sync-version:  -->\n# notion-sync workspace\n"
	if got := ParseAgentsMDVersion(in); got != "" {
		t.Errorf("ParseAgentsMDVersion = %q, want empty", got)
	}
}

// TestEmbeddedTemplateHasPlaceholder guards the embed refactor: the embedded
// agents.md.tmpl must still carry the {{VERSION}} placeholder, and renderAgentsMD
// must substitute it (not leave the literal placeholder behind).
func TestEmbeddedTemplateHasPlaceholder(t *testing.T) {
	if !strings.Contains(agentsMDTemplate, "{{VERSION}}") {
		t.Fatalf("embedded template lost the {{VERSION}} placeholder")
	}

	prev := Version
	Version = "vEMBED"
	defer func() { Version = prev }()

	got := renderAgentsMD()
	if strings.Contains(got, "{{VERSION}}") {
		t.Errorf("renderAgentsMD left the {{VERSION}} placeholder unsubstituted")
	}
	if !strings.Contains(got, "<!-- notion-sync-version: vEMBED -->") {
		t.Errorf("renderAgentsMD did not stamp the version, got head:\n%.80s", got)
	}
}
