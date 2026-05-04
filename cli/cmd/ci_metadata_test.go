package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectGitHubActionsCIMetadataReadsPullRequestEvent(t *testing.T) {
	dir := t.TempDir()
	eventPath := filepath.Join(dir, "event.json")
	if err := os.WriteFile(eventPath, []byte(`{"pull_request":{"number":123}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(event) error: %v", err)
	}
	env := map[string]string{
		"GITHUB_ACTIONS":     "true",
		"GITHUB_REPOSITORY":  "acme/agent",
		"GITHUB_HEAD_REF":    "feature/gate",
		"GITHUB_REF":         "refs/pull/123/merge",
		"GITHUB_SHA":         "abc123",
		"GITHUB_WORKFLOW":    "AgentClash gate",
		"GITHUB_RUN_ID":      "987",
		"GITHUB_RUN_ATTEMPT": "2",
		"GITHUB_EVENT_NAME":  "pull_request",
		"GITHUB_EVENT_PATH":  eventPath,
	}

	metadata := detectGitHubActionsCIMetadata(func(key string) string {
		return env[key]
	}, os.ReadFile)

	if metadata.Repository != "acme/agent" || metadata.Branch != "feature/gate" || metadata.WorkflowRunURL != "https://github.com/acme/agent/actions/runs/987" {
		t.Fatalf("metadata = %+v, want repository, branch, and workflow URL", metadata)
	}
	if metadata.PullRequestNumber == nil || *metadata.PullRequestNumber != 123 {
		t.Fatalf("pull request number = %v, want 123", metadata.PullRequestNumber)
	}
}

func TestDetectGitHubActionsCIMetadataUsesPullRequestRefFallback(t *testing.T) {
	env := map[string]string{
		"GITHUB_ACTIONS":    "true",
		"GITHUB_REPOSITORY": "acme/agent",
		"GITHUB_REF":        "refs/pull/55/merge",
		"GITHUB_REF_NAME":   "55/merge",
		"GITHUB_RUN_ID":     "987",
	}

	metadata := detectGitHubActionsCIMetadata(func(key string) string {
		return env[key]
	}, func(string) ([]byte, error) {
		t.Fatal("event file should not be read when pull request number is in ref")
		return nil, nil
	})

	if metadata.PullRequestNumber == nil || *metadata.PullRequestNumber != 55 {
		t.Fatalf("pull request number = %v, want 55", metadata.PullRequestNumber)
	}
}
