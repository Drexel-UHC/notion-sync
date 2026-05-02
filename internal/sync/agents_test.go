package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestEnsureAgentsMDCurrent_DryRun(t *testing.T) {
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

func TestWriteAgentsMD_PreservesExisting(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "AGENTS.md")
	custom := "# my hand-edited AGENTS.md\n"
	if err := os.WriteFile(dest, []byte(custom), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := WriteAgentsMD(tmp); err != nil {
		t.Fatalf("WriteAgentsMD: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != custom {
		t.Errorf("WriteAgentsMD overwrote existing file:\nwant: %q\ngot:  %q", custom, got)
	}
}

func TestWriteAgentsMD_StampsVersion(t *testing.T) {
	tmp := t.TempDir()

	prev := Version
	Version = "vTEST"
	defer func() { Version = prev }()

	if err := WriteAgentsMD(tmp); err != nil {
		t.Fatalf("WriteAgentsMD: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(tmp, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(got), "<!-- notion-sync-version: vTEST -->") {
		t.Errorf("AGENTS.md missing version stamp, got:\n%s", got)
	}
}
