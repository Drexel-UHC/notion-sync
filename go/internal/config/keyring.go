package config

import (
	"github.com/zalando/go-keyring"
)

const (
	keyringService = "notion-sync"
	keyringAccount = "api-key"
)

// GetKeyringAPIKey retrieves the API key from the OS keyring.
// Returns empty string if not found or keyring unavailable.
func GetKeyringAPIKey() string {
	secret, err := keyring.Get(keyringService, keyringAccount)
	if err != nil {
		return ""
	}
	return secret
}

// SetKeyringAPIKey stores the API key in the OS keyring.
// Returns true if successful, false if keyring unavailable.
func SetKeyringAPIKey(apiKey string) bool {
	err := keyring.Set(keyringService, keyringAccount, apiKey)
	return err == nil
}

// DeleteKeyringAPIKey removes the API key from the OS keyring.
func DeleteKeyringAPIKey() {
	_ = keyring.Delete(keyringService, keyringAccount)
}
