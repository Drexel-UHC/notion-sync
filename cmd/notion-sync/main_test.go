package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ran-codes/notion-sync/internal/sync"
)

// previewOf wraps a queue (and optional local halts) into the *PushPreview
// shape confirmPush now takes. Keeps the tests below readable.
func previewOf(queue []string, halts ...sync.FileClassification) *sync.PushPreview {
	return &sync.PushPreview{Queue: queue, LocalHalts: halts}
}

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
	ok := confirmPush(previewOf([]string{"a/page.md"}), true, false, strings.NewReader(""), &stderr)
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
	ok := confirmPush(previewOf([]string{"a/page.md"}), false, false, strings.NewReader(""), &stderr)
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
		ok := confirmPush(previewOf([]string{"a/page.md"}), false, true, strings.NewReader(ans), &stderr)
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
		ok := confirmPush(previewOf([]string{"a/page.md"}), false, true, strings.NewReader(ans), &stderr)
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
	confirmPush(previewOf(queue), true, false, strings.NewReader(""), &stderr)

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
	confirmPush(previewOf([]string{"only.md"}), true, false, strings.NewReader(""), &stderr)
	if !strings.Contains(stderr.String(), "1 file)") {
		t.Errorf("expected '1 file)' (singular), got:\n%s", stderr.String())
	}
}

// Local halts (stray .md, malformed YAML) must surface in the confirmation
// preview — otherwise the user confirms a queue and gets halted on a file
// they were never shown. The DAG calls this the fix-once-rerun-once UX.
func TestConfirmPush_Preview_ListsLocalHaltsBeforePrompt(t *testing.T) {
	preview := &sync.PushPreview{
		Queue: []string{"notion/db/page-001.md"},
		LocalHalts: []sync.FileClassification{
			{Path: "notion/db/stray.md", Class: sync.ClassHaltUnexpected, Reason: "no notion-id"},
			{Path: "notion/db/broken.md", Class: sync.ClassHaltMalformed, Reason: "could not parse YAML"},
		},
	}
	var stderr bytes.Buffer
	confirmPush(preview, true, false, strings.NewReader(""), &stderr)

	out := stderr.String()
	if !strings.Contains(out, "Will halt") {
		t.Errorf("expected halt warning in preview, got:\n%s", out)
	}
	for _, name := range []string{"stray.md", "broken.md"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected halt list to mention %s, got:\n%s", name, out)
		}
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

// --- CLI e2e tests for the push confirmation gate ---
//
// These exercise the full CLI wiring (flag parsing → API-key validation →
// gate → push flow) that the in-process confirmPush unit tests above don't
// cover. Subprocess stdin is non-TTY, so isStdinTTY() returns false in the
// child process — exactly the CI / piped-input scenario we care about.

// dummyAPIKey is a syntactically valid Notion API key (ntn_ prefix, >= 20
// chars) that passes config.ValidateAPIKey but won't authenticate against
// the real API. Lets these tests reach the gate without a real key, and
// guarantees any subsequent Notion call fails fast.
const dummyAPIKey = "ntn_0000000000000000000000000000000000000000000000"

// setupPushFolder creates a tmp folder with a minimal _database.json plus
// one .md file with a notion-id frontmatter — enough for BuildPushQueue to
// return a non-empty queue so the gate actually fires.
func setupPushFolder(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()

	meta := `{
  "databaseId": "00000000-0000-0000-0000-000000000001",
  "title": "Test DB",
  "url": "",
  "folderPath": "",
  "lastSyncedAt": "",
  "entryCount": 1
}
`
	if err := os.WriteFile(filepath.Join(tmp, "_database.json"), []byte(meta), 0644); err != nil {
		t.Fatal(err)
	}
	page := "---\nnotion-id: 00000000-0000-0000-0000-000000000002\ntitle: Page One\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(tmp, "page-one.md"), []byte(page), 0644); err != nil {
		t.Fatal(err)
	}
	return tmp
}

// pushCmd builds a `go run . push <folder> [extra...]` command with a dummy
// API key in the env so validation passes and the run reaches the gate.
func pushCmd(folder string, extra ...string) *exec.Cmd {
	args := append([]string{"run", ".", "push", folder}, extra...)
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "NOTION_SYNC_API_KEY="+dummyAPIKey)
	return cmd
}

