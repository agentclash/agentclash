package auth

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/api"
	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
)

const (
	defaultDevicePollInterval = 5 * time.Second
	defaultDeviceExpiresIn    = 10 * time.Minute
	maxPollNetworkFailures    = 3
)

var openBrowserFunc = OpenBrowser
var waitForPoll = func(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// LoginResult holds the result of a successful login.
type LoginResult struct {
	UserID  string
	Email   string
	Display string
}

type createDeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type pollDeviceTokenResponse struct {
	Token string `json:"token"`
}

// VerificationLogin performs the CLI verification flow:
// 1. Request a pending device/verification code from the API
// 2. Print the verification URL and code
// 3. Optionally open the browser to that URL
// 4. Poll until the user approves in the web app
// 5. Validate and return the issued CLI token
func VerificationLogin(ctx context.Context, client *api.Client, autoOpen bool) (*LoginResult, string, error) {
	resp, err := client.Post(ctx, "/v1/cli-auth/device", map[string]any{})
	if err != nil {
		return nil, "", fmt.Errorf("requesting verification code: %w", err)
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, "", fmt.Errorf("device code request failed: %s", apiErr.Message)
	}

	var deviceResp createDeviceCodeResponse
	if err := resp.DecodeJSON(&deviceResp); err != nil {
		return nil, "", fmt.Errorf("parsing verification response: %w", err)
	}
	verifyURL, err := deviceVerificationURL(deviceResp)
	if err != nil {
		return nil, "", err
	}

	fmt.Fprintf(os.Stderr, "\n  Verify this login in your browser:\n  %s\n", output.Bold(verifyURL))
	fmt.Fprintf(os.Stderr, "  Code: %s\n\n", output.Bold(deviceResp.UserCode))

	if autoOpen {
		fmt.Fprintf(os.Stderr, "%s Opening browser to continue login...\n", output.Cyan("▸"))
		if err := openBrowserFunc(verifyURL); err != nil {
			fmt.Fprintf(os.Stderr, "%s Could not open browser automatically. Open the link above manually.\n", output.Yellow("!"))
		}
	}

	interval := time.Duration(deviceResp.Interval) * time.Second
	if interval <= 0 {
		interval = defaultDevicePollInterval
	}
	expiresIn := time.Duration(deviceResp.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = defaultDeviceExpiresIn
	}
	deadline := time.Now().Add(expiresIn)
	sp := output.NewSpinner("Waiting for browser verification...", false)
	networkFailures := 0

	for time.Now().Before(deadline) {
		if err := waitForPoll(ctx, interval); err != nil {
			sp.Stop()
			return nil, "", err
		}

		pollResp, err := client.Post(ctx, "/v1/cli-auth/device/token", map[string]any{
			"device_code": deviceResp.DeviceCode,
		})
		if err != nil {
			networkFailures++
			if networkFailures >= maxPollNetworkFailures {
				sp.StopWithError("Verification failed")
				return nil, "", fmt.Errorf("polling verification failed after %d attempts: %w", networkFailures, err)
			}
			continue
		}
		networkFailures = 0
		if apiErr := pollResp.ParseError(); apiErr != nil {
			switch apiErr.Code {
			case "authorization_pending":
				continue
			case "slow_down":
				interval += 5 * time.Second
				sp.Update("Waiting for browser verification...")
				continue
			case "access_denied":
				sp.StopWithError("Authorization denied")
				return nil, "", fmt.Errorf("authorization denied by user")
			case "expired_token":
				sp.StopWithError("Verification expired")
				return nil, "", fmt.Errorf("verification expired - run 'agentclash auth login' again")
			default:
				sp.StopWithError("Verification failed")
				return nil, "", fmt.Errorf("verification failed: %s", apiErr.Message)
			}
		}

		var tokenResp pollDeviceTokenResponse
		if err := pollResp.DecodeJSON(&tokenResp); err != nil {
			sp.StopWithError("Verification failed")
			return nil, "", fmt.Errorf("parsing token response: %w", err)
		}
		if tokenResp.Token == "" {
			sp.StopWithError("Verification failed")
			return nil, "", fmt.Errorf("token response missing token")
		}

		loginResult, err := ValidateToken(ctx, api.NewClient(client.BaseURL(), tokenResp.Token))
		if err != nil {
			sp.StopWithError("Login verification failed")
			return nil, "", err
		}

		sp.StopWithSuccess("Verified")
		return loginResult, tokenResp.Token, nil
	}

	sp.StopWithError("Timed out")
	return nil, "", fmt.Errorf("verification expired - run 'agentclash auth login' again")
}

func deviceVerificationURL(resp createDeviceCodeResponse) (string, error) {
	if resp.UserCode == "" {
		return "", fmt.Errorf("verification response missing user_code")
	}
	if resp.DeviceCode == "" {
		return "", fmt.Errorf("verification response missing device_code")
	}
	if resp.VerificationURIComplete != "" {
		return resp.VerificationURIComplete, nil
	}
	if resp.VerificationURI == "" {
		return "", fmt.Errorf("verification response missing verification_uri_complete and verification_uri")
	}

	parsed, err := url.Parse(resp.VerificationURI)
	if err != nil {
		return "", fmt.Errorf("parsing verification_uri: %w", err)
	}
	if !parsed.IsAbs() {
		return "", fmt.Errorf("verification response missing verification_uri_complete and absolute verification_uri")
	}
	q := parsed.Query()
	q.Set("user_code", resp.UserCode)
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
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
