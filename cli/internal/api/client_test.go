package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

func TestClientGetSendsAuthHeader(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token-123")
	_, err := client.Get(context.Background(), "/v1/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotHeader != "Bearer test-token-123" {
		t.Fatalf("auth header = %q, want %q", gotHeader, "Bearer test-token-123")
	}
}

func TestClientDevModeSendsCustomHeaders(t *testing.T) {
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", WithDevMode("user-uuid", "org1:org_admin", "ws1:workspace_member"))
	_, err := client.Get(context.Background(), "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if headers.Get("X-Agentclash-User-Id") != "user-uuid" {
		t.Fatalf("user id header = %q, want %q", headers.Get("X-Agentclash-User-Id"), "user-uuid")
	}
	if headers.Get("X-Agentclash-Org-Memberships") != "org1:org_admin" {
		t.Fatalf("org memberships header = %q, want %q", headers.Get("X-Agentclash-Org-Memberships"), "org1:org_admin")
	}
	if headers.Get("X-Agentclash-Workspace-Memberships") != "ws1:workspace_member" {
		t.Fatalf("workspace memberships header = %q, want %q", headers.Get("X-Agentclash-Workspace-Memberships"), "ws1:workspace_member")
	}
	if headers.Get("Authorization") != "" {
		t.Fatalf("authorization header should be empty in dev mode, got %q", headers.Get("Authorization"))
	}
}

func TestClientGetPassesQueryParams(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok")
	q := url.Values{"limit": {"10"}, "offset": {"5"}}
	_, err := client.Get(context.Background(), "/items", q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotQuery.Get("limit") != "10" {
		t.Fatalf("limit = %q, want %q", gotQuery.Get("limit"), "10")
	}
	if gotQuery.Get("offset") != "5" {
		t.Fatalf("offset = %q, want %q", gotQuery.Get("offset"), "5")
	}
}

func TestClientPostSendsJSON(t *testing.T) {
	var gotBody map[string]any
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"123"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok")
	resp, err := client.Post(context.Background(), "/items", map[string]string{"name": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if gotContentType != "application/json" {
		t.Fatalf("content-type = %q, want %q", gotContentType, "application/json")
	}
	if gotBody["name"] != "test" {
		t.Fatalf("body name = %v, want %q", gotBody["name"], "test")
	}
}

func TestResponseParseErrorReturnsNilOnSuccess(t *testing.T) {
	resp := &Response{StatusCode: 200, Body: []byte(`{"items":[]}`)}
	if apiErr := resp.ParseError(); apiErr != nil {
		t.Fatalf("expected nil error, got %v", apiErr)
	}
}

func TestResponseParseErrorExtractsEnvelope(t *testing.T) {
	body := `{"error":{"code":"not_found","message":"resource not found"}}`
	resp := &Response{StatusCode: 404, Body: []byte(body)}
	apiErr := resp.ParseError()
	if apiErr == nil {
		t.Fatal("expected error, got nil")
	}
	if apiErr.Code != "not_found" {
		t.Fatalf("code = %q, want %q", apiErr.Code, "not_found")
	}
	if apiErr.Message != "resource not found" {
		t.Fatalf("message = %q, want %q", apiErr.Message, "resource not found")
	}
	if apiErr.StatusCode != 404 {
		t.Fatalf("status = %d, want %d", apiErr.StatusCode, 404)
	}
}

func TestResponseParseErrorFallsBackOnMalformedBody(t *testing.T) {
	resp := &Response{StatusCode: 500, Body: []byte("internal error")}
	apiErr := resp.ParseError()
	if apiErr == nil {
		t.Fatal("expected error, got nil")
	}
	if apiErr.Code != "Internal Server Error" {
		t.Fatalf("code = %q, want %q", apiErr.Code, "Internal Server Error")
	}
}

func TestResponseDecodeJSON(t *testing.T) {
	resp := &Response{StatusCode: 200, Body: []byte(`{"id":"abc","name":"test"}`)}
	var result map[string]string
	if err := resp.DecodeJSON(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if result["id"] != "abc" {
		t.Fatalf("id = %q, want %q", result["id"], "abc")
	}
	if result["name"] != "test" {
		t.Fatalf("name = %q, want %q", result["name"], "test")
	}
}

func TestClientRetriesOn5xx(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(`{"error":{"code":"bad_gateway","message":"retry"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok")
	resp, err := client.Get(context.Background(), "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
}

func TestClientDoesNotRetryOn4xx(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"code":"bad_request","message":"invalid"}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok")
	resp, err := client.Get(context.Background(), "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want 1 (should not retry 4xx)", attempts.Load())
	}
}

func TestClientDeleteSendsMethod(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok")
	resp, err := client.Delete(context.Background(), "/items/123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q, want %q", gotMethod, http.MethodDelete)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestClientBaseURL(t *testing.T) {
	client := NewClient("https://api.example.com/", "tok")
	if client.BaseURL() != "https://api.example.com" {
		t.Fatalf("base url = %q, want trailing slash trimmed", client.BaseURL())
	}
}
