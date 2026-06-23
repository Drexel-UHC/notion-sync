package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/ran-codes/notion-sync/internal/config"
	"github.com/ran-codes/notion-sync/internal/notion"
	"github.com/ran-codes/notion-sync/internal/sync"
)

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
