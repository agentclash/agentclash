package local

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

// KeychainService is the OS keyring service name for local provider keys.
const KeychainService = "agentclash.local.providers"

// ErrKeychainMiss means the OS keychain has no entry for the provider.
var ErrKeychainMiss = errors.New("provider key not found in OS keychain")

// Keychain stores and loads provider API keys from the platform keyring.
type Keychain interface {
	Get(providerKey string) (string, error)
	Set(providerKey, apiKey string) error
	Delete(providerKey string) error
}

// OSKeychain reads/writes provider keys via zalando/go-keyring (macOS Keychain,
// Windows Credential Manager, Linux Secret Service).
type OSKeychain struct {
	Service string
	get     func(service, user string) (string, error)
	set     func(service, user, password string) error
	del     func(service, user string) error
}

func (k OSKeychain) service() string {
	if k.Service != "" {
		return k.Service
	}
	return KeychainService
}

// Get implements Keychain.
func (k OSKeychain) Get(providerKey string) (string, error) {
	key, err := requireKnownProvider(providerKey)
	if err != nil {
		return "", err
	}
	get := k.get
	if get == nil {
		get = keyring.Get
	}
	value, err := get(k.service(), key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) || isKeychainUnavailable(err) {
			return "", ErrKeychainMiss
		}
		return "", fmt.Errorf("os keychain get %q: %w", key, err)
	}
	if strings.TrimSpace(value) == "" {
		return "", ErrKeychainMiss
	}
	return value, nil
}

// Set implements Keychain.
func (k OSKeychain) Set(providerKey, apiKey string) error {
	key, err := requireKnownProvider(providerKey)
	if err != nil {
		return err
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("api key for provider %q is empty", key)
	}
	set := k.set
	if set == nil {
		set = keyring.Set
	}
	if err := set(k.service(), key, apiKey); err != nil {
		return fmt.Errorf("os keychain set %q: %w", key, err)
	}
	return nil
}

// Delete implements Keychain.
func (k OSKeychain) Delete(providerKey string) error {
	key, err := requireKnownProvider(providerKey)
	if err != nil {
		return err
	}
	del := k.del
	if del == nil {
		del = keyring.Delete
	}
	if err := del(k.service(), key); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("os keychain delete %q: %w", key, err)
	}
	return nil
}

// isKeychainUnavailable treats broad keyring/backend failures as a miss so
// headless Linux (no D-Bus / Secret Service) can fall through the credential
// chain instead of hard-failing. A genuinely broken keychain therefore looks
// identical to "key not stored" — intentional for local-first UX.
func isKeychainUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "cannot autolaunch") ||
		strings.Contains(msg, "no such file") ||
		strings.Contains(msg, "secret service") ||
		strings.Contains(msg, "dbus") ||
		strings.Contains(msg, "not available")
}
