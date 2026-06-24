package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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
				EvalPackVersionID: uuid.New(),
				Status:                 domain.RunStatusQueued,
				ExecutionMode:          "single_agent",
				CreatedAt:              time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
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
		stubEvalPackReadService{},
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

func TestCreateRunEndpointPropagatesCIMetadata(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()
	runID := uuid.New()
	prNumber := int32(17)
	service := &fakeRunCreationService{
		result: CreateRunResult{
			Run: domain.Run{
				ID:                     runID,
				WorkspaceID:            workspaceID,
				EvalPackVersionID: uuid.New(),
				Status:                 domain.RunStatusQueued,
				ExecutionMode:          "single_agent",
				CIMetadata: &domain.RunCIMetadata{
					Provider:          "github_actions",
					Repository:        "acme/agent",
					PullRequestNumber: &prNumber,
					Branch:            "feature/gate",
					Ref:               "refs/pull/17/merge",
					CommitSHA:         "abc123",
					Workflow:          "AgentClash gate",
					WorkflowRunURL:    "https://github.com/acme/agent/actions/runs/99",
					EventName:         "pull_request",
					DefaultBranch:     "main",
				},
				CreatedAt: time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
		"agent_deployment_ids":["`+uuid.New().String()+`"],
		"ci_metadata":{
			"provider":" github_actions ",
			"repository":" acme/agent ",
			"pull_request_number":17,
			"branch":" feature/gate ",
			"ref":"refs/pull/17/merge",
			"commit_sha":"abc123",
			"workflow":"AgentClash gate",
			"workflow_run_url":"https://github.com/acme/agent/actions/runs/99",
			"event_name":"pull_request",
			"default_branch":" main "
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
		stubEvalPackReadService{},
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
		t.Fatalf("status = %d, want %d\n%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if service.input.CIMetadata == nil || service.input.CIMetadata.Repository != "acme/agent" || service.input.CIMetadata.Branch != "feature/gate" || service.input.CIMetadata.DefaultBranch != "main" {
		t.Fatalf("input ci metadata = %+v, want trimmed request metadata", service.input.CIMetadata)
	}
	if service.input.CIMetadata.PullRequestNumber == nil || *service.input.CIMetadata.PullRequestNumber != prNumber {
		t.Fatalf("input pr number = %v, want %d", service.input.CIMetadata.PullRequestNumber, prNumber)
	}

	var response createRunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.CIMetadata == nil || response.CIMetadata.WorkflowRunURL != "https://github.com/acme/agent/actions/runs/99" || response.CIMetadata.DefaultBranch != "main" {
		t.Fatalf("response ci metadata = %+v, want workflow link and default branch", response.CIMetadata)
	}
}

func TestCreateRunEndpointPropagatesModeAndReturnsVoiceMetadata(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()
	runID := uuid.New()
	service := &fakeRunCreationService{
		result: CreateRunResult{
			Run: domain.Run{
				ID:                     runID,
				WorkspaceID:            workspaceID,
				EvalPackVersionID: uuid.New(),
				Status:                 domain.RunStatusQueued,
				ExecutionMode:          "single_agent",
				ExecutionPlan:          json.RawMessage(`{"mode":"text-sim","modality":"voice","voice":{"mode":"text-sim","modality":"voice","transport":"text_sim"}}`),
				CreatedAt:              time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
		"agent_deployment_ids":["`+uuid.New().String()+`"],
		"mode":" text-sim "
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
		stubEvalPackReadService{},
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
		t.Fatalf("status = %d, want %d\n%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if service.input.Mode != "text-sim" {
		t.Fatalf("service input mode = %q, want text-sim", service.input.Mode)
	}

	var response createRunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Mode != "text-sim" || response.Modality != "voice" {
		t.Fatalf("response mode/modality = %q/%q, want text-sim/voice", response.Mode, response.Modality)
	}
	if response.Voice == nil || response.Voice.Mode != "text-sim" || response.Voice.Transport != "text_sim" {
		t.Fatalf("response voice = %+v, want text-sim/text_sim", response.Voice)
	}
}

func TestCreateRunEndpointRejectsInvalidCIMetadata(t *testing.T) {
	workspaceID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
		"agent_deployment_ids":["`+uuid.New().String()+`"],
		"ci_metadata":{"pull_request_number":0}
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
		stubEvalPackReadService{},
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
	if !bytes.Contains(recorder.Body.Bytes(), []byte("invalid_ci_metadata")) {
		t.Fatalf("body = %s, want invalid_ci_metadata", recorder.Body.String())
	}
}

func TestCreateRunEndpointRejectsUnsafeCIMetadataURL(t *testing.T) {
	workspaceID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
		"agent_deployment_ids":["`+uuid.New().String()+`"],
		"ci_metadata":{"workflow_run_url":"javascript:alert(1)"}
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
		stubEvalPackReadService{},
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
	if !bytes.Contains(recorder.Body.Bytes(), []byte("ci_metadata.workflow_run_url must be an http or https URL")) {
		t.Fatalf("body = %s, want workflow_run_url validation error", recorder.Body.String())
	}
}

func TestCreateRunEndpointRejectsOverlongDefaultBranchCIMetadata(t *testing.T) {
	workspaceID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
		"agent_deployment_ids":["`+uuid.New().String()+`"],
		"ci_metadata":{"default_branch":"`+strings.Repeat("a", 513)+`"}
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
		stubEvalPackReadService{},
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
	if !bytes.Contains(recorder.Body.Bytes(), []byte("ci_metadata.default_branch must be 512 characters or fewer")) {
		t.Fatalf("body = %s, want default_branch validation error", recorder.Body.String())
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
		stubEvalPackReadService{},
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
		"eval_pack_version_id":"`+uuid.New().String()+`",
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
					EvalPackVersionID: uuid.New(),
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
		stubEvalPackReadService{},
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
		"eval_pack_version_id":"`+uuid.New().String()+`",
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
		stubEvalPackReadService{},
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
		"eval_pack_version_id":"`+uuid.New().String()+`",
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
		stubEvalPackReadService{},
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

func TestCreateRunEndpointPropagatesRaceContext(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()
	runID := uuid.New()
	minStepGap := int32(5)
	service := &fakeRunCreationService{
		result: CreateRunResult{
			Run: domain.Run{
				ID:                     runID,
				WorkspaceID:            workspaceID,
				EvalPackVersionID: uuid.New(),
				Status:                 domain.RunStatusQueued,
				ExecutionMode:          "comparison",
				CreatedAt:              time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC),
				RaceContext:            true,
				RaceContextMinStepGap:  &minStepGap,
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
		"agent_deployment_ids":["`+uuid.New().String()+`","`+uuid.New().String()+`"],
		"race_context":true,
		"race_context_min_step_gap":5
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
		stubEvalPackReadService{},
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
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if !service.input.RaceContext {
		t.Fatalf("service saw RaceContext = false, want true")
	}
	if service.input.RaceContextMinStepGap == nil || *service.input.RaceContextMinStepGap != 5 {
		t.Fatalf("service saw MinStepGap = %v, want 5", service.input.RaceContextMinStepGap)
	}

	var response createRunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.RaceContext {
		t.Fatalf("response race_context = false, want true")
	}
	if response.RaceContextMinStepGap == nil || *response.RaceContextMinStepGap != 5 {
		t.Fatalf("response min_step_gap = %v, want 5", response.RaceContextMinStepGap)
	}
}

func TestDecodeCreateRunRequestAcceptsMaxIterations(t *testing.T) {
	workspaceID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
		"agent_deployment_ids":["`+uuid.New().String()+`"],
		"max_iterations":7
	}`))

	input, err := decodeCreateRunRequest(req)
	if err != nil {
		t.Fatalf("decodeCreateRunRequest returned error: %v", err)
	}
	if input.MaxIterations == nil || *input.MaxIterations != 7 {
		t.Fatalf("MaxIterations = %v, want 7", input.MaxIterations)
	}
}

func TestDecodeCreateRunRequestAcceptsMode(t *testing.T) {
	workspaceID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
		"agent_deployment_ids":["`+uuid.New().String()+`"],
		"mode":" TEXT-SIM "
	}`))

	input, err := decodeCreateRunRequest(req)
	if err != nil {
		t.Fatalf("decodeCreateRunRequest returned error: %v", err)
	}
	if input.Mode != "text-sim" {
		t.Fatalf("Mode = %q, want text-sim", input.Mode)
	}
}

func TestDecodeCreateRunRequestRejectsInvalidMaxIterations(t *testing.T) {
	workspaceID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
		"agent_deployment_ids":["`+uuid.New().String()+`"],
		"max_iterations":0
	}`))

	_, err := decodeCreateRunRequest(req)
	var validationErr RunCreationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want RunCreationValidationError", err)
	}
	if validationErr.Code != "invalid_max_iterations" {
		t.Fatalf("code = %q, want invalid_max_iterations", validationErr.Code)
	}
}

func TestCreateRunEndpointRejectsRaceContextCadenceOutOfRange(t *testing.T) {
	workspaceID := uuid.New()
	// 11 is out of range (valid is [1, 10]).
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
		"agent_deployment_ids":["`+uuid.New().String()+`","`+uuid.New().String()+`"],
		"race_context":true,
		"race_context_min_step_gap":11
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
		stubEvalPackReadService{},
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
		t.Fatalf("status = %d, want 400 (body: %s)", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("invalid_race_context_min_step_gap")) {
		t.Fatalf("expected error code invalid_race_context_min_step_gap in body, got %s", recorder.Body.String())
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
			SeededRuns: []EvalSessionSeededRun{
				{RunID: runID, Seed: 1},
			},
			SeriesRuns: []EvalSessionSeriesRun{
				{RunID: runID, MatrixKey: "default:seed-1", DeploymentLineup: "default", Seed: int64Ptr(1)},
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/eval-sessions", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
		"participants":[{"agent_deployment_id":"`+uuid.New().String()+`","label":"Primary"}],
		"execution_mode":"single_agent",
		"max_iterations":7,
		"eval_session":{
			"repetitions":2,
			"aggregation":{"method":"mean","report_variance":true,"confidence_interval":0.95},
			"routing_task_snapshot":{"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}},
			"seed_fanout":{"strategy":"explicit","seeds":[1,2]},
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
		stubEvalPackReadService{},
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
	if len(response.SeededRuns) != 1 || response.SeededRuns[0].RunID != runID || response.SeededRuns[0].Seed != 1 {
		t.Fatalf("seeded runs = %+v, want first run seed", response.SeededRuns)
	}
	if len(response.SeriesRuns) != 1 || response.SeriesRuns[0].RunID != runID || response.SeriesRuns[0].DeploymentLineup != "default" || response.SeriesRuns[0].Seed == nil || *response.SeriesRuns[0].Seed != 1 {
		t.Fatalf("series runs = %+v, want first run metadata", response.SeriesRuns)
	}
	if service.evalSessionInput.WorkspaceID != workspaceID {
		t.Fatalf("workspace id = %s, want %s", service.evalSessionInput.WorkspaceID, workspaceID)
	}
	if service.evalSessionInput.MaxIterations == nil || *service.evalSessionInput.MaxIterations != 7 {
		t.Fatalf("max iterations = %v, want 7", service.evalSessionInput.MaxIterations)
	}
	if len(service.evalSessionInput.Participants) != 1 || service.evalSessionInput.Participants[0].AgentDeploymentID == nil {
		t.Fatalf("participants = %+v, want one participant deployment id", service.evalSessionInput.Participants)
	}
	if got := service.evalSessionInput.EvalSession.SeedFanout; len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("seed fanout = %v, want [1 2]", got)
	}
}

func TestCreateEvalSessionEndpointAcceptsRunMatrix(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()
	firstDeploymentID := uuid.New()
	secondDeploymentID := uuid.New()
	service := &fakeRunCreationService{
		evalSessionResult: CreateEvalSessionResult{
			Session: domain.EvalSession{
				ID:                     uuid.New(),
				Status:                 domain.EvalSessionStatusQueued,
				Repetitions:            2,
				AggregationConfig:      domain.EvalSessionSnapshot{Document: []byte(`{"schema_version":1,"method":"mean","report_variance":true,"confidence_interval":0.95}`)},
				SuccessThresholdConfig: domain.EvalSessionSnapshot{Document: []byte(`{"schema_version":1}`)},
				RoutingTaskSnapshot:    domain.EvalSessionSnapshot{Document: []byte(`{"schema_version":1,"routing":{"mode":"series"},"task":{"pack_version":"v1"}}`)},
				SchemaVersion:          1,
				CreatedAt:              time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
				UpdatedAt:              time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
			},
			RunIDs: []uuid.UUID{uuid.New(), uuid.New()},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/eval-sessions", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
		"execution_mode":"single_agent",
		"eval_session":{
			"repetitions":2,
			"aggregation":{"method":"mean","report_variance":true,"confidence_interval":0.95},
			"routing_task_snapshot":{"routing":{"mode":"series"},"task":{"pack_version":"v1"}},
			"run_matrix":[
				{"key":"default:seed-1","deployment_lineup":"default","seed":1,"participants":[{"agent_deployment_id":"`+firstDeploymentID.String()+`","label":"Primary"}]},
				{"key":"smoke:seed-1","deployment_lineup":"smoke","seed":1,"participants":[{"agent_deployment_id":"`+secondDeploymentID.String()+`","label":"Primary"}]}
			],
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
		stubEvalPackReadService{},
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
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	matrix := service.evalSessionInput.EvalSession.RunMatrix
	if len(matrix) != 2 || matrix[0].Key != "default:seed-1" || matrix[1].DeploymentLineup != "smoke" {
		t.Fatalf("run matrix = %+v, want two entries", matrix)
	}
	if matrix[0].Seed == nil || *matrix[0].Seed != 1 {
		t.Fatalf("run matrix seed = %v, want 1", matrix[0].Seed)
	}
	if len(matrix[1].Participants) != 1 || matrix[1].Participants[0].AgentDeploymentID == nil || *matrix[1].Participants[0].AgentDeploymentID != secondDeploymentID {
		t.Fatalf("second matrix participants = %+v, want second deployment", matrix[1].Participants)
	}
}

func TestCreateEvalSessionEndpointRejectsParticipantsWithRunMatrix(t *testing.T) {
	workspaceID := uuid.New()
	deploymentID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/eval-sessions", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
		"participants":[{"agent_deployment_id":"`+deploymentID.String()+`","label":"Primary"}],
		"execution_mode":"single_agent",
		"eval_session":{
			"repetitions":1,
			"aggregation":{"method":"mean","report_variance":true,"confidence_interval":0.95},
			"routing_task_snapshot":{"routing":{"mode":"series"},"task":{"pack_version":"v1"}},
			"run_matrix":[
				{"key":"default:seed-1","deployment_lineup":"default","seed":1,"participants":[{"agent_deployment_id":"`+deploymentID.String()+`","label":"default"}]}
			],
			"schema_version":1
		}
	}`))
	_, err := decodeCreateEvalSessionRequest(req.Context(), req)
	var validationErr evalSessionValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want evalSessionValidationError", err)
	}
	if len(validationErr.Errors) != 1 || validationErr.Errors[0].Code != "participants.run_matrix_conflict" {
		t.Fatalf("validation errors = %+v, want participants/run_matrix conflict", validationErr.Errors)
	}
}

func TestDecodeEvalSessionConfigRejectsInvalidSeedFanout(t *testing.T) {
	cases := []struct {
		name string
		body string
		code string
	}{
		{
			name: "length_mismatch",
			body: `{"strategy":"explicit","seeds":[1]}`,
			code: "eval_session.seed_fanout.seeds.length_mismatch",
		},
		{
			name: "duplicate",
			body: `{"strategy":"explicit","seeds":[1,1]}`,
			code: "eval_session.seed_fanout.seeds.duplicate",
		},
		{
			name: "non_positive",
			body: `{"strategy":"explicit","seeds":[1,0]}`,
			code: "eval_session.seed_fanout.seeds.invalid",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeEvalSessionConfig(json.RawMessage(`{
				"repetitions":2,
				"aggregation":{"method":"mean","report_variance":true,"confidence_interval":0.95},
				"routing_task_snapshot":{"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}},
				"seed_fanout":` + tc.body + `,
				"schema_version":1
			}`))
			var validationErr evalSessionValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("error = %v, want evalSessionValidationError", err)
			}
			if len(validationErr.Errors) == 0 || validationErr.Errors[0].Code != tc.code {
				t.Fatalf("validation errors = %+v, want first code %s", validationErr.Errors, tc.code)
			}
		})
	}
}

func TestDecodeEvalSessionConfigPreservesMatrixParticipantErrorCode(t *testing.T) {
	_, err := decodeEvalSessionConfig(json.RawMessage(`{
		"repetitions":1,
		"aggregation":{"method":"mean","report_variance":true,"confidence_interval":0.95},
		"routing_task_snapshot":{"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}},
		"run_matrix":[
			{"key":"default:seed-1","seed":1,"participants":[{"agent_deployment_id":"not-a-uuid","label":"Primary"}]}
		],
		"schema_version":1
	}`))
	var validationErr evalSessionValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want evalSessionValidationError", err)
	}
	if len(validationErr.Errors) != 1 {
		t.Fatalf("validation errors = %+v, want one participant error", validationErr.Errors)
	}
	if validationErr.Errors[0].Field != "eval_session.run_matrix[0].participants[0]" || validationErr.Errors[0].Code != "invalid_participants" {
		t.Fatalf("validation error = %+v, want field path and invalid_participants code", validationErr.Errors[0])
	}
}

func TestDecodeEvalSessionConfigRejectsInvalidRunMatrix(t *testing.T) {
	cases := []struct {
		name     string
		matrix   string
		extra    string
		wantCode string
	}{
		{
			name:     "length_mismatch",
			matrix:   `[{"key":"default:seed-1","seed":1,"participants":[{"agent_deployment_id":"` + uuid.New().String() + `","label":"Primary"}]}]`,
			wantCode: "eval_session.run_matrix.length_mismatch",
		},
		{
			name:     "duplicate_key",
			matrix:   `[{"key":"dup","seed":1,"participants":[{"agent_deployment_id":"` + uuid.New().String() + `","label":"Primary"}]},{"key":"dup","seed":2,"participants":[{"agent_deployment_id":"` + uuid.New().String() + `","label":"Primary"}]}]`,
			wantCode: "eval_session.run_matrix.key.duplicate",
		},
		{
			name:     "non_positive_seed",
			matrix:   `[{"key":"default:seed-1","seed":0,"participants":[{"agent_deployment_id":"` + uuid.New().String() + `","label":"Primary"}]},{"key":"default:seed-2","seed":2,"participants":[{"agent_deployment_id":"` + uuid.New().String() + `","label":"Primary"}]}]`,
			wantCode: "eval_session.run_matrix.seed.invalid",
		},
		{
			name:     "seed_fanout_conflict",
			matrix:   `[{"key":"default:seed-1","seed":1,"participants":[{"agent_deployment_id":"` + uuid.New().String() + `","label":"Primary"}]},{"key":"default:seed-2","seed":2,"participants":[{"agent_deployment_id":"` + uuid.New().String() + `","label":"Primary"}]}]`,
			extra:    `,"seed_fanout":{"strategy":"explicit","seeds":[1,2]}`,
			wantCode: "eval_session.run_matrix.seed_fanout_conflict",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeEvalSessionConfig(json.RawMessage(`{
				"repetitions":2,
				"aggregation":{"method":"mean","report_variance":true,"confidence_interval":0.95},
				"routing_task_snapshot":{"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}},
				"run_matrix":` + tc.matrix + tc.extra + `,
				"schema_version":1
			}`))
			var validationErr evalSessionValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("error = %v, want evalSessionValidationError", err)
			}
			for _, detail := range validationErr.Errors {
				if detail.Code == tc.wantCode {
					return
				}
			}
			t.Fatalf("validation errors = %+v, want code %s", validationErr.Errors, tc.wantCode)
		})
	}
}

func TestCreateEvalSessionEndpointRejectsWeightedMeanWithoutWeights(t *testing.T) {
	workspaceID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/eval-sessions", bytes.NewBufferString(`{
		"workspace_id":"`+workspaceID.String()+`",
		"eval_pack_version_id":"`+uuid.New().String()+`",
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
		stubEvalPackReadService{},
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
		"eval_pack_version_id":"`+uuid.New().String()+`",
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
		stubEvalPackReadService{},
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
		"eval_pack_version_id":"`+uuid.New().String()+`",
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
		stubEvalPackReadService{},
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
		"eval_pack_version_id":"`+uuid.New().String()+`",
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
		stubEvalPackReadService{},
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
