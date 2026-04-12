package provider

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRetryAfterInteger(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "20")
	if got := parseRetryAfter(h); got != 20*time.Second {
		t.Fatalf("parseRetryAfter = %s, want 20s", got)
	}
}

func TestParseRetryAfterFloat(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "15.5")
	if got := parseRetryAfter(h); got != 15500*time.Millisecond {
		t.Fatalf("parseRetryAfter = %s, want 15.5s", got)
	}
}

func TestParseRetryAfterMissing(t *testing.T) {
	h := http.Header{}
	if got := parseRetryAfter(h); got != 0 {
		t.Fatalf("parseRetryAfter = %s, want 0", got)
	}
}

func TestParseRetryAfterNonNumeric(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "Wed, 21 Oct 2026 07:28:00 GMT")
	if got := parseRetryAfter(h); got != 0 {
		t.Fatalf("parseRetryAfter = %s, want 0 for date format", got)
	}
}

func TestParseRetryAfterNegative(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "-5")
	if got := parseRetryAfter(h); got != 0 {
		t.Fatalf("parseRetryAfter = %s, want 0 for negative", got)
	}
}
