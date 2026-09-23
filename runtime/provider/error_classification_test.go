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
	h.Set("Retry-After", "not-a-date")
	if got := parseRetryAfter(h); got != 0 {
		t.Fatalf("parseRetryAfter = %s, want 0 for malformed value", got)
	}
}

func TestParseRetryAfterNegative(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "-5")
	if got := parseRetryAfter(h); got != 0 {
		t.Fatalf("parseRetryAfter = %s, want 0 for negative", got)
	}
}

func TestParseRetryAfterDateAndOverflow(t *testing.T) {
	future := time.Now().Add(time.Minute).UTC().Format(http.TimeFormat)
	got := parseRetryAfter(http.Header{"Retry-After": []string{future}})
	if got < 58*time.Second || got > time.Minute {
		t.Fatalf("date delay = %v", got)
	}
	for _, value := range []string{"NaN", "+Inf", "1e100", "Wed, 21 Oct 2015 07:28:00 GMT"} {
		if got := parseRetryAfter(http.Header{"Retry-After": []string{value}}); got != 0 {
			t.Fatalf("%s = %v", value, got)
		}
	}
}
