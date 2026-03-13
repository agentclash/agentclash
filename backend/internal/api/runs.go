package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/google/uuid"
)

const maxCreateRunRequestBytes = 1 << 20

type RunCreationService interface {
	CreateRun(ctx context.Context, caller Caller, input CreateRunInput) (CreateRunResult, error)
}

type createRunRequest struct {
	WorkspaceID            string   `json:"workspace_id"`
	ChallengePackVersionID string   `json:"challenge_pack_version_id"`
	ChallengeInputSetID    *string  `json:"challenge_input_set_id,omitempty"`
	Name                   string   `json:"name,omitempty"`
	AgentDeploymentIDs     []string `json:"agent_deployment_ids"`
}

type CreateRunInput struct {
	WorkspaceID            uuid.UUID
	ChallengePackVersionID uuid.UUID
	ChallengeInputSetID    *uuid.UUID
	Name                   string
	AgentDeploymentIDs     []uuid.UUID
}

type CreateRunResult struct {
	Run domain.Run
}

type createRunResponse struct {
	ID                     uuid.UUID        `json:"id"`
	WorkspaceID            uuid.UUID        `json:"workspace_id"`
	ChallengePackVersionID uuid.UUID        `json:"challenge_pack_version_id"`
	ChallengeInputSetID    *uuid.UUID       `json:"challenge_input_set_id,omitempty"`
	Status                 domain.RunStatus `json:"status"`
	ExecutionMode          string           `json:"execution_mode"`
	CreatedAt              time.Time        `json:"created_at"`
	QueuedAt               *time.Time       `json:"queued_at,omitempty"`
	Links                  runLinksResponse `json:"links"`
}

type runLinksResponse struct {
	Self   string `json:"self"`
	Agents string `json:"agents"`
}

type RunCreationValidationError struct {
	Code    string
	Message string
}

func (e RunCreationValidationError) Error() string {
	return e.Message
}

type RunWorkflowStartError struct {
	Run   domain.Run
	Cause error
}

func (e RunWorkflowStartError) Error() string {
	return fmt.Sprintf("start run workflow for run %s: %v", e.Run.ID, e.Cause)
}

func (e RunWorkflowStartError) Unwrap() error {
	return e.Cause
}

func createRunHandler(service RunCreationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxCreateRunRequestBytes)
		input, err := decodeCreateRunRequest(r)
		if err != nil {
			var validationErr RunCreationValidationError
			if errors.As(err, &validationErr) {
				writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
				return
			}
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body must be 1 MiB or smaller")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		result, err := service.CreateRun(r.Context(), caller, input)
		if err != nil {
			switch {
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				var validationErr RunCreationValidationError
				if errors.As(err, &validationErr) {
					writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
					return
				}

				var workflowStartErr RunWorkflowStartError
				if errors.As(err, &workflowStartErr) {
					writeJSON(w, http.StatusBadGateway, createRunErrorResponse{
						Error: apiError{
							Code:    "workflow_start_failed",
							Message: "run was created but the workflow could not be started",
						},
						Run: buildCreateRunResponse(workflowStartErr.Run),
					})
					return
				}

				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		writeJSON(w, http.StatusCreated, buildCreateRunResponse(result.Run))
	}
}

type createRunErrorResponse struct {
	Error apiError          `json:"error"`
	Run   createRunResponse `json:"run"`
}

func buildCreateRunResponse(run domain.Run) createRunResponse {
	return createRunResponse{
		ID:                     run.ID,
		WorkspaceID:            run.WorkspaceID,
		ChallengePackVersionID: run.ChallengePackVersionID,
		ChallengeInputSetID:    run.ChallengeInputSetID,
		Status:                 run.Status,
		ExecutionMode:          run.ExecutionMode,
		CreatedAt:              run.CreatedAt,
		QueuedAt:               run.QueuedAt,
		Links: runLinksResponse{
			Self:   fmt.Sprintf("/v1/runs/%s", run.ID),
			Agents: fmt.Sprintf("/v1/runs/%s/agents", run.ID),
		},
	}
}

func decodeCreateRunRequest(r *http.Request) (CreateRunInput, error) {
	var body createRunRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		if errors.Is(err, io.EOF) {
			return CreateRunInput{}, RunCreationValidationError{
				Code:    "invalid_request",
				Message: "request body is required",
			}
		}
		return CreateRunInput{}, RunCreationValidationError{
			Code:    "invalid_request",
			Message: "request body must be valid JSON",
		}
	}
	if decoder.More() {
		return CreateRunInput{}, RunCreationValidationError{
			Code:    "invalid_request",
			Message: "request body must contain exactly one JSON object",
		}
	}

	workspaceID, err := parseRequiredUUID(body.WorkspaceID, "workspace_id", "invalid_workspace_id")
	if err != nil {
		return CreateRunInput{}, err
	}
	challengePackVersionID, err := parseRequiredUUID(body.ChallengePackVersionID, "challenge_pack_version_id", "invalid_challenge_pack_version_id")
	if err != nil {
		return CreateRunInput{}, err
	}

	var challengeInputSetID *uuid.UUID
	if body.ChallengeInputSetID != nil && strings.TrimSpace(*body.ChallengeInputSetID) != "" {
		parsedID, parseErr := parseRequiredUUID(*body.ChallengeInputSetID, "challenge_input_set_id", "invalid_challenge_input_set_id")
		if parseErr != nil {
			return CreateRunInput{}, parseErr
		}
		challengeInputSetID = &parsedID
	}

	deploymentIDs := make([]uuid.UUID, 0, len(body.AgentDeploymentIDs))
	for _, rawID := range body.AgentDeploymentIDs {
		deploymentID, parseErr := parseRequiredUUID(rawID, "agent_deployment_ids", "invalid_agent_deployment_ids")
		if parseErr != nil {
			return CreateRunInput{}, parseErr
		}
		deploymentIDs = append(deploymentIDs, deploymentID)
	}

	return CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		ChallengeInputSetID:    challengeInputSetID,
		Name:                   strings.TrimSpace(body.Name),
		AgentDeploymentIDs:     deploymentIDs,
	}, nil
}

func parseRequiredUUID(raw string, field string, code string) (uuid.UUID, error) {
	if strings.TrimSpace(raw) == "" {
		return uuid.Nil, RunCreationValidationError{
			Code:    code,
			Message: fmt.Sprintf("%s is required", field),
		}
	}

	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.Nil, RunCreationValidationError{
			Code:    code,
			Message: fmt.Sprintf("%s must be a valid UUID", field),
		}
	}

	return id, nil
}

func requireJSONContentType(r *http.Request) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return fmt.Errorf("content type must be application/json")
	}
	if mediaType != "application/json" {
		return fmt.Errorf("content type must be application/json")
	}

	return nil
}
