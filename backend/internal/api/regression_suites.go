package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/failurereview"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type RegressionRepository interface {
	CreateRegressionSuite(ctx context.Context, params repository.CreateRegressionSuiteParams) (repository.RegressionSuite, error)
	GetRegressionSuiteByID(ctx context.Context, id uuid.UUID) (repository.RegressionSuite, error)
	ListVisibleChallengePacks(ctx context.Context, workspaceID uuid.UUID) ([]repository.ChallengePackSummary, error)
	ListRegressionSuitesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) ([]repository.RegressionSuite, error)
	CountRegressionSuitesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (int64, error)
	PatchRegressionSuite(ctx context.Context, params repository.PatchRegressionSuiteParams) (repository.RegressionSuite, error)
	GetRegressionCaseByID(ctx context.Context, id uuid.UUID) (repository.RegressionCase, error)
	ListRegressionCasesBySuiteID(ctx context.Context, suiteID uuid.UUID) ([]repository.RegressionCase, error)
	PatchRegressionCase(ctx context.Context, params repository.PatchRegressionCaseParams) (repository.RegressionCase, error)
	GetRunByID(ctx context.Context, id uuid.UUID) (domain.Run, error)
	ListRunFailureReviewItems(ctx context.Context, runID uuid.UUID, agentID *uuid.UUID) ([]failurereview.Item, error)
	GetRunAgentExecutionContextByID(ctx context.Context, runAgentID uuid.UUID) (repository.RunAgentExecutionContext, error)
	GetRunAgentScorecardByRunAgentID(ctx context.Context, runAgentID uuid.UUID) (repository.RunAgentScorecard, error)
	GetEvaluationSpecByID(ctx context.Context, id uuid.UUID) (repository.EvaluationSpecRecord, error)
	PromoteFailure(ctx context.Context, params repository.PromoteFailureParams) (repository.PromoteFailureResult, error)
}

type RegressionService interface {
	CreateRegressionSuite(ctx context.Context, caller Caller, input CreateRegressionSuiteInput) (repository.RegressionSuite, error)
	ListRegressionSuites(ctx context.Context, caller Caller, input ListRegressionSuitesInput) (ListRegressionSuitesResult, error)
	GetRegressionSuite(ctx context.Context, caller Caller, input GetRegressionSuiteInput) (repository.RegressionSuite, error)
	PatchRegressionSuite(ctx context.Context, caller Caller, input PatchRegressionSuiteInput) (repository.RegressionSuite, error)
	ListRegressionCases(ctx context.Context, caller Caller, input ListRegressionCasesInput) ([]repository.RegressionCase, error)
	PatchRegressionCase(ctx context.Context, caller Caller, input PatchRegressionCaseInput) (repository.RegressionCase, error)
	PromoteFailure(ctx context.Context, caller Caller, input PromoteFailureInput) (PromoteFailureResult, error)
}

type CreateRegressionSuiteInput struct {
	WorkspaceID           uuid.UUID
	SourceChallengePackID uuid.UUID
	Name                  string
	Description           string
	DefaultGateSeverity   domain.RegressionSeverity
}

type ListRegressionSuitesInput struct {
	WorkspaceID uuid.UUID
	Limit       int32
	Offset      int32
}

type GetRegressionSuiteInput struct {
	WorkspaceID uuid.UUID
	SuiteID     uuid.UUID
}

type PatchRegressionSuiteInput struct {
	WorkspaceID         uuid.UUID
	SuiteID             uuid.UUID
	Name                *string
	Description         *string
	Status              *domain.RegressionSuiteStatus
	DefaultGateSeverity *domain.RegressionSeverity
}

type ListRegressionCasesInput struct {
	WorkspaceID uuid.UUID
	SuiteID     uuid.UUID
}

type PatchRegressionCaseInput struct {
	WorkspaceID uuid.UUID
	CaseID      uuid.UUID
	Title       *string
	Description *string
	Status      *domain.RegressionCaseStatus
	Severity    *domain.RegressionSeverity
}

type PromoteFailureInput struct {
	WorkspaceID         uuid.UUID
	RunID               uuid.UUID
	ChallengeIdentityID uuid.UUID
	Request             domain.PromotionRequest
}

