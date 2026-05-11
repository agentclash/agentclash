package cmd

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildVersionCreateSendsTemplateWithSpecOverrides(t *testing.T) {
	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/agent-builds/build-1/versions": func(w http.ResponseWriter, r *http.Request) {
			decodeJSONBody(t, r, &gotBody)
			jsonHandler(http.StatusCreated, map[string]any{
				"id":             "version-1",
				"version_number": 1,
			})(w, r)
		},
	})
	defer srv.Close()

	specFile := filepath.Join(t.TempDir(), "build-version.json")
	if err := os.WriteFile(specFile, []byte(`{"policy_spec":{"instructions":"Use the local rubric."}}`), 0o600); err != nil {
		t.Fatalf("write spec file: %v", err)
	}
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"build", "version", "create", "build-1",
		"--template", "code-reviewer",
		"--agent-kind", "llm_agent",
		"--spec-file", specFile,
	}, srv.URL); err != nil {
		t.Fatalf("build version create error: %v", err)
	}

	if gotBody["template"] != "code-reviewer" {
		t.Fatalf("template = %v, want code-reviewer; body=%v", gotBody["template"], gotBody)
	}
	if gotBody["agent_kind"] != "llm_agent" {
		t.Fatalf("agent_kind = %v, want llm_agent; body=%v", gotBody["agent_kind"], gotBody)
	}
	policy, ok := gotBody["policy_spec"].(map[string]any)
	if !ok || policy["instructions"] != "Use the local rubric." {
		t.Fatalf("policy_spec = %#v, want spec-file override", gotBody["policy_spec"])
	}
}

func TestBuildVersionTemplatesListsCatalog(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/agent-build-version-templates": func(w http.ResponseWriter, r *http.Request) {
			called = true
			jsonHandler(http.StatusOK, map[string]any{
				"items": []map[string]any{
					{
						"key":         "honest-agent",
						"name":        "Honest Agent",
						"agent_kind":  "llm_agent",
						"description": "Answers directly.",
					},
					{
						"key":         "code-reviewer",
						"name":        "Code Reviewer",
						"agent_kind":  "llm_agent",
						"description": "Reviews code.",
					},
				},
			})(w, r)
		},
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"build", "version", "templates"}, srv.URL); err != nil {
		t.Fatalf("build version templates error: %v", err)
	}
	if !called {
		t.Fatal("template catalog endpoint was not called")
	}

	out := stdout.finish()
	for _, snippet := range []string{"honest-agent", "code-reviewer", "llm_agent"} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("template list output missing %q\n---\n%s", snippet, out)
		}
	}
}
