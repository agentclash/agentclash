package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/failurereview"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/scoring"
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
	CreateRegressionCase(ctx context.Context, params repository.CreateRegressionCaseParams) (repository.RegressionCase, error)
	GetRegressionCaseByID(ctx context.Context, id uuid.UUID) (repository.RegressionCase, error)
	ListRegressionCasesBySuiteID(ctx context.Context, suiteID uuid.UUID) ([]repository.RegressionCase, error)
	ListRegressionCasesByWorkspaceID(ctx context.Context, params repository.ListRegressionCasesByWorkspaceIDParams) ([]repository.RegressionCase, error)
	CountRegressionCasesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, status *domain.RegressionCaseStatus) (int64, error)
	PatchRegressionCase(ctx context.Context, params repository.PatchRegressionCaseParams) (repository.RegressionCase, error)
	GetRunnableChallengePackVersionByID(ctx context.Context, id uuid.UUID) (repository.RunnableChallengePackVersion, error)
	GetChallengeInputSetByID(ctx context.Context, id uuid.UUID) (repository.ChallengeInputSet, error)
	ListChallengeIdentityIDsByPackVersionID(ctx context.Context, challengePackVersionID uuid.UUID) ([]uuid.UUID, error)
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
	ListWorkspaceRegressionCases(ctx context.Context, caller Caller, input ListWorkspaceRegressionCasesInput) (ListWorkspaceRegressionCasesResult, error)
	PatchRegressionCase(ctx context.Context, caller Caller, input PatchRegressionCaseInput) (repository.RegressionCase, error)
	PromoteFailure(ctx context.Context, caller Caller, input PromoteFailureInput) (PromoteFailureResult, error)
	CaptureProductionFailure(ctx context.Context, caller Caller, input CaptureProductionFailureInput) (repository.RegressionCase, error)
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

type ListWorkspaceRegressionCasesInput struct {
	WorkspaceID uuid.UUID
	Status      *domain.RegressionCaseStatus
	Limit       int32
	Offset      int32
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
	RunAgentID          *uuid.UUID
	Request             domain.PromotionRequest
}

