package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// scrubStructuredJSON must recursively scrub header-shaped secrets out of structured audit payloads
// while preserving shape (ids/keys/non-secret values). No DB needed.
func TestScrubStructuredJSON(t *testing.T) {
	in := json.RawMessage(`{"run_id":"abc-123","note":"Authorization: Bearer sk-supersecret-token","ok":true,"nested":["plain","x-api-key: leakvalue"]}`)
	out := scrubStructuredJSON(in)

	s := string(out)
	if strings.Contains(s, "sk-supersecret-token") || strings.Contains(s, "leakvalue") {
		t.Fatalf("secret survived scrubbing: %s", s)
	}
	if !strings.Contains(s, "[redacted]") {
		t.Fatalf("expected redaction marker in scrubbed output: %s", s)
	}
	// Structure + non-secret values preserved.
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("scrubbed output is not valid JSON: %v", err)
	}
	if got["run_id"] != "abc-123" || got["ok"] != true {
		t.Fatalf("non-secret fields changed: %+v", got)
	}
}

// Empty / nil payloads round-trip unchanged.
func TestScrubStructuredJSONEmpty(t *testing.T) {
	if out := scrubStructuredJSON(nil); len(out) != 0 {
		t.Fatalf("nil → %q, want empty", out)
	}
}
