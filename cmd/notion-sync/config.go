package main

import (
	"fmt"
	"strings"

	"github.com/ran-codes/notion-sync/internal/config"
)

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
