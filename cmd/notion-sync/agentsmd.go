package main

import (
	"flag"
	"fmt"

	"github.com/ran-codes/notion-sync/internal/sync"
)

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
