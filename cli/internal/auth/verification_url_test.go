package auth

import (
	"strings"
	"testing"
)

func TestValidateVerificationURL(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
		errSub  string
	}{
		{"https public", "https://agentclash.dev/device?user_code=A", false, ""},
		{"http localhost", "http://localhost:8080/device?user_code=A", false, ""},
		{"http 127.0.0.1", "http://127.0.0.1:8080/device", false, ""},
		{"http v6 loopback", "http://[::1]:8080/device", false, ""},
		{"http public rejected", "http://evil.example.com/device", true, "must use https"},
		{"javascript rejected", "javascript:alert(1)", true, ""},
		{"file rejected", "file:///etc/passwd", true, ""},
		{"relative rejected", "/device", true, "must be absolute"},
		{"opaque rejected", "https:foo", true, "opaque"},
		{"hostless rejected", "https:///path", true, "missing a host"},
		{"port-only hostname rejected", "https://:443/auth/device", true, "missing a host"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateVerificationURL(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.raw)
				}
				if tc.errSub != "" && !strings.Contains(err.Error(), tc.errSub) {
					t.Fatalf("error %q did not contain %q", err, tc.errSub)
				}
			} else if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.raw, err)
			}
		})
	}
}

func TestDeviceVerificationURLRejectsHostileScheme(t *testing.T) {
	_, err := deviceVerificationURL(createDeviceCodeResponse{
		DeviceCode:              "dc",
		UserCode:                "AB-CD",
		VerificationURIComplete: "http://evil.example.com/verify?user_code=AB-CD",
	})
	if err == nil {
		t.Fatal("expected error for http public URL")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeviceVerificationURLRejectsCompleteDisagreement(t *testing.T) {
	// Hostile server: base says agentclash.dev, complete says evil.example.
	_, err := deviceVerificationURL(createDeviceCodeResponse{
		DeviceCode:              "dc",
		UserCode:                "AB-CD",
		VerificationURI:         "https://agentclash.dev/device",
		VerificationURIComplete: "https://evil.example.com/device?user_code=AB-CD",
	})
	if err == nil || !strings.Contains(err.Error(), "disagrees") {
		t.Fatalf("expected origin/path disagreement error, got %v", err)
	}
}

func TestDeviceVerificationURLRejectsUserCodeMismatch(t *testing.T) {
	_, err := deviceVerificationURL(createDeviceCodeResponse{
		DeviceCode:              "dc",
		UserCode:                "AB-CD",
		VerificationURI:         "https://agentclash.dev/device",
		VerificationURIComplete: "https://agentclash.dev/device?user_code=FORGED",
	})
	if err == nil || !strings.Contains(err.Error(), "user_code") {
		t.Fatalf("expected user_code mismatch error, got %v", err)
	}
}

func TestDeviceVerificationURLAcceptsMatchingPair(t *testing.T) {
	got, err := deviceVerificationURL(createDeviceCodeResponse{
		DeviceCode:              "dc",
		UserCode:                "AB-CD",
		VerificationURI:         "https://agentclash.dev/device",
		VerificationURIComplete: "https://agentclash.dev/device?user_code=AB-CD",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://agentclash.dev/device?user_code=AB-CD" {
		t.Fatalf("got %q", got)
	}
}

func TestDeviceVerificationURLRebuildsFromBaseWhenCompleteMissing(t *testing.T) {
	got, err := deviceVerificationURL(createDeviceCodeResponse{
		DeviceCode:      "dc",
		UserCode:        "AB-CD",
		VerificationURI: "https://agentclash.dev/device",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "user_code=AB-CD") || !strings.HasPrefix(got, "https://agentclash.dev/device?") {
		t.Fatalf("rebuilt URL = %q", got)
	}
}
