package api

import (
	"context"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/workflow"
	"github.com/google/uuid"
	temporalsdk "go.temporal.io/sdk/client"
)

func TestTemporalRunWorkflowStarterStartRunWorkflowPersistsTemporalIDs(t *testing.T) {
	runID := uuid.New()
	client := &fakeTemporalClient{}
	repo := &fakeRunTemporalIDRepository{}

	err := NewTemporalRunWorkflowStarter(client, repo).StartRunWorkflow(context.Background(), runID)
	if err != nil {
		t.Fatalf("StartRunWorkflow returned error: %v", err)
	}

	if client.workflowName != workflow.RunWorkflowName {
		t.Fatalf("workflow name = %q, want %q", client.workflowName, workflow.RunWorkflowName)
	}
	if client.options.ID != workflow.RunWorkflowName+"/"+runID.String() {
		t.Fatalf("workflow id = %q, want %q", client.options.ID, workflow.RunWorkflowName+"/"+runID.String())
	}
	if client.options.TaskQueue != workflow.WorkflowTaskQueue {
		t.Fatalf("task queue = %q, want %q", client.options.TaskQueue, workflow.WorkflowTaskQueue)
	}

	input, ok := client.args[0].(workflow.RunWorkflowInput)
	if !ok {
		t.Fatalf("workflow input type = %T, want workflow.RunWorkflowInput", client.args[0])
	}
	if input.RunID != runID {
		t.Fatalf("run id = %s, want %s", input.RunID, runID)
	}
	if repo.params.RunID != runID {
		t.Fatalf("persisted run id = %s, want %s", repo.params.RunID, runID)
	}
	if repo.params.TemporalWorkflowID != client.options.ID || repo.params.TemporalRunID != "run-id" {
		t.Fatalf("persisted temporal ids = %#v", repo.params)
	}
}

func TestTemporalRunWorkflowStarterReturnsTemporalIDPersistenceError(t *testing.T) {
	runID := uuid.New()
	persistErr := errors.New("db down")

	err := NewTemporalRunWorkflowStarter(&fakeTemporalClient{}, &fakeRunTemporalIDRepository{err: persistErr}).
		StartRunWorkflow(context.Background(), runID)
	if !errors.Is(err, persistErr) {
		t.Fatalf("StartRunWorkflow error = %v, want %v", err, persistErr)
	}
}

func TestTemporalEvalSessionWorkflowStarterStartEvalSessionWorkflow(t *testing.T) {
	sessionID := uuid.New()
	client := &fakeTemporalClient{}

	err := NewTemporalEvalSessionWorkflowStarter(client).StartEvalSessionWorkflow(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("StartEvalSessionWorkflow returned error: %v", err)
	}

	if client.workflowName != workflow.EvalSessionWorkflowName {
		t.Fatalf("workflow name = %q, want %q", client.workflowName, workflow.EvalSessionWorkflowName)
	}
	if client.options.ID != workflow.EvalSessionWorkflowName+"/"+sessionID.String() {
		t.Fatalf("workflow id = %q, want %q", client.options.ID, workflow.EvalSessionWorkflowName+"/"+sessionID.String())
	}
	if client.options.TaskQueue != workflow.WorkflowTaskQueue {
		t.Fatalf("task queue = %q, want %q", client.options.TaskQueue, workflow.WorkflowTaskQueue)
	}

	input, ok := client.args[0].(workflow.EvalSessionWorkflowInput)
	if !ok {
		t.Fatalf("workflow input type = %T, want workflow.EvalSessionWorkflowInput", client.args[0])
	}
	if input.EvalSessionID != sessionID {
		t.Fatalf("eval session id = %s, want %s", input.EvalSessionID, sessionID)
	}
}

func TestTemporalAgentHarnessExecutionWorkflowStarter(t *testing.T) {
	executionID := uuid.New()
	client := &fakeTemporalClient{}

	ref, err := NewTemporalAgentHarnessExecutionWorkflowStarter(client).StartAgentHarnessExecutionWorkflow(context.Background(), executionID, 600)
	if err != nil {
		t.Fatalf("StartAgentHarnessExecutionWorkflow returned error: %v", err)
	}

	if client.workflowName != workflow.AgentHarnessExecutionWorkflowName {
		t.Fatalf("workflow name = %q, want %q", client.workflowName, workflow.AgentHarnessExecutionWorkflowName)
	}
	if client.options.ID != workflow.AgentHarnessExecutionWorkflowName+"/"+executionID.String() {
		t.Fatalf("workflow id = %q, want %q", client.options.ID, workflow.AgentHarnessExecutionWorkflowName+"/"+executionID.String())
	}
	if ref.WorkflowID != client.options.ID || ref.RunID != "run-id" {
		t.Fatalf("workflow ref = %#v", ref)
	}
	input, ok := client.args[0].(workflow.AgentHarnessExecutionWorkflowInput)
	if !ok {
		t.Fatalf("workflow input type = %T, want workflow.AgentHarnessExecutionWorkflowInput", client.args[0])
	}
	if input.ExecutionID != executionID {
		t.Fatalf("execution id = %s, want %s", input.ExecutionID, executionID)
	}
	if input.TimeoutSeconds != 600 {
		t.Fatalf("timeout seconds = %d, want 600", input.TimeoutSeconds)
	}
}

type fakeTemporalClient struct {
	options      temporalsdk.StartWorkflowOptions
	workflowName string
	args         []interface{}
}

func (f *fakeTemporalClient) ExecuteWorkflow(_ context.Context, options temporalsdk.StartWorkflowOptions, workflowFn interface{}, args ...interface{}) (temporalsdk.WorkflowRun, error) {
	f.options = options
	name, _ := workflowFn.(string)
	f.workflowName = name
	f.args = append([]interface{}(nil), args...)
	return fakeWorkflowRun{id: options.ID, runID: "run-id"}, nil
}

func (f *fakeTemporalClient) SignalWorkflow(context.Context, string, string, string, interface{}) error {
	return nil
}

func (f *fakeTemporalClient) CancelWorkflow(context.Context, string, string) error {
	return nil
}

type fakeWorkflowRun struct {
	id    string
	runID string
}

func (f fakeWorkflowRun) GetID() string {
	return f.id
}

func (f fakeWorkflowRun) GetRunID() string {
	return f.runID
}

func (f fakeWorkflowRun) Get(context.Context, interface{}) error {
	return nil
}

func (f fakeWorkflowRun) GetWithOptions(context.Context, interface{}, temporalsdk.WorkflowRunGetOptions) error {
	return nil
}

type fakeRunTemporalIDRepository struct {
	params repository.SetRunTemporalIDsParams
	err    error
}

func (f *fakeRunTemporalIDRepository) SetRunTemporalIDs(_ context.Context, params repository.SetRunTemporalIDsParams) (domain.Run, error) {
	f.params = params
	return domain.Run{ID: params.RunID}, f.err
}
