//go:build e2e

package reasoning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// E2E tests for the reasoning lane. These require the full stack:
//   - PostgreSQL (docker compose up postgres)
//   - Temporal (external)
//   - Go API server (make api-server)
//   - Go worker with REASONING_SERVICE_ENABLED=true (make worker)
//   - Python reasoning service (make reasoning-service)
//
// Run with: make test-e2e
// Or: cd backend && go test -race -count=1 -tags=e2e ./internal/reasoning/...
//
// Environment variables:
//   - E2E_API_URL: API server URL (default: http://localhost:8080)
//   - E2E_REASONING_URL: Reasoning service URL (default: http://localhost:8000)

func apiURL() string {
	if url := os.Getenv("E2E_API_URL"); url != "" {
		return url
	}
	return "http://localhost:8080"
}

func reasoningURL() string {
	if url := os.Getenv("E2E_REASONING_URL"); url != "" {
		return url
	}
	return "http://localhost:8000"
}

func TestE2EReasoningServiceHealthcheck(t *testing.T) {
	resp, err := http.Get(reasoningURL() + "/healthz")
	if err != nil {
		t.Skipf("reasoning service not reachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reasoning service healthz returned %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode healthz response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("healthz status = %q, want ok", body["status"])
	}
}

func TestE2EAPIServerHealthcheck(t *testing.T) {
	resp, err := http.Get(apiURL() + "/healthz")
	if err != nil {
		t.Skipf("API server not reachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("API server healthz returned %d", resp.StatusCode)
	}
}

// TestE2EReasoningServiceStartRejectsMissingCredential verifies that the
// reasoning service rejects a start request when no provider credential
// is present in the execution context.
func TestE2EReasoningServiceStartRejectsMissingCredential(t *testing.T) {
	body := map[string]any{
		"run_id":          "00000000-0000-0000-0000-000000000001",
		"run_agent_id":    "00000000-0000-0000-0000-000000000002",
		"idempotency_key": "e2e-test-no-cred",
		"execution_context": map[string]any{
			"Deployment": map[string]any{
				"RuntimeProfile":    map[string]any{"MaxIterations": 2},
				"ProviderAccount":   map[string]any{"ProviderKey": "openai"},
				"ModelAlias":        map[string]any{"ModelCatalogEntry": map[string]any{"ProviderModelID": "gpt-4o"}},
				"AgentBuildVersion": map[string]any{},
			},
		},
		"tools":          []any{},
		"callback_url":   "http://localhost:9999/events",
		"callback_token": "fake-token",
		"deadline_at":    time.Now().Add(5 * time.Minute).Format(time.RFC3339),
	}

	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post(reasoningURL()+"/reasoning/runs", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		t.Skipf("reasoning service not reachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	respBody, _ := io.ReadAll(resp.Body)
	json.Unmarshal(respBody, &result)

	if accepted, ok := result["accepted"].(bool); ok && accepted {
		t.Fatal("expected rejected (no credential), but got accepted")
	}
}

// TestE2EReasoningServiceCancelNonexistent verifies that cancelling a
// nonexistent run returns 404.
func TestE2EReasoningServiceCancelNonexistent(t *testing.T) {
	body := map[string]any{
		"idempotency_key": "cancel-e2e",
		"reason":          "test",
	}
	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post(reasoningURL()+"/reasoning/runs/nonexistent-id/cancel", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		t.Skipf("reasoning service not reachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// TestE2EReasoningServiceToolResultsNonexistent verifies that submitting
// tool results for a nonexistent run returns 404.
func TestE2EReasoningServiceToolResultsNonexistent(t *testing.T) {
	body := map[string]any{
		"idempotency_key": "tools-e2e",
		"tool_results":    []any{},
	}
	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post(reasoningURL()+"/reasoning/runs/nonexistent-id/tool-results", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		t.Skipf("reasoning service not reachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// TestE2ECallbackHandlerRejectsInvalidToken verifies that the Go callback
// handler rejects events with an invalid bearer token.
func TestE2ECallbackHandlerRejectsInvalidToken(t *testing.T) {
	event := map[string]any{
		"event_id":       "test-event-1",
		"schema_version": "2026-03-15",
		"run_id":         "00000000-0000-0000-0000-000000000001",
		"run_agent_id":   "00000000-0000-0000-0000-000000000002",
		"event_type":     "system.run.started",
		"source":         "reasoning_engine",
		"occurred_at":    time.Now().UTC().Format(time.RFC3339),
		"payload":        map[string]any{},
	}
	jsonBody, _ := json.Marshal(event)

	req, _ := http.NewRequest(http.MethodPost,
		apiURL()+"/v1/integrations/reasoning-runs/00000000-0000-0000-0000-000000000001/events",
		bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-token-value")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("API server not reachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 401, got %d: %s", resp.StatusCode, body)
	}
}

// TestE2ECallbackHandlerRejectsMissingToken verifies that the callback
// handler returns 401 when no Authorization header is present.
func TestE2ECallbackHandlerRejectsMissingToken(t *testing.T) {
	event := map[string]any{
		"event_id":       "test-event-2",
		"schema_version": "2026-03-15",
		"run_id":         "00000000-0000-0000-0000-000000000001",
		"run_agent_id":   "00000000-0000-0000-0000-000000000002",
		"event_type":     "system.run.started",
		"source":         "reasoning_engine",
		"occurred_at":    time.Now().UTC().Format(time.RFC3339),
		"payload":        map[string]any{},
	}
	jsonBody, _ := json.Marshal(event)

	req, _ := http.NewRequest(http.MethodPost,
		apiURL()+"/v1/integrations/reasoning-runs/00000000-0000-0000-0000-000000000001/events",
		bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("API server not reachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 401, got %d: %s", resp.StatusCode, body)
	}
}

// Placeholder for full-workflow E2E tests. These require a real LLM provider
// credential and full Temporal workflow orchestration. They will be implemented
// when the reasoning lane is wired into a live environment.
//
// Planned tests (Phase 5 execution plan):
//   - E2E-1: Tool-free success (model returns direct answer)
//   - E2E-2: Tool-using success (model proposes read_file, sandbox has file)
//   - E2E-3: Blocked tool batch (model proposes tool not in allowlist)
//   - E2E-4: Protocol violation (malformed event sequence)
//   - E2E-5: Cancellation (cancel before completion)
//   - E2E-6: Timeout (Python delays, short run timeout)
//   - E2E-7: Output validation repair (first output malformed, second corrected)
//
// These are tracked in the execution plan under Phase 5 section 5.3.

func init() {
	// Ensure unused imports don't cause build failures.
	_ = context.Background
	_ = fmt.Sprintf
}
