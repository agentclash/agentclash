package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"
)

const (
	oldNonSuccessText = "connector returned a non-success response; execution outcome is unknown"
	oldTransportText  = "connector outcome unknown; remote execution may have started"
)

// A refusal status proves that the remote did not run the request. The error
// keeps the old text, and it holds no part of the response body.
func TestDoJSONRefusedStatusIsTyped(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 405, 406, 411, 413, 414, 415, 422, 429, 431} {
		c, _ := localClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"remote-body-secret"}`))
		})
		_, err := c.DoJSON(context.Background(), "https://example.com/invoke", "bearer", "token", []byte(`{}`), time.Now().Add(time.Second))
		if err == nil || err.Error() != oldNonSuccessText {
			t.Fatalf("status %d: err=%v, want old text", status, err)
		}
		var refused *RefusedError
		if !errors.As(err, &refused) || refused.Status != status {
			t.Fatalf("status %d: refused=%+v err=%T", status, refused, err)
		}
		if !errors.Is(err, ErrRefused) || errors.Is(err, ErrNotSent) {
			t.Fatalf("status %d: sentinel match is wrong", status)
		}
		reason, got, ok := Rejection(err)
		if !ok || reason != RejectionRefused || got != status {
			t.Fatalf("status %d: rejection=%q %d %v", status, reason, got, ok)
		}
		if strings.Contains(err.Error(), "remote-body-secret") {
			t.Fatalf("status %d: response body in error", status)
		}
	}
}

// Other statuses can come from a remote that started or finished the work, so
// the outcome stays unknown. A 409 can come from a remote that already has
// the run.
func TestDoJSONUnknownStatusStaysUnknown(t *testing.T) {
	for _, status := range []int{301, 302, 307, 408, 409, 425, 499, 500, 502, 503, 504} {
		c, _ := localClient(t, func(w http.ResponseWriter, r *http.Request) {
			if status/100 == 3 {
				w.Header().Set("Location", "https://example.com/next")
			}
			w.WriteHeader(status)
		})
		_, err := c.DoJSON(context.Background(), "https://example.com/invoke", "bearer", "token", []byte(`{}`), time.Now().Add(time.Second))
		if err == nil || err.Error() != oldNonSuccessText {
			t.Fatalf("status %d: err=%v, want old text", status, err)
		}
		if _, _, ok := Rejection(err); ok {
			t.Fatalf("status %d: outcome must stay unknown", status)
		}
		var refused *RefusedError
		var notSent *NotSentError
		if errors.As(err, &refused) || errors.As(err, &notSent) || errors.Is(err, ErrRefused) || errors.Is(err, ErrNotSent) {
			t.Fatalf("status %d: typed as a rejection", status)
		}
	}
}

func TestDoJSONTransportFailureStaysUnknown(t *testing.T) {
	c, _ := localClient(t, func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			conn.Close()
		}
	})
	_, err := c.DoJSON(context.Background(), "https://example.com/invoke", "bearer", "token", []byte(`{}`), time.Now().Add(time.Second))
	if err == nil || err.Error() != oldTransportText {
		t.Fatalf("err=%v, want old text", err)
	}
	if _, _, ok := Rejection(err); ok {
		t.Fatal("lost response typed as a rejection")
	}
}

// Each failure before the send is a NotSentError. It keeps the old text and
// the identity of its cause.
func TestDoJSONFailureBeforeSendIsNotSent(t *testing.T) {
	errRevoked := errors.New("fixture approval revoked")
	for _, tc := range []struct {
		name     string
		mutate   func(*Client)
		auth     string
		secret   string
		payload  []byte
		deadline time.Time
		want     string
		cause    error
	}{
		{name: "oversize", payload: make([]byte, MaxBodyBytes+1), want: "connector request exceeds 1 MiB"},
		{name: "deadline", deadline: time.Now().Add(-time.Second), want: "connector deadline elapsed"},
		{name: "private_dns", mutate: func(c *Client) { c.Resolver = fixtureResolver{netip.MustParseAddr("127.0.0.1")} }},
		{name: "secret_revoked", mutate: func(c *Client) {
			c.ResolveSecret = func(context.Context) (string, error) { return "", errRevoked }
		}, want: errRevoked.Error(), cause: errRevoked},
		{name: "invalid_secret", secret: "bad\nsecret", want: "invalid workspace credential"},
		{name: "unsupported_auth", auth: "basic", want: "unsupported connector authentication"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c, dials := localClient(t, func(w http.ResponseWriter, r *http.Request) { calls++ })
			if tc.mutate != nil {
				tc.mutate(&c)
			}
			auth, secret, payload, deadline := "bearer", "token", []byte(`{}`), time.Now().Add(time.Second)
			if tc.auth != "" {
				auth = tc.auth
			}
			if tc.secret != "" {
				secret = tc.secret
			}
			if tc.payload != nil {
				payload = tc.payload
			}
			if !tc.deadline.IsZero() {
				deadline = tc.deadline
			}
			_, err := c.DoJSON(context.Background(), "https://example.com/invoke", auth, secret, payload, deadline)
			if err == nil || calls != 0 || *dials != 0 {
				t.Fatalf("err=%v calls=%d dials=%d", err, calls, *dials)
			}
			if tc.want != "" && err.Error() != tc.want {
				t.Fatalf("text=%q, want %q", err.Error(), tc.want)
			}
			var notSent *NotSentError
			if !errors.As(err, &notSent) || !errors.Is(err, ErrNotSent) || errors.Is(err, ErrRefused) {
				t.Fatalf("err=%T %v is not a NotSentError", err, err)
			}
			if notSent.Error() != err.Error() || notSent.Unwrap() == nil {
				t.Fatalf("wrapper changed the text or lost the cause: %v", err)
			}
			if tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("cause lost through the wrapper: %v", err)
			}
			if reason, status, ok := Rejection(err); !ok || reason != RejectionNotSent || status != 0 {
				t.Fatalf("rejection=%q %d %v", reason, status, ok)
			}
		})
	}
}

func TestInvokeHTTPJSONMappingFailureIsNotSent(t *testing.T) {
	calls := 0
	c, _ := localClient(t, func(w http.ResponseWriter, r *http.Request) { calls++ })
	_, err := c.InvokeHTTPJSON(context.Background(), fixtureConfig(), "token", json.RawMessage(`{"other":"hello"}`), time.Now().Add(time.Second))
	if err == nil || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	if reason, _, ok := Rejection(err); !ok || reason != RejectionNotSent {
		t.Fatalf("mapping failure is not typed as not sent: %v", err)
	}
	if !strings.HasPrefix(err.Error(), "input mapping question") {
		t.Fatalf("text changed: %q", err.Error())
	}
}

// A response that came back is never a rejection, also when it is not valid.
func TestInvokeHTTPJSONBadSuccessResponseStaysUnknown(t *testing.T) {
	c, _ := localClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{} {}`)) })
	_, err := c.InvokeHTTPJSON(context.Background(), fixtureConfig(), "token", json.RawMessage(`{"input":{"text":"hello"}}`), time.Now().Add(time.Second))
	if err == nil {
		t.Fatal("trailing body accepted")
	}
	if _, _, ok := Rejection(err); ok {
		t.Fatalf("bad success response typed as a rejection: %v", err)
	}
}
