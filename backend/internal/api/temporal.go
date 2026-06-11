package api

import (
	"context"
	"fmt"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/hostedruns"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/workflow"
	"github.com/google/uuid"
	temporalsdk "go.temporal.io/sdk/client"
)

type TemporalClient interface {
	ExecuteWorkflow(ctx context.Context, options temporalsdk.StartWorkflowOptions, workflow interface{}, args ...interface{}) (temporalsdk.WorkflowRun, error)
	SignalWorkflow(ctx context.Context, workflowID string, runID string, signalName string, arg interface{}) error
	CancelWorkflow(ctx context.Context, workflowID string, runID string) error
}

type TemporalRunWorkflowStarter struct {
	client TemporalClient
	repo   RunTemporalIDRepository
}

type RunTemporalIDRepository interface {
	SetRunTemporalIDs(ctx context.Context, params repository.SetRunTemporalIDsParams) (domain.Run, error)
}

func NewTemporalRunWorkflowStarter(client TemporalClient, repo RunTemporalIDRepository) TemporalRunWorkflowStarter {
	return TemporalRunWorkflowStarter{client: client, repo: repo}
}

func (s TemporalRunWorkflowStarter) StartRunWorkflow(ctx context.Context, runID uuid.UUID) error {
	workflowID := fmt.Sprintf("%s/%s", workflow.RunWorkflowName, runID)
	run, err := s.client.ExecuteWorkflow(ctx, temporalsdk.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: workflow.WorkflowTaskQueue,
	}, workflow.RunWorkflowName, workflow.RunWorkflowInput{
		RunID: runID,
	})
	if err != nil {
		return err
	}
	if s.repo == nil {
		return nil
	}
	_, err = s.repo.SetRunTemporalIDs(ctx, repository.SetRunTemporalIDsParams{
		RunID:              runID,
		TemporalWorkflowID: run.GetID(),
		TemporalRunID:      run.GetRunID(),
	})
	return err
}

type TemporalRunWorkflowCanceller struct {
	client TemporalClient
}

func NewTemporalRunWorkflowCanceller(client TemporalClient) TemporalRunWorkflowCanceller {
	return TemporalRunWorkflowCanceller{client: client}
}

func (c TemporalRunWorkflowCanceller) CancelRunWorkflow(ctx context.Context, workflowID string, runID string) error {
	return c.client.CancelWorkflow(ctx, workflowID, runID)
}

type TemporalEvalSessionWorkflowStarter struct {
	client TemporalClient
}

func NewTemporalEvalSessionWorkflowStarter(client TemporalClient) TemporalEvalSessionWorkflowStarter {
	return TemporalEvalSessionWorkflowStarter{client: client}
}

func (s TemporalEvalSessionWorkflowStarter) StartEvalSessionWorkflow(ctx context.Context, evalSessionID uuid.UUID) error {
	workflowID := fmt.Sprintf("%s/%s", workflow.EvalSessionWorkflowName, evalSessionID)
	_, err := s.client.ExecuteWorkflow(ctx, temporalsdk.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: workflow.WorkflowTaskQueue,
	}, workflow.EvalSessionWorkflowName, workflow.EvalSessionWorkflowInput{
		EvalSessionID: evalSessionID,
	})
	return err
}

type TemporalAgentHarnessExecutionWorkflowStarter struct {
	client TemporalClient
}

func NewTemporalAgentHarnessExecutionWorkflowStarter(client TemporalClient) TemporalAgentHarnessExecutionWorkflowStarter {
	return TemporalAgentHarnessExecutionWorkflowStarter{client: client}
}

func (s TemporalAgentHarnessExecutionWorkflowStarter) StartAgentHarnessExecutionWorkflow(ctx context.Context, executionID uuid.UUID, timeoutSeconds int) (AgentHarnessExecutionWorkflowRef, error) {
	workflowID := fmt.Sprintf("%s/%s", workflow.AgentHarnessExecutionWorkflowName, executionID)
	run, err := s.client.ExecuteWorkflow(ctx, temporalsdk.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: workflow.WorkflowTaskQueue,
	}, workflow.AgentHarnessExecutionWorkflowName, workflow.AgentHarnessExecutionWorkflowInput{
		ExecutionID:    executionID,
		TimeoutSeconds: timeoutSeconds,
	})
	if err != nil {
		return AgentHarnessExecutionWorkflowRef{}, err
	}
	return AgentHarnessExecutionWorkflowRef{WorkflowID: run.GetID(), RunID: run.GetRunID()}, nil
}

