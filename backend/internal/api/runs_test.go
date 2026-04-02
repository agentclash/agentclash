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

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
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

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		service,
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
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

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		&fakeRunCreationService{},
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
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

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
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

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		&fakeRunCreationService{},
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
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

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		&fakeRunCreationService{},
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
}

type fakeRunCreationService struct {
	caller Caller
	input  CreateRunInput
	result CreateRunResult
	err    error
}

func (f *fakeRunCreationService) CreateRun(_ context.Context, caller Caller, input CreateRunInput) (CreateRunResult, error) {
	f.caller = caller
	f.input = input
	return f.result, f.err
}
