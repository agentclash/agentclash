package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
)

type fakeWorkspaceToolLookup struct {
	rows            map[uuid.UUID]repository.ToolRow
	calls           int
	lastWorkspaceID uuid.UUID
	lastToolIDs     []uuid.UUID
}

func (f *fakeWorkspaceToolLookup) ListToolsByIDs(_ context.Context, workspaceID uuid.UUID, ids []uuid.UUID) ([]repository.ToolRow, error) {
	f.calls++
	f.lastWorkspaceID = workspaceID
	f.lastToolIDs = append([]uuid.UUID(nil), ids...)

	rows := make([]repository.ToolRow, 0, len(ids))
	for _, id := range ids {
		row, ok := f.rows[id]
		if !ok {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func TestBuildToolRegistry_IncludesWorkspaceTools(t *testing.T) {
	workspaceID := uuid.New()
	toolID := uuid.New()

	registry, err := buildToolRegistry(
		sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindNetwork}, AllowNetwork: true},
		[]byte(`{"challenge":"fixture"}`),
		nil,
		nil,
		workspaceToolBinding{
			Tool: repository.ToolRow{
				ID:            toolID,
				WorkspaceID:   &workspaceID,
				Name:          "GitHub Create Issue",
				Slug:          "github_create_issue",
				ToolKind:      toolKindNetwork,
				CapabilityKey: composioExecuteCapability,
				Definition:    []byte(`{"tool_slug":"github_create_issue","credential_reference":"env://COMPOSIO_API_KEY","description":"Create a GitHub issue","parameters":{"type":"object","properties":{"title":{"type":"string"}}},"user_id":"user-123"}`),
			},
			Binding: repository.AgentBuildVersionToolBinding{
				ToolID:        toolID,
				BindingConfig: []byte(`{"tool_name":"github_create_issue"}`),
			},
		},
	)
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}

	assertRegistryVisibleTools(t, registry, "github_create_issue", httpRequestToolName, submitToolName)
	tool, ok := registry.Resolve("github_create_issue")
	if !ok {
		t.Fatalf("workspace tool was not visible")
	}
	if tool.Category() != ToolCategoryWorkspace {
		t.Fatalf("tool category = %q, want workspace", tool.Category())
	}
}

