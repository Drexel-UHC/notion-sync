package main

import (
	"flag"
	"fmt"

	"github.com/ran-codes/notion-sync/internal/clean"
)

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
