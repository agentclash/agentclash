package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestRedactTryoutSummaryForPublicStripsSensitiveKeys(t *testing.T) {
	raw := json.RawMessage(`{
		"verdict": "ready_to_inspect",
		"score": 0.92,
		"workspace_id": "ws-secret",
		"created_by_user_id": "user-secret",
		"nested": {"api_key": "sk-live-xxx", "tool_calls": 4, "fingerprint_hash": "deadbeef"},
		"steps": [{"summary": "planned", "session_token": "tok"}]
	}`)

	cleaned := redactTryoutSummaryForPublic(raw)
	got := string(cleaned)

	for _, banned := range []string{"workspace_id", "created_by_user_id", "api_key", "fingerprint_hash", "session_token", "sk-live-xxx", "ws-secret", "user-secret"} {
		if strings.Contains(got, banned) {
			t.Fatalf("redacted summary leaked %q: %s", banned, got)
		}
	}
	for _, kept := range []string{"verdict", "ready_to_inspect", "score", "tool_calls", "steps", "planned"} {
		if !strings.Contains(got, kept) {
			t.Fatalf("redacted summary dropped safe field %q: %s", kept, got)
		}
	}
}

func TestRedactTryoutSummaryForPublicKeepsTokenUsageMetrics(t *testing.T) {
	// Token-usage metrics are legitimate public data and must survive the
	// key-name redactor — only credential-shaped *_token / *_id keys are dropped.
	raw := json.RawMessage(`{
		"total_tokens": 1234,
		"input_tokens": 1000,
		"output_tokens": 234,
		"token_count": 1234,
		"session_count": 2,
		"access_token": "should-drop"
	}`)
	got := string(redactTryoutSummaryForPublic(raw))

	for _, kept := range []string{"total_tokens", "input_tokens", "output_tokens", "token_count", "session_count"} {
		if !strings.Contains(got, kept) {
			t.Fatalf("redaction dropped legitimate metric %q: %s", kept, got)
		}
	}
	if strings.Contains(got, "access_token") || strings.Contains(got, "should-drop") {
		t.Fatalf("redaction kept a credential-shaped key: %s", got)
	}
}

func TestRedactTryoutSummaryForPublicDropsInvalidJSON(t *testing.T) {
	if out := redactTryoutSummaryForPublic(json.RawMessage(`{not json`)); out != nil {
		t.Fatalf("invalid summary JSON should be dropped, got %s", out)
	}
	if out := redactTryoutSummaryForPublic(nil); out != nil {
		t.Fatalf("nil summary should stay nil, got %s", out)
	}
}

func TestParseTemplateArtifactAllowlist(t *testing.T) {
	snapshot := json.RawMessage(`{
		"slug": "meeting-minutes",
		"runtime": {
			"expected_artifacts": [
				{"key": "action_plan", "type": "markdown", "path": "action-plan.md"},
				{"key": "structured_minutes", "type": "json", "path": "minutes.json"},
				{"type": "json"}
			]
		}
	}`)

	allow := parseTemplateArtifactAllowlist(snapshot)
	if len(allow) != 2 {
		t.Fatalf("expected 2 allowlist entries (one dropped for empty key+path), got %d: %+v", len(allow), allow)
	}
	if allow[0].Key != "action_plan" || allow[0].Path != "action-plan.md" || allow[0].Type != "markdown" {
		t.Fatalf("unexpected first allowlist entry: %+v", allow[0])
	}

	if got := parseTemplateArtifactAllowlist(json.RawMessage(`{"runtime":{}}`)); got != nil {
		t.Fatalf("empty runtime should yield nil allowlist, got %+v", got)
	}
}

type fakeArtifactContentSigner struct {
	expiresAt time.Time
}

func (s fakeArtifactContentSigner) SignedArtifactContentURL(artifactID uuid.UUID, baseURL string, _ time.Time) (string, time.Time, error) {
	return baseURL + "/artifacts/" + artifactID.String() + "/content?signature=test", s.expiresAt, nil
}

func TestBuildSharedTryoutArtifactsDefaultDeny(t *testing.T) {
	allow := []templateAllowedArtifact{
		{Key: "action_plan", Type: "markdown", Path: "action-plan.md"},
	}
	approvedID := uuid.New()
	contentType := "text/markdown"
	size := int64(2048)
	checksum := "abc123"
	signer := fakeArtifactContentSigner{expiresAt: time.Unix(1_700_000_000, 0).UTC()}

	artifacts := []repository.Artifact{
		{
			ID:             approvedID,
			StorageBucket:  "secret-bucket",
			StorageKey:     "workspaces/ws/secret/key",
			ContentType:    &contentType,
			SizeBytes:      &size,
			ChecksumSHA256: &checksum,
			Metadata:       json.RawMessage(`{"artifact_key":"action_plan","original_filename":"action-plan.md"}`),
		},
		{
			// Not in the allowlist → must be dropped (default-deny).
			ID:       uuid.New(),
			Metadata: json.RawMessage(`{"artifact_key":"internal_debug_dump"}`),
		},
		{
			// No recognized identity → must be dropped.
			ID:       uuid.New(),
			Metadata: json.RawMessage(`{"foo":"bar"}`),
		},
	}

	out := buildSharedTryoutArtifacts(artifacts, allow, signer, "https://api.test", time.Now())
	if len(out) != 1 {
		t.Fatalf("expected exactly 1 approved artifact, got %d: %+v", len(out), out)
	}
	got := out[0]
	if got.Key != "action_plan" || got.Type != "markdown" || got.Path != "action-plan.md" {
		t.Fatalf("descriptor uses template-canonical identity; got %+v", got)
	}
	if got.SizeBytes == nil || *got.SizeBytes != size || got.ChecksumSHA256 == nil || *got.ChecksumSHA256 != checksum {
		t.Fatalf("descriptor should carry non-sensitive artifact facts; got %+v", got)
	}
	if !strings.Contains(got.DownloadURL, approvedID.String()) {
		t.Fatalf("descriptor should carry a signed download URL; got %q", got.DownloadURL)
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal artifacts: %v", err)
	}
	for _, leaked := range []string{"secret-bucket", "workspaces/ws/secret/key", "storage_key", "storage_bucket", "internal_debug_dump", "original_filename"} {
		if strings.Contains(string(encoded), leaked) {
			t.Fatalf("shared artifact descriptor leaked %q: %s", leaked, encoded)
		}
	}
}

func TestBuildSharedTryoutArtifactsMatchesByPath(t *testing.T) {
	allow := []templateAllowedArtifact{{Key: "diff", Type: "patch", Path: "changes.patch"}}
	artifacts := []repository.Artifact{{
		ID:       uuid.New(),
		Metadata: json.RawMessage(`{"relative_path":"changes.patch"}`),
	}}
	out := buildSharedTryoutArtifacts(artifacts, allow, nil, "", time.Now())
	if len(out) != 1 || out[0].Key != "diff" {
		t.Fatalf("expected path-matched approved artifact, got %+v", out)
	}
	if out[0].DownloadURL != "" {
		t.Fatalf("no signer/baseURL should yield no download URL, got %q", out[0].DownloadURL)
	}
}
