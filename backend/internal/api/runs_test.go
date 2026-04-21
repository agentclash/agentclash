package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/google/uuid"
)

func TestCreateRunEndpointReturnsCreated(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()
	runID := uuid.New()
	service := &fakeRunCreationService{
		result: CreateRunResult{
			Run: domain.Run{
				ID:                     runID,
				WorkspaceID:            workspaceID,
				ChallengePackVersionID: uuid.New(),
				Status:                 domain.RunStatusQueued,
				ExecutionMode:          "single_agent",
				CreatedAt:              time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"challenge_pack_version_id":"`+uuid.New().String()+`",
		"agent_deployment_ids":["`+uuid.New().String()+`"]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		service,
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}

	var response createRunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.ID != runID {
		t.Fatalf("run id = %s, want %s", response.ID, runID)
	}
	if service.caller.UserID != userID {
		t.Fatalf("caller user id = %s, want %s", service.caller.UserID, userID)
	}
	if service.input.WorkspaceID != workspaceID {
		t.Fatalf("workspace id = %s, want %s", service.input.WorkspaceID, workspaceID)
	}
}

func TestCreateRunEndpointRejectsInvalidPayload(t *testing.T) {
	workspaceID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{"workspace_id":"`+workspaceID.String()+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		&fakeRunCreationService{},
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestCreateRunEndpointReturnsQueuedRunOnWorkflowStartFailure(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"challenge_pack_version_id":"`+uuid.New().String()+`",
		"agent_deployment_ids":["`+uuid.New().String()+`"]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		&fakeRunCreationService{
			err: RunWorkflowStartError{
				Run: domain.Run{
					ID:                     runID,
					WorkspaceID:            workspaceID,
					ChallengePackVersionID: uuid.New(),
					Status:                 domain.RunStatusQueued,
					ExecutionMode:          "single_agent",
					CreatedAt:              time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC),
				},
				Cause: errors.New("temporal unavailable"),
			},
		},
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadGateway)
	}

	var response createRunErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Run.ID != runID {
		t.Fatalf("run id = %s, want %s", response.Run.ID, runID)
	}
	if response.Error.Code != "workflow_start_failed" {
		t.Fatalf("error code = %q, want workflow_start_failed", response.Error.Code)
	}
}

func TestCreateRunEndpointRejectsNonJSONContentType(t *testing.T) {
	workspaceID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"challenge_pack_version_id":"`+uuid.New().String()+`",
		"agent_deployment_ids":["`+uuid.New().String()+`"]
	}`))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		&fakeRunCreationService{},
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnsupportedMediaType)
	}
}

func TestCreateRunEndpointRejectsOversizedRequestBody(t *testing.T) {
	workspaceID := uuid.New()
	oversizedName := bytes.Repeat([]byte("a"), maxCreateRunRequestBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"challenge_pack_version_id":"`+uuid.New().String()+`",
		"name":"`+string(oversizedName)+`",
		"agent_deployment_ids":["`+uuid.New().String()+`"]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		&fakeRunCreationService{},
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestCreateEvalSessionEndpointReturnsCreated(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	service := &fakeRunCreationService{
		evalSessionResult: CreateEvalSessionResult{
			Session: domain.EvalSession{
				ID:                     sessionID,
				Status:                 domain.EvalSessionStatusQueued,
				Repetitions:            2,
				AggregationConfig:      domain.EvalSessionSnapshot{Document: []byte(`{"schema_version":1,"method":"mean","report_variance":true,"confidence_interval":0.95}`)},
				SuccessThresholdConfig: domain.EvalSessionSnapshot{Document: []byte(`{"schema_version":1}`)},
				RoutingTaskSnapshot:    domain.EvalSessionSnapshot{Document: []byte(`{"schema_version":1,"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}}`)},
				SchemaVersion:          1,
				CreatedAt:              time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
				UpdatedAt:              time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
			},
			RunIDs: []uuid.UUID{runID, uuid.New()},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/eval-sessions", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"challenge_pack_version_id":"`+uuid.New().String()+`",
		"participants":[{"agent_deployment_id":"`+uuid.New().String()+`","label":"Primary"}],
		"execution_mode":"single_agent",
		"eval_session":{
			"repetitions":2,
			"aggregation":{"method":"mean","report_variance":true,"confidence_interval":0.95},
			"routing_task_snapshot":{"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}},
			"schema_version":1
		}
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		service,
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}

	var response createEvalSessionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.EvalSession.ID != sessionID {
		t.Fatalf("session id = %s, want %s", response.EvalSession.ID, sessionID)
	}
	if len(response.RunIDs) != 2 || response.RunIDs[0] != runID {
		t.Fatalf("run ids = %v, want first id %s", response.RunIDs, runID)
	}
	if service.evalSessionInput.WorkspaceID != workspaceID {
		t.Fatalf("workspace id = %s, want %s", service.evalSessionInput.WorkspaceID, workspaceID)
	}
	if len(service.evalSessionInput.Participants) != 1 || service.evalSessionInput.Participants[0].AgentDeploymentID == nil {
		t.Fatalf("participants = %+v, want one participant deployment id", service.evalSessionInput.Participants)
	}
}

func TestCreateEvalSessionEndpointRejectsWeightedMeanWithoutWeights(t *testing.T) {
	workspaceID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/eval-sessions", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"challenge_pack_version_id":"`+uuid.New().String()+`",
		"participants":[{"agent_deployment_id":"`+uuid.New().String()+`","label":"Primary"}],
		"execution_mode":"single_agent",
		"eval_session":{
			"repetitions":2,
			"aggregation":{"method":"weighted_mean","report_variance":true,"confidence_interval":0.95},
			"routing_task_snapshot":{"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}},
			"schema_version":1
		}
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		&fakeRunCreationService{},
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}

	var response evalSessionValidationEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode validation response: %v", err)
	}
	if len(response.Errors) != 1 || response.Errors[0].Code != "eval_session.reliability_weights.required" {
		t.Fatalf("validation errors = %+v, want eval_session.reliability_weights.required", response.Errors)
	}
}

