package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReorderArgs_FlagsBeforePositional(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "already ordered",
			args: []string{"--force", "folder"},
			want: []string{"--force", "folder"},
		},
		{
			name: "positional before flag",
			args: []string{"folder", "--force"},
			want: []string{"--force", "folder"},
		},
		{
			name: "flag with value after positional",
			args: []string{"folder", "--api-key", "mykey"},
			want: []string{"--api-key", "mykey", "folder"},
		},
		{
			name: "boolean flag -f",
			args: []string{"folder", "-f"},
			want: []string{"-f", "folder"},
		},
		{
			name: "flag with equals",
			args: []string{"folder", "--output=./out"},
			want: []string{"--output=./out", "folder"},
		},
		{
			name: "double dash stops",
			args: []string{"--force", "--", "folder", "--not-a-flag"},
			want: []string{"--force", "--", "folder", "--not-a-flag"},
		},
		{
			name: "mixed complex",
			args: []string{"mydb", "--output", "./out", "-f", "--api-key", "key123"},
			want: []string{"--output", "./out", "-f", "--api-key", "key123", "mydb"},
		},
		{
			name: "empty",
			args: []string{},
			want: nil,
		},
		{
			name: "only positional",
			args: []string{"folder"},
			want: []string{"folder"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reorderArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reorderArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestCLI_NoArgs_ExitZero(t *testing.T) {
	cmd := exec.Command("go", "run", ".", )
	err := cmd.Run()
	if err != nil {
		t.Errorf("expected exit 0 with no args, got %v", err)
	}
}

func TestCLI_UnknownCommand_ExitOne(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "nonexistent-command")
	err := cmd.Run()
	if err == nil {
		t.Error("expected non-zero exit for unknown command")
	}
}

func TestCLI_AgentsMD_OverwritesExisting(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "AGENTS.md")
	if err := os.WriteFile(dest, []byte("# old\n"), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("go", "run", ".", "agents-md", tmp).CombinedOutput()
	if err != nil {
		t.Fatalf("agents-md failed: %v\n%s", err, out)
	}

	if !strings.Contains(string(out), "Wrote AGENTS.md") {
		t.Errorf("expected confirmation message in stdout, got:\n%s", out)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(got), "<!-- notion-sync-version:") {
		t.Errorf("AGENTS.md missing version stamp:\n%s", got)
	}
	if strings.Contains(string(got), "# old") {
		t.Errorf("AGENTS.md was not overwritten:\n%s", got)
	}
}

// --- confirmPush tests (DAG n13 — consent gate before any Notion write) ---

func TestConfirmPush_YesFlag_Proceeds(t *testing.T) {
	var stderr bytes.Buffer
	ok := confirmPush([]string{"a/page.md"}, true, false, strings.NewReader(""), &stderr)
	if !ok {
		t.Error("expected confirmPush to return true with --yes flag")
	}
	if !strings.Contains(stderr.String(), "Proceeding (--yes)") {
		t.Errorf("expected 'Proceeding (--yes)' notice, got:\n%s", stderr.String())
	}
}

// Non-interactive runs without --yes must cancel — that's the whole point of
// requiring a flag in CI / piped contexts. Hint must mention --yes so the
// user/agent knows how to opt in.
func TestConfirmPush_NonTTY_NoFlag_Cancels(t *testing.T) {
	var stderr bytes.Buffer
	ok := confirmPush([]string{"a/page.md"}, false, false, strings.NewReader(""), &stderr)
	if ok {
		t.Error("expected confirmPush to cancel in non-TTY without --yes")
	}
	out := stderr.String()
	if !strings.Contains(out, "Cancelled") {
		t.Errorf("expected cancellation message, got:\n%s", out)
	}
	if !strings.Contains(out, "--yes") {
		t.Errorf("expected hint to mention --yes, got:\n%s", out)
	}
}

func TestConfirmPush_TTY_Yes_Proceeds(t *testing.T) {
	for _, ans := range []string{"y\n", "Y\n", "yes\n", "YES\n"} {
		var stderr bytes.Buffer
		ok := confirmPush([]string{"a/page.md"}, false, true, strings.NewReader(ans), &stderr)
		if !ok {
			t.Errorf("answer %q: expected proceed, got cancel\nstderr: %s", ans, stderr.String())
		}
	}
}

// Default-N is the safety property. Empty input (Enter), explicit "n", or
// anything other than y/yes must cancel.
func TestConfirmPush_TTY_DefaultN_Cancels(t *testing.T) {
	for _, ans := range []string{"\n", "n\n", "N\n", "no\n", "maybe\n", ""} {
		var stderr bytes.Buffer
		ok := confirmPush([]string{"a/page.md"}, false, true, strings.NewReader(ans), &stderr)
		if ok {
			t.Errorf("answer %q: expected cancel, got proceed", ans)
		}
		if !strings.Contains(stderr.String(), "Cancelled") {
			t.Errorf("answer %q: expected cancellation message, got:\n%s", ans, stderr.String())
		}
	}
}

// Preview must show every queued file and the count *before* the prompt
// fires. Acceptance criterion from confirmation-gate.md.
func TestConfirmPush_Preview_ListsFilesAndCount(t *testing.T) {
	queue := []string{
		"notion/db/page-001.md",
		"notion/db/page-002.md",
		"notion/db/page-003.md",
	}
	var stderr bytes.Buffer
	confirmPush(queue, true, false, strings.NewReader(""), &stderr)

	out := stderr.String()
	if !strings.Contains(out, "3 files") {
		t.Errorf("expected '3 files' in preview, got:\n%s", out)
	}
	for _, p := range queue {
		base := filepath.Base(p)
		if !strings.Contains(out, base) {
			t.Errorf("expected %s in preview, got:\n%s", base, out)
		}
	}
}

// Singular noun for one file — small UX detail but worth pinning.
func TestConfirmPush_Preview_SingularForOneFile(t *testing.T) {
	var stderr bytes.Buffer
	confirmPush([]string{"only.md"}, true, false, strings.NewReader(""), &stderr)
	if !strings.Contains(stderr.String(), "1 file)") {
		t.Errorf("expected '1 file)' (singular), got:\n%s", stderr.String())
	}
}

func TestCLI_Version(t *testing.T) {
	out, err := exec.Command("go", "run", ".", "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("expected version output")
	}
}
