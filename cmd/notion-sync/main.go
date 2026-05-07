package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ran-codes/notion-sync/internal/clean"
	"github.com/ran-codes/notion-sync/internal/config"
	"github.com/ran-codes/notion-sync/internal/notion"
	"github.com/ran-codes/notion-sync/internal/sync"
)

// version is set at build time via -ldflags.
var version = "dev"

// 0. Arg Parsing & Usage ----

// boolFlags lists flags that don't take a value argument.
var boolFlags = map[string]bool{
	"--force": true, "-f": true,
	"--keep-presigned-params": true,
	"--dry-run":               true,
	"--yes":                   true, "-y": true,
}

// reorderArgs moves flag arguments (starting with "-") before positional arguments
// so that Go's flag package can parse them regardless of order.
func reorderArgs(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// "--" marks end of flags; keep everything after as positional.
		if arg == "--" {
			positional = append(positional, args[i:]...)
			break
		}

		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			// Don't consume next arg if flag already has "=" value or is boolean.
			if strings.Contains(arg, "=") || boolFlags[arg] {
				continue
			}
			// Consume next arg as the flag's value if it exists and isn't a flag.
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			positional = append(positional, arg)
		}
	}
	return append(flags, positional...)
}

var usage = `notion-sync — Sync Notion databases and pages to Markdown (v` + version + `)

Usage:
  notion-sync import <database-or-page-id> [--output <folder>] [--api-key <key>] [--keep-presigned-params]
  notion-sync refresh <folder> [--ids id1,id2] [--force] [--api-key <key>] [--keep-presigned-params]
  notion-sync push <folder> [--force] [--dry-run] [--yes] [--api-key <key>]
  notion-sync list [<output-folder>]
  notion-sync clean <folder> [--dry-run]
  notion-sync agents-md <folder>
  notion-sync config set <key> <value>

Commands:
  import    Import a Notion database or standalone page to local Markdown files
            Automatically detects whether the ID is a database or page.
            --keep-presigned-params  Keep AWS S3 pre-signed query strings on file URLs
                                     (default: stripped to keep diffs stable)
  refresh   Refresh an existing synced database or standalone page (incremental update)
            --ids id1,id2  Refresh only specific pages by ID (databases only)
            --force, -f    Resync all entries, ignoring timestamps
            --keep-presigned-params  Keep AWS S3 pre-signed query strings on file URLs
  push      Push local frontmatter property changes back to Notion (properties only, no body)
            Previews the push queue and prompts y/N before any API write (TTY only).
            Non-interactive runs (CI / pipes) must pass --yes; otherwise the run is cancelled.
            Refuses to push files where Notion has been edited since last sync, unless --force.
            --force, -f    Push even if Notion has newer edits (overwrites Notion-side changes)
            --dry-run      Show what would be pushed without writing to Notion (still reads from Notion for conflict detection)
            --yes, -y      Skip the confirmation prompt (required in non-interactive runs)
  list      List all synced databases and pages in a folder
  clean     Strip AWS S3 pre-signed query strings from existing .md files in a folder.
            Useful one-time backfill after upgrading. No API calls.
            Also regenerates AGENTS.md if its version stamp is older than this binary.
            --dry-run  Show what would change without writing
  agents-md Regenerate AGENTS.md in a workspace from the running binary.
            Always overwrites any existing AGENTS.md (the command name is the consent).
            No API calls.
  config    Manage configuration (apiKey, defaultOutputFolder)

Examples:
  notion-sync import abc123de-f456-7890-abcd-ef1234567890 --output ./my-notes
  notion-sync refresh ./notion/My\ Database
  notion-sync refresh ./notion/pages/My\ Page_abc12345
  notion-sync refresh ./notion/My\ Database --force
  notion-sync push ./notion/My\ Database
  notion-sync push ./notion/My\ Database --dry-run
  notion-sync clean ./notion --dry-run
  notion-sync list ./notion

API Key Priority:
  1. NOTION_SYNC_API_KEY env var
  2. OS keychain (set via: notion-sync config set apiKey <key>)
  3. Config file fallback (~/.notion-sync.json)
`

