package local

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestOSKeychainRoundTrip(t *testing.T) {
	keyring.MockInit()
	kc := OSKeychain{}

	if err := kc.Set("openai", "sk-from-keychain"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := kc.Get("openai")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "sk-from-keychain" {
		t.Fatalf("Get = %q, want sk-from-keychain", got)
	}
	if err := kc.Delete("openai"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = kc.Get("openai")
	if !errors.Is(err, ErrKeychainMiss) {
		t.Fatalf("Get after delete error = %v, want ErrKeychainMiss", err)
	}
}

func TestOSKeychainRejectsUnknownProvider(t *testing.T) {
	kc := OSKeychain{}
	if err := kc.Set("nope", "x"); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("Set error = %v, want ErrUnknownProvider", err)
	}
	if _, err := kc.Get("nope"); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("Get error = %v, want ErrUnknownProvider", err)
	}
	if err := kc.Delete("nope"); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("Delete error = %v, want ErrUnknownProvider", err)
	}
}

func TestOSKeychainMiss(t *testing.T) {
	keyring.MockInit()
	_, err := OSKeychain{}.Get("openai")
	if !errors.Is(err, ErrKeychainMiss) {
		t.Fatalf("Get error = %v, want ErrKeychainMiss", err)
	}
}
