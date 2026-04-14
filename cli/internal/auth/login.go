package auth

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/api"
)

// LoginResult holds the result of a login attempt.
type LoginResult struct {
	UserID  string
	Email   string
	Display string
}

// InteractiveLogin prompts the user for a token and validates it.
func InteractiveLogin(ctx context.Context, client *api.Client) (*LoginResult, string, error) {
	fmt.Fprintln(os.Stderr, "Paste your API token (from the dashboard):")
	fmt.Fprint(os.Stderr, "> ")

	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil {
		return nil, "", fmt.Errorf("reading token: %w", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, "", fmt.Errorf("token cannot be empty")
	}

	// Validate the token by calling the session endpoint.
	authedClient := api.NewClient(client.BaseURL(), token)
	result, err := ValidateToken(ctx, authedClient)
	if err != nil {
		return nil, "", err
	}

	return result, token, nil
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
