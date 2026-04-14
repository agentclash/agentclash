package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/api"
	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
)

// LoginResult holds the result of a successful login.
type LoginResult struct {
	UserID  string
	Email   string
	Display string
}

// WebLogin performs browser-based login:
// 1. Start a localhost callback server
// 2. Open browser to the web app's /auth/cli page
// 3. Wait for the callback with the token
// 4. Validate the token
func WebLogin(ctx context.Context, client *api.Client, webURL string) (*LoginResult, string, error) {
	// Generate cryptographic state for CSRF protection.
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, "", fmt.Errorf("generating state: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	// Start local callback server.
	resultCh, port, err := StartCallbackServer(ctx, state)
	if err != nil {
		return nil, "", fmt.Errorf("starting callback server: %w", err)
	}

	// Build the auth URL.
	authURL := fmt.Sprintf("%s/auth/cli?port=%d&state=%s", webURL, port, state)

	fmt.Fprintf(os.Stderr, "%s Opening browser to authenticate...\n", output.Cyan("▸"))

	if err := OpenBrowser(authURL); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not open browser. Visit this URL manually:\n", output.Yellow("!"))
		fmt.Fprintf(os.Stderr, "  %s\n\n", authURL)
	}

	// Wait for callback.
	result := <-resultCh

	if result.Error != "" {
		return nil, "", fmt.Errorf("login failed: %s", result.Error)
	}

	// Validate the token by calling /v1/auth/session.
	authedClient := api.NewClient(client.BaseURL(), result.Token)
	loginResult, err := ValidateToken(ctx, authedClient)
	if err != nil {
		return nil, "", err
	}

	return loginResult, result.Token, nil
}

// ValidateToken checks the token against the session endpoint.
func ValidateToken(ctx context.Context, client *api.Client) (*LoginResult, error) {
	resp, err := client.Get(ctx, "/v1/auth/session", nil)
	if err != nil {
		return nil, fmt.Errorf("validating token: %w", err)
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, fmt.Errorf("invalid token: %s", apiErr.Message)
	}

	var session struct {
		UserID      string `json:"user_id"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
	}
	if err := resp.DecodeJSON(&session); err != nil {
		return nil, fmt.Errorf("parsing session: %w", err)
	}

	return &LoginResult{
		UserID:  session.UserID,
		Email:   session.Email,
		Display: session.DisplayName,
	}, nil
}