type PromoteFailureResult struct {
	Case    repository.RegressionCase
	Created bool
}

type ListRegressionSuitesResult struct {
	Items  []repository.RegressionSuite
	Total  int64
	Limit  int32
	Offset int32
}

type RegressionManager struct {
	authorizer WorkspaceAuthorizer
	repo       RegressionRepository
}

var ErrChallengePackNotFound = errors.New("challenge pack not found")

var (
	ErrFailureReviewItemNotFound       = errors.New("failure review item not found")
	ErrFailureReviewItemAmbiguous      = errors.New("failure review item is ambiguous")
	ErrFailurePromotionNotAllowed      = errors.New("failure review item is not promotable")
	ErrFailurePromotionModeUnavailable = errors.New("promotion mode unavailable for failure review item")
	ErrRegressionSuiteArchived         = errors.New("regression suite is archived")
	ErrRegressionSuitePackMismatch     = errors.New("regression suite source pack does not match run pack")
)

func NewRegressionManager(authorizer WorkspaceAuthorizer, repo RegressionRepository) *RegressionManager {
	return &RegressionManager{authorizer: authorizer, repo: repo}
}

func (m *RegressionManager) CreateRegressionSuite(ctx context.Context, caller Caller, input CreateRegressionSuiteInput) (repository.RegressionSuite, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManageRegressions); err != nil {
		return repository.RegressionSuite{}, err
	}
	packs, err := m.repo.ListVisibleChallengePacks(ctx, input.WorkspaceID)
	if err != nil {
		return repository.RegressionSuite{}, fmt.Errorf("list visible challenge packs: %w", err)
	}
	foundPack := false
	for _, pack := range packs {
		if pack.ID == input.SourceChallengePackID {
			foundPack = true
			break
		}
	}
	if !foundPack {
		return repository.RegressionSuite{}, ErrChallengePackNotFound
	}

	return m.repo.CreateRegressionSuite(ctx, repository.CreateRegressionSuiteParams{
		WorkspaceID:           input.WorkspaceID,
		SourceChallengePackID: input.SourceChallengePackID,
		Name:                  strings.TrimSpace(input.Name),
		Description:           input.Description,
		Status:                domain.RegressionSuiteStatusActive,
		SourceMode:            "derived_only",
		DefaultGateSeverity:   input.DefaultGateSeverity,
		CreatedByUserID:       caller.UserID,
	})
}

func (m *RegressionManager) ListRegressionSuites(ctx context.Context, caller Caller, input ListRegressionSuitesInput) (ListRegressionSuitesResult, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionReadWorkspace); err != nil {
		return ListRegressionSuitesResult{}, err
	}

	suites, err := m.repo.ListRegressionSuitesByWorkspaceID(ctx, input.WorkspaceID, input.Limit, input.Offset)
	if err != nil {
		return ListRegressionSuitesResult{}, err
	}
	total, err := m.repo.CountRegressionSuitesByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return ListRegressionSuitesResult{}, err
	}

	return ListRegressionSuitesResult{
		Items:  suites,
		Total:  total,
		Limit:  input.Limit,
		Offset: input.Offset,
	}, nil
}

func (m *RegressionManager) GetRegressionSuite(ctx context.Context, caller Caller, input GetRegressionSuiteInput) (repository.RegressionSuite, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionReadWorkspace); err != nil {
		return repository.RegressionSuite{}, err
	}

	suite, err := m.repo.GetRegressionSuiteByID(ctx, input.SuiteID)
	if err != nil {
		return repository.RegressionSuite{}, err
	}
	if suite.WorkspaceID != input.WorkspaceID {
		return repository.RegressionSuite{}, ErrForbidden
	}
	return suite, nil
}

func (m *RegressionManager) PatchRegressionSuite(ctx context.Context, caller Caller, input PatchRegressionSuiteInput) (repository.RegressionSuite, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManageRegressions); err != nil {
		return repository.RegressionSuite{}, err
	}

	suite, err := m.repo.GetRegressionSuiteByID(ctx, input.SuiteID)
	if err != nil {
		return repository.RegressionSuite{}, err
	}
	if suite.WorkspaceID != input.WorkspaceID {
		return repository.RegressionSuite{}, ErrForbidden
	}

	return m.repo.PatchRegressionSuite(ctx, repository.PatchRegressionSuiteParams{
		ID:                  input.SuiteID,
		Name:                cloneStringPtr(input.Name),
		Description:         cloneStringPtr(input.Description),
		Status:              input.Status,
		DefaultGateSeverity: input.DefaultGateSeverity,
	})
}

