package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

const (
	maxCreateRunRequestBytes = 1 << 20
	maxRunMaxIterations      = 1000
)

type RunCreationService interface {
	CreateRun(ctx context.Context, caller Caller, input CreateRunInput) (CreateRunResult, error)
	CreateEvalSession(ctx context.Context, caller Caller, input CreateEvalSessionInput) (CreateEvalSessionResult, error)
}

type createRunRequest struct {
	WorkspaceID            string   `json:"workspace_id"`
	ChallengePackVersionID string   `json:"challenge_pack_version_id"`
	ChallengeInputSetID    *string  `json:"challenge_input_set_id,omitempty"`
	Name                   string   `json:"name,omitempty"`
	Mode                   string   `json:"mode,omitempty"`
	AgentDeploymentIDs     []string `json:"agent_deployment_ids"`
	RegressionSuiteIDs     []string `json:"regression_suite_ids,omitempty"`
	RegressionCaseIDs      []string `json:"regression_case_ids,omitempty"`
	OfficialPackMode       string   `json:"official_pack_mode,omitempty"`
	// IncludeProposedRegressions lets explicit validation runs execute proposed
	// cases before they are activated into CI gate scope.
	IncludeProposedRegressions bool `json:"include_proposed_regressions,omitempty"`
	// RaceContext opts the run into live peer-standings injection. See #400.
	// Requires at least two agents; a request with true + one agent is 400.
	RaceContext bool `json:"race_context,omitempty"`
	// RaceContextMinStepGap overrides the default cadence threshold. When
	// omitted, the executor uses the backend default. Valid range [1, 10].
	RaceContextMinStepGap *int                  `json:"race_context_min_step_gap,omitempty"`
	MaxIterations         *int                  `json:"max_iterations,omitempty"`
	CIMetadata            *domain.RunCIMetadata `json:"ci_metadata,omitempty"`
}

type CreateRunInput struct {
	WorkspaceID                uuid.UUID
	ChallengePackVersionID     uuid.UUID
	ChallengeInputSetID        *uuid.UUID
	OfficialPackMode           domain.OfficialPackMode
	Name                       string
	Mode                       string
	AgentDeploymentIDs         []uuid.UUID
	RegressionSuiteIDs         []uuid.UUID
	RegressionCaseIDs          []uuid.UUID
	IncludeProposedRegressions bool
	RaceContext                bool
	RaceContextMinStepGap      *int32
	MaxIterations              *int32
	Seed                       *int64
	SeriesMatrixKey            string
	SeriesDeploymentLineup     string
	CIMetadata                 *domain.RunCIMetadata
	DatasetEvalRun             *repository.RecordDatasetEvalRunParams
	// RunID, when set, preallocates the run id (the guide confirmed path supplies it so the eval-credit
	// reservation and the run share a stable, idempotent id). Zero ⇒ a fresh id is minted.
	RunID uuid.UUID
	// ReservationMicros, when > 0, reserves that much managed eval credit atomically with run creation
	// (keyed "run:<RunID>"). 0 ⇒ no reservation (BYOK / REST). Requires RunID to be set.
	ReservationMicros int64
}

type CreateRunResult struct {
	Run domain.Run
}

