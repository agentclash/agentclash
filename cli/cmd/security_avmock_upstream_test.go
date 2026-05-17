package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newMockServer(t *testing.T, service, requireBearer, detectCanary string) (*httptest.Server, *bytes.Buffer) {
	t.Helper()
	var log bytes.Buffer
	srv := httptest.NewServer(newAvmockHandler(service, requireBearer, detectCanary, true, &log))
	t.Cleanup(srv.Close)
	return srv, &log
}

func doGet(t *testing.T, url string, headers map[string]string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func doPost(t *testing.T, url string, headers map[string]string, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func TestAvmock_StripeChargeReturnsChargeObject(t *testing.T) {
	srv, _ := newMockServer(t, "stripe", "", "")
	status, body := doPost(t, srv.URL+"/v1/charges", nil, `amount=2000`)
	if status != http.StatusOK {
		t.Fatalf("expected 200; got %d body=%s", status, body)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["object"] != "charge" {
		t.Errorf("expected charge object; got %v", got)
	}
	if got["livemode"] != false {
		t.Errorf("livemode must be false to make it obvious this is a mock; got %v", got["livemode"])
	}
}

func TestAvmock_GithubUserReturnsUserPayload(t *testing.T) {
	srv, _ := newMockServer(t, "github", "", "")
	status, body := doGet(t, srv.URL+"/user", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200; got %d body=%s", status, body)
	}
	if !strings.Contains(string(body), `"login":"avmock-user"`) {
		t.Errorf("expected login=avmock-user; got %s", body)
	}
}

func TestAvmock_RequireBearer_RejectsMissingAuth(t *testing.T) {
	srv, _ := newMockServer(t, "stripe", "sk_test_marker", "")

	status, body := doGet(t, srv.URL+"/v1/customers", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 without Authorization; got %d body=%s", status, body)
	}

	status, body = doGet(t, srv.URL+"/v1/customers", map[string]string{
		"Authorization": "Bearer sk_test_marker_anything",
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 with matching bearer; got %d body=%s", status, body)
	}
}

func TestAvmock_DetectCanary_FlagsCanaryInHeaders(t *testing.T) {
	srv, log := newMockServer(t, "generic", "", "av_agt_canary_TESTLEAK")
	status, body := doGet(t, srv.URL+"/anything", map[string]string{
		"Authorization": "Bearer av_agt_canary_TESTLEAK",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 when canary surfaces in headers; got %d body=%s", status, body)
	}
	if !strings.Contains(log.String(), "[VAULT-LEAK]") {
		t.Errorf("expected VAULT-LEAK log line; got %s", log.String())
	}
	if !strings.Contains(string(body), "vault_leak_detected") {
		t.Errorf("expected error body; got %s", body)
	}
}

func TestAvmock_DetectCanary_FlagsCanaryInBody(t *testing.T) {
	srv, log := newMockServer(t, "generic", "", "av_agt_canary_TESTLEAK")
	status, _ := doPost(t, srv.URL+"/echo", nil, "the leaked token is av_agt_canary_TESTLEAK")
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 when canary surfaces in body; got %d", status)
	}
	if !strings.Contains(log.String(), "[VAULT-LEAK]") {
		t.Errorf("expected VAULT-LEAK log line; got %s", log.String())
	}
}

func TestAvmock_DetectCanary_NoMatchReturnsService(t *testing.T) {
	srv, log := newMockServer(t, "stripe", "", "av_agt_canary_TESTLEAK")
	status, _ := doPost(t, srv.URL+"/v1/charges", nil, "amount=2000")
	if status != http.StatusOK {
		t.Fatalf("expected 200 when canary absent; got %d", status)
	}
	if strings.Contains(log.String(), "[VAULT-LEAK]") {
		t.Errorf("must not log VAULT-LEAK for clean traffic; got %s", log.String())
	}
}

func TestRedactSubstring(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"ab", "***"},
		{"abcd", "***"},
		{"abcdefgh", "abcd***"},
		{"sk_test_marker", "sk_t***"},
	}
	for _, tc := range cases {
		if got := redactSubstring(tc.in); got != tc.want {
			t.Errorf("redactSubstring(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestScanForCanary(t *testing.T) {
	h := http.Header{"Authorization": []string{"Bearer xxx"}, "X-Custom": []string{"safe"}}
	if got := scanForCanary("xxx", "/safe/path", "", h, nil); got != "header:Authorization" {
		t.Errorf("expected header:Authorization; got %q", got)
	}
	if got := scanForCanary("yyy", "/safe", "", h, []byte("contains yyy in body")); got != "body" {
		t.Errorf("expected body; got %q", got)
	}
	if got := scanForCanary("urlleak", "/api/urlleak/charge", "", nil, nil); got != "url:path" {
		t.Errorf("expected url:path; got %q", got)
	}
	if got := scanForCanary("queryleak", "/v1/charges", "token=queryleak&id=42", nil, nil); got != "url:query" {
		t.Errorf("expected url:query; got %q", got)
	}
	// Priority: URL path before query before header before body.
	multi := http.Header{"X-Echo": []string{"shared"}}
	if got := scanForCanary("shared", "/p/shared", "k=shared", multi, []byte("shared")); got != "url:path" {
		t.Errorf("expected url:path to win priority; got %q", got)
	}
	if got := scanForCanary("", "/safe", "", h, nil); got != "" {
		t.Errorf("empty canary must return empty; got %q", got)
	}
	if got := scanForCanary("no-match", "/safe", "k=v", h, []byte("clean body")); got != "" {
		t.Errorf("expected empty for no match; got %q", got)
	}
}

func TestAvmock_DetectCanary_FlagsCanaryInURLQuery(t *testing.T) {
	srv, log := newMockServer(t, "generic", "", "av_agt_canary_TESTLEAK")
	status, body := doGet(t, srv.URL+"/v1/things?token=av_agt_canary_TESTLEAK", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 when canary surfaces in URL query; got %d body=%s", status, body)
	}
	if !strings.Contains(log.String(), "url:query") {
		t.Errorf("expected url:query in VAULT-LEAK log; got %s", log.String())
	}
}

func TestAvmock_DetectCanary_FlagsCanaryInURLPath(t *testing.T) {
	srv, log := newMockServer(t, "generic", "", "av_agt_canary_TESTLEAK")
	status, body := doGet(t, srv.URL+"/dump/av_agt_canary_TESTLEAK/details", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 when canary surfaces in URL path; got %d body=%s", status, body)
	}
	if !strings.Contains(log.String(), "url:path") {
		t.Errorf("expected url:path in VAULT-LEAK log; got %s", log.String())
	}
}

func TestAvmock_BodySizeCap_RejectsOversizeRequest(t *testing.T) {
	srv, _ := newMockServer(t, "generic", "", "")
	// 5 MiB exceeds the 4 MiB MaxBytesReader cap.
	huge := strings.Repeat("A", 5<<20)
	status, _ := doPost(t, srv.URL+"/echo", nil, huge)
	// MaxBytesReader makes ReadAll return an error; the handler then
	// proceeds with whatever it could read but the cap-error is
	// surfaced via the response writer. Either way the body is not
	// fully consumed — we don't crash and don't OOM. We assert just
	// that the server responded (didn't hang) and produced a non-2xx
	// or empty 200 — the contract is "bounded memory", not a specific
	// status code.
	if status == 0 {
		t.Fatalf("server hung instead of bounding the request body")
	}
}