// Non-TTY (subprocess stdin) without --yes must cancel cleanly with exit 0
// and a stderr hint pointing at --yes. The push flow ("Pushing properties
// to Notion...") must NOT execute — proves the gate fired before any
// Notion API call.
func TestCLI_Push_NonTTY_NoYes_Cancels(t *testing.T) {
	tmp := setupPushFolder(t)

	out, err := pushCmd(tmp).CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0 on cancel, got %v\n%s", err, out)
	}
	s := string(out)
	if !strings.Contains(s, "Cancelled") {
		t.Errorf("expected 'Cancelled' in output, got:\n%s", s)
	}
	if !strings.Contains(s, "--yes") {
		t.Errorf("expected '--yes' hint in output, got:\n%s", s)
	}
	if strings.Contains(s, "Pushing properties to Notion...") {
		t.Errorf("gate should fire before push flow, but push started:\n%s", s)
	}
}

// --yes bypasses the gate even in non-TTY. We don't care that the push
// itself fails (dummy key → 401); we only need to prove control reached
// the push flow past the gate.
func TestCLI_Push_Yes_PassesGate(t *testing.T) {
	tmp := setupPushFolder(t)

	out, _ := pushCmd(tmp, "--yes").CombinedOutput()
	s := string(out)
	if strings.Contains(s, "Cancelled — non-interactive") {
		t.Errorf("--yes should bypass gate, but got cancellation:\n%s", s)
	}
	if !strings.Contains(s, "Pushing properties to Notion...") {
		t.Errorf("expected push flow to start past the gate, got:\n%s", s)
	}
}

// renderHaltedResult formats the user-visible halt summary the CLI prints
// when the validation gate aborts. Pins the exact output shape so a refactor
// that breaks the "Halted:" header, drops the [class] label, mangles the
// inspected/halts counts, or stops base-naming the halt path trips this test.
// The full subprocess CLI test can't reach this code path (the dummy API key
// dies on schema fetch before the gate fires) — extracting + testing the
// renderer directly is the path that actually pins the contract.
func TestRenderHaltedResult_FormatsHeaderAndPerHaltLines(t *testing.T) {
	result := &sync.PushResult{
		Title:  "Test DB",
		Total:  4, // 4 inspected: 2 ready + 2 halted
		Halted: true,
		Halts: []sync.FileClassification{
			{Path: "/tmp/folder/page-2.md", Class: sync.ClassHaltConflict, Reason: "Notion's row has changed since last sync"},
			{Path: "/tmp/folder/stray.md", Class: sync.ClassHaltUnexpected, Reason: "file has no notion-id"},
		},
	}
	var buf bytes.Buffer
	renderHaltedResult(result, &buf)
	out := buf.String()

	// Header: title quoted, inspected count from result.Total (NOT len(Halts)),
	// halts count + the "nothing pushed" hint.
	if !strings.Contains(out, `Halted: "Test DB"`) {
		t.Errorf("expected quoted title in 'Halted:' header, got:\n%s", out)
	}
	if !strings.Contains(out, "Inspected: 4") {
		t.Errorf("expected 'Inspected: 4' (from result.Total), got:\n%s", out)
	}
	if !strings.Contains(out, "Halts:     2 (nothing pushed") {
		t.Errorf("expected halts count + 'nothing pushed' hint, got:\n%s", out)
	}

	// Per-halt lines: basename only (not full path), [haltClassLabel], reason.
	// Two different classes prove the haltClassLabel mapping is wired — if the
	// switch breaks, both labels fall through to "halt" and these miss.
	if !strings.Contains(out, "page-2.md [conflict] — Notion's row has changed since last sync") {
		t.Errorf("expected page-2.md line with [conflict] label and reason, got:\n%s", out)
	}
	if !strings.Contains(out, "stray.md [stray] — file has no notion-id") {
		t.Errorf("expected stray.md line with [stray] label and reason, got:\n%s", out)
	}
	// Full path must NOT appear — basename only, otherwise output bloats with
	// the user's tmp paths.
	if strings.Contains(out, "/tmp/folder/page-2.md") {
		t.Errorf("expected basename only in halt line, got full path in:\n%s", out)
	}
}

// --dry-run skips the gate entirely (no writes, no consent needed). The
// gate's preview header ("Push queue (") must not appear, and the dry-run
// banner must.
func TestCLI_Push_DryRun_SkipsGate(t *testing.T) {
	tmp := setupPushFolder(t)

	out, _ := pushCmd(tmp, "--dry-run").CombinedOutput()
	s := string(out)
	if strings.Contains(s, "Push queue (") {
		t.Errorf("--dry-run should skip the gate, but preview ran:\n%s", s)
	}
	if !strings.Contains(s, "Pushing properties (dry run)...") {
		t.Errorf("expected dry-run banner, got:\n%s", s)
	}
}