func TestComposioWorkspaceTool_Execute(t *testing.T) {
	t.Setenv("COMPOSIO_API_KEY", "test-composio-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v3/tools/execute/github_create_issue" {
			t.Fatalf("path = %s, want Composio execute path", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "test-composio-key" {
			t.Fatalf("x-api-key = %q, want test-composio-key", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if payload["user_id"] != "user-123" {
			t.Fatalf("user_id = %#v, want user-123", payload["user_id"])
		}
		arguments, ok := payload["arguments"].(map[string]any)
		if !ok {
			t.Fatalf("arguments = %#v, want object", payload["arguments"])
		}
		if arguments["title"] != "Bug report" {
			t.Fatalf("arguments.title = %#v, want Bug report", arguments["title"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"issue_number":42},"successful":true,"log_id":"log_123"}`))
	}))
	defer server.Close()

	tool, err := newWorkspaceTool(workspaceToolBinding{
		Tool: repository.ToolRow{
			ID:            uuid.New(),
			Name:          "GitHub Create Issue",
			Slug:          "github_create_issue",
			ToolKind:      toolKindNetwork,
			CapabilityKey: composioExecuteCapability,
			Definition:    []byte(fmt.Sprintf(`{"tool_slug":"github_create_issue","credential_reference":"env://COMPOSIO_API_KEY","base_url":%q,"user_id":"user-123"}`, server.URL)),
		},
	})
	if err != nil {
		t.Fatalf("newWorkspaceTool returned error: %v", err)
	}

	result, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"title":"Bug report"}`),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error payload: %s", result.Content)
	}
	if !strings.Contains(result.Content, `"issue_number":42`) {
		t.Fatalf("result content = %s, want issue_number", result.Content)
	}
}

func TestComposioWorkspaceTool_MarksUnsuccessfulResponseAsError(t *testing.T) {
	t.Setenv("COMPOSIO_API_KEY", "test-composio-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"successful":false,"error":"connection missing"}`))
	}))
	defer server.Close()

	tool, err := newWorkspaceTool(workspaceToolBinding{
		Tool: repository.ToolRow{
			ID:            uuid.New(),
			Name:          "GitHub Create Issue",
			Slug:          "github_create_issue",
			ToolKind:      toolKindNetwork,
			CapabilityKey: composioExecuteCapability,
			Definition:    []byte(fmt.Sprintf(`{"tool_slug":"github_create_issue","credential_reference":"env://COMPOSIO_API_KEY","base_url":%q,"user_id":"user-123"}`, server.URL)),
		},
	})
	if err != nil {
		t.Fatalf("newWorkspaceTool returned error: %v", err)
	}

	result, err := tool.Execute(t.Context(), ToolExecutionRequest{Args: json.RawMessage(`{"title":"Bug report"}`)})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error for unsuccessful response")
	}
	if !strings.Contains(result.Content, `"successful":false`) {
		t.Fatalf("error content = %s, want unsuccessful response payload", result.Content)
	}
}

func TestNativeExecutor_RegistersAndExecutesWorkspaceTool(t *testing.T) {
	t.Setenv("COMPOSIO_API_KEY", "test-composio-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/tools/execute/github_create_issue" {
			t.Fatalf("path = %s, want Composio execute path", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"issue_number":77},"successful":true}`))
	}))
	defer server.Close()

	workspaceID := uuid.New()
	toolID := uuid.New()
	lookup := &fakeWorkspaceToolLookup{
		rows: map[uuid.UUID]repository.ToolRow{
			toolID: {
				ID:            toolID,
				WorkspaceID:   &workspaceID,
				Name:          "GitHub Create Issue",
				Slug:          "github_create_issue",
				ToolKind:      toolKindNetwork,
				CapabilityKey: composioExecuteCapability,
				Definition:    []byte(fmt.Sprintf(`{"tool_slug":"github_create_issue","credential_reference":"env://COMPOSIO_API_KEY","base_url":%q,"user_id":"user-123","parameters":{"type":"object","properties":{"title":{"type":"string"}}}}`, server.URL)),
			},
		},
	}

	session := sandbox.NewFakeSession("native-workspace-tool")
	client := &scriptedProviderClient{
		t: t,
		steps: []providerStep{
			{
				validate: func(t *testing.T, request provider.Request) {
					names := toolDefinitionNames(request.Tools)
					if !slices.Contains(names, "github_create_issue") {
						t.Fatalf("tool definitions = %#v, want github_create_issue", names)
					}
					if !slices.Contains(names, submitToolName) {
						t.Fatalf("tool definitions = %#v, want submit", names)
					}
				},
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-composio",
							Name:      "github_create_issue",
							Arguments: []byte(`{"title":"Bug report"}`),
						},
					},
				},
			},
			{
				validate: func(t *testing.T, request provider.Request) {
					last := request.Messages[len(request.Messages)-1]
					if last.Role != "tool" || last.ToolCallID != "call-composio" {
						t.Fatalf("last message = %#v, want tool result for call-composio", last)
					}
					if last.IsError {
						t.Fatalf("tool result unexpectedly marked as error")
					}
					if !strings.Contains(last.Content, `"issue_number":77`) {
						t.Fatalf("tool result = %s, want issue_number", last.Content)
					}
				},
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-submit",
							Name:      submitToolName,
							Arguments: []byte(`{"answer":"opened issue"}`),
						},
					},
				},
			},
		},
	}

	executionContext := nativeExecutionContext()
	executionContext.Run.WorkspaceID = workspaceID
	executionContext.ChallengePackVersion.Manifest = []byte(`{"tool_policy":{"allowed_tool_kinds":["network"],"allow_network":true}}`)
	executionContext.Deployment.AgentBuildVersion.Tools = []repository.AgentBuildVersionToolBinding{
		{
			ToolID:        toolID,
			BindingRole:   "default",
			BindingConfig: []byte(`{"tool_name":"github_create_issue"}`),
		},
	}

	executor := NewNativeExecutor(client, &sandbox.FakeProvider{NextSession: session}, NoopObserver{}).WithWorkspaceToolLookup(lookup)
	result, err := executor.Execute(context.Background(), executionContext)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("stop reason = %s, want completed", result.StopReason)
	}
	if result.FinalOutput != "opened issue" {
		t.Fatalf("final output = %q, want opened issue", result.FinalOutput)
	}
	if lookup.calls != 1 {
		t.Fatalf("workspace lookup calls = %d, want 1", lookup.calls)
	}
	if lookup.lastWorkspaceID != workspaceID {
		t.Fatalf("workspace lookup workspace_id = %s, want %s", lookup.lastWorkspaceID, workspaceID)
	}
}

func toolDefinitionNames(definitions []provider.ToolDefinition) []string {
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}
	return names
}
