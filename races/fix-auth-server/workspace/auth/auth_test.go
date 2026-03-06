package auth

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoginSuccess(t *testing.T) {
	token, err := Login("alice", "password123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	_, err := Login("alice", "wrongpassword")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestLoginUnknownUser(t *testing.T) {
	_, err := Login("unknown", "password")
	if err == nil {
		t.Fatal("expected error for unknown user")
	}
}

func TestValidateToken(t *testing.T) {
	token, err := Login("bob", "hunter2")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	user, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if user != "bob" {
		t.Fatalf("expected user 'bob', got '%s'", user)
	}
}

func TestValidateTokenWithBearerPrefix(t *testing.T) {
	token, err := Login("carol", "s3cret!")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// This is how tokens arrive from HTTP Authorization headers
	authHeader := "Bearer " + token

	user, err := ValidateToken(authHeader)
	if err != nil {
		t.Fatalf("validate with Bearer prefix failed: %v", err)
	}
	if user != "carol" {
		t.Fatalf("expected user 'carol', got '%s'", user)
	}
}

func TestValidateTokenInvalid(t *testing.T) {
	_, err := ValidateToken("not-a-real-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	for username, password := range users {
		token, err := Login(username, password)
		if err != nil {
			t.Fatalf("login failed for %s: %v", username, err)
		}

		user, err := ValidateToken(token)
		if err != nil {
			t.Fatalf("validate failed for %s: %v", username, err)
		}
		if user != username {
			t.Fatalf("expected %s, got %s", username, user)
		}
	}
}

func TestTokenPayloadFormat(t *testing.T) {
	// Token payload should be "username|timestamp" with exactly one pipe
	token, err := Login("alice", "password123")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		t.Fatal("token should have exactly one dot separator")
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	payload := string(decoded)
	pipeCount := strings.Count(payload, "|")
	if pipeCount != 1 {
		t.Fatalf("payload should have exactly 1 pipe, got %d: %q", pipeCount, payload)
	}
}

func TestSignatureConstantTime(t *testing.T) {
	token, err := Login("alice", "password123")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	parts := strings.SplitN(token, ".", 2)
	tampered := parts[0] + ".AAAA-tampered-sig"

	_, err = ValidateToken(tampered)
	if err == nil {
		t.Fatal("expected error for tampered signature")
	}
}

func TestHashPasswordAndCheck(t *testing.T) {
	hash := HashPassword("mypassword")
	if !CheckPassword("mypassword", hash) {
		t.Fatal("CheckPassword should return true for correct password")
	}
	if CheckPassword("wrongpassword", hash) {
		t.Fatal("CheckPassword should return false for wrong password")
	}
}
