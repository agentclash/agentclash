package auth

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/api"
	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
)

// DeviceLoginResult holds the result of a device code login.
type DeviceLoginResult struct {
	Token  string
	UserID string
	Email  string
}

// DeviceLogin performs the device code flow:
// 1. Request device code from server
// 2. Display user code + URL
// 3. Poll until approved/denied/expired
func DeviceLogin(ctx context.Context, client *api.Client, webURL string) (*DeviceLoginResult, error) {
	// Step 1: Request device code.
	resp, err := client.Post(ctx, "/v1/auth/device", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, fmt.Errorf("device code request failed: %s", apiErr.Message)
	}

	var deviceResp struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}
	if err := resp.DecodeJSON(&deviceResp); err != nil {
		return nil, fmt.Errorf("parsing device code response: %w", err)
	}

	// Step 2: Display instructions.
	verifyURL := webURL + deviceResp.VerificationURI
	fmt.Fprintf(os.Stderr, "\n  To authenticate, visit: %s\n", output.Bold(verifyURL))
	fmt.Fprintf(os.Stderr, "  Enter code: %s\n\n", output.Bold(deviceResp.UserCode))

	// Try to open browser (best effort).
	OpenBrowser(verifyURL)

	// Step 3: Poll for token.
	interval := time.Duration(deviceResp.Interval) * time.Second
	if interval < 3*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(deviceResp.ExpiresIn) * time.Second)

	sp := output.NewSpinner("Waiting for authorization...", false)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			sp.Stop()
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		pollResp, err := client.Post(ctx, "/v1/auth/device/token", map[string]any{
			"device_code": deviceResp.DeviceCode,
		})
		if err != nil {
			continue
		}

		// Check for error responses (authorization_pending, etc.)
		if apiErr := pollResp.ParseError(); apiErr != nil {
			switch apiErr.Code {
			case "authorization_pending":
				continue
			case "access_denied":
				sp.StopWithError("Authorization denied")
				return nil, fmt.Errorf("authorization denied by user")
			case "expired_token":
				sp.StopWithError("Device code expired")
				return nil, fmt.Errorf("device code expired — run 'agentclash auth login' again")
			default:
				continue
			}
		}

		// Success — extract token.
		var tokenResp struct {
			Status string `json:"status"`
			Token  string `json:"token"`
			UserID string `json:"user_id"`
			Email  string `json:"email"`
		}
		if err := pollResp.DecodeJSON(&tokenResp); err != nil {
			continue
		}

		if tokenResp.Token != "" {
			sp.StopWithSuccess("Authorized")
			return &DeviceLoginResult{
				Token:  tokenResp.Token,
				UserID: tokenResp.UserID,
				Email:  tokenResp.Email,
			}, nil
		}
	}

	sp.StopWithError("Timed out")
	return nil, fmt.Errorf("device code expired — run 'agentclash auth login' again")
}