func (m *RegressionManager) ListRegressionCases(ctx context.Context, caller Caller, input ListRegressionCasesInput) ([]repository.RegressionCase, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionReadWorkspace); err != nil {
		return nil, err
	}

	suite, err := m.repo.GetRegressionSuiteByID(ctx, input.SuiteID)
	if err != nil {
		return nil, err
	}
	if suite.WorkspaceID != input.WorkspaceID {
		return nil, ErrForbidden
	}
	return m.repo.ListRegressionCasesBySuiteID(ctx, input.SuiteID)
}

func (m *RegressionManager) PatchRegressionCase(ctx context.Context, caller Caller, input PatchRegressionCaseInput) (repository.RegressionCase, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManageRegressions); err != nil {
		return repository.RegressionCase{}, err
	}

	regressionCase, err := m.repo.GetRegressionCaseByID(ctx, input.CaseID)
	if err != nil {
		return repository.RegressionCase{}, err
	}
	if regressionCase.WorkspaceID != input.WorkspaceID {
		return repository.RegressionCase{}, ErrForbidden
	}

	return m.repo.PatchRegressionCase(ctx, repository.PatchRegressionCaseParams{
		ID:          input.CaseID,
		Title:       cloneStringPtr(input.Title),
		Description: cloneStringPtr(input.Description),
		Status:      input.Status,
		Severity:    input.Severity,
	})
}

func (m *RegressionManager) PromoteFailure(ctx context.Context, caller Caller, input PromoteFailureInput) (PromoteFailureResult, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManageRegressions); err != nil {
		return PromoteFailureResult{}, err
	}

	run, err := m.repo.GetRunByID(ctx, input.RunID)
	if err != nil {
		return PromoteFailureResult{}, err
	}
	if run.WorkspaceID != input.WorkspaceID {
		return PromoteFailureResult{}, repository.ErrRunNotFound
	}

	suite, err := m.repo.GetRegressionSuiteByID(ctx, input.Request.SuiteID)
	if err != nil {
		return PromoteFailureResult{}, err
	}
	if suite.WorkspaceID != input.WorkspaceID {
		return PromoteFailureResult{}, repository.ErrRegressionSuiteNotFound
	}
	if suite.Status != domain.RegressionSuiteStatusActive {
		return PromoteFailureResult{}, ErrRegressionSuiteArchived
	}

	item, err := m.findFailureReviewItem(ctx, input.RunID, input.ChallengeIdentityID)
	if err != nil {
		return PromoteFailureResult{}, err
	}
	if !item.Promotable {
		return PromoteFailureResult{}, ErrFailurePromotionNotAllowed
	}
	if !supportsPromotionMode(item, input.Request.PromotionMode) {
		return PromoteFailureResult{}, ErrFailurePromotionModeUnavailable
	}

	executionContext, err := m.repo.GetRunAgentExecutionContextByID(ctx, item.RunAgentID)
	if err != nil {
		return PromoteFailureResult{}, err
	}
	if suite.SourceChallengePackID != executionContext.ChallengePackVersion.ChallengePackID {
		return PromoteFailureResult{}, ErrRegressionSuitePackMismatch
	}
	scorecard, err := m.repo.GetRunAgentScorecardByRunAgentID(ctx, item.RunAgentID)
	if err != nil {
		return PromoteFailureResult{}, err
	}
	evaluationSpec, err := m.repo.GetEvaluationSpecByID(ctx, scorecard.EvaluationSpecID)
	if err != nil {
		return PromoteFailureResult{}, err
	}
	expectedContract, err := expectedContractSubset(evaluationSpec.Definition)
	if err != nil {
		return PromoteFailureResult{}, fmt.Errorf("build expected contract: %w", err)
	}

	severity := domain.DefaultPromotionSeverityForFailureClass(string(item.FailureClass))
	if input.Request.Severity != nil {
		severity = *input.Request.Severity
	}

	sourceEventRefs, err := json.Marshal(item.ReplayStepRefs)
	if err != nil {
		return PromoteFailureResult{}, fmt.Errorf("marshal source event refs: %w", err)
	}
	promotionSnapshot, err := marshalPromotionSnapshot(item, input.Request, severity)
	if err != nil {
		return PromoteFailureResult{}, fmt.Errorf("marshal promotion snapshot: %w", err)
	}

	result, err := m.repo.PromoteFailure(ctx, repository.PromoteFailureParams{
		SuiteID:             input.Request.SuiteID,
		RunID:               input.RunID,
		RunAgentID:          item.RunAgentID,
		ChallengeIdentityID: input.ChallengeIdentityID,
		Title:               input.Request.Title,
		FailureSummary:      input.Request.FailureSummary,
		Severity:            severity,
		PromotionMode:       input.Request.PromotionMode,
		FailureClass:        string(item.FailureClass),
		EvidenceTier:        string(item.EvidenceTier),
		SourceCaseKey:       item.CaseKey,
		SourceItemKey:       optionalStringPtr(item.ItemKey),
		ExpectedContract:    expectedContract,
		ValidatorOverrides:  input.Request.ValidatorOverrides,
		Metadata:            input.Request.Metadata,
		SourceEventRefs:     sourceEventRefs,
		PromotionSnapshot:   promotionSnapshot,
		PromotedByUserID:    caller.UserID,
	})
	if err != nil {
		return PromoteFailureResult{}, err
	}

	return PromoteFailureResult{
		Case:    result.Case,
		Created: result.Created,
	}, nil
}

