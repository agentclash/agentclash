package secrets

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
)

func newTestCipher(t *testing.T) *AESGCMCipher {
	t.Helper()
	key := make([]byte, MasterKeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	c, err := newAESGCMCipherFromKey(key)
	if err != nil {
		t.Fatalf("construct cipher: %v", err)
	}
	return c
}

func TestAESGCMCipher_RoundTrip(t *testing.T) {
	c := newTestCipher(t)
	cases := [][]byte{
		[]byte(""),
		[]byte("hello"),
		[]byte("postgres://user:pass@host:5432/db?sslmode=require"),
		bytes.Repeat([]byte{0xAA}, 1<<12),
	}
	for _, plaintext := range cases {
		encrypted, err := c.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("encrypt: %v", err)
		}
		if len(plaintext) > 0 && bytes.Contains(encrypted, plaintext) {
			t.Fatalf("ciphertext leaks plaintext bytes")
		}
		decrypted, err := c.Decrypt(encrypted)
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		if !bytes.Equal(decrypted, plaintext) {
			t.Fatalf("round-trip mismatch: got %q want %q", decrypted, plaintext)
		}
	}
}

func TestAESGCMCipher_NonceUniqueness(t *testing.T) {
	c := newTestCipher(t)
	plaintext := []byte("same-plaintext")
	a, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt a: %v", err)
	}
	b, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt b: %v", err)
	}
	if bytes.Equal(a, b) {
		t.Fatalf("identical ciphertext for identical plaintext implies nonce reuse")
	}
}

func TestAESGCMCipher_TamperedCiphertextRejected(t *testing.T) {
	c := newTestCipher(t)
	encrypted, err := c.Encrypt([]byte("sensitive"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	encrypted[len(encrypted)-1] ^= 0x01
	if _, err := c.Decrypt(encrypted); !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("expected ErrInvalidCiphertext, got %v", err)
	}
}

func TestAESGCMCipher_WrongKeyRejected(t *testing.T) {
	a := newTestCipher(t)
	b := newTestCipher(t)
	encrypted, err := a.Encrypt([]byte("sensitive"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := b.Decrypt(encrypted); !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("expected ErrInvalidCiphertext when decrypting with wrong key, got %v", err)
	}
}

func TestAESGCMCipher_RejectsShortCiphertext(t *testing.T) {
	c := newTestCipher(t)
	cases := [][]byte{
		nil,
		{},
		{cipherVersionAESGCM},
		make([]byte, 10),
	}
	for _, stored := range cases {
		if _, err := c.Decrypt(stored); !errors.Is(err, ErrInvalidCiphertext) {
			t.Fatalf("expected ErrInvalidCiphertext for len %d, got %v", len(stored), err)
		}
	}
}

func TestAESGCMCipher_RejectsUnknownVersion(t *testing.T) {
	c := newTestCipher(t)
	encrypted, err := c.Encrypt([]byte("sensitive"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	encrypted[0] = 0xFF
	if _, err := c.Decrypt(encrypted); !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("expected ErrInvalidCiphertext for unknown version, got %v", err)
	}
}

func TestNewAESGCMCipher_RejectsBadKeys(t *testing.T) {
	cases := []struct {
		name   string
		keyB64 string
	}{
		{"empty", ""},
		{"not base64", "!!!not-base64!!!"},
		{"too short", base64.StdEncoding.EncodeToString(make([]byte, 16))},
		{"too long", base64.StdEncoding.EncodeToString(make([]byte, 64))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewAESGCMCipher(tc.keyB64); !errors.Is(err, ErrInvalidMasterKey) {
				t.Fatalf("expected ErrInvalidMasterKey, got %v", err)
			}
		})
	}
}

func TestNewAESGCMCipher_AcceptsValidKey(t *testing.T) {
	key := make([]byte, MasterKeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	c, err := NewAESGCMCipher(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	encrypted, err := c.Encrypt([]byte("smoke"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	plaintext, err := c.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(plaintext) != "smoke" {
		t.Fatalf("round-trip mismatch: %q", plaintext)
	}
}