type CaptureProductionFailureInput struct {
	WorkspaceID                  uuid.UUID
	SuiteID                      uuid.UUID
	SourceChallengePackVersionID uuid.UUID
	SourceChallengeInputSetID    *uuid.UUID
	SourceChallengeIdentityID    uuid.UUID
	SourceCaseKey                string
	SourceItemKey                *string
	Title                        string
	FailureSummary               string
	FailureClass                 string
	EvidenceTier                 string
	Severity                     *domain.RegressionSeverity
	PromotionMode                *domain.RegressionPromotionMode
	PayloadSnapshot              json.RawMessage
	ExpectedContract             json.RawMessage
	ValidatorOverrides           json.RawMessage
	Metadata                     json.RawMessage
	IncidentID                   string
	ExternalURL                  string
	Source                       string
	ObservedAt                   *time.Time
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

type ListWorkspaceRegressionCasesResult struct {
	Items  []repository.RegressionCase
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
	ErrRegressionSuitePackMismatch     = errors.New("regression suite source pack does not match failure source pack")
	ErrRegressionChallengeMismatch     = errors.New("challenge identity does not belong to challenge pack version")
	ErrRegressionInputSetMismatch      = errors.New("challenge input set does not belong to challenge pack version")
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

func (m *RegressionManager) ListWorkspaceRegressionCases(ctx context.Context, caller Caller, input ListWorkspaceRegressionCasesInput) (ListWorkspaceRegressionCasesResult, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionReadWorkspace); err != nil {
		return ListWorkspaceRegressionCasesResult{}, err
	}

	cases, err := m.repo.ListRegressionCasesByWorkspaceID(ctx, repository.ListRegressionCasesByWorkspaceIDParams{
		WorkspaceID: input.WorkspaceID,
		Status:      input.Status,
		Limit:       input.Limit,
		Offset:      input.Offset,
	})
	if err != nil {
		return ListWorkspaceRegressionCasesResult{}, err
	}
	total, err := m.repo.CountRegressionCasesByWorkspaceID(ctx, input.WorkspaceID, input.Status)
	if err != nil {
		return ListWorkspaceRegressionCasesResult{}, err
	}

	return ListWorkspaceRegressionCasesResult{
		Items:  cases,
		Total:  total,
		Limit:  input.Limit,
		Offset: input.Offset,
	}, nil
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

	item, err := m.findFailureReviewItem(ctx, input.RunID, input.ChallengeIdentityID, input.RunAgentID)
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
	status := domain.RegressionCaseStatusActive
	if input.Request.Status != nil {
		status = *input.Request.Status
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
		Status:              status,
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

func (m *RegressionManager) CaptureProductionFailure(ctx context.Context, caller Caller, input CaptureProductionFailureInput) (repository.RegressionCase, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManageRegressions); err != nil {
		return repository.RegressionCase{}, err
	}

	suite, err := m.repo.GetRegressionSuiteByID(ctx, input.SuiteID)
	if err != nil {
		return repository.RegressionCase{}, err
	}
	if suite.WorkspaceID != input.WorkspaceID {
		return repository.RegressionCase{}, repository.ErrRegressionSuiteNotFound
	}
	if suite.Status != domain.RegressionSuiteStatusActive {
		return repository.RegressionCase{}, ErrRegressionSuiteArchived
	}

	version, err := m.repo.GetRunnableChallengePackVersionByID(ctx, input.SourceChallengePackVersionID)
	if err != nil {
		return repository.RegressionCase{}, err
	}
	if version.WorkspaceID != nil && *version.WorkspaceID != input.WorkspaceID {
		return repository.RegressionCase{}, repository.ErrChallengePackVersionNotFound
	}
	if version.ChallengePackID != suite.SourceChallengePackID {
		return repository.RegressionCase{}, ErrRegressionSuitePackMismatch
	}
	if input.SourceChallengeInputSetID != nil {
		inputSet, err := m.repo.GetChallengeInputSetByID(ctx, *input.SourceChallengeInputSetID)
		if err != nil {
			return repository.RegressionCase{}, err
		}
		if inputSet.ChallengePackVersionID != input.SourceChallengePackVersionID {
			return repository.RegressionCase{}, ErrRegressionInputSetMismatch
		}
	}
	if err := m.ensureChallengeIdentityInVersion(ctx, input.SourceChallengePackVersionID, input.SourceChallengeIdentityID); err != nil {
		return repository.RegressionCase{}, err
	}

	failureClass := strings.TrimSpace(input.FailureClass)
	if failureClass == "" {
		failureClass = string(failurereview.FailureClassOther)
	}
	evidenceTier := strings.TrimSpace(input.EvidenceTier)
	if evidenceTier == "" {
		evidenceTier = string(failurereview.EvidenceTierHostedBlackBox)
	}
	severity := domain.DefaultPromotionSeverityForFailureClass(failureClass)
	if input.Severity != nil {
		severity = *input.Severity
	}
	promotionMode := domain.RegressionPromotionModeOutputOnly
	if input.PromotionMode != nil {
		promotionMode = *input.PromotionMode
	}

	metadata, err := productionFailureMetadata(input, failureClass)
	if err != nil {
		return repository.RegressionCase{}, err
	}
	sourceItemKey := ""
	if input.SourceItemKey != nil {
		sourceItemKey = *input.SourceItemKey
	}

	return m.repo.CreateRegressionCase(ctx, repository.CreateRegressionCaseParams{
		SuiteID:                      input.SuiteID,
		Title:                        strings.TrimSpace(input.Title),
		Description:                  "",
		Status:                       domain.RegressionCaseStatusProposed,
		Severity:                     severity,
		PromotionMode:                promotionMode,
		SourceRunID:                  nil,
		SourceRunAgentID:             nil,
		SourceReplayID:               nil,
		SourceChallengePackVersionID: input.SourceChallengePackVersionID,
		SourceChallengeInputSetID:    cloneUUIDPtr(input.SourceChallengeInputSetID),
		SourceChallengeIdentityID:    input.SourceChallengeIdentityID,
		SourceCaseKey:                strings.TrimSpace(input.SourceCaseKey),
		SourceItemKey:                optionalStringPtr(sourceItemKey),
		EvidenceTier:                 evidenceTier,
		FailureClass:                 failureClass,
		FailureSummary:               strings.TrimSpace(input.FailureSummary),
		PayloadSnapshot:              input.PayloadSnapshot,
		ExpectedContract:             input.ExpectedContract,
		ValidatorOverrides:           input.ValidatorOverrides,
		Metadata:                     metadata,
	})
}

func (m *RegressionManager) ensureChallengeIdentityInVersion(ctx context.Context, versionID, challengeIdentityID uuid.UUID) error {
	ids, err := m.repo.ListChallengeIdentityIDsByPackVersionID(ctx, versionID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == challengeIdentityID {
			return nil
		}
	}
	return ErrRegressionChallengeMismatch
}

func (m *RegressionManager) findFailureReviewItem(ctx context.Context, runID, challengeIdentityID uuid.UUID, runAgentID *uuid.UUID) (failurereview.Item, error) {
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
		if runAgentID != nil && item.RunAgentID != *runAgentID {
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
			SuiteID            uuid.UUID                      `json:"suite_id"`
			PromotionMode      domain.RegressionPromotionMode `json:"promotion_mode"`
			Title              string                         `json:"title"`
			FailureSummary     string                         `json:"failure_summary,omitempty"`
			Status             *domain.RegressionCaseStatus   `json:"status,omitempty"`
			Severity           domain.RegressionSeverity      `json:"severity"`
			ValidatorOverrides json.RawMessage                `json:"validator_overrides,omitempty"`
			Metadata           json.RawMessage                `json:"metadata,omitempty"`
		} `json:"request"`
		FailureReviewItem failurereview.Item `json:"failure_review_item"`
	}{FailureReviewItem: item}

	snapshot.Request.SuiteID = request.SuiteID
	snapshot.Request.PromotionMode = request.PromotionMode
	snapshot.Request.Title = request.Title
	snapshot.Request.FailureSummary = request.FailureSummary
	snapshot.Request.Status = request.Status
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

func productionFailureMetadata(input CaptureProductionFailureInput, failureClass string) (json.RawMessage, error) {
	metadata := map[string]any{}
	trimmed := strings.TrimSpace(string(input.Metadata))
	if trimmed != "" && trimmed != "null" {
		if err := json.Unmarshal([]byte(trimmed), &metadata); err != nil {
			return nil, fmt.Errorf("decode production failure metadata: %w", err)
		}
	}

	source := productionFailureSource(input.Source)
	incidentID := strings.TrimSpace(input.IncidentID)
	externalURL := strings.TrimSpace(input.ExternalURL)
	sourceCaseKey := strings.TrimSpace(input.SourceCaseKey)

	metadata["origin"] = "production_failure"
	metadata["source"] = source
	metadata["source_challenge_pack_version_id"] = input.SourceChallengePackVersionID.String()
	metadata["source_challenge_identity_id"] = input.SourceChallengeIdentityID.String()
	metadata["source_case_key"] = sourceCaseKey
	metadata["source_failure_fingerprint"] = productionFailureFingerprint(input, failureClass)
	metadata["source_failure_cluster_key"] = productionFailureClusterKey(input, failureClass)
	if input.SourceChallengeInputSetID != nil {
		metadata["source_challenge_input_set_id"] = input.SourceChallengeInputSetID.String()
	}
	if incidentID != "" {
		metadata["production_incident_id"] = incidentID
	}
	if externalURL != "" {
		metadata["production_external_url"] = externalURL
	}
	if input.ObservedAt != nil {
		metadata["production_observed_at"] = input.ObservedAt.UTC().Format(time.RFC3339)
	}

	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal production failure metadata: %w", err)
	}
	return encoded, nil
}

func productionFailureFingerprint(input CaptureProductionFailureInput, failureClass string) string {
	incidentID := strings.TrimSpace(input.IncidentID)
	if incidentID == "" {
		incidentID = strings.TrimSpace(input.FailureSummary)
	}
	hash := sha256.Sum256([]byte(strings.Join([]string{
		input.SourceChallengePackVersionID.String(),
		input.SourceChallengeIdentityID.String(),
		strings.TrimSpace(input.SourceCaseKey),
		productionFailureSource(input.Source),
		failureClass,
		incidentID,
	}, "\x00")))
	return fmt.Sprintf("prod:%x", hash[:16])
}

func productionFailureClusterKey(input CaptureProductionFailureInput, failureClass string) string {
	hash := sha256.Sum256([]byte(strings.Join([]string{
		input.SourceChallengePackVersionID.String(),
		input.SourceChallengeIdentityID.String(),
		failureClass,
	}, "\x00")))
	return fmt.Sprintf("prod-cluster:%x", hash[:16])
}

func productionFailureSource(raw string) string {
	source := strings.TrimSpace(raw)
	if source == "" {
		return "production"
	}
	return source
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
	CaseCount             int                          `json:"case_count"`
	CreatedByUserID       uuid.UUID                    `json:"created_by_user_id"`
	CreatedAt             time.Time                    `json:"created_at"`
	UpdatedAt             time.Time                    `json:"updated_at"`
}

type regressionPromotionResponse struct {
	ID                        uuid.UUID       `json:"id"`
	WorkspaceRegressionCaseID uuid.UUID       `json:"workspace_regression_case_id"`
	SourceRunID               uuid.UUID       `json:"source_run_id"`
	SourceRunAgentID          uuid.UUID       `json:"source_run_agent_id"`
	SourceEventRefs           json.RawMessage `json:"source_event_refs"`
	PromotedByUserID          uuid.UUID       `json:"promoted_by_user_id"`
	PromotionReason           string          `json:"promotion_reason"`
	PromotionSnapshot         json.RawMessage `json:"promotion_snapshot"`
	CreatedAt                 time.Time       `json:"created_at"`
}

type regressionCaseResponse struct {
	ID                           uuid.UUID                        `json:"id"`
	SuiteID                      uuid.UUID                        `json:"suite_id"`
	WorkspaceID                  uuid.UUID                        `json:"workspace_id"`
	SuiteName                    string                           `json:"suite_name,omitempty"`
	Title                        string                           `json:"title"`
	Description                  string                           `json:"description"`
	Status                       domain.RegressionCaseStatus      `json:"status"`
	Severity                     domain.RegressionSeverity        `json:"severity"`
	PromotionMode                domain.RegressionPromotionMode   `json:"promotion_mode"`
	SourceRunID                  *uuid.UUID                       `json:"source_run_id,omitempty"`
	SourceRunAgentID             *uuid.UUID                       `json:"source_run_agent_id,omitempty"`
	SourceReplayID               *uuid.UUID                       `json:"source_replay_id,omitempty"`
	SourceChallengePackVersionID uuid.UUID                        `json:"source_challenge_pack_version_id"`
	SourceChallengeInputSetID    *uuid.UUID                       `json:"source_challenge_input_set_id,omitempty"`
	SourceChallengeIdentityID    uuid.UUID                        `json:"source_challenge_identity_id"`
	SourceChallengeKey           *string                          `json:"source_challenge_key,omitempty"`
	SourceCaseKey                string                           `json:"source_case_key"`
	SourceItemKey                *string                          `json:"source_item_key,omitempty"`
	SourceFailureFingerprint     *string                          `json:"source_failure_fingerprint,omitempty"`
	SourceFailureClusterKey      *string                          `json:"source_failure_cluster_key,omitempty"`
	EvidenceTier                 string                           `json:"evidence_tier"`
	FailureClass                 string                           `json:"failure_class"`
	FailureSummary               string                           `json:"failure_summary"`
	PayloadSnapshot              json.RawMessage                  `json:"payload_snapshot"`
	ExpectedContract             json.RawMessage                  `json:"expected_contract"`
	ValidatorOverrides           json.RawMessage                  `json:"validator_overrides,omitempty"`
	Metadata                     json.RawMessage                  `json:"metadata"`
	LatestPromotion              *regressionPromotionResponse     `json:"latest_promotion,omitempty"`
	Validation                   regressionCaseValidationResponse `json:"validation"`
	CreatedAt                    time.Time                        `json:"created_at"`
	UpdatedAt                    time.Time                        `json:"updated_at"`
}

type regressionCaseValidationResponse struct {
	Status                string     `json:"status"`
	MaintenanceStatus     string     `json:"maintenance_status"`
	RunCount              int        `json:"run_count"`
	FailureCount          int        `json:"failure_count"`
	PassCount             int        `json:"pass_count"`
	ReproductionRate      *float64   `json:"reproduction_rate,omitempty"`
	ReproductionThreshold float64    `json:"reproduction_threshold"`
	RequiredRuns          int        `json:"required_runs"`
	RemainingRuns         int        `json:"remaining_runs"`
	LastOutcome           *string    `json:"last_outcome,omitempty"`
	LastValidatedAt       *time.Time `json:"last_validated_at,omitempty"`
	RecommendedAction     string     `json:"recommended_action"`
	MaintenanceAction     string     `json:"maintenance_action"`
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

type listWorkspaceRegressionCasesResponse struct {
	Items  []regressionCaseResponse `json:"items"`
	Total  int64                    `json:"total"`
	Limit  int32                    `json:"limit"`
	Offset int32                    `json:"offset"`
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

func listWorkspaceRegressionCasesHandler(logger *slog.Logger, service RegressionService) http.HandlerFunc {
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

		var status *domain.RegressionCaseStatus
		if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
			parsed, parseErr := domain.ParseRegressionCaseStatus(raw)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "status must be proposed, active, muted, archived, or rejected")
				return
			}
			status = &parsed
		}

		result, err := service.ListWorkspaceRegressionCases(r.Context(), caller, ListWorkspaceRegressionCasesInput{
			WorkspaceID: workspaceID,
			Status:      status,
			Limit:       limit,
			Offset:      offset,
		})
		if err != nil {
			handleRegressionError(w, logger, err)
			return
		}

		items := make([]regressionCaseResponse, 0, len(result.Items))
		for _, regressionCase := range result.Items {
			items = append(items, buildRegressionCaseResponse(regressionCase))
		}
		writeJSON(w, http.StatusOK, listWorkspaceRegressionCasesResponse{
			Items:  items,
			Total:  result.Total,
			Limit:  result.Limit,
			Offset: result.Offset,
		})
	}
}

