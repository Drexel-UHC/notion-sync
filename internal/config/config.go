// Package config manages application configuration, environment variables, and OS keyring access.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds application configuration.
type Config struct {
	APIKey              string `json:"apiKey,omitempty"`
	DefaultOutputFolder string `json:"defaultOutputFolder,omitempty"`
	OutputMode          string `json:"outputMode,omitempty"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		DefaultOutputFolder: "./notion",
	}
}

// GetConfigPath returns the path to the config file.
func GetConfigPath() string {
	// Check XDG_CONFIG_HOME first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "notion-sync", "config.json")
	}

	// Fall back to home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return ".notion-sync.json"
	}
	return filepath.Join(home, ".notion-sync.json")
}

// LoadConfig loads configuration from file, keyring, and environment.
func LoadConfig() (Config, error) {
	config := DefaultConfig()
	configPath := GetConfigPath()

	// Try to read config file
	var fileConfig Config
	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := json.Unmarshal(data, &fileConfig); err == nil {
			if fileConfig.DefaultOutputFolder != "" {
				config.DefaultOutputFolder = fileConfig.DefaultOutputFolder
			}
			if fileConfig.OutputMode != "" {
				config.OutputMode = fileConfig.OutputMode
			}
		}
	}

	// Determine API key: env var > keychain > config file
	if envKey := os.Getenv("NOTION_SYNC_API_KEY"); envKey != "" {
		config.APIKey = envKey
	} else if keyringKey := GetKeyringAPIKey(); keyringKey != "" {
		config.APIKey = keyringKey
	} else if fileConfig.APIKey != "" {
		fmt.Fprintln(os.Stderr, "Warning: Reading API key from plaintext config file. "+
			"Run `notion-sync config set apiKey <key>` to store it in the OS keychain.")
		config.APIKey = fileConfig.APIKey
	}

	return config, nil
}

// SaveConfig saves a configuration key-value pair.
func SaveConfig(key, value string) error {
	if key == "apiKey" {
		if SetKeyringAPIKey(value) {
			// Also remove from config file if present
			if err := removeAPIKeyFromConfigFile(); err != nil {
				// Ignore errors - config file might not exist
			}
			return nil
		}
		fmt.Fprintln(os.Stderr, "Warning: OS keychain unavailable. Storing API key in plaintext config file.")
	}

	configPath := GetConfigPath()

	// Read existing config
	var existing map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(data, &existing)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	// Update value
	existing[key] = value

	// Write back
	newData, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(configPath, append(newData, '\n'), 0600)
}

func removeAPIKeyFromConfigFile() error {
	configPath := GetConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var existing map[string]interface{}
	if err := json.Unmarshal(data, &existing); err != nil {
		return err
	}

	if _, ok := existing["apiKey"]; !ok {
		return nil // Nothing to remove
	}

	delete(existing, "apiKey")

	newData, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, append(newData, '\n'), 0600)
}

// MigrateAPIKeyToKeychain migrates API key from config file to keyring.
// Idempotent: skips if keychain already has a key or config file has none.
func MigrateAPIKeyToKeychain() {
	// If keychain already has a key, nothing to do
	if GetKeyringAPIKey() != "" {
		return
	}

	configPath := GetConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return // No config file
	}

	var fileConfig map[string]interface{}
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		return
	}

	apiKey, ok := fileConfig["apiKey"].(string)
	if !ok || apiKey == "" {
		return
	}

	if SetKeyringAPIKey(apiKey) {
		// Remove apiKey from config file
		delete(fileConfig, "apiKey")
		newData, err := json.MarshalIndent(fileConfig, "", "  ")
		if err == nil {
			os.WriteFile(configPath, append(newData, '\n'), 0600)
		}
		fmt.Println("Migrated API key from config file to OS keychain.")
	}
}
