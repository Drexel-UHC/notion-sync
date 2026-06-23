package main

import (
	"fmt"
	"os"

	"github.com/ran-codes/notion-sync/internal/config"
	"github.com/ran-codes/notion-sync/internal/sync"
)

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