func (s TemporalAgentHarnessExecutionWorkflowStarter) CancelAgentHarnessExecutionWorkflow(ctx context.Context, workflowID string, runID string) error {
	return s.client.CancelWorkflow(ctx, workflowID, runID)
}

type TemporalPublicAgentTryoutExecutionWorkflowStarter struct {
	client TemporalClient
}

func NewTemporalPublicAgentTryoutExecutionWorkflowStarter(client TemporalClient) TemporalPublicAgentTryoutExecutionWorkflowStarter {
	return TemporalPublicAgentTryoutExecutionWorkflowStarter{client: client}
}

func (s TemporalPublicAgentTryoutExecutionWorkflowStarter) StartPublicAgentTryoutExecutionWorkflow(ctx context.Context, tryoutID uuid.UUID) (AgentHarnessExecutionWorkflowRef, error) {
	workflowID := fmt.Sprintf("%s/%s", workflow.PublicAgentTryoutExecutionWorkflowName, tryoutID)
	run, err := s.client.ExecuteWorkflow(ctx, temporalsdk.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: workflow.WorkflowTaskQueue,
	}, workflow.PublicAgentTryoutExecutionWorkflowName, workflow.PublicAgentTryoutExecutionWorkflowInput{
		TryoutID: tryoutID,
	})
	if err != nil {
		return AgentHarnessExecutionWorkflowRef{}, err
	}
	return AgentHarnessExecutionWorkflowRef{WorkflowID: run.GetID(), RunID: run.GetRunID()}, nil
}

type TemporalPlaygroundWorkflowStarter struct {
	client TemporalClient
}

func NewTemporalPlaygroundWorkflowStarter(client TemporalClient) TemporalPlaygroundWorkflowStarter {
	return TemporalPlaygroundWorkflowStarter{client: client}
}

func (s TemporalPlaygroundWorkflowStarter) StartPlaygroundExperimentWorkflow(ctx context.Context, experimentID uuid.UUID) error {
	workflowID := fmt.Sprintf("%s/%s", workflow.PlaygroundExperimentWorkflowName, experimentID)
	_, err := s.client.ExecuteWorkflow(ctx, temporalsdk.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: workflow.WorkflowTaskQueue,
	}, workflow.PlaygroundExperimentWorkflowName, workflow.PlaygroundExperimentWorkflowInput{
		ExperimentID: experimentID,
	})
	return err
}

type TemporalSyntheticDatasetGenerationWorkflowStarter struct {
	client TemporalClient
}

func NewTemporalSyntheticDatasetGenerationWorkflowStarter(client TemporalClient) TemporalSyntheticDatasetGenerationWorkflowStarter {
	return TemporalSyntheticDatasetGenerationWorkflowStarter{client: client}
}

func (s TemporalSyntheticDatasetGenerationWorkflowStarter) StartSyntheticDatasetGenerationWorkflow(ctx context.Context, jobID uuid.UUID) error {
	workflowID := fmt.Sprintf("%s/%s", workflow.SyntheticDatasetGenerationWorkflowName, jobID)
	_, err := s.client.ExecuteWorkflow(ctx, temporalsdk.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: workflow.WorkflowTaskQueue,
	}, workflow.SyntheticDatasetGenerationWorkflowName, workflow.SyntheticDatasetGenerationWorkflowInput{
		JobID: jobID,
	})
	return err
}

type TemporalHostedRunWorkflowSignaler struct {
	client TemporalClient
}

func NewTemporalHostedRunWorkflowSignaler(client TemporalClient) TemporalHostedRunWorkflowSignaler {
	return TemporalHostedRunWorkflowSignaler{client: client}
}

func (s TemporalHostedRunWorkflowSignaler) SignalRunAgentWorkflow(ctx context.Context, runID uuid.UUID, runAgentID uuid.UUID, event hostedruns.Event) error {
	workflowID := fmt.Sprintf("%s/%s/%s", workflow.RunAgentWorkflowName, runID, runAgentID)
	return s.client.SignalWorkflow(ctx, workflowID, "", workflow.HostedRunEventSignal, event)
}
