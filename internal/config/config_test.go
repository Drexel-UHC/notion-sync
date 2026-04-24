package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_EnvOverridesFile(t *testing.T) {
	// Create a config file with an API key
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	data, _ := json.Marshal(map[string]string{"apiKey": "file-key"})
	os.WriteFile(configPath, data, 0600)

	// Point XDG_CONFIG_HOME to temp dir so GetConfigPath() finds our file
	t.Setenv("XDG_CONFIG_HOME", dir)
	// Rename so GetConfigPath returns the right path
	os.MkdirAll(filepath.Join(dir, "notion-sync"), 0755)
	os.Rename(configPath, filepath.Join(dir, "notion-sync", "config.json"))

	// Set env var — should take priority
	t.Setenv("NOTION_SYNC_API_KEY", "env-key")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("APIKey = %q, want env-key (env should override file)", cfg.APIKey)
	}
}

func TestLoadConfig_FileFallback(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "notion-sync"), 0755)
	configPath := filepath.Join(dir, "notion-sync", "config.json")
	data, _ := json.Marshal(map[string]string{
		"apiKey":              "file-key",
		"defaultOutputFolder": "/custom/output",
	})
	os.WriteFile(configPath, data, 0600)

	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("NOTION_SYNC_API_KEY", "") // explicitly clear

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Since keyring is not set in test and env is empty, file should be used
	// (we can't guarantee keyring is empty in CI, but file-key should appear
	// if keyring returns empty)
	if cfg.DefaultOutputFolder != "/custom/output" {
		t.Errorf("DefaultOutputFolder = %q, want /custom/output", cfg.DefaultOutputFolder)
	}
}

func TestLoadConfig_DefaultValues(t *testing.T) {
	// Point to empty dir with no config file
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "notion-sync"), 0755)

	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("NOTION_SYNC_API_KEY", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.DefaultOutputFolder != "./notion" {
		t.Errorf("DefaultOutputFolder = %q, want ./notion (default)", cfg.DefaultOutputFolder)
	}
}

func TestGetConfigPath_XDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	path := GetConfigPath()
	expected := filepath.Join("/tmp/xdg", "notion-sync", "config.json")
	if path != expected {
		t.Errorf("GetConfigPath() = %q, want %q", path, expected)
	}
}

