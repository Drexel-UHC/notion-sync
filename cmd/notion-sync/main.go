package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

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
  notion-sync import <database-or-page-id> [--output <folder>] [--output-mode both|markdown|sqlite] [--api-key <key>]
  notion-sync refresh <folder> [--ids id1,id2] [--force] [--output-mode both|markdown|sqlite] [--api-key <key>]
  notion-sync list [<output-folder>]
  notion-sync config set <key> <value>

Commands:
  import    Import a Notion database or standalone page to local Markdown files
            Automatically detects whether the ID is a database or page.
  refresh   Refresh an existing synced database or standalone page (incremental update)
            --ids id1,id2  Refresh only specific pages by ID (databases only)
            --force, -f    Resync all entries, ignoring timestamps
  list      List all synced databases and pages in a folder
  config    Manage configuration (apiKey, defaultOutputFolder, outputMode)

Examples:
  notion-sync import abc123de-f456-7890-abcd-ef1234567890 --output ./my-notes
  notion-sync refresh ./notion/My\ Database
  notion-sync refresh ./notion/pages/My\ Page_abc12345
  notion-sync refresh ./notion/My\ Database --force
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
	case "list":
		err = runList(args)
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
	outputMode := fs.String("output-mode", "", "Output mode: both, markdown, sqlite (default: both)")

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
	if key == "" {
		return fmt.Errorf("no API key provided.\n" +
			"Set it via: notion-sync config set apiKey <key> (stored in OS keychain)\n" +
			"Or pass --api-key <key>, or set NOTION_SYNC_API_KEY env var")
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

	mode, err := resolveOutputModeFlag(*outputMode, cfg)
	if err != nil {
		return err
	}

	// Auto-detect: try database first, fall back to page
	_, dbErr := client.GetDatabase(notionID)
	if dbErr != nil && notion.IsNotFoundError(dbErr) {
		// Not a database — try as a standalone page
		return runImportPage(client, notionID, outputFolder, mode)
	}
	if dbErr != nil {
		return fmt.Errorf("fetch database: %w", dbErr)
	}

	// It's a database — proceed with database import
	fmt.Println("Importing database...")
	var dbTitle string

	result, err := sync.FreshDatabaseImport(sync.DatabaseImportOptions{
		Client:       client,
		DatabaseID:   notionID,
		OutputFolder: outputFolder,
		OutputMode:   mode,
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

func runImportPage(client *notion.Client, pageID, outputFolder string, mode sync.OutputMode) error {
	fmt.Println("Importing standalone page...")

	result, err := sync.FreezeStandalonePage(sync.StandalonePageImportOptions{
		Client:       client,
		PageID:       pageID,
		OutputFolder: outputFolder,
		OutputMode:   mode,
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
	outputMode := fs.String("output-mode", "", "Output mode: both, markdown, sqlite (default: both)")

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
	if key == "" {
		return fmt.Errorf("no API key provided")
	}

	folderPath := fs.Arg(0)
	forceRefresh := *force || *forceShort

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

	mode, err := resolveOutputModeFlag(*outputMode, cfg)
	if err != nil {
		return err
	}

	// Check if the folder is a standalone page (has _page.json)
	pageMeta, _ := sync.ReadPageMetadata(folderPath)
	if pageMeta != nil {
		return runRefreshPage(client, folderPath, forceRefresh, mode)
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
		Client:     client,
		FolderPath: folderPath,
		Force:      forceRefresh,
		PageIDs:    pageIDs,
		OutputMode: mode,
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

func runRefreshPage(client *notion.Client, folderPath string, force bool, mode sync.OutputMode) error {
	if force {
		fmt.Println("Force refreshing page (ignoring timestamps)...")
	} else {
		fmt.Println("Refreshing page...")
	}

	result, err := sync.RefreshStandalonePage(sync.RefreshStandalonePageOptions{
		Client:     client,
		FolderPath: folderPath,
		Force:      force,
		OutputMode: mode,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Done: \"%s\"\n", result.Title)
	fmt.Printf("  Status: %s\n", result.Status)

	return nil
}

//// 2.3 List ----

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

//// 2.4 Config ----

func runConfig(args []string) error {
	if len(args) < 3 || args[0] != "set" {
		return fmt.Errorf("usage: notion-sync config set <key> <value>\n" +
			"Keys: apiKey, defaultOutputFolder")
	}

	key := args[1]
	value := args[2]

	validKeys := []string{"apiKey", "defaultOutputFolder", "outputMode"}
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

	if err := config.SaveConfig(key, value); err != nil {
		return err
	}

	fmt.Printf("Saved %s\n", key)
	return nil
}

// 3. Helpers ----

// resolveOutputModeFlag resolves the output mode from flag > config > default.
func resolveOutputModeFlag(flagValue string, cfg config.Config) (sync.OutputMode, error) {
	mode := flagValue
	if mode == "" {
		mode = cfg.OutputMode
	}
	if mode == "" {
		return sync.OutputBoth, nil
	}
	switch sync.OutputMode(mode) {
	case sync.OutputBoth, sync.OutputMarkdown, sync.OutputSQLite:
		return sync.OutputMode(mode), nil
	default:
		return "", fmt.Errorf("invalid output-mode %q (valid: both, markdown, sqlite)", mode)
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
