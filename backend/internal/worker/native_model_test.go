package worker

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
)

func TestNativeModelInvokerPreparesSandboxAndInvokesProvider(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-1")
	fakeSandboxProvider := &sandbox.FakeProvider{
		NextSession: session,
	}
	fakeProviderClient := &observingProviderClient{
		t:       t,
		session: session,
		response: provider.Response{
			ProviderKey:     "openai",
			ProviderModelID: "gpt-4.1",
			OutputText:      "ok",
		},
	}

	invoker := NewNativeModelInvoker(fakeProviderClient, fakeSandboxProvider)
	executionContext := nativeModelExecutionContext()

	response, err := invoker.InvokeNativeModel(context.Background(), executionContext)
	if err != nil {
		t.Fatalf("InvokeNativeModel returned error: %v", err)
	}
	if response.OutputText != "ok" {
		t.Fatalf("response output = %q, want ok", response.OutputText)
	}
	if fakeProviderClient.callCount != 1 {
		t.Fatalf("provider request count = %d, want 1", fakeProviderClient.callCount)
	}
	if len(fakeSandboxProvider.CreateRequests) != 1 {
		t.Fatalf("sandbox create count = %d, want 1", len(fakeSandboxProvider.CreateRequests))
	}

	createRequest := fakeSandboxProvider.CreateRequests[0]
	if createRequest.RunAgentID != executionContext.RunAgent.ID {
		t.Fatalf("run agent id = %s, want %s", createRequest.RunAgentID, executionContext.RunAgent.ID)
	}
	if createRequest.ToolPolicy.AllowNetwork {
		t.Fatalf("allow_network = true, want false")
	}
	if createRequest.ToolPolicy.AllowShell {
		t.Fatalf("allow_shell = true, want false")
	}
	if createRequest.ToolPolicy.MaxToolCalls != executionContext.Deployment.RuntimeProfile.MaxToolCalls {
		t.Fatalf("max_tool_calls = %d, want %d", createRequest.ToolPolicy.MaxToolCalls, executionContext.Deployment.RuntimeProfile.MaxToolCalls)
	}
	if !reflect.DeepEqual(createRequest.ToolPolicy.AllowedToolKinds, []string{"file", "search"}) {
		t.Fatalf("allowed_tool_kinds = %v, want [file search]", createRequest.ToolPolicy.AllowedToolKinds)
	}
	if createRequest.Filesystem.WorkingDirectory != "/workspace/native" {
		t.Fatalf("working_directory = %q, want /workspace/native", createRequest.Filesystem.WorkingDirectory)
	}
	if !reflect.DeepEqual(createRequest.Filesystem.ReadableRoots, []string{"/workspace/native", "/tmp"}) {
		t.Fatalf("readable_roots = %v, want [/workspace/native /tmp]", createRequest.Filesystem.ReadableRoots)
	}
	if !reflect.DeepEqual(createRequest.Filesystem.WritableRoots, []string{"/workspace/native"}) {
		t.Fatalf("writable_roots = %v, want [/workspace/native]", createRequest.Filesystem.WritableRoots)
	}
	if createRequest.Filesystem.MaxWorkspaceBytes != 2048 {
		t.Fatalf("max_workspace_bytes = %d, want 2048", createRequest.Filesystem.MaxWorkspaceBytes)
	}

	files := session.Files()
	content, ok := files["/workspace/agentclash/run-context.json"]
	if !ok {
		t.Fatalf("run-context.json was not uploaded: %v", files)
	}

	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("unmarshal run-context.json returned error: %v", err)
	}
	if payload["run_agent_id"] != executionContext.RunAgent.ID.String() {
		t.Fatalf("run_context run_agent_id = %v, want %s", payload["run_agent_id"], executionContext.RunAgent.ID)
	}
	if session.DestroyCalls() != 1 {
		t.Fatalf("DestroyCalls = %d, want 1", session.DestroyCalls())
	}
}

