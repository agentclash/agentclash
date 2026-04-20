package auth

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/agentclash/agentclash/cli/internal/api"
	"github.com/agentclash/agentclash/cli/internal/output"
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

	// If the server sent us an ABSOLUTE verification_uri, treat it as the
	// authoritative base and require verification_uri_complete (if present)
	// to agree with it on scheme/host/port/path AND carry our expected
	// user_code. Otherwise rebuild the complete URL ourselves.
	//
	// A relative verification_uri is legal per RFC 8628 — GitHub's device
	// flow, for example, ships one. In that case there's nothing to compare
	// against, so we just accept verification_uri_complete as the sole
	// source and rely on validateVerificationURL for scheme/host guarantees.
	//
	// The gap we're closing: a hostile server shipping
	//   verification_uri:          https://agentclash.dev/device
	//   verification_uri_complete: https://evil.example/device?user_code=XXX
	// and the CLI opening the second one because it's convenient.
	var base *url.URL
	if resp.VerificationURI != "" {
		parsed, err := url.Parse(resp.VerificationURI)
		if err != nil {
			return "", fmt.Errorf("parsing verification_uri: %w", err)
		}
		if parsed.IsAbs() {
			base = parsed
		} else if resp.VerificationURIComplete == "" {
			// Relative verification_uri is only tolerable when the server
			// also provided verification_uri_complete — otherwise we have
			// nothing to open.
			return "", fmt.Errorf("verification response missing verification_uri_complete and absolute verification_uri")
		}
	}

	var candidate string
	switch {
	case base != nil && resp.VerificationURIComplete != "":
		completeURL, err := url.Parse(resp.VerificationURIComplete)
		if err != nil {
			return "", fmt.Errorf("parsing verification_uri_complete: %w", err)
		}
		if !verificationURLsSameOrigin(base, completeURL) {
			return "", fmt.Errorf("verification_uri_complete disagrees with verification_uri on origin/path")
		}
		if got := completeURL.Query().Get("user_code"); got != resp.UserCode {
			return "", fmt.Errorf("verification_uri_complete user_code %q does not match response user_code", got)
		}
		candidate = resp.VerificationURIComplete
	case resp.VerificationURIComplete != "":
		candidate = resp.VerificationURIComplete
	case base != nil:
		q := base.Query()
		q.Set("user_code", resp.UserCode)
		base.RawQuery = q.Encode()
		candidate = base.String()
	default:
		return "", fmt.Errorf("verification response missing verification_uri_complete and verification_uri")
	}

	if err := validateVerificationURL(candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

// verificationURLsSameOrigin returns true if a and b share scheme + hostname
// + port + path. Query string and fragment are intentionally ignored — the
// point is to make sure the "complete" variant still lands on the same
// device-verification endpoint the server first advertised.
func verificationURLsSameOrigin(a, b *url.URL) bool {
	if a.Scheme != b.Scheme {
		return false
	}
	if a.Hostname() != b.Hostname() {
		return false
	}
	if a.Port() != b.Port() {
		return false
	}
	return a.Path == b.Path
}

// validateVerificationURL enforces https for the device-verification URL. A
// compromised backend or MITM on an http API endpoint could otherwise redirect
// the user to an attacker-controlled http:// verification page. http is
// permitted only for loopback hosts — the local dev loop against
// http://localhost:8080.
//
// url.Parse treats opaque forms like "https:foo" as absolute URLs with no
// host, so IsAbs() alone is not enough: we also require a hierarchical URL
// (Opaque == "") with a real host.
func validateVerificationURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parsing verification URL: %w", err)
	}
	if !parsed.IsAbs() {
		return fmt.Errorf("verification URL must be absolute: %q", raw)
	}
	if parsed.Opaque != "" {
		return fmt.Errorf("verification URL must be hierarchical, not opaque: %q", raw)
	}
	// Parse.Hostname strips any :port; an input like "https://:443/auth" has
	// Host=":443" but Hostname()=="", which is still missing the real host.
	if parsed.Hostname() == "" {
		return fmt.Errorf("verification URL is missing a host: %q", raw)
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopbackHost(parsed.Hostname()) {
			return nil
		}
		return fmt.Errorf("verification URL must use https (got %q)", raw)
	default:
		return fmt.Errorf("verification URL scheme %q is not allowed", parsed.Scheme)
	}
}

func isLoopbackHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
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