func (m *RegressionManager) findFailureReviewItem(ctx context.Context, runID, challengeIdentityID uuid.UUID) (failurereview.Item, error) {
	items, err := m.repo.ListRunFailureReviewItems(ctx, runID, nil)
	if err != nil {
		return failurereview.Item{}, err
	}

	var match *failurereview.Item
	for i := range items {
		item := items[i]
		if item.ChallengeIdentityID == nil || *item.ChallengeIdentityID != challengeIdentityID {
			continue
		}
		if match != nil {
			return failurereview.Item{}, ErrFailureReviewItemAmbiguous
		}
		candidate := item
		match = &candidate
	}
	if match == nil {
		return failurereview.Item{}, ErrFailureReviewItemNotFound
	}
	return *match, nil
}

func supportsPromotionMode(item failurereview.Item, mode domain.RegressionPromotionMode) bool {
	for _, candidate := range item.PromotionModeAvailable {
		if string(candidate) == string(mode) {
			return true
		}
	}
	return false
}

func marshalPromotionSnapshot(item failurereview.Item, request domain.PromotionRequest, severity domain.RegressionSeverity) (json.RawMessage, error) {
	snapshot := struct {
		Request struct {
			SuiteID            uuid.UUID                    `json:"suite_id"`
			PromotionMode      domain.RegressionPromotionMode `json:"promotion_mode"`
			Title              string                       `json:"title"`
			FailureSummary     string                       `json:"failure_summary,omitempty"`
			Severity           domain.RegressionSeverity    `json:"severity"`
			ValidatorOverrides json.RawMessage             `json:"validator_overrides,omitempty"`
			Metadata           json.RawMessage             `json:"metadata,omitempty"`
		} `json:"request"`
		FailureReviewItem failurereview.Item `json:"failure_review_item"`
	}{FailureReviewItem: item}

	snapshot.Request.SuiteID = request.SuiteID
	snapshot.Request.PromotionMode = request.PromotionMode
	snapshot.Request.Title = request.Title
	snapshot.Request.FailureSummary = request.FailureSummary
	snapshot.Request.Severity = severity
	snapshot.Request.ValidatorOverrides = request.ValidatorOverrides
	snapshot.Request.Metadata = request.Metadata

	return json.Marshal(snapshot)
}

