package main

import (
	"flag"
	"fmt"

	"github.com/ran-codes/notion-sync/internal/config"
	"github.com/ran-codes/notion-sync/internal/notion"
	"github.com/ran-codes/notion-sync/internal/sync"
)

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
