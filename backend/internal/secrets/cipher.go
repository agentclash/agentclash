package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const MasterKeySize = 32

const (
	cipherVersionAESGCM byte = 1
	gcmNonceSize             = 12
)

var (
	ErrInvalidMasterKey  = errors.New("secrets: invalid master key")
	ErrInvalidCiphertext = errors.New("secrets: invalid ciphertext")
)

type AESGCMCipher struct {
	aead cipher.AEAD
}

// NewAESGCMCipher constructs an AES-256-GCM cipher from a base64-encoded 32-byte key.
func NewAESGCMCipher(keyB64 string) (*AESGCMCipher, error) {
	if keyB64 == "" {
		return nil, fmt.Errorf("%w: master key is empty", ErrInvalidMasterKey)
	}
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("%w: decode base64: %v", ErrInvalidMasterKey, err)
	}
	return newAESGCMCipherFromKey(key)
}

func newAESGCMCipherFromKey(key []byte) (*AESGCMCipher, error) {
	if len(key) != MasterKeySize {
		return nil, fmt.Errorf("%w: expected %d bytes, got %d", ErrInvalidMasterKey, MasterKeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMasterKey, err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMasterKey, err)
	}
	return &AESGCMCipher{aead: aead}, nil
}

// Encrypt seals plaintext with a fresh random nonce. The stored layout is
// version(1) || nonce(12) || sealed, where sealed is the GCM ciphertext+tag.
// The version prefix reserves room for future algorithm changes without
// breaking at-rest compatibility.
func (c *AESGCMCipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("secrets: generate nonce: %w", err)
	}
	sealed := c.aead.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, 1+gcmNonceSize+len(sealed))
	out = append(out, cipherVersionAESGCM)
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

func (c *AESGCMCipher) Decrypt(stored []byte) ([]byte, error) {
	if len(stored) < 1+gcmNonceSize+c.aead.Overhead() {
		return nil, fmt.Errorf("%w: too short", ErrInvalidCiphertext)
	}
	if stored[0] != cipherVersionAESGCM {
		return nil, fmt.Errorf("%w: unknown version %d", ErrInvalidCiphertext, stored[0])
	}
	nonce := stored[1 : 1+gcmNonceSize]
	sealed := stored[1+gcmNonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCiphertext, err)
	}
	return plaintext, nil
}