func expectedContractSubset(definition json.RawMessage) (json.RawMessage, error) {
	spec, err := scoring.DecodeDefinition(definition)
	if err != nil {
		return nil, err
	}

	subset := struct {
		JudgeMode           scoring.JudgeMode              `json:"judge_mode"`
		Validators          []scoring.ValidatorDeclaration `json:"validators,omitempty"`
		Metrics             []scoring.MetricDeclaration    `json:"metrics,omitempty"`
		Behavioral          *scoring.BehavioralConfig      `json:"behavioral,omitempty"`
		LLMJudges           []scoring.LLMJudgeDeclaration  `json:"llm_judges,omitempty"`
		PostExecutionChecks []scoring.PostExecutionCheck   `json:"post_execution_checks,omitempty"`
		Scorecard           scoring.ScorecardDeclaration   `json:"scorecard"`
	}{
		JudgeMode:           spec.JudgeMode,
		Validators:          spec.Validators,
		Metrics:             spec.Metrics,
		Behavioral:          spec.Behavioral,
		LLMJudges:           spec.LLMJudges,
		PostExecutionChecks: spec.PostExecutionChecks,
		Scorecard:           spec.Scorecard,
	}

	encoded, err := json.Marshal(subset)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func optionalStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

type regressionSuiteResponse struct {
	ID                    uuid.UUID                    `json:"id"`
	WorkspaceID           uuid.UUID                    `json:"workspace_id"`
	SourceChallengePackID uuid.UUID                    `json:"source_challenge_pack_id"`
	Name                  string                       `json:"name"`
	Description           string                       `json:"description"`
	Status                domain.RegressionSuiteStatus `json:"status"`
	SourceMode            string                       `json:"source_mode"`
	DefaultGateSeverity   domain.RegressionSeverity    `json:"default_gate_severity"`
	CreatedByUserID       uuid.UUID                    `json:"created_by_user_id"`
	CreatedAt             time.Time                    `json:"created_at"`
	UpdatedAt             time.Time                    `json:"updated_at"`
}

type regressionCaseResponse struct {
	ID                           uuid.UUID                      `json:"id"`
	SuiteID                      uuid.UUID                      `json:"suite_id"`
	WorkspaceID                  uuid.UUID                      `json:"workspace_id"`
	Title                        string                         `json:"title"`
	Description                  string                         `json:"description"`
	Status                       domain.RegressionCaseStatus    `json:"status"`
	Severity                     domain.RegressionSeverity      `json:"severity"`
	PromotionMode                domain.RegressionPromotionMode `json:"promotion_mode"`
	SourceRunID                  *uuid.UUID                     `json:"source_run_id,omitempty"`
	SourceRunAgentID             *uuid.UUID                     `json:"source_run_agent_id,omitempty"`
	SourceReplayID               *uuid.UUID                     `json:"source_replay_id,omitempty"`
	SourceChallengePackVersionID uuid.UUID                      `json:"source_challenge_pack_version_id"`
	SourceChallengeInputSetID    *uuid.UUID                     `json:"source_challenge_input_set_id,omitempty"`
	SourceChallengeIdentityID    uuid.UUID                      `json:"source_challenge_identity_id"`
	SourceCaseKey                string                         `json:"source_case_key"`
	SourceItemKey                *string                        `json:"source_item_key,omitempty"`
	EvidenceTier                 string                         `json:"evidence_tier"`
	FailureClass                 string                         `json:"failure_class"`
	FailureSummary               string                         `json:"failure_summary"`
	PayloadSnapshot              json.RawMessage                `json:"payload_snapshot"`
	ExpectedContract             json.RawMessage                `json:"expected_contract"`
	ValidatorOverrides           json.RawMessage                `json:"validator_overrides,omitempty"`
	Metadata                     json.RawMessage                `json:"metadata"`
	CreatedAt                    time.Time                      `json:"created_at"`
	UpdatedAt                    time.Time                      `json:"updated_at"`
}

type listRegressionSuitesResponse struct {
	Items  []regressionSuiteResponse `json:"items"`
	Total  int64                     `json:"total"`
	Limit  int32                     `json:"limit"`
	Offset int32                     `json:"offset"`
}

type listRegressionCasesResponse struct {
	Items []regressionCaseResponse `json:"items"`
}

func createRegressionSuiteHandler(logger *slog.Logger, service RegressionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace ID is malformed")
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		var req struct {
			SourceChallengePackID uuid.UUID `json:"source_challenge_pack_id"`
			Name                  string    `json:"name"`
			Description           string    `json:"description"`
			DefaultGateSeverity   *string   `json:"default_gate_severity,omitempty"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "name is required")
			return
		}
		if req.SourceChallengePackID == uuid.Nil {
			writeError(w, http.StatusBadRequest, "validation_error", "source_challenge_pack_id is required")
			return
		}

		defaultGateSeverity := domain.RegressionSeverityWarning
		if req.DefaultGateSeverity != nil {
			parsed, parseErr := domain.ParseRegressionSeverity(*req.DefaultGateSeverity)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "default_gate_severity must be info, warning, or blocking")
				return
			}
			defaultGateSeverity = parsed
		}

		suite, err := service.CreateRegressionSuite(r.Context(), caller, CreateRegressionSuiteInput{
			WorkspaceID:           workspaceID,
			SourceChallengePackID: req.SourceChallengePackID,
			Name:                  req.Name,
			Description:           req.Description,
			DefaultGateSeverity:   defaultGateSeverity,
		})
		if err != nil {
			handleRegressionError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusCreated, buildRegressionSuiteResponse(suite))
	}
}

func listRegressionSuitesHandler(logger *slog.Logger, service RegressionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace ID is malformed")
			return
		}

		limit := int32(50)
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, parseErr := strconv.Atoi(raw)
			if parseErr != nil || parsed <= 0 {
				writeError(w, http.StatusBadRequest, "validation_error", "limit must be a positive integer")
				return
			}
			limit = int32(parsed)
		}
		if limit > 100 {
			limit = 100
		}
		offset := int32(0)
		if raw := r.URL.Query().Get("offset"); raw != "" {
			parsed, parseErr := strconv.Atoi(raw)
			if parseErr != nil || parsed < 0 {
				writeError(w, http.StatusBadRequest, "validation_error", "offset must be a non-negative integer")
				return
			}
			offset = int32(parsed)
		}

		result, err := service.ListRegressionSuites(r.Context(), caller, ListRegressionSuitesInput{
			WorkspaceID: workspaceID,
			Limit:       limit,
			Offset:      offset,
		})
		if err != nil {
			handleRegressionError(w, logger, err)
			return
		}

		items := make([]regressionSuiteResponse, 0, len(result.Items))
		for _, suite := range result.Items {
			items = append(items, buildRegressionSuiteResponse(suite))
		}
		writeJSON(w, http.StatusOK, listRegressionSuitesResponse{
			Items:  items,
			Total:  result.Total,
			Limit:  result.Limit,
			Offset: result.Offset,
		})
	}
}

func getRegressionSuiteHandler(logger *slog.Logger, service RegressionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace ID is malformed")
			return
		}
		suiteID, err := regressionSuiteIDFromURLParam("suiteID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_regression_suite_id", err.Error())
			return
		}

		suite, err := service.GetRegressionSuite(r.Context(), caller, GetRegressionSuiteInput{
			WorkspaceID: workspaceID,
			SuiteID:     suiteID,
		})
		if err != nil {
			handleRegressionError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, buildRegressionSuiteResponse(suite))
	}
}

func patchRegressionSuiteHandler(logger *slog.Logger, service RegressionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace ID is malformed")
			return
		}
		suiteID, err := regressionSuiteIDFromURLParam("suiteID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_regression_suite_id", err.Error())
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		var req struct {
			Name                *string `json:"name,omitempty"`
			Description         *string `json:"description,omitempty"`
			Status              *string `json:"status,omitempty"`
			DefaultGateSeverity *string `json:"default_gate_severity,omitempty"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		if req.Name == nil && req.Description == nil && req.Status == nil && req.DefaultGateSeverity == nil {
			writeError(w, http.StatusBadRequest, "validation_error", "at least one field must be provided")
			return
		}

		var status *domain.RegressionSuiteStatus
		if req.Status != nil {
			parsed, parseErr := domain.ParseRegressionSuiteStatus(*req.Status)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "status must be active or archived")
				return
			}
			status = &parsed
		}

		var defaultGateSeverity *domain.RegressionSeverity
		if req.DefaultGateSeverity != nil {
			parsed, parseErr := domain.ParseRegressionSeverity(*req.DefaultGateSeverity)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "default_gate_severity must be info, warning, or blocking")
				return
			}
			defaultGateSeverity = &parsed
		}

		suite, err := service.PatchRegressionSuite(r.Context(), caller, PatchRegressionSuiteInput{
			WorkspaceID:         workspaceID,
			SuiteID:             suiteID,
			Name:                req.Name,
			Description:         req.Description,
			Status:              status,
			DefaultGateSeverity: defaultGateSeverity,
		})
		if err != nil {
			handleRegressionError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, buildRegressionSuiteResponse(suite))
	}
}