func TestCreateEvalSessionEndpointAcceptsAggregationReliabilityWeight(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()
	service := &fakeRunCreationService{
		evalSessionResult: CreateEvalSessionResult{
			Session: domain.EvalSession{
				ID:          uuid.New(),
				Status:      domain.EvalSessionStatusQueued,
				Repetitions: 2,
				AggregationConfig: domain.EvalSessionSnapshot{Document: []byte(
					`{"schema_version":1,"method":"mean","report_variance":true,"confidence_interval":0.95,"reliability_weight":0.85}`,
				)},
				SuccessThresholdConfig: domain.EvalSessionSnapshot{Document: []byte(`{"schema_version":1}`)},
				RoutingTaskSnapshot:    domain.EvalSessionSnapshot{Document: []byte(`{"schema_version":1,"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}}`)},
				SchemaVersion:          1,
				CreatedAt:              time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
				UpdatedAt:              time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
			},
			RunIDs: []uuid.UUID{uuid.New(), uuid.New()},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/eval-sessions", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"challenge_pack_version_id":"`+uuid.New().String()+`",
		"participants":[{"agent_deployment_id":"`+uuid.New().String()+`","label":"Primary"}],
		"execution_mode":"single_agent",
		"eval_session":{
			"repetitions":2,
			"aggregation":{"method":"mean","report_variance":true,"confidence_interval":0.95,"reliability_weight":0.85},
			"routing_task_snapshot":{"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}},
			"schema_version":1
		}
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		service,
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if service.evalSessionInput.EvalSession.Aggregation.ReliabilityWeight == nil {
		t.Fatal("reliability weight = nil, want 0.85")
	}
	if got := *service.evalSessionInput.EvalSession.Aggregation.ReliabilityWeight; got != 0.85 {
		t.Fatalf("reliability weight = %.2f, want 0.85", got)
	}
}

func TestCreateEvalSessionEndpointRejectsInvalidAggregationReliabilityWeight(t *testing.T) {
	workspaceID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/eval-sessions", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"challenge_pack_version_id":"`+uuid.New().String()+`",
		"participants":[{"agent_deployment_id":"`+uuid.New().String()+`","label":"Primary"}],
		"execution_mode":"single_agent",
		"eval_session":{
			"repetitions":2,
			"aggregation":{"method":"mean","report_variance":true,"confidence_interval":0.95,"reliability_weight":1.25},
			"routing_task_snapshot":{"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}},
			"schema_version":1
		}
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		&fakeRunCreationService{},
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}

	var response evalSessionValidationEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode validation response: %v", err)
	}
	if len(response.Errors) != 1 || response.Errors[0].Code != "eval_session.aggregation.reliability_weight.invalid" {
		t.Fatalf("validation errors = %+v, want eval_session.aggregation.reliability_weight.invalid", response.Errors)
	}
}

func TestCreateEvalSessionEndpointAcceptsNullOptionalObjects(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()
	service := &fakeRunCreationService{
		evalSessionResult: CreateEvalSessionResult{
			Session: domain.EvalSession{
				ID:                     uuid.New(),
				Status:                 domain.EvalSessionStatusQueued,
				Repetitions:            2,
				AggregationConfig:      domain.EvalSessionSnapshot{Document: []byte(`{"schema_version":1,"method":"mean","report_variance":true,"confidence_interval":0.95}`)},
				SuccessThresholdConfig: domain.EvalSessionSnapshot{Document: []byte(`{"schema_version":1}`)},
				RoutingTaskSnapshot:    domain.EvalSessionSnapshot{Document: []byte(`{"schema_version":1,"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}}`)},
				SchemaVersion:          1,
				CreatedAt:              time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
				UpdatedAt:              time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
			},
			RunIDs: []uuid.UUID{uuid.New(), uuid.New()},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/eval-sessions", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"challenge_pack_version_id":"`+uuid.New().String()+`",
		"participants":[{"agent_deployment_id":"`+uuid.New().String()+`","label":"Primary"}],
		"execution_mode":"single_agent",
		"eval_session":{
			"repetitions":2,
			"aggregation":{"method":"mean","report_variance":true,"confidence_interval":0.95},
			"success_threshold":null,
			"routing_task_snapshot":{"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}},
			"reliability_weights":null,
			"schema_version":1
		}
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		service,
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if service.evalSessionInput.EvalSession.SuccessThreshold != nil {
		t.Fatalf("success threshold = %+v, want nil", service.evalSessionInput.EvalSession.SuccessThreshold)
	}
	if service.evalSessionInput.EvalSession.ReliabilityWeights != nil {
		t.Fatalf("reliability weights = %+v, want nil", service.evalSessionInput.EvalSession.ReliabilityWeights)
	}
}

type fakeRunCreationService struct {
	caller            Caller
	input             CreateRunInput
	result            CreateRunResult
	err               error
	evalSessionInput  CreateEvalSessionInput
	evalSessionResult CreateEvalSessionResult
	evalSessionErr    error
}

func (f *fakeRunCreationService) CreateRun(_ context.Context, caller Caller, input CreateRunInput) (CreateRunResult, error) {
	f.caller = caller
	f.input = input
	return f.result, f.err
}

func (f *fakeRunCreationService) CreateEvalSession(_ context.Context, caller Caller, input CreateEvalSessionInput) (CreateEvalSessionResult, error) {
	f.caller = caller
	f.evalSessionInput = input
	return f.evalSessionResult, f.evalSessionErr
}
