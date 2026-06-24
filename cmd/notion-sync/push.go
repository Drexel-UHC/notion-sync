package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"

	"github.com/ran-codes/notion-sync/internal/config"
	"github.com/ran-codes/notion-sync/internal/notion"
	"github.com/ran-codes/notion-sync/internal/sync"
)

// confirmPush previews the push queue to stderr and gates execution on user
// consent. Returns true if push should proceed, false to cancel cleanly
// (caller exits 0). TTY: y/N prompt, default N. Non-TTY: requires `yes`,
// otherwise cancels with a stderr hint. Caller must short-circuit on empty
// queue + empty halts before calling.
//
// Local halts (stray .md, malformed YAML — detected without any Notion
// API call) are surfaced here so the user sees them at consent time
// instead of confirming a queue and then getting halted on a file they
// were never shown. The validation gate enumerates the full halt list
// later; this preview is the early warning.
func confirmPush(preview *sync.PushPreview, yes, isTTY bool, stdin io.Reader, stderr io.Writer) bool {
	noun := "file"
	if len(preview.Queue) != 1 {
		noun = "files"
	}
	fmt.Fprintf(stderr, "Push queue (%d %s):\n", len(preview.Queue), noun)
	for _, p := range preview.Queue {
		fmt.Fprintf(stderr, "  %s\n", filepath.Base(p))
	}

	if len(preview.LocalHalts) > 0 {
		haltNoun := "file"
		if len(preview.LocalHalts) != 1 {
			haltNoun = "files"
		}
		fmt.Fprintf(stderr, "\nWill halt the run (%d %s — fix before continuing):\n", len(preview.LocalHalts), haltNoun)
		for _, h := range preview.LocalHalts {
			fmt.Fprintf(stderr, "  %s — %s\n", filepath.Base(h.Path), h.Reason)
		}
	}
	fmt.Fprintln(stderr)

	if yes {
		fmt.Fprintln(stderr, "Proceeding (--yes).")
		return true
	}
	if !isTTY {
		fmt.Fprintln(stderr, "Cancelled — non-interactive run requires --yes to push.")
		return false
	}

	fmt.Fprint(stderr, "Proceed? [y/N]: ")
	line, _ := bufio.NewReader(stdin).ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer == "y" || answer == "yes" {
		return true
	}
	fmt.Fprintln(stderr, "Cancelled — nothing pushed to Notion.")
	return false
}

