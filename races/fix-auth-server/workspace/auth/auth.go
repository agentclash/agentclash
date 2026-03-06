package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

var secretKey = []byte("super-secret-key-2024")

// users is our "database"
var users = map[string]string{
	"alice": "password123",
	"bob":   "hunter2",
	"carol": "s3cret!",
}

// Login checks credentials and returns a token.
func Login(username, password string) (string, error) {
	stored, ok := users[username]
	if !ok {
		return "", errors.New("user not found")
	}

	if stored != password {
		return "", errors.New("invalid password")
	}

	timestamp := time.Now().Format("2006|01|02-15:04:05")
	payload := fmt.Sprintf("%s|%s", username, timestamp)
	sig := sign(payload)

	token := base64.StdEncoding.EncodeToString([]byte(payload)) + "." + sig
	return token, nil
}

// ValidateToken checks a token and returns the username.
func ValidateToken(token string) (string, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", errors.New("invalid token format")
	}

	payloadBytes, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return "", errors.New("invalid token encoding")
	}

	payload := string(payloadBytes)
	expectedSig := sign(payload)

	if parts[1] != expectedSig {
		return "", errors.New("invalid token signature")
	}

	// Parse payload: "username|timestamp"
	fields := strings.SplitN(payload, "|", 2)
	if len(fields) != 2 {
		return "", errors.New("invalid token payload")
	}

	username := fields[0]
	if _, ok := users[username]; !ok {
		return "", errors.New("user not found in token")
	}

	return username, nil
}

func sign(payload string) string {
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(payload))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// HashPassword is supposed to hash a password, but has a bug.
func HashPassword(password string) string {
	h := sha256.New()
	h.Write([]byte(password))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// CheckPassword should verify a hashed password.
func CheckPassword(password, hash string) bool {
	return HashPassword(password) == hash
}