type createRunResponse struct {
	ID                     uuid.UUID                 `json:"id"`
	WorkspaceID            uuid.UUID                 `json:"workspace_id"`
	ChallengePackVersionID uuid.UUID                 `json:"challenge_pack_version_id"`
	ChallengeInputSetID    *uuid.UUID                `json:"challenge_input_set_id,omitempty"`
	OfficialPackMode       string                    `json:"official_pack_mode"`
	Status                 domain.RunStatus          `json:"status"`
	ExecutionMode          string                    `json:"execution_mode"`
	Mode                   string                    `json:"mode,omitempty"`
	Modality               string                    `json:"modality,omitempty"`
	Voice                  *runVoiceMetadataResponse `json:"voice,omitempty"`
	CreatedAt              time.Time                 `json:"created_at"`
	QueuedAt               *time.Time                `json:"queued_at,omitempty"`
	RaceContext            bool                      `json:"race_context"`
	RaceContextMinStepGap  *int32                    `json:"race_context_min_step_gap,omitempty"`
	CIMetadata             *domain.RunCIMetadata     `json:"ci_metadata,omitempty"`
	Links                  runLinksResponse          `json:"links"`
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

func createRunHandler(logger *slog.Logger, service RunCreationService) http.HandlerFunc {
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
			logger.Error("failed to decode create run request",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		result, err := service.CreateRun(r.Context(), caller, input)
		if err != nil {
			switch {
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				var gateErr billing.GateError
				if errors.As(err, &gateErr) {
					writeBillingGateError(w, gateErr.Decision)
					return
				}

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

				logger.Error("create run request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"error", err,
				)
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

type runWorkflowErrorResponse struct {
	Error apiError       `json:"error"`
	Run   getRunResponse `json:"run"`
}

func buildCreateRunResponse(run domain.Run) createRunResponse {
	mode, modality, voice := runMetadataFromExecutionPlan(run.ExecutionPlan)
	return createRunResponse{
		ID:                     run.ID,
		WorkspaceID:            run.WorkspaceID,
		ChallengePackVersionID: run.ChallengePackVersionID,
		ChallengeInputSetID:    run.ChallengeInputSetID,
		OfficialPackMode:       string(run.OfficialPackMode),
		Status:                 run.Status,
		ExecutionMode:          run.ExecutionMode,
		Mode:                   mode,
		Modality:               modality,
		Voice:                  voice,
		CreatedAt:              run.CreatedAt,
		QueuedAt:               run.QueuedAt,
		RaceContext:            run.RaceContext,
		RaceContextMinStepGap:  run.RaceContextMinStepGap,
		CIMetadata:             cloneRunCIMetadata(run.CIMetadata),
		Links:                  buildRunLinks(run.ID),
	}
}

func buildRunLinks(runID uuid.UUID) runLinksResponse {
	return runLinksResponse{
		Self:   fmt.Sprintf("/v1/runs/%s", runID),
		Agents: fmt.Sprintf("/v1/runs/%s/agents", runID),
	}
}

func cancelRunHandler(logger *slog.Logger, service RunReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		runID, err := runIDFromURLParam("runID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_id", err.Error())
			return
		}

		result, err := service.CancelRun(r.Context(), caller, runID)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrRunNotFound):
				writeError(w, http.StatusNotFound, "run_not_found", "run not found")
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				var workflowErr RunCancellationWorkflowError
				if errors.As(err, &workflowErr) {
					writeJSON(w, http.StatusBadGateway, runWorkflowErrorResponse{
						Error: apiError{
							Code:    "workflow_cancel_failed",
							Message: "run could not be cancelled in Temporal",
						},
						Run: buildGetRunResponse(workflowErr.Run, nil),
					})
					return
				}

				logger.Error("cancel run request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"run_id", runID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		writeJSON(w, http.StatusOK, buildGetRunResponse(result.Run, nil))
	}
}

func decodeCreateRunRequest(r *http.Request) (CreateRunInput, error) {
	var body createRunRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return CreateRunInput{}, err
		}
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

	officialPackMode := domain.OfficialPackModeFull
	if trimmed := strings.TrimSpace(body.OfficialPackMode); trimmed != "" {
		parsedMode, parseErr := domain.ParseOfficialPackMode(trimmed)
		if parseErr != nil {
			return CreateRunInput{}, RunCreationValidationError{
				Code:    "invalid_official_pack_mode",
				Message: "official_pack_mode must be either full or suite_only",
			}
		}
		officialPackMode = parsedMode
	}

	deploymentIDs := make([]uuid.UUID, 0, len(body.AgentDeploymentIDs))
	for _, rawID := range body.AgentDeploymentIDs {
		deploymentID, parseErr := parseRequiredUUID(rawID, "agent_deployment_ids", "invalid_agent_deployment_ids")
		if parseErr != nil {
			return CreateRunInput{}, parseErr
		}
		deploymentIDs = append(deploymentIDs, deploymentID)
	}

	regressionSuiteIDs := make([]uuid.UUID, 0, len(body.RegressionSuiteIDs))
	for _, rawID := range body.RegressionSuiteIDs {
		suiteID, parseErr := parseRequiredUUID(rawID, "regression_suite_ids", "invalid_regression_suite_ids")
		if parseErr != nil {
			return CreateRunInput{}, parseErr
		}
		regressionSuiteIDs = append(regressionSuiteIDs, suiteID)
	}

	regressionCaseIDs := make([]uuid.UUID, 0, len(body.RegressionCaseIDs))
	for _, rawID := range body.RegressionCaseIDs {
		caseID, parseErr := parseRequiredUUID(rawID, "regression_case_ids", "invalid_regression_case_ids")
		if parseErr != nil {
			return CreateRunInput{}, parseErr
		}
		regressionCaseIDs = append(regressionCaseIDs, caseID)
	}

	var raceContextMinStepGap *int32
	if body.RaceContextMinStepGap != nil {
		gap := *body.RaceContextMinStepGap
		if gap < 1 || gap > 10 {
			return CreateRunInput{}, RunCreationValidationError{
				Code:    "invalid_race_context_min_step_gap",
				Message: "race_context_min_step_gap must be between 1 and 10",
			}
		}
		gap32 := int32(gap)
		raceContextMinStepGap = &gap32
	}

	var maxIterations *int32
	if body.MaxIterations != nil {
		value := *body.MaxIterations
		if value < 1 || value > maxRunMaxIterations {
			return CreateRunInput{}, RunCreationValidationError{
				Code:    "invalid_max_iterations",
				Message: fmt.Sprintf("max_iterations must be between 1 and %d", maxRunMaxIterations),
			}
		}
		value32 := int32(value)
		maxIterations = &value32
	}
	ciMetadata, err := normalizeCreateRunCIMetadata(body.CIMetadata)
	if err != nil {
		return CreateRunInput{}, err
	}
	mode := normalizeRunMode(body.Mode)

	return CreateRunInput{
		WorkspaceID:                workspaceID,
		ChallengePackVersionID:     challengePackVersionID,
		ChallengeInputSetID:        challengeInputSetID,
		OfficialPackMode:           officialPackMode,
		Name:                       strings.TrimSpace(body.Name),
		Mode:                       mode,
		AgentDeploymentIDs:         deploymentIDs,
		RegressionSuiteIDs:         regressionSuiteIDs,
		RegressionCaseIDs:          regressionCaseIDs,
		IncludeProposedRegressions: body.IncludeProposedRegressions,
		RaceContext:                body.RaceContext,
		RaceContextMinStepGap:      raceContextMinStepGap,
		MaxIterations:              maxIterations,
		CIMetadata:                 ciMetadata,
	}, nil
}

func normalizeCreateRunCIMetadata(metadata *domain.RunCIMetadata) (*domain.RunCIMetadata, error) {
	if metadata == nil {
		return nil, nil
	}
	if metadata.PullRequestNumber != nil && *metadata.PullRequestNumber <= 0 {
		return nil, RunCreationValidationError{
			Code:    "invalid_ci_metadata",
			Message: "ci_metadata.pull_request_number must be greater than 0",
		}
	}
	normalized := &domain.RunCIMetadata{
		Provider:           strings.TrimSpace(metadata.Provider),
		Repository:         strings.TrimSpace(metadata.Repository),
		PullRequestNumber:  cloneInt32Ptr(metadata.PullRequestNumber),
		Branch:             strings.TrimSpace(metadata.Branch),
		Ref:                strings.TrimSpace(metadata.Ref),
		CommitSHA:          strings.TrimSpace(metadata.CommitSHA),
		Workflow:           strings.TrimSpace(metadata.Workflow),
		WorkflowRunID:      strings.TrimSpace(metadata.WorkflowRunID),
		WorkflowRunAttempt: strings.TrimSpace(metadata.WorkflowRunAttempt),
		WorkflowRunURL:     strings.TrimSpace(metadata.WorkflowRunURL),
		EventName:          strings.TrimSpace(metadata.EventName),
		DefaultBranch:      strings.TrimSpace(metadata.DefaultBranch),
	}
	if normalized.Empty() {
		return nil, nil
	}
	for field, value := range map[string]string{
		"ci_metadata.provider":             normalized.Provider,
		"ci_metadata.repository":           normalized.Repository,
		"ci_metadata.branch":               normalized.Branch,
		"ci_metadata.ref":                  normalized.Ref,
		"ci_metadata.commit_sha":           normalized.CommitSHA,
		"ci_metadata.workflow":             normalized.Workflow,
		"ci_metadata.workflow_run_id":      normalized.WorkflowRunID,
		"ci_metadata.workflow_run_attempt": normalized.WorkflowRunAttempt,
		"ci_metadata.event_name":           normalized.EventName,
		"ci_metadata.default_branch":       normalized.DefaultBranch,
	} {
		if len(value) > 512 {
			return nil, RunCreationValidationError{Code: "invalid_ci_metadata", Message: field + " must be 512 characters or fewer"}
		}
	}
	if len(normalized.WorkflowRunURL) > 2048 {
		return nil, RunCreationValidationError{Code: "invalid_ci_metadata", Message: "ci_metadata.workflow_run_url must be 2048 characters or fewer"}
	}
	if normalized.WorkflowRunURL != "" && !isHTTPURL(normalized.WorkflowRunURL) {
		return nil, RunCreationValidationError{Code: "invalid_ci_metadata", Message: "ci_metadata.workflow_run_url must be an http or https URL"}
	}
	return normalized, nil
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func cloneInt32Ptr(value *int32) *int32 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneRunCIMetadata(metadata *domain.RunCIMetadata) *domain.RunCIMetadata {
	if metadata == nil || metadata.Empty() {
		return nil
	}
	return &domain.RunCIMetadata{
		Provider:           metadata.Provider,
		Repository:         metadata.Repository,
		PullRequestNumber:  cloneInt32Ptr(metadata.PullRequestNumber),
		Branch:             metadata.Branch,
		Ref:                metadata.Ref,
		CommitSHA:          metadata.CommitSHA,
		Workflow:           metadata.Workflow,
		WorkflowRunID:      metadata.WorkflowRunID,
		WorkflowRunAttempt: metadata.WorkflowRunAttempt,
		WorkflowRunURL:     metadata.WorkflowRunURL,
		EventName:          metadata.EventName,
		DefaultBranch:      metadata.DefaultBranch,
	}
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