// isStdinTTY reports whether stdin is connected to a terminal (vs a pipe or
// redirect). Uses x/term so mintty-based shells (Git Bash, MSYS2, Cygwin)
// are detected correctly — the stdlib `os.ModeCharDevice` check misses them
// because mintty wraps stdio in named pipes.
func isStdinTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func runPush(args []string) error {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	apiKey := fs.String("api-key", "", "Notion API key")
	force := fs.Bool("force", false, "Skip the validation gate entirely (bypasses conflicts, strays, malformed YAML, and unreachable rows)")
	forceShort := fs.Bool("f", false, "Skip the validation gate entirely (shorthand)")
	dryRun := fs.Bool("dry-run", false, "Show what would be pushed without writing to Notion")
	yes := fs.Bool("yes", false, "Skip the confirmation prompt (required for non-interactive runs)")
	yesShort := fs.Bool("y", false, "Skip the confirmation prompt (shorthand)")
	allowNewOptions := fs.Bool("allow-new-options", false, "Let unknown select/multi_select values auto-create options in Notion (status values are always validated)")

	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("missing folder path\n" +
			"Usage: notion-sync push <folder> [--force] [--dry-run] [--yes] [--allow-new-options] [--api-key <key>]\n" +
			"Example: notion-sync push ./notion/My\\ Database")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	key := *apiKey
	if key == "" {
		key = cfg.APIKey
	}
	if msg := config.ValidateAPIKey(key); msg != "" {
		return fmt.Errorf("%s", msg)
	}

	folderPath := fs.Arg(0)
	forceFlag := *force || *forceShort
	yesFlag := *yes || *yesShort

	// Confirmation gate (DAG nodes n12b → n13 → n13a in v1.4.0 push design).
	// Push is the only command that writes to Notion; gate fires before any
	// API call. Skipped under --dry-run since no writes occur.
	if !*dryRun {
		preview, err := sync.BuildPushQueue(folderPath)
		if err != nil {
			return err
		}
		if len(preview.Queue) == 0 && len(preview.LocalHalts) == 0 {
			fmt.Fprintln(os.Stderr, "Nothing to push: no synced .md files in folder.")
			return nil
		}
		if !confirmPush(preview, yesFlag, isStdinTTY(), os.Stdin, os.Stderr) {
			return nil
		}
	}

	if *dryRun {
		fmt.Println("Pushing properties (dry run)...")
	} else if forceFlag {
		fmt.Println("Force pushing properties (ignoring conflicts)...")
	} else {
		fmt.Println("Pushing properties to Notion...")
	}

	client := notion.NewClient(key)
	var dbTitle string

	result, err := sync.PushDatabase(sync.PushOptions{
		Client:          client,
		FolderPath:      folderPath,
		Force:           forceFlag,
		DryRun:          *dryRun,
		AllowNewOptions: *allowNewOptions,
	}, func(p sync.ProgressPhase) {
		if p.Phase == sync.PhasePushing {
			dbTitle = p.Title
		}
		fmt.Printf("\r%-60s", formatPushProgress(p, dbTitle))
	})

	if err != nil {
		fmt.Println()
		return err
	}

	fmt.Println()

	// Validation halted (DAG n22a) — print the enumerated halt list and
	// exit non-zero so scripts/CI can detect the abort. No "Done"
	// message because nothing was pushed.
	if result.Halted {
		renderHaltedResult(result, os.Stdout)
		return fmt.Errorf("push halted by validation gate (%d halt(s))", len(result.Halts))
	}

	if *dryRun {
		fmt.Printf("Dry run: \"%s\"\n", result.Title)
	} else {
		fmt.Printf("Done: \"%s\"\n", result.Title)
	}
	fmt.Printf("  Total:     %d\n", result.Total)
	if *dryRun {
		fmt.Printf("  Would push: %d\n", result.Pushed)
	} else {
		fmt.Printf("  Pushed:     %d\n", result.Pushed)
	}
	fmt.Printf("  Skipped:   %d\n", result.Skipped)
	if result.SkippedNoOp > 0 {
		fmt.Printf("  Unchanged: %d (already in sync — nothing to push)\n", result.SkippedNoOp)
	}

	if result.Conflicts > 0 {
		fmt.Printf("  Conflicts: %d (Notion has newer edits — use --force to overwrite)\n", result.Conflicts)
		for _, f := range result.ConflictFiles {
			fmt.Printf("    - %s\n", f)
		}
	}

	if result.Failed > 0 {
		fmt.Printf("  Failed:    %d\n", result.Failed)
		for _, e := range result.Errors {
			fmt.Printf("    - %s\n", e)
		}
	}

	if result.Conflicts > 0 {
		return fmt.Errorf("%d file(s) have conflicts; use --force to overwrite", result.Conflicts)
	}
	if result.Failed > 0 {
		return fmt.Errorf("%d file(s) failed to push", result.Failed)
	}

	return nil
}

// renderHaltedResult writes the validation-gate halt summary (DAG n22a)
// to w. Extracted so the user-visible halt format — header lines, per-halt
// line shape, and the haltClassLabel mapping — is unit-testable without a
// subprocess or a fake Notion server.
func renderHaltedResult(result *sync.PushResult, w io.Writer) {
	fmt.Fprintf(w, "Halted: \"%s\"\n", result.Title)
	fmt.Fprintf(w, "  Inspected: %d\n", result.Total)
	fmt.Fprintf(w, "  Halts:     %d (nothing pushed — fix all before retrying)\n", len(result.Halts))
	for _, h := range result.Halts {
		fmt.Fprintf(w, "    - %s [%s] — %s\n", filepath.Base(h.Path), haltClassLabel(h.Class), h.Reason)
	}
}

// haltClassLabel renders a Classification as a short user-facing label
// for the halt list. Mirrors the DAG node taxonomy so the user can map
// an output line back to the spec without a code dive.
func haltClassLabel(c sync.Classification) string {
	switch c {
	case sync.ClassHaltConflict:
		return "conflict"
	case sync.ClassHaltUnexpected:
		return "stray"
	case sync.ClassHaltUnreachable:
		return "unreachable"
	case sync.ClassHaltMalformed:
		return "malformed"
	case sync.ClassHaltUnreadable:
		return "unreadable"
	case sync.ClassHaltInvalidOption:
		return "invalid-option"
	default:
		return "halt"
	}
}
