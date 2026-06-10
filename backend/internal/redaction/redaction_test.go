package redaction

import "testing"

func TestScrubHeaderSecrets(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"authorization header", "Authorization: Bearer sk-secret-123", "[redacted]"},
		{"cookie header", "Cookie: session=abc123", "[redacted]"},
		// set-cookie is ordered before cookie so the full header line is scrubbed (not the
		// quirky "Set-[redacted]" the original engine ordering produced).
		{"set-cookie", "Set-Cookie: tok=zzz; HttpOnly", "[redacted]"},
		{"x-api-key header", "X-API-Key: live_abc", "[redacted]"},
		{"api_key colon", "api_key: 12345", "[redacted]"},
		{"bearer token", "got bearer abcDEF.token here", "got [redacted] here"},
		{"basic auth", "Basic QWxhZGRpbjpvcGVuc2VzYW1l", "[redacted]"},
		{"benign text untouched", "the build failed: exit code 1", "the build failed: exit code 1"},
		{"only matching line scrubbed", "ok line\nAuthorization: x\nnext ok", "ok line\n[redacted]\nnext ok"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ScrubHeaderSecrets(tc.in); got != tc.want {
				t.Fatalf("ScrubHeaderSecrets(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMarkerStable(t *testing.T) {
	if Marker != "[redacted]" {
		t.Fatalf("Marker = %q, want [redacted] (engine relies on this value)", Marker)
	}
}