func TestNativeModelInvokerReturnsDestroyErrorAfterProviderCall(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-2")
	session.SetDestroyError(errors.New("sandbox cleanup failed"))
	fakeSandboxProvider := &sandbox.FakeProvider{
		NextSession: session,
	}
	fakeProviderClient := &provider.FakeClient{
		Response: provider.Response{
			ProviderKey:     "openai",
			ProviderModelID: "gpt-4.1",
			OutputText:      "ok",
		},
	}

	invoker := NewNativeModelInvoker(fakeProviderClient, fakeSandboxProvider)

	_, err := invoker.InvokeNativeModel(context.Background(), nativeModelExecutionContext())
	if err == nil {
		t.Fatalf("expected destroy error")
	}
	if err.Error() != "destroy native sandbox: sandbox cleanup failed" {
		t.Fatalf("error = %q, want destroy error", err.Error())
	}
	if len(fakeProviderClient.Requests) != 1 {
		t.Fatalf("provider request count = %d, want 1", len(fakeProviderClient.Requests))
	}
}

func TestNativeModelInvokerFailsClosedWhenSandboxProviderIsMissing(t *testing.T) {
	fakeProviderClient := &provider.FakeClient{}
	invoker := NewNativeModelInvoker(fakeProviderClient, sandbox.UnconfiguredProvider{})

	_, err := invoker.InvokeNativeModel(context.Background(), nativeModelExecutionContext())
	if !errors.Is(err, sandbox.ErrProviderNotConfigured) {
		t.Fatalf("error = %v, want ErrProviderNotConfigured", err)
	}
	if len(fakeProviderClient.Requests) != 0 {
		t.Fatalf("provider request count = %d, want 0", len(fakeProviderClient.Requests))
	}
}

func nativeModelExecutionContext() repository.RunAgentExecutionContext {
	runID := uuid.New()
	runAgentID := uuid.New()

	return repository.RunAgentExecutionContext{
		Run:      repositoryRun(runID),
		RunAgent: repositoryRunAgent(runID, runAgentID),
		ChallengePackVersion: repository.ChallengePackVersionExecutionContext{
			ID:       uuid.New(),
			Manifest: []byte(`{"challenge":"fixture","tool_policy":{"allowed_tool_kinds":["file","search"]}}`),
		},
		ChallengeInputSet: &repository.ChallengeInputSetExecutionContext{
			ID:            uuid.New(),
			InputKey:      "default",
			Name:          "Default Input",
			InputChecksum: "checksum",
		},
		Deployment: repository.AgentDeploymentExecutionContext{
			DeploymentType: "native",
			SnapshotConfig: []byte(`{"entrypoint":"runner"}`),
			RuntimeProfile: repository.RuntimeProfileExecutionContext{
				ExecutionTarget: "native",
				TraceMode:       "preferred",
				MaxToolCalls:    7,
				ProfileConfig:   []byte(`{"sandbox":{"provider":"fake","working_directory":"/workspace/native","readable_roots":["/workspace/native","/tmp"],"writable_roots":["/workspace/native"],"max_workspace_bytes":2048}}`),
			},
			ProviderAccount: &repository.ProviderAccountExecutionContext{
				ID:                  uuid.New(),
				ProviderKey:         "openai",
				CredentialReference: "env://OPENAI_API_KEY",
			},
			ModelAlias: &repository.ModelAliasExecutionContext{
				ModelCatalogEntry: repository.ModelCatalogEntryExecutionContext{
					ProviderModelID: "gpt-4.1",
				},
			},
		},
	}
}

func repositoryRun(runID uuid.UUID) domain.Run {
	return domain.Run{
		ID: runID,
	}
}

func repositoryRunAgent(runID uuid.UUID, runAgentID uuid.UUID) domain.RunAgent {
	return domain.RunAgent{
		ID:    runAgentID,
		RunID: runID,
	}
}

type observingProviderClient struct {
	t         *testing.T
	session   *sandbox.FakeSession
	response  provider.Response
	callCount int
}

func (c *observingProviderClient) InvokeModel(_ context.Context, request provider.Request) (provider.Response, error) {
	c.callCount++
	if c.session.DestroyCalls() != 0 {
		c.t.Fatalf("sandbox destroyed before provider invocation")
	}
	files := c.session.Files()
	if _, ok := files["/workspace/agentclash/run-context.json"]; !ok {
		c.t.Fatalf("sandbox context file missing during provider invocation: %v", files)
	}
	if len(request.Messages) < 2 || request.Messages[1].Content == "" {
		c.t.Fatalf("provider request user payload was empty")
	}
	return c.response, nil
}
