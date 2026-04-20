package api

import (
	"net/http"
	"net/url"
	"testing"
)

func mustParse(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return u
}

func TestStrictRedirectPolicy(t *testing.T) {
	cases := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{"same host same port", "https://api.example.com/a", "https://api.example.com/b", false},
		{"cross scheme rejected", "https://api.example.com/a", "http://api.example.com/a", true},
		{"cross host rejected", "https://api.example.com/a", "https://evil.example.com/a", true},
		{"subdomain rejected", "https://agentclash.dev/a", "https://auth.agentclash.dev/a", true},
		{"port change rejected", "https://api.example.com:8080/", "https://api.example.com:9090/", true},
		{"localhost ↔ 127.0.0.1 allowed", "http://localhost:8080/x", "http://127.0.0.1:8080/x", false},
		{"localhost different port rejected", "http://localhost:8080/x", "http://localhost:9090/x", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			via := []*http.Request{{URL: mustParse(t, tc.from)}}
			req := &http.Request{URL: mustParse(t, tc.to)}
			err := StrictRedirectPolicy(req, via)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
