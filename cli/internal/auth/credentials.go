package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/config"
)

// Credentials holds stored authentication data.
type Credentials struct {
	Token     string     `json:"token"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	UserID    string     `json:"user_id,omitempty"`
	Email     string     `json:"email,omitempty"`
}

// CredentialsPath returns the full path to the credentials file.
func CredentialsPath() string {
	return filepath.Join(config.ConfigDir(), "credentials.json")
}

// SaveCredentials writes credentials to disk with restricted permissions.
func SaveCredentials(creds Credentials) error {
	dir := config.ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(CredentialsPath(), data, 0600)
}

// LoadCredentials reads credentials from disk.
// Returns empty credentials (no error) if the file does not exist.
func LoadCredentials() (Credentials, error) {
	var creds Credentials

	// Check env var first (always takes precedence).
	if token := os.Getenv("AGENTCLASH_TOKEN"); token != "" {
		return Credentials{Token: token}, nil
	}

	data, err := os.ReadFile(CredentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return creds, nil
		}
		return creds, err
	}
	err = json.Unmarshal(data, &creds)
	return creds, err
}

// DeleteCredentials removes the stored credentials file.
func DeleteCredentials() error {
	err := os.Remove(CredentialsPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
