package api

import (
	"context"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/workflow"
	"github.com/google/uuid"
	temporalsdk "go.temporal.io/sdk/client"
)

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
	return nil, nil
}

func (f *fakeTemporalClient) SignalWorkflow(context.Context, string, string, string, interface{}) error {
	return nil
}