// 1. Entry Point ----

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(0)
	}

	// Handle help and version flags
	if os.Args[1] == "--help" || os.Args[1] == "-h" {
		fmt.Print(usage)
		os.Exit(0)
	}
	if os.Args[1] == "--version" || os.Args[1] == "-v" {
		fmt.Printf("notion-sync %s\n", version)
		os.Exit(0)
	}

	// Expose version to sync package for metadata
	sync.Version = version

	// Migrate API key on startup
	config.MigrateAPIKeyToKeychain()

	command := os.Args[1]
	args := os.Args[2:]

	var err error
	switch command {
	case "import":
		err = runImport(args)
	case "refresh":
		err = runRefresh(args)
	case "push":
		err = runPush(args)
	case "list":
		err = runList(args)
	case "clean":
		err = runClean(args)
	case "agents-md":
		err = runAgentsMD(args)
	case "config":
		err = runConfig(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		fmt.Print(usage)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// 2. Commands ----

//// 2.1 Import ----

func runImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	output := fs.String("output", "", "Output folder")
	outputAlt := fs.String("out", "", "Output folder (alias)")
	output2 := fs.String("o", "", "Output folder (shorthand)")
	apiKey := fs.String("api-key", "", "Notion API key")
	keepPresigned := fs.Bool("keep-presigned-params", false, "Keep AWS S3 pre-signed query strings on file URLs")

	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("missing Notion database or page URL/ID\n" +
			"Usage: notion-sync import <database-or-page-id> [--output <folder>] [--api-key <key>]")
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

	outputFolder := *output
	if outputFolder == "" {
		outputFolder = *outputAlt
	}
	if outputFolder == "" {
		outputFolder = *output2
	}
	if outputFolder == "" {
		outputFolder = cfg.DefaultOutputFolder
	}

	rawID := fs.Arg(0)
	notionID, err := notion.NormalizeNotionID(rawID)
	if err != nil {
		return err
	}

	client := notion.NewClient(key)

	stripPresigned := !*keepPresigned

	// Auto-detect: try database first, fall back to page
	_, dbErr := client.GetDatabase(notionID)
	if dbErr != nil && notion.IsNotFoundError(dbErr) {
		// Not a database — try as a standalone page
		return runImportPage(client, notionID, outputFolder, stripPresigned)
	}
	if dbErr != nil {
		return fmt.Errorf("fetch database: %w", dbErr)
	}

	// It's a database — proceed with database import
	fmt.Println("Importing database...")
	var dbTitle string

	result, err := sync.FreshDatabaseImport(sync.DatabaseImportOptions{
		Client:         client,
		DatabaseID:     notionID,
		OutputFolder:   outputFolder,
		StripPresigned: stripPresigned,
	}, func(p sync.ProgressPhase) {
		if p.Phase == sync.PhaseImporting {
			dbTitle = p.Title
		}
		fmt.Printf("\r%-60s", formatProgress(p, dbTitle))
	})

	if err != nil {
		fmt.Println()
		return err
	}

	fmt.Println()
	fmt.Printf("Done: \"%s\"\n", result.Title)
	fmt.Printf("  Folder:  %s\n", result.FolderPath)
	fmt.Printf("  Total:   %d\n", result.Total)
	fmt.Printf("  Created: %d\n", result.Created)
	fmt.Printf("  Updated: %d\n", result.Updated)
	fmt.Printf("  Skipped: %d\n", result.Skipped)

	if result.Failed > 0 {
		fmt.Printf("  Failed:  %d\n", result.Failed)
		for _, e := range result.Errors {
			fmt.Printf("    - %s\n", e)
		}
	}

	return nil
}

func runImportPage(client *notion.Client, pageID, outputFolder string, stripPresigned bool) error {
	fmt.Println("Importing standalone page...")
	fmt.Println("Note: Pages with deeply nested content (bullet points, toggles, callouts)")
	fmt.Println("      require one API call per nesting level. Notion limits ~3 requests/sec.")

	result, err := sync.FreezeStandalonePage(sync.StandalonePageImportOptions{
		Client:         client,
		PageID:         pageID,
		OutputFolder:   outputFolder,
		StripPresigned: stripPresigned,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Done: \"%s\"\n", result.Title)
	fmt.Printf("  Folder: %s\n", result.FolderPath)
	fmt.Printf("  Status: %s\n", result.Status)

	return nil
}

//// 2.2 Refresh ----

func runRefresh(args []string) error {
	fs := flag.NewFlagSet("refresh", flag.ExitOnError)
	apiKey := fs.String("api-key", "", "Notion API key")
	force := fs.Bool("force", false, "Resync all entries, ignoring timestamps")
	forceShort := fs.Bool("f", false, "Resync all entries (shorthand)")
	ids := fs.String("ids", "", "Comma-separated Notion page IDs to refresh")
	keepPresigned := fs.Bool("keep-presigned-params", false, "Keep AWS S3 pre-signed query strings on file URLs")

	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("missing folder path\n" +
			"Usage: notion-sync refresh <folder> [--ids id1,id2] [--force] [--api-key <key>]\n" +
			"Example: notion-sync refresh ./notion/My\\ Database")
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
	forceRefresh := *force || *forceShort
	stripPresigned := !*keepPresigned

	// Parse --ids flag
	var pageIDs []string
	if *ids != "" {
		for _, raw := range strings.Split(*ids, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			normalized, err := notion.NormalizeNotionID(raw)
			if err != nil {
				return fmt.Errorf("invalid ID %q: %w", raw, err)
			}
			pageIDs = append(pageIDs, normalized)
		}
	}

	client := notion.NewClient(key)

	// Check if the folder is a standalone page (has _page.json)
	pageMeta, _ := sync.ReadPageMetadata(folderPath)
	if pageMeta != nil {
		return runRefreshPage(client, folderPath, forceRefresh, stripPresigned)
	}

	if len(pageIDs) > 0 {
		fmt.Printf("Refreshing %d specific page(s)...\n", len(pageIDs))
	} else if forceRefresh {
		fmt.Println("Force refreshing database (ignoring timestamps)...")
	} else {
		fmt.Println("Refreshing database...")
	}
	var dbTitle string

	result, err := sync.RefreshDatabase(sync.RefreshOptions{
		Client:         client,
		FolderPath:     folderPath,
		Force:          forceRefresh,
		PageIDs:        pageIDs,
		StripPresigned: stripPresigned,
	}, func(p sync.ProgressPhase) {
		if p.Phase == sync.PhaseImporting {
			dbTitle = p.Title
		}
		fmt.Printf("\r%-60s", formatProgress(p, dbTitle))
	})

	if err != nil {
		fmt.Println()
		return err
	}

	fmt.Println()
	fmt.Printf("Done: \"%s\"\n", result.Title)
	fmt.Printf("  Total:   %d\n", result.Total)
	fmt.Printf("  Created: %d\n", result.Created)
	fmt.Printf("  Updated: %d\n", result.Updated)
	fmt.Printf("  Skipped: %d\n", result.Skipped)
	fmt.Printf("  Deleted: %d\n", result.Deleted)

	if result.Failed > 0 {
		fmt.Printf("  Failed:  %d\n", result.Failed)
		for _, e := range result.Errors {
			fmt.Printf("    - %s\n", e)
		}
	}

	return nil
}

func runRefreshPage(client *notion.Client, folderPath string, force, stripPresigned bool) error {
	if force {
		fmt.Println("Force refreshing page (ignoring timestamps)...")
	} else {
		fmt.Println("Refreshing page...")
	}

	result, err := sync.RefreshStandalonePage(sync.RefreshStandalonePageOptions{
		Client:         client,
		FolderPath:     folderPath,
		Force:          force,
		StripPresigned: stripPresigned,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Done: \"%s\"\n", result.Title)
	fmt.Printf("  Status: %s\n", result.Status)

	return nil
}

//// 2.3 Push ----

// confirmPush previews the push queue to stderr and gates execution on user
// consent. Returns true if push should proceed, false to cancel cleanly
// (caller exits 0). TTY: y/N prompt, default N. Non-TTY: requires `yes`,
// otherwise cancels with a stderr hint. Caller must short-circuit on empty
// queue before calling.
func confirmPush(queue []string, yes, isTTY bool, stdin io.Reader, stderr io.Writer) bool {
	noun := "file"
	if len(queue) != 1 {
		noun = "files"
	}
	fmt.Fprintf(stderr, "Push queue (%d %s):\n", len(queue), noun)
	for _, p := range queue {
		fmt.Fprintf(stderr, "  %s\n", filepath.Base(p))
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
// redirect). Stdlib-only check that works on Windows and Unix.
func isStdinTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func runPush(args []string) error {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	apiKey := fs.String("api-key", "", "Notion API key")
	force := fs.Bool("force", false, "Push even if Notion has newer edits")
	forceShort := fs.Bool("f", false, "Push even if Notion has newer edits (shorthand)")
	dryRun := fs.Bool("dry-run", false, "Show what would be pushed without writing to Notion")
	yes := fs.Bool("yes", false, "Skip the confirmation prompt (required for non-interactive runs)")
	yesShort := fs.Bool("y", false, "Skip the confirmation prompt (shorthand)")

	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("missing folder path\n" +
			"Usage: notion-sync push <folder> [--force] [--dry-run] [--yes] [--api-key <key>]\n" +
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
		queue, err := sync.BuildPushQueue(folderPath)
		if err != nil {
			return err
		}
		if len(queue) == 0 {
			fmt.Fprintln(os.Stderr, "Nothing to push: no synced .md files in folder.")
			return nil
		}
		if !confirmPush(queue, yesFlag, isStdinTTY(), os.Stdin, os.Stderr) {
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
		Client:     client,
		FolderPath: folderPath,
		Force:      forceFlag,
		DryRun:     *dryRun,
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

//// 2.4 List ----

func runList(args []string) error {
	outputFolder := "./notion"
	if len(args) > 0 {
		outputFolder = args[0]
	}

	databases, err := sync.ListSyncedDatabases(outputFolder)
	if err != nil {
		return err
	}

	pages, err := sync.ListSyncedPages(outputFolder)
	if err != nil {
		return err
	}

	if len(databases) == 0 && len(pages) == 0 {
		fmt.Printf("No synced databases or pages found in %s\n", outputFolder)
		return nil
	}

	if len(databases) > 0 {
		fmt.Printf("Synced databases in %s:\n\n", outputFolder)
		for _, db := range databases {
			fmt.Printf("  %s\n", db.Title)
			fmt.Printf("    Folder:      %s\n", db.FolderPath)
			fmt.Printf("    Database ID: %s\n", db.DatabaseID)
			fmt.Printf("    Entries:     %d\n", db.EntryCount)
			fmt.Printf("    Last synced: %s\n", db.LastSyncedAt)
			fmt.Println()
		}
	}

	if len(pages) > 0 {
		fmt.Printf("Synced pages in %s:\n\n", outputFolder)
		for _, p := range pages {
			fmt.Printf("  %s\n", p.Title)
			fmt.Printf("    Folder:      %s\n", p.FolderPath)
			fmt.Printf("    Page ID:     %s\n", p.PageID)
			fmt.Printf("    Last synced: %s\n", p.LastSyncedAt)
			fmt.Println()
		}
	}

	return nil
}

//// 2.6 Clean ----

func runClean(args []string) error {
	fs := flag.NewFlagSet("clean", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "Show what would change without writing")

	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("missing folder path\n" +
			"Usage: notion-sync clean <folder> [--dry-run]\n" +
			"Example: notion-sync clean ./notion")
	}

	folder := fs.Arg(0)
	if *dryRun {
		fmt.Printf("Scanning %s (dry run)...\n", folder)
	} else {
		fmt.Printf("Cleaning %s...\n", folder)
	}

	r, err := clean.Folder(folder, *dryRun)
	if err != nil {
		return err
	}

	label := "Modified"
	bumpLabel := "Stamped"
	if *dryRun {
		label = "Would modify"
		bumpLabel = "Would stamp"
	}
	fmt.Printf("Scanned: %d files\n", r.FilesScanned)
	fmt.Printf("%s: %d files (%d URLs stripped, %d URLs canonicalized, %d folderPaths normalized, %d trailing newlines added, %d notion-frozen-at lines stripped)\n",
		label, r.FilesChanged, r.URLsStripped, r.URLsCanonicalized, r.FolderPathsNormalized, r.NewlinesFixed, r.FrozenAtStripped)
	fmt.Printf("%s syncVersion in: %d folder(s)\n", bumpLabel, r.MetadataBumped)
	if r.AgentsMDWritten > 0 {
		agentsLabel := "Regenerated"
		if *dryRun {
			agentsLabel = "Would regenerate"
		}
		fmt.Printf("%s AGENTS.md\n", agentsLabel)
	}
	return nil
}

//// 2.7 AgentsMD ----

func runAgentsMD(args []string) error {
	fs := flag.NewFlagSet("agents-md", flag.ExitOnError)

	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("missing folder path\n" +
			"Usage: notion-sync agents-md <folder>\n" +
			"Example: notion-sync agents-md ./notion")
	}

	folder := fs.Arg(0)
	if err := sync.RegenerateAgentsMD(folder); err != nil {
		return err
	}
	fmt.Printf("Wrote AGENTS.md in %s (notion-sync %s)\n", folder, version)
	return nil
}

//// 2.8 Config ----

func runConfig(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: notion-sync config get [key]\n" +
			"       notion-sync config set <key> <value>\n" +
			"Keys: apiKey, defaultOutputFolder")
	}

	if args[0] == "get" {
		return runConfigGet(args[1:])
	}

	if args[0] != "set" || len(args) < 3 {
		return fmt.Errorf("usage: notion-sync config get [key]\n" +
			"       notion-sync config set <key> <value>\n" +
			"Keys: apiKey, defaultOutputFolder")
	}

	key := args[1]
	value := args[2]

	validKeys := []string{"apiKey", "defaultOutputFolder"}
	isValid := false
	for _, k := range validKeys {
		if k == key {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("unknown config key: %s\nValid keys: %s", key, strings.Join(validKeys, ", "))
	}

	if key == "apiKey" {
		if msg := config.ValidateAPIKey(value); msg != "" {
			return fmt.Errorf("%s", msg)
		}
	}

	if err := config.SaveConfig(key, value); err != nil {
		return err
	}

	fmt.Printf("Saved %s\n", key)
	return nil
}

func runConfigGet(args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	// If a specific key is requested
	if len(args) > 0 {
		switch args[0] {
		case "apiKey":
			printAPIKeyStatus(cfg.APIKey)
		case "defaultOutputFolder":
			fmt.Println(cfg.DefaultOutputFolder)
		default:
			return fmt.Errorf("unknown config key: %s", args[0])
		}
		return nil
	}

	// Show all config
	fmt.Println("Config:")
	fmt.Printf("  apiKey:              ")
	printAPIKeyStatus(cfg.APIKey)
	fmt.Printf("  defaultOutputFolder: %s\n", cfg.DefaultOutputFolder)
	fmt.Printf("\n  Config file: %s\n", config.GetConfigPath())
	return nil
}

func printAPIKeyStatus(key string) {
	if key == "" {
		fmt.Println("(not set)")
	} else if len(key) > 8 {
		fmt.Printf("...%s (set, %d chars)\n", key[len(key)-4:], len(key))
	} else {
		fmt.Printf("(set, %d chars)\n", len(key))
	}
}

// 3. Helpers ----

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