func listRegressionCasesHandler(logger *slog.Logger, service RegressionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace ID is malformed")
			return
		}
		suiteID, err := regressionSuiteIDFromURLParam("suiteID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_regression_suite_id", err.Error())
			return
		}

		cases, err := service.ListRegressionCases(r.Context(), caller, ListRegressionCasesInput{
			WorkspaceID: workspaceID,
			SuiteID:     suiteID,
		})
		if err != nil {
			handleRegressionError(w, logger, err)
			return
		}

		items := make([]regressionCaseResponse, 0, len(cases))
		for _, regressionCase := range cases {
			items = append(items, buildRegressionCaseResponse(regressionCase))
		}
		writeJSON(w, http.StatusOK, listRegressionCasesResponse{Items: items})
	}
}

func patchRegressionCaseHandler(logger *slog.Logger, service RegressionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace ID is malformed")
			return
		}
		caseID, err := regressionCaseIDFromURLParam("caseID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_regression_case_id", err.Error())
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		var req struct {
			Title       *string `json:"title,omitempty"`
			Description *string `json:"description,omitempty"`
			Status      *string `json:"status,omitempty"`
			Severity    *string `json:"severity,omitempty"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		if req.Title == nil && req.Description == nil && req.Status == nil && req.Severity == nil {
			writeError(w, http.StatusBadRequest, "validation_error", "at least one field must be provided")
			return
		}

		var status *domain.RegressionCaseStatus
		if req.Status != nil {
			parsed, parseErr := domain.ParseRegressionCaseStatus(*req.Status)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "status must be active, muted, or archived")
				return
			}
			status = &parsed
		}

		var severity *domain.RegressionSeverity
		if req.Severity != nil {
			parsed, parseErr := domain.ParseRegressionSeverity(*req.Severity)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "severity must be info, warning, or blocking")
				return
			}
			severity = &parsed
		}

		regressionCase, err := service.PatchRegressionCase(r.Context(), caller, PatchRegressionCaseInput{
			WorkspaceID: workspaceID,
			CaseID:      caseID,
			Title:       req.Title,
			Description: req.Description,
			Status:      status,
			Severity:    severity,
		})
		if err != nil {
			handleRegressionError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, buildRegressionCaseResponse(regressionCase))
	}
}

func buildRegressionSuiteResponse(suite repository.RegressionSuite) regressionSuiteResponse {
	return regressionSuiteResponse{
		ID:                    suite.ID,
		WorkspaceID:           suite.WorkspaceID,
		SourceChallengePackID: suite.SourceChallengePackID,
		Name:                  suite.Name,
		Description:           suite.Description,
		Status:                suite.Status,
		SourceMode:            suite.SourceMode,
		DefaultGateSeverity:   suite.DefaultGateSeverity,
		CreatedByUserID:       suite.CreatedByUserID,
		CreatedAt:             suite.CreatedAt,
		UpdatedAt:             suite.UpdatedAt,
	}
}

func buildRegressionCaseResponse(regressionCase repository.RegressionCase) regressionCaseResponse {
	return regressionCaseResponse{
		ID:                           regressionCase.ID,
		SuiteID:                      regressionCase.SuiteID,
		WorkspaceID:                  regressionCase.WorkspaceID,
		Title:                        regressionCase.Title,
		Description:                  regressionCase.Description,
		Status:                       regressionCase.Status,
		Severity:                     regressionCase.Severity,
		PromotionMode:                regressionCase.PromotionMode,
		SourceRunID:                  regressionCase.SourceRunID,
		SourceRunAgentID:             regressionCase.SourceRunAgentID,
		SourceReplayID:               regressionCase.SourceReplayID,
		SourceChallengePackVersionID: regressionCase.SourceChallengePackVersionID,
		SourceChallengeInputSetID:    regressionCase.SourceChallengeInputSetID,
		SourceChallengeIdentityID:    regressionCase.SourceChallengeIdentityID,
		SourceCaseKey:                regressionCase.SourceCaseKey,
		SourceItemKey:                regressionCase.SourceItemKey,
		EvidenceTier:                 regressionCase.EvidenceTier,
		FailureClass:                 regressionCase.FailureClass,
		FailureSummary:               regressionCase.FailureSummary,
		PayloadSnapshot:              regressionCase.PayloadSnapshot,
		ExpectedContract:             regressionCase.ExpectedContract,
		ValidatorOverrides:           regressionCase.ValidatorOverrides,
		Metadata:                     regressionCase.Metadata,
		CreatedAt:                    regressionCase.CreatedAt,
		UpdatedAt:                    regressionCase.UpdatedAt,
	}
}

func regressionSuiteIDFromURLParam(name string) func(*http.Request) (uuid.UUID, error) {
	return func(r *http.Request) (uuid.UUID, error) {
		raw := chi.URLParam(r, name)
		if raw == "" {
			return uuid.Nil, errors.New("regression suite id is required")
		}
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return uuid.Nil, errors.New("regression suite id is malformed")
		}
		return parsed, nil
	}
}

func regressionCaseIDFromURLParam(name string) func(*http.Request) (uuid.UUID, error) {
	return func(r *http.Request) (uuid.UUID, error) {
		raw := chi.URLParam(r, name)
		if raw == "" {
			return uuid.Nil, errors.New("regression case id is required")
		}
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return uuid.Nil, errors.New("regression case id is malformed")
		}
		return parsed, nil
	}
}

func handleRegressionError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
	case errors.Is(err, repository.ErrRegressionSuiteNotFound):
		writeError(w, http.StatusNotFound, "regression_suite_not_found", "regression suite not found")
	case errors.Is(err, repository.ErrRegressionCaseNotFound):
		writeError(w, http.StatusNotFound, "regression_case_not_found", "regression case not found")
	case errors.Is(err, repository.ErrRunNotFound):
		writeError(w, http.StatusNotFound, "run_not_found", "run not found")
	case errors.Is(err, ErrFailureReviewItemNotFound):
		writeError(w, http.StatusNotFound, "failure_review_item_not_found", "failure review item not found")
	case errors.Is(err, ErrChallengePackNotFound):
		writeError(w, http.StatusNotFound, "challenge_pack_not_found", "challenge pack not found")
	case errors.Is(err, repository.ErrRegressionSuiteNameConflict):
		writeError(w, http.StatusConflict, "regression_suite_name_conflict", "an active regression suite with this name already exists in the workspace")
	case errors.Is(err, repository.ErrInvalidTransition):
		writeError(w, http.StatusBadRequest, "invalid_transition", "invalid regression status transition")
	case errors.Is(err, ErrFailureReviewItemAmbiguous):
		writeError(w, http.StatusBadRequest, "failure_review_item_ambiguous", "challenge identity matches multiple failure review items")
	case errors.Is(err, ErrFailurePromotionNotAllowed):
		writeError(w, http.StatusBadRequest, "failure_not_promotable", "failure review item is not promotable")
	case errors.Is(err, ErrFailurePromotionModeUnavailable):
		writeError(w, http.StatusBadRequest, "promotion_mode_unavailable", "promotion mode is not available for this failure")
	case errors.Is(err, ErrRegressionSuiteArchived):
		writeError(w, http.StatusBadRequest, "regression_suite_archived", "regression suite must be active to accept promotions")
	case errors.Is(err, ErrRegressionSuitePackMismatch):
		writeError(w, http.StatusBadRequest, "regression_suite_pack_mismatch", "regression suite source pack must match the run source pack")
	case errors.Is(err, domain.ErrInvalidPromotionOverrides):
		writeError(w, http.StatusBadRequest, "invalid_promotion_overrides", err.Error())
	case errors.Is(err, repository.ErrTransitionConflict):
		writeError(w, http.StatusConflict, "transition_conflict", "regression status changed before the update could be applied")
	default:
		logger.Error("regression operation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