func captureProductionFailureHandler(logger *slog.Logger, service RegressionService) http.HandlerFunc {
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
			SourceChallengePackVersionID uuid.UUID       `json:"source_challenge_pack_version_id"`
			SourceChallengeInputSetID    *uuid.UUID      `json:"source_challenge_input_set_id,omitempty"`
			SourceChallengeIdentityID    uuid.UUID       `json:"source_challenge_identity_id"`
			SourceCaseKey                string          `json:"source_case_key"`
			SourceItemKey                *string         `json:"source_item_key,omitempty"`
			Title                        string          `json:"title"`
			FailureSummary               string          `json:"failure_summary"`
			FailureClass                 string          `json:"failure_class,omitempty"`
			EvidenceTier                 string          `json:"evidence_tier,omitempty"`
			Severity                     *string         `json:"severity,omitempty"`
			PromotionMode                *string         `json:"promotion_mode,omitempty"`
			PayloadSnapshot              json.RawMessage `json:"payload_snapshot"`
			ExpectedContract             json.RawMessage `json:"expected_contract,omitempty"`
			ValidatorOverrides           json.RawMessage `json:"validator_overrides,omitempty"`
			Metadata                     json.RawMessage `json:"metadata,omitempty"`
			IncidentID                   string          `json:"incident_id,omitempty"`
			ExternalURL                  string          `json:"external_url,omitempty"`
			Source                       string          `json:"source,omitempty"`
			ObservedAt                   *time.Time      `json:"observed_at,omitempty"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		if req.SourceChallengePackVersionID == uuid.Nil {
			writeError(w, http.StatusBadRequest, "validation_error", "source_challenge_pack_version_id is required")
			return
		}
		if req.SourceChallengeIdentityID == uuid.Nil {
			writeError(w, http.StatusBadRequest, "validation_error", "source_challenge_identity_id is required")
			return
		}
		if strings.TrimSpace(req.SourceCaseKey) == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "source_case_key is required")
			return
		}
		if strings.TrimSpace(req.Title) == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "title is required")
			return
		}
		if strings.TrimSpace(req.FailureSummary) == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "failure_summary is required")
			return
		}
		failureClass := strings.TrimSpace(req.FailureClass)
		if failureClass != "" {
			parsed, parseErr := parseFailureClass(failureClass)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, "validation_error", parseErr.Error())
				return
			}
			failureClass = string(parsed)
		}
		evidenceTier := strings.TrimSpace(req.EvidenceTier)
		if evidenceTier != "" {
			parsed, parseErr := parseEvidenceTier(evidenceTier)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, "validation_error", parseErr.Error())
				return
			}
			evidenceTier = string(parsed)
		}

		payloadSnapshot, ok := regressionJSONObject(w, req.PayloadSnapshot, "payload_snapshot", true)
		if !ok {
			return
		}
		expectedContract, ok := regressionJSONObject(w, req.ExpectedContract, "expected_contract", false)
		if !ok {
			return
		}
		metadata, ok := regressionJSONObject(w, req.Metadata, "metadata", false)
		if !ok {
			return
		}
		validatorOverrides, err := domain.ValidatePromotionOverrides(req.ValidatorOverrides)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_promotion_overrides", err.Error())
			return
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

		var promotionMode *domain.RegressionPromotionMode
		if req.PromotionMode != nil {
			parsed, parseErr := domain.ParseRegressionPromotionMode(*req.PromotionMode)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "promotion_mode must be full_executable, output_only, or manual")
				return
			}
			promotionMode = &parsed
		}

		regressionCase, err := service.CaptureProductionFailure(r.Context(), caller, CaptureProductionFailureInput{
			WorkspaceID:                  workspaceID,
			SuiteID:                      suiteID,
			SourceChallengePackVersionID: req.SourceChallengePackVersionID,
			SourceChallengeInputSetID:    cloneUUIDPtr(req.SourceChallengeInputSetID),
			SourceChallengeIdentityID:    req.SourceChallengeIdentityID,
			SourceCaseKey:                req.SourceCaseKey,
			SourceItemKey:                cloneStringPtr(req.SourceItemKey),
			Title:                        req.Title,
			FailureSummary:               req.FailureSummary,
			FailureClass:                 failureClass,
			EvidenceTier:                 evidenceTier,
			Severity:                     severity,
			PromotionMode:                promotionMode,
			PayloadSnapshot:              payloadSnapshot,
			ExpectedContract:             expectedContract,
			ValidatorOverrides:           validatorOverrides,
			Metadata:                     metadata,
			IncidentID:                   req.IncidentID,
			ExternalURL:                  req.ExternalURL,
			Source:                       req.Source,
			ObservedAt:                   req.ObservedAt,
		})
		if err != nil {
			handleRegressionError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, buildRegressionCaseResponse(regressionCase))
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
				writeError(w, http.StatusBadRequest, "validation_error", "status must be proposed, active, muted, archived, or rejected")
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

func regressionJSONObject(w http.ResponseWriter, raw json.RawMessage, field string, required bool) (json.RawMessage, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		if required {
			writeError(w, http.StatusBadRequest, "validation_error", field+" is required")
			return nil, false
		}
		return nil, true
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", field+" must be a JSON object")
		return nil, false
	}
	return json.RawMessage(trimmed), true
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
		CaseCount:             suite.CaseCount,
		CreatedByUserID:       suite.CreatedByUserID,
		CreatedAt:             suite.CreatedAt,
		UpdatedAt:             suite.UpdatedAt,
	}
}

func buildRegressionCaseResponse(regressionCase repository.RegressionCase) regressionCaseResponse {
	var latestPromotion *regressionPromotionResponse
	if regressionCase.LatestPromotion != nil {
		latestPromotion = &regressionPromotionResponse{
			ID:                        regressionCase.LatestPromotion.ID,
			WorkspaceRegressionCaseID: regressionCase.LatestPromotion.WorkspaceRegressionCaseID,
			SourceRunID:               regressionCase.LatestPromotion.SourceRunID,
			SourceRunAgentID:          regressionCase.LatestPromotion.SourceRunAgentID,
			SourceEventRefs:           regressionCase.LatestPromotion.SourceEventRefs,
			PromotedByUserID:          regressionCase.LatestPromotion.PromotedByUserID,
			PromotionReason:           regressionCase.LatestPromotion.PromotionReason,
			PromotionSnapshot:         regressionCase.LatestPromotion.PromotionSnapshot,
			CreatedAt:                 regressionCase.LatestPromotion.CreatedAt,
		}
	}
	provenance := regressionFailureProvenanceFromMetadata(regressionCase.Metadata)

	return regressionCaseResponse{
		ID:                           regressionCase.ID,
		SuiteID:                      regressionCase.SuiteID,
		WorkspaceID:                  regressionCase.WorkspaceID,
		SuiteName:                    regressionCase.SuiteName,
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
		SourceChallengeKey:           provenance.SourceChallengeKey,
		SourceCaseKey:                regressionCase.SourceCaseKey,
		SourceItemKey:                regressionCase.SourceItemKey,
		SourceFailureFingerprint:     provenance.SourceFailureFingerprint,
		SourceFailureClusterKey:      provenance.SourceFailureClusterKey,
		EvidenceTier:                 regressionCase.EvidenceTier,
		FailureClass:                 regressionCase.FailureClass,
		FailureSummary:               regressionCase.FailureSummary,
		PayloadSnapshot:              regressionCase.PayloadSnapshot,
		ExpectedContract:             regressionCase.ExpectedContract,
		ValidatorOverrides:           regressionCase.ValidatorOverrides,
		Metadata:                     regressionCase.Metadata,
		LatestPromotion:              latestPromotion,
		Validation:                   buildRegressionCaseValidationResponse(regressionCase.ValidationStats),
		CreatedAt:                    regressionCase.CreatedAt,
		UpdatedAt:                    regressionCase.UpdatedAt,
	}
}

const (
	regressionValidationRequiredRuns          = 5
	regressionValidationReproductionThreshold = 0.6

	regressionValidationStatusNotValidated     = "not_validated"
	regressionValidationStatusCollectingSignal = "collecting_signal"
	regressionValidationStatusReproducing      = "reproducing"
	regressionValidationStatusPassing          = "passing"
	regressionValidationStatusFlaky            = "flaky"

	regressionMaintenanceStatusNeedsSignal    = "needs_signal"
	regressionMaintenanceStatusKeepActive     = "keep_active"
	regressionMaintenanceStatusPruneCandidate = "prune_candidate"
	regressionMaintenanceStatusReviewFlaky    = "review_flaky"
)

func buildRegressionCaseValidationResponse(stats *repository.RegressionCaseValidationStats) regressionCaseValidationResponse {
	response := regressionCaseValidationResponse{
		Status:                regressionValidationStatusNotValidated,
		MaintenanceStatus:     regressionMaintenanceStatusNeedsSignal,
		ReproductionThreshold: regressionValidationReproductionThreshold,
		RequiredRuns:          regressionValidationRequiredRuns,
		RemainingRuns:         regressionValidationRequiredRuns,
		RecommendedAction:     "Run this regression case in CI or suite-only mode to establish a reproduction signal.",
		MaintenanceAction:     "Schedule more reruns before changing this case's suite role.",
	}
	if stats == nil || stats.RunCount == 0 {
		return response
	}

	response.RunCount = stats.RunCount
	response.FailureCount = stats.FailureCount
	response.PassCount = stats.PassCount
	response.RemainingRuns = max(regressionValidationRequiredRuns-stats.RunCount, 0)
	reproductionRate := stats.ReproductionRate
	response.ReproductionRate = &reproductionRate
	if strings.TrimSpace(stats.LastOutcome) != "" {
		outcome := stats.LastOutcome
		response.LastOutcome = &outcome
	}
	response.LastValidatedAt = stats.LastValidatedAt

	switch {
	case stats.RunCount < regressionValidationRequiredRuns:
		response.Status = regressionValidationStatusCollectingSignal
		response.MaintenanceStatus = regressionMaintenanceStatusNeedsSignal
		response.RecommendedAction = fmt.Sprintf("Collect %d more scored run(s) before treating this case as statistically validated.", response.RemainingRuns)
		response.MaintenanceAction = "Keep this case in evidence-gathering mode until the validation window is full."
	case stats.ReproductionRate >= regressionValidationReproductionThreshold:
		response.Status = regressionValidationStatusReproducing
		response.MaintenanceStatus = regressionMaintenanceStatusKeepActive
		response.RecommendedAction = "Failure reproduces at or above threshold; keep this case active in CI gates."
		response.MaintenanceAction = "Leave this case in the active gate set."
	case stats.FailureCount == 0:
		response.Status = regressionValidationStatusPassing
		response.MaintenanceStatus = regressionMaintenanceStatusPruneCandidate
		response.RecommendedAction = "No failures observed across the validation window; keep as a guardrail or archive if obsolete."
		response.MaintenanceAction = "Open a pruning review before archiving or downgrading it."
	default:
		response.Status = regressionValidationStatusFlaky
		response.MaintenanceStatus = regressionMaintenanceStatusReviewFlaky
		response.RecommendedAction = "Mixed outcomes are below the reproduction threshold; inspect replay evidence before making this case blocking."
		response.MaintenanceAction = "Rewrite, split, or mute this case if replay evidence is nondeterministic."
	}
	return response
}

type regressionFailureProvenance struct {
	SourceChallengeKey       *string
	SourceFailureFingerprint *string
	SourceFailureClusterKey  *string
}

func regressionFailureProvenanceFromMetadata(metadata json.RawMessage) regressionFailureProvenance {
	values := map[string]any{}
	if len(metadata) == 0 {
		return regressionFailureProvenance{}
	}
	if err := json.Unmarshal(metadata, &values); err != nil {
		return regressionFailureProvenance{}
	}
	return regressionFailureProvenance{
		SourceChallengeKey:       optionalMetadataString(values, "source_challenge_key"),
		SourceFailureFingerprint: optionalMetadataString(values, "source_failure_fingerprint"),
		SourceFailureClusterKey:  optionalMetadataString(values, "source_failure_cluster_key"),
	}
}

func optionalMetadataString(values map[string]any, key string) *string {
	value, ok := values[key].(string)
	if !ok {
		return nil
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
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
	case errors.Is(err, repository.ErrChallengePackVersionNotFound):
		writeError(w, http.StatusNotFound, "challenge_pack_version_not_found", "challenge pack version not found")
	case errors.Is(err, repository.ErrChallengeInputSetNotFound):
		writeError(w, http.StatusNotFound, "challenge_input_set_not_found", "challenge input set not found")
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
		writeError(w, http.StatusBadRequest, "regression_suite_pack_mismatch", "regression suite source pack must match the failure source pack")
	case errors.Is(err, ErrRegressionChallengeMismatch):
		writeError(w, http.StatusBadRequest, "challenge_identity_mismatch", "challenge identity must belong to the challenge pack version")
	case errors.Is(err, ErrRegressionInputSetMismatch):
		writeError(w, http.StatusBadRequest, "challenge_input_set_mismatch", "challenge input set must belong to the challenge pack version")
	case errors.Is(err, domain.ErrInvalidPromotionOverrides):
		writeError(w, http.StatusBadRequest, "invalid_promotion_overrides", err.Error())
	case errors.Is(err, repository.ErrTransitionConflict):
		writeError(w, http.StatusConflict, "transition_conflict", "regression status changed before the update could be applied")
	default:
		logger.Error("regression operation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
