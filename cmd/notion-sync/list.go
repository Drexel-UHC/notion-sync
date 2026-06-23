package main

import (
	"fmt"

	"github.com/ran-codes/notion-sync/internal/sync"
)

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
