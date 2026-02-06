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

const usage = `notion-sync — Sync Notion databases to Markdown

Usage:
  notion-sync import <database-id> [--output <folder>] [--api-key <key>]
  notion-sync refresh <database-folder> [--force] [--api-key <key>]
  notion-sync list [<output-folder>]
  notion-sync config set <key> <value>

Commands:
  import    Import a Notion database to local Markdown files
  refresh   Refresh an existing synced database (incremental update)
            --force, -f  Resync all entries, ignoring timestamps
  list      List all synced databases in a folder
  config    Manage configuration (apiKey, defaultOutputFolder)

Examples:
  notion-sync import abc123de-f456-7890-abcd-ef1234567890 --output ./my-notes
  notion-sync refresh ./notion/My\ Database
  notion-sync refresh ./notion/My\ Database --force
  notion-sync list ./notion

API Key Priority:
  1. NOTION_SYNC_API_KEY env var
  2. OS keychain (set via: notion-sync config set apiKey <key>)
  3. Config file fallback (~/.notion-sync.json)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(0)
	}

	// Handle help flags
	if os.Args[1] == "--help" || os.Args[1] == "-h" {
		fmt.Print(usage)
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

func runImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	output := fs.String("output", "", "Output folder")
	outputAlt := fs.String("out", "", "Output folder (alias)")
	output2 := fs.String("o", "", "Output folder (shorthand)")
	apiKey := fs.String("api-key", "", "Notion API key")

	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("missing Notion database URL or ID\n" +
			"Usage: notion-sync import <database-id> [--output <folder>] [--api-key <key>]")
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
	databaseID, err := notion.NormalizeNotionID(rawID)
	if err != nil {
		return err
	}

	client := notion.NewClient(key)

	fmt.Println("Importing database...")
	var dbTitle string

	result, err := sync.FreshDatabaseImport(sync.DatabaseImportOptions{
		Client:       client,
		DatabaseID:   databaseID,
		OutputFolder: outputFolder,
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

func runRefresh(args []string) error {
	fs := flag.NewFlagSet("refresh", flag.ExitOnError)
	apiKey := fs.String("api-key", "", "Notion API key")
	force := fs.Bool("force", false, "Resync all entries, ignoring timestamps")
	forceShort := fs.Bool("f", false, "Resync all entries (shorthand)")

	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("missing database folder path\n" +
			"Usage: notion-sync refresh <database-folder> [--force] [--api-key <key>]\n" +
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

	client := notion.NewClient(key)

	if forceRefresh {
		fmt.Println("Force refreshing database (ignoring timestamps)...")
	} else {
		fmt.Println("Refreshing database...")
	}
	var dbTitle string

	result, err := sync.RefreshDatabase(sync.RefreshOptions{
		Client:     client,
		FolderPath: folderPath,
		Force:      forceRefresh,
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

func runList(args []string) error {
	outputFolder := "./notion"
	if len(args) > 0 {
		outputFolder = args[0]
	}

	databases, err := sync.ListSyncedDatabases(outputFolder)
	if err != nil {
		return err
	}

	if len(databases) == 0 {
		fmt.Printf("No synced databases found in %s\n", outputFolder)
		return nil
	}

	fmt.Printf("Synced databases in %s:\n\n", outputFolder)
	for _, db := range databases {
		fmt.Printf("  %s\n", db.Title)
		fmt.Printf("    Folder:      %s\n", db.FolderPath)
		fmt.Printf("    Database ID: %s\n", db.DatabaseID)
		fmt.Printf("    Entries:     %d\n", db.EntryCount)
		fmt.Printf("    Last synced: %s\n", db.LastSyncedAt)
		fmt.Println()
	}

	return nil
}

func runConfig(args []string) error {
	if len(args) < 3 || args[0] != "set" {
		return fmt.Errorf("usage: notion-sync config set <key> <value>\n" +
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

	if err := config.SaveConfig(key, value); err != nil {
		return err
	}

	fmt.Printf("Saved %s\n", key)
	return nil
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
