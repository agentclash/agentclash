package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubAuthenticator struct {
	caller Caller
	err    error
	calls  int
}

func (s *stubAuthenticator) Authenticate(_ *http.Request) (Caller, error) {
	s.calls++
	if s.err != nil {
		return Caller{}, s.err
	}
	return s.caller, nil
}

func TestCompositeAuthenticatorRoutesCLITokensToCLIAuthenticator(t *testing.T) {
	primary := &stubAuthenticator{err: errors.New("should not be called")}
	cli := &stubAuthenticator{caller: Caller{Email: "cli@example.com"}}
	auth := NewCompositeAuthenticator(primary, cli)

	req := httptest.NewRequest("GET", "/v1/auth/session", nil)
	req.Header.Set("Authorization", "bearer clitok_test")

	caller, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if caller.Email != "cli@example.com" {
		t.Fatalf("caller email = %q, want %q", caller.Email, "cli@example.com")
	}
	if primary.calls != 0 {
		t.Fatalf("primary calls = %d, want 0", primary.calls)
	}
	if cli.calls != 1 {
		t.Fatalf("cli calls = %d, want 1", cli.calls)
	}
}

func TestCompositeAuthenticatorFallsBackToPrimaryWithoutBearerToken(t *testing.T) {
	primary := &stubAuthenticator{caller: Caller{Email: "primary@example.com"}}
	cli := &stubAuthenticator{err: errors.New("should not be called")}
	auth := NewCompositeAuthenticator(primary, cli)

	req := httptest.NewRequest("GET", "/v1/auth/session", nil)
	req.Header.Set(headerUserID, "11111111-1111-1111-1111-111111111111")

	caller, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if caller.Email != "primary@example.com" {
		t.Fatalf("caller email = %q, want %q", caller.Email, "primary@example.com")
	}
	if primary.calls != 1 {
		t.Fatalf("primary calls = %d, want 1", primary.calls)
	}
	if cli.calls != 0 {
		t.Fatalf("cli calls = %d, want 0", cli.calls)
	}
}
