package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type RegressionSuite struct {
	ID                    uuid.UUID
	WorkspaceID           uuid.UUID
	SourceChallengePackID uuid.UUID
	Name                  string
	Description           string
	Status                domain.RegressionSuiteStatus
	SourceMode            string
	DefaultGateSeverity   domain.RegressionSeverity
	CaseCount             int
	CreatedByUserID       uuid.UUID
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type RegressionCase struct {
	ID                           uuid.UUID
	SuiteID                      uuid.UUID
	WorkspaceID                  uuid.UUID
	Title                        string
	Description                  string
	Status                       domain.RegressionCaseStatus
	Severity                     domain.RegressionSeverity
	PromotionMode                domain.RegressionPromotionMode
	SourceRunID                  *uuid.UUID
	SourceRunAgentID             *uuid.UUID
	SourceReplayID               *uuid.UUID
	SourceChallengePackVersionID uuid.UUID
	SourceChallengeInputSetID    *uuid.UUID
	SourceChallengeIdentityID    uuid.UUID
	SourceCaseKey                string
	SourceItemKey                *string
	EvidenceTier                 string
	FailureClass                 string
	FailureSummary               string
	PayloadSnapshot              json.RawMessage
	ExpectedContract             json.RawMessage
	ValidatorOverrides           json.RawMessage
	Metadata                     json.RawMessage
	LatestPromotion              *RegressionPromotion
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}

type RegressionPromotion struct {
	ID                        uuid.UUID
	WorkspaceRegressionCaseID uuid.UUID
	SourceRunID               uuid.UUID
	SourceRunAgentID          uuid.UUID
	SourceEventRefs           json.RawMessage
	PromotedByUserID          uuid.UUID
	PromotionReason           string
	PromotionSnapshot         json.RawMessage
	CreatedAt                 time.Time
}

const regressionSuiteActiveNameIndex = "workspace_regression_suites_workspace_name_active_idx"
const regressionPromotionUniquenessIndex = "workspace_regression_cases_suite_run_agent_challenge_idx"

type CreateRegressionSuiteParams struct {
	WorkspaceID           uuid.UUID
	SourceChallengePackID uuid.UUID
	Name                  string
	Description           string
	Status                domain.RegressionSuiteStatus
	SourceMode            string
	DefaultGateSeverity   domain.RegressionSeverity
	CreatedByUserID       uuid.UUID
}

type PatchRegressionSuiteParams struct {
	ID                  uuid.UUID
	Name                *string
	Description         *string
	Status              *domain.RegressionSuiteStatus
	DefaultGateSeverity *domain.RegressionSeverity
}

type CreateRegressionCaseParams struct {
	SuiteID                      uuid.UUID
	Title                        string
	Description                  string
	Status                       domain.RegressionCaseStatus
	Severity                     domain.RegressionSeverity
	PromotionMode                domain.RegressionPromotionMode
	SourceRunID                  *uuid.UUID
	SourceRunAgentID             *uuid.UUID
	SourceReplayID               *uuid.UUID
	SourceChallengePackVersionID uuid.UUID
	SourceChallengeInputSetID    *uuid.UUID
	SourceChallengeIdentityID    uuid.UUID
	SourceCaseKey                string
	SourceItemKey                *string
	EvidenceTier                 string
	FailureClass                 string
	FailureSummary               string
	PayloadSnapshot              json.RawMessage
	ExpectedContract             json.RawMessage
	ValidatorOverrides           json.RawMessage
	Metadata                     json.RawMessage
}

type PatchRegressionCaseParams struct {
	ID          uuid.UUID
	Title       *string
	Description *string
	Status      *domain.RegressionCaseStatus
	Severity    *domain.RegressionSeverity
}

type CreateRegressionPromotionParams struct {
	WorkspaceRegressionCaseID uuid.UUID
	SourceRunID               uuid.UUID
	SourceRunAgentID          uuid.UUID
	SourceEventRefs           json.RawMessage
	PromotedByUserID          uuid.UUID
	PromotionReason           string
	PromotionSnapshot         json.RawMessage
}

type PromoteFailureParams struct {
	SuiteID             uuid.UUID
	RunID               uuid.UUID
	RunAgentID          uuid.UUID
	ChallengeIdentityID uuid.UUID
	Title               string
	FailureSummary      string
	Severity            domain.RegressionSeverity
	PromotionMode       domain.RegressionPromotionMode
	FailureClass        string
	EvidenceTier        string
	SourceCaseKey       string
	SourceItemKey       *string
	ExpectedContract    json.RawMessage
	ValidatorOverrides  json.RawMessage
	Metadata            json.RawMessage
	SourceEventRefs     json.RawMessage
	PromotionSnapshot   json.RawMessage
	PromotedByUserID    uuid.UUID
}

type PromoteFailureResult struct {
	Case    RegressionCase
	Created bool
}

func (r *Repository) CreateRegressionSuite(ctx context.Context, params CreateRegressionSuiteParams) (RegressionSuite, error) {
	if !params.Status.Valid() {
		return RegressionSuite{}, fmt.Errorf("%w: %q", domain.ErrInvalidRegressionSuiteStatus, params.Status)
	}
	if !params.DefaultGateSeverity.Valid() {
		return RegressionSuite{}, fmt.Errorf("%w: %q", domain.ErrInvalidRegressionSeverity, params.DefaultGateSeverity)
	}

	row, err := r.queries.CreateRegressionSuite(ctx, repositorysqlc.CreateRegressionSuiteParams{
		WorkspaceID:           params.WorkspaceID,
		SourceChallengePackID: params.SourceChallengePackID,
		Name:                  strings.TrimSpace(params.Name),
		Description:           params.Description,
		Status:                string(params.Status),
		SourceMode:            params.SourceMode,
		DefaultGateSeverity:   string(params.DefaultGateSeverity),
		CreatedByUserID:       params.CreatedByUserID,
	})
	if err != nil {
		if isRegressionSuiteNameConflict(err) {
			return RegressionSuite{}, ErrRegressionSuiteNameConflict
		}
		return RegressionSuite{}, fmt.Errorf("create regression suite: %w", err)
	}

	suite, err := mapRegressionSuite(row)
	if err != nil {
		return RegressionSuite{}, fmt.Errorf("map regression suite: %w", err)
	}
	return suite, nil
}

func (r *Repository) GetRegressionSuiteByID(ctx context.Context, id uuid.UUID) (RegressionSuite, error) {
	row, err := r.queries.GetRegressionSuiteByID(ctx, repositorysqlc.GetRegressionSuiteByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RegressionSuite{}, ErrRegressionSuiteNotFound
		}
		return RegressionSuite{}, fmt.Errorf("get regression suite by id: %w", err)
	}

	suite, err := mapRegressionSuite(row)
	if err != nil {
		return RegressionSuite{}, fmt.Errorf("map regression suite: %w", err)
	}
	suite.CaseCount, err = r.countRegressionCasesBySuiteID(ctx, suite.ID)
	if err != nil {
		return RegressionSuite{}, err
	}
	return suite, nil
}

func (r *Repository) ListRegressionSuitesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) ([]RegressionSuite, error) {
	rows, err := r.queries.ListRegressionSuitesByWorkspaceID(ctx, repositorysqlc.ListRegressionSuitesByWorkspaceIDParams{
		WorkspaceID:  workspaceID,
		ResultLimit:  limit,
		ResultOffset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list regression suites by workspace id: %w", err)
	}

	suites := make([]RegressionSuite, 0, len(rows))
	for _, row := range rows {
		suite, mapErr := mapRegressionSuite(row)
		if mapErr != nil {
			return nil, fmt.Errorf("map regression suite %s: %w", row.ID, mapErr)
		}
		suite.CaseCount, mapErr = r.countRegressionCasesBySuiteID(ctx, suite.ID)
		if mapErr != nil {
			return nil, mapErr
		}
		suites = append(suites, suite)
	}
	return suites, nil
}

func (r *Repository) CountRegressionSuitesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (int64, error) {
	count, err := r.queries.CountRegressionSuitesByWorkspaceID(ctx, repositorysqlc.CountRegressionSuitesByWorkspaceIDParams{
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return 0, fmt.Errorf("count regression suites by workspace id: %w", err)
	}
	return count, nil
}

func (r *Repository) PatchRegressionSuite(ctx context.Context, params PatchRegressionSuiteParams) (RegressionSuite, error) {
	var (
		fromStatus *string
		toStatus   *string
	)

	if params.Status != nil {
		if !params.Status.Valid() {
			return RegressionSuite{}, fmt.Errorf("%w: %q", domain.ErrInvalidRegressionSuiteStatus, *params.Status)
		}

		current, err := r.GetRegressionSuiteByID(ctx, params.ID)
		if err != nil {
			return RegressionSuite{}, err
		}
		if !current.Status.CanTransitionTo(*params.Status) {
			return RegressionSuite{}, InvalidTransitionError{
				Entity: "regression_suite",
				From:   string(current.Status),
				To:     string(*params.Status),
			}
		}
		fromStatus = stringPtr(string(current.Status))
		toStatus = stringPtr(string(*params.Status))
	}

	var defaultGateSeverity *string
	if params.DefaultGateSeverity != nil {
		if !params.DefaultGateSeverity.Valid() {
			return RegressionSuite{}, fmt.Errorf("%w: %q", domain.ErrInvalidRegressionSeverity, *params.DefaultGateSeverity)
		}
		defaultGateSeverity = stringPtr(string(*params.DefaultGateSeverity))
	}

	row, err := r.queries.PatchRegressionSuite(ctx, repositorysqlc.PatchRegressionSuiteParams{
		Name:                cloneStringPtr(params.Name),
		Description:         cloneStringPtr(params.Description),
		ToStatus:            toStatus,
		DefaultGateSeverity: defaultGateSeverity,
		ID:                  params.ID,
		FromStatus:          fromStatus,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if params.Status != nil && toStatus != nil {
				return RegressionSuite{}, TransitionConflictError{
					Entity:   "regression_suite",
					ID:       params.ID,
					Expected: fmt.Sprint(fromStatus),
				}
			}
			return RegressionSuite{}, ErrRegressionSuiteNotFound
		}
		if isRegressionSuiteNameConflict(err) {
			return RegressionSuite{}, ErrRegressionSuiteNameConflict
		}
		return RegressionSuite{}, fmt.Errorf("patch regression suite: %w", err)
	}

	suite, err := mapRegressionSuite(row)
	if err != nil {
		return RegressionSuite{}, fmt.Errorf("map regression suite: %w", err)
	}
	suite.CaseCount, err = r.countRegressionCasesBySuiteID(ctx, suite.ID)
	if err != nil {
		return RegressionSuite{}, err
	}
	return suite, nil
}

func (r *Repository) CreateRegressionCase(ctx context.Context, params CreateRegressionCaseParams) (RegressionCase, error) {
	if !params.Status.Valid() {
		return RegressionCase{}, fmt.Errorf("%w: %q", domain.ErrInvalidRegressionCaseStatus, params.Status)
	}
	if !params.Severity.Valid() {
		return RegressionCase{}, fmt.Errorf("%w: %q", domain.ErrInvalidRegressionSeverity, params.Severity)
	}
	if !params.PromotionMode.Valid() {
		return RegressionCase{}, fmt.Errorf("%w: %q", domain.ErrInvalidPromotionMode, params.PromotionMode)
	}

	row, err := r.queries.CreateRegressionCase(ctx, repositorysqlc.CreateRegressionCaseParams{
		SuiteID:                      params.SuiteID,
		Title:                        strings.TrimSpace(params.Title),
		Description:                  params.Description,
		Status:                       string(params.Status),
		Severity:                     string(params.Severity),
		PromotionMode:                string(params.PromotionMode),
		SourceRunID:                  cloneUUIDPtr(params.SourceRunID),
		SourceRunAgentID:             cloneUUIDPtr(params.SourceRunAgentID),
		SourceReplayID:               cloneUUIDPtr(params.SourceReplayID),
		SourceChallengePackVersionID: params.SourceChallengePackVersionID,
		SourceChallengeInputSetID:    cloneUUIDPtr(params.SourceChallengeInputSetID),
		SourceChallengeIdentityID:    params.SourceChallengeIdentityID,
		SourceCaseKey:                params.SourceCaseKey,
		SourceItemKey:                cloneStringPtr(params.SourceItemKey),
		EvidenceTier:                 params.EvidenceTier,
		FailureClass:                 params.FailureClass,
		FailureSummary:               params.FailureSummary,
		PayloadSnapshot:              normalizeJSONObject(params.PayloadSnapshot),
		ExpectedContract:             normalizeJSONObject(params.ExpectedContract),
		ValidatorOverrides:           cloneJSON(params.ValidatorOverrides),
		Metadata:                     normalizeJSONObject(params.Metadata),
	})
	if err != nil {
		return RegressionCase{}, fmt.Errorf("create regression case: %w", err)
	}

	created, err := mapRegressionCaseFromTableRowPartial(row)
	if err != nil {
		return RegressionCase{}, fmt.Errorf("map regression case: %w", err)
	}
	// The insert row does not include workspace_id; re-read through the joined
	// query so callers always receive a fully populated case record.
	return r.GetRegressionCaseByID(ctx, created.ID)
}

func (r *Repository) GetRegressionCaseByID(ctx context.Context, id uuid.UUID) (RegressionCase, error) {
	row, err := r.queries.GetRegressionCaseByID(ctx, repositorysqlc.GetRegressionCaseByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RegressionCase{}, ErrRegressionCaseNotFound
		}
		return RegressionCase{}, fmt.Errorf("get regression case by id: %w", err)
	}

	regressionCase, err := mapRegressionCaseFromJoinedRow(row)
	if err != nil {
		return RegressionCase{}, fmt.Errorf("map regression case: %w", err)
	}
	regressionCase.LatestPromotion, err = r.latestRegressionPromotionByCaseID(ctx, regressionCase.ID)
	if err != nil {
		return RegressionCase{}, err
	}
	return regressionCase, nil
}

func (r *Repository) ListRegressionCasesBySuiteID(ctx context.Context, suiteID uuid.UUID) ([]RegressionCase, error) {
	rows, err := r.queries.ListRegressionCasesBySuiteID(ctx, repositorysqlc.ListRegressionCasesBySuiteIDParams{
		SuiteID: suiteID,
	})
	if err != nil {
		return nil, fmt.Errorf("list regression cases by suite id: %w", err)
	}

	cases := make([]RegressionCase, 0, len(rows))
	for _, row := range rows {
		regressionCase, mapErr := mapRegressionCaseFromListRow(row)
		if mapErr != nil {
			return nil, fmt.Errorf("map regression case %s: %w", row.ID, mapErr)
		}
		regressionCase.LatestPromotion, mapErr = r.latestRegressionPromotionByCaseID(ctx, regressionCase.ID)
		if mapErr != nil {
			return nil, mapErr
		}
		cases = append(cases, regressionCase)
	}
	return cases, nil
}

func (r *Repository) PatchRegressionCase(ctx context.Context, params PatchRegressionCaseParams) (RegressionCase, error) {
	var (
		fromStatus *string
		toStatus   *string
	)

	if params.Status != nil {
		if !params.Status.Valid() {
			return RegressionCase{}, fmt.Errorf("%w: %q", domain.ErrInvalidRegressionCaseStatus, *params.Status)
		}

		current, err := r.GetRegressionCaseByID(ctx, params.ID)
		if err != nil {
			return RegressionCase{}, err
		}
		if !current.Status.CanTransitionTo(*params.Status) {
			return RegressionCase{}, InvalidTransitionError{
				Entity: "regression_case",
				From:   string(current.Status),
				To:     string(*params.Status),
			}
		}
		fromStatus = stringPtr(string(current.Status))
		toStatus = stringPtr(string(*params.Status))
	}

	var severity *string
	if params.Severity != nil {
		if !params.Severity.Valid() {
			return RegressionCase{}, fmt.Errorf("%w: %q", domain.ErrInvalidRegressionSeverity, *params.Severity)
		}
		severity = stringPtr(string(*params.Severity))
	}

	row, err := r.queries.PatchRegressionCase(ctx, repositorysqlc.PatchRegressionCaseParams{
		Title:       cloneStringPtr(params.Title),
		Description: cloneStringPtr(params.Description),
		ToStatus:    toStatus,
		Severity:    severity,
		ID:          params.ID,
		FromStatus:  fromStatus,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if params.Status != nil && toStatus != nil {
				return RegressionCase{}, TransitionConflictError{
					Entity:   "regression_case",
					ID:       params.ID,
					Expected: fmt.Sprint(fromStatus),
				}
			}
			return RegressionCase{}, ErrRegressionCaseNotFound
		}
		return RegressionCase{}, fmt.Errorf("patch regression case: %w", err)
	}

	updated, err := mapRegressionCaseFromTableRowPartial(row)
	if err != nil {
		return RegressionCase{}, fmt.Errorf("map regression case: %w", err)
	}
	// Patch returns the base table row shape, so fetch the joined record to
	// preserve workspace_id for downstream authz-sensitive callers.
	return r.GetRegressionCaseByID(ctx, updated.ID)
}

func (r *Repository) CreateRegressionPromotion(ctx context.Context, params CreateRegressionPromotionParams) (RegressionPromotion, error) {
	row, err := r.queries.CreateRegressionPromotion(ctx, repositorysqlc.CreateRegressionPromotionParams{
		WorkspaceRegressionCaseID: params.WorkspaceRegressionCaseID,
		SourceRunID:               params.SourceRunID,
		SourceRunAgentID:          params.SourceRunAgentID,
		SourceEventRefs:           normalizeJSONArray(params.SourceEventRefs),
		PromotedByUserID:          params.PromotedByUserID,
		PromotionReason:           params.PromotionReason,
		PromotionSnapshot:         normalizeJSONObject(params.PromotionSnapshot),
	})
	if err != nil {
		return RegressionPromotion{}, fmt.Errorf("create regression promotion: %w", err)
	}

	promotion, err := mapRegressionPromotion(row)
	if err != nil {
		return RegressionPromotion{}, fmt.Errorf("map regression promotion: %w", err)
	}
	return promotion, nil
}

func (r *Repository) PromoteFailure(ctx context.Context, params PromoteFailureParams) (PromoteFailureResult, error) {
	if !params.Severity.Valid() {
		return PromoteFailureResult{}, fmt.Errorf("%w: %q", domain.ErrInvalidRegressionSeverity, params.Severity)
	}
	if !params.PromotionMode.Valid() {
		return PromoteFailureResult{}, fmt.Errorf("%w: %q", domain.ErrInvalidPromotionMode, params.PromotionMode)
	}
	existingID, err := r.queries.GetRegressionCaseIDByPromotionSource(ctx, repositorysqlc.GetRegressionCaseIDByPromotionSourceParams{
		SuiteID:                   params.SuiteID,
		SourceRunAgentID:          &params.RunAgentID,
		SourceChallengeIdentityID: params.ChallengeIdentityID,
	})
	switch {
	case err == nil:
		existing, getErr := r.GetRegressionCaseByID(ctx, existingID)
		if getErr != nil {
			return PromoteFailureResult{}, getErr
		}
		return PromoteFailureResult{Case: existing, Created: false}, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return PromoteFailureResult{}, fmt.Errorf("lookup existing regression case: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return PromoteFailureResult{}, fmt.Errorf("begin regression promotion transaction: %w", err)
	}
	defer rollback(ctx, tx)

	txQueries := r.queries.WithTx(tx)
	executionContextRow, err := txQueries.GetRunAgentExecutionContextByID(ctx, repositorysqlc.GetRunAgentExecutionContextByIDParams{
		ID: params.RunAgentID,
	})
	if err != nil {
		return PromoteFailureResult{}, fmt.Errorf("load run agent execution context: %w", err)
	}
	executionContext, err := mapRunAgentExecutionContext(executionContextRow)
	if err != nil {
		return PromoteFailureResult{}, fmt.Errorf("map run agent execution context: %w", err)
	}

	payloadSnapshot, err := payloadSnapshotForChallenge(executionContext, params.ChallengeIdentityID)
	if err != nil {
		return PromoteFailureResult{}, err
	}

	var replayID *uuid.UUID
	replayRow, err := txQueries.GetRunAgentReplayByRunAgentID(ctx, repositorysqlc.GetRunAgentReplayByRunAgentIDParams{
		RunAgentID: params.RunAgentID,
	})
	switch {
	case err == nil:
		replayID = &replayRow.ID
	case errors.Is(err, pgx.ErrNoRows):
		replayID = nil
	default:
		return PromoteFailureResult{}, fmt.Errorf("load run agent replay: %w", err)
	}

	createdCase, err := r.createRegressionCaseWithQueries(ctx, txQueries, CreateRegressionCaseParams{
		SuiteID:                      params.SuiteID,
		Title:                        strings.TrimSpace(params.Title),
		Description:                  "",
		Status:                       domain.RegressionCaseStatusActive,
		Severity:                     params.Severity,
		PromotionMode:                params.PromotionMode,
		SourceRunID:                  &params.RunID,
		SourceRunAgentID:             &params.RunAgentID,
		SourceReplayID:               replayID,
		SourceChallengePackVersionID: executionContext.ChallengePackVersion.ID,
		SourceChallengeInputSetID:    challengeInputSetID(executionContext.ChallengeInputSet),
		SourceChallengeIdentityID:    params.ChallengeIdentityID,
		SourceCaseKey:                params.SourceCaseKey,
		SourceItemKey:                cloneStringPtr(params.SourceItemKey),
		EvidenceTier:                 params.EvidenceTier,
		FailureClass:                 params.FailureClass,
		FailureSummary:               params.FailureSummary,
		PayloadSnapshot:              payloadSnapshot,
		ExpectedContract:             cloneJSON(params.ExpectedContract),
		ValidatorOverrides:           cloneJSON(params.ValidatorOverrides),
		Metadata:                     cloneJSON(params.Metadata),
	})
	if err != nil {
		if isRegressionPromotionDuplicate(err) {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				return PromoteFailureResult{}, fmt.Errorf("rollback duplicated regression promotion transaction: %w", rollbackErr)
			}
			existingID, lookupErr := r.queries.GetRegressionCaseIDByPromotionSource(ctx, repositorysqlc.GetRegressionCaseIDByPromotionSourceParams{
				SuiteID:                   params.SuiteID,
				SourceRunAgentID:          &params.RunAgentID,
				SourceChallengeIdentityID: params.ChallengeIdentityID,
			})
			if lookupErr != nil {
				return PromoteFailureResult{}, fmt.Errorf("lookup duplicated regression case: %w", lookupErr)
			}
			existing, getErr := r.GetRegressionCaseByID(ctx, existingID)
			if getErr != nil {
				return PromoteFailureResult{}, getErr
			}
			return PromoteFailureResult{Case: existing, Created: false}, nil
		}
		return PromoteFailureResult{}, err
	}

	if _, err := r.createRegressionPromotionWithQueries(ctx, txQueries, CreateRegressionPromotionParams{
		WorkspaceRegressionCaseID: createdCase.ID,
		SourceRunID:               params.RunID,
		SourceRunAgentID:          params.RunAgentID,
		SourceEventRefs:           normalizeJSONArray(params.SourceEventRefs),
		PromotedByUserID:          params.PromotedByUserID,
		PromotionReason:           params.FailureSummary,
		PromotionSnapshot:         normalizeJSONObject(params.PromotionSnapshot),
	}); err != nil {
		return PromoteFailureResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PromoteFailureResult{}, fmt.Errorf("commit regression promotion transaction: %w", err)
	}

	regressionCase, err := r.GetRegressionCaseByID(ctx, createdCase.ID)
	if err != nil {
		return PromoteFailureResult{}, err
	}
	return PromoteFailureResult{Case: regressionCase, Created: true}, nil
}

func (r *Repository) createRegressionCaseWithQueries(ctx context.Context, queries *repositorysqlc.Queries, params CreateRegressionCaseParams) (RegressionCase, error) {
	row, err := queries.CreateRegressionCase(ctx, repositorysqlc.CreateRegressionCaseParams{
		SuiteID:                      params.SuiteID,
		Title:                        strings.TrimSpace(params.Title),
		Description:                  params.Description,
		Status:                       string(params.Status),
		Severity:                     string(params.Severity),
		PromotionMode:                string(params.PromotionMode),
		SourceRunID:                  cloneUUIDPtr(params.SourceRunID),
		SourceRunAgentID:             cloneUUIDPtr(params.SourceRunAgentID),
		SourceReplayID:               cloneUUIDPtr(params.SourceReplayID),
		SourceChallengePackVersionID: params.SourceChallengePackVersionID,
		SourceChallengeInputSetID:    cloneUUIDPtr(params.SourceChallengeInputSetID),
		SourceChallengeIdentityID:    params.SourceChallengeIdentityID,
		SourceCaseKey:                params.SourceCaseKey,
		SourceItemKey:                cloneStringPtr(params.SourceItemKey),
		EvidenceTier:                 params.EvidenceTier,
		FailureClass:                 params.FailureClass,
		FailureSummary:               params.FailureSummary,
		PayloadSnapshot:              normalizeJSONObject(params.PayloadSnapshot),
		ExpectedContract:             normalizeJSONObject(params.ExpectedContract),
		ValidatorOverrides:           cloneJSON(params.ValidatorOverrides),
		Metadata:                     normalizeJSONObject(params.Metadata),
	})
	if err != nil {
		return RegressionCase{}, fmt.Errorf("create regression case: %w", err)
	}
	return mapRegressionCaseFromTableRowPartial(row)
}

func (r *Repository) createRegressionPromotionWithQueries(ctx context.Context, queries *repositorysqlc.Queries, params CreateRegressionPromotionParams) (RegressionPromotion, error) {
	row, err := queries.CreateRegressionPromotion(ctx, repositorysqlc.CreateRegressionPromotionParams{
		WorkspaceRegressionCaseID: params.WorkspaceRegressionCaseID,
		SourceRunID:               params.SourceRunID,
		SourceRunAgentID:          params.SourceRunAgentID,
		SourceEventRefs:           normalizeJSONArray(params.SourceEventRefs),
		PromotedByUserID:          params.PromotedByUserID,
		PromotionReason:           params.PromotionReason,
		PromotionSnapshot:         normalizeJSONObject(params.PromotionSnapshot),
	})
	if err != nil {
		return RegressionPromotion{}, fmt.Errorf("create regression promotion: %w", err)
	}
	return mapRegressionPromotion(row)
}

func payloadSnapshotForChallenge(executionContext RunAgentExecutionContext, challengeIdentityID uuid.UUID) (json.RawMessage, error) {
	if executionContext.ChallengeInputSet == nil {
		return nil, fmt.Errorf("missing challenge input set for promoted failure")
	}
	for _, item := range executionContext.ChallengeInputSet.Cases {
		if item.ChallengeIdentityID == challengeIdentityID {
			return normalizeJSONObject(item.Payload), nil
		}
	}
	return nil, fmt.Errorf("challenge identity %s not found in run agent execution context", challengeIdentityID)
}

func challengeInputSetID(inputSet *ChallengeInputSetExecutionContext) *uuid.UUID {
	if inputSet == nil {
		return nil
	}
	id := inputSet.ID
	return &id
}

func (r *Repository) countRegressionCasesBySuiteID(ctx context.Context, suiteID uuid.UUID) (int, error) {
	count, err := r.queries.CountRegressionCasesBySuiteID(ctx, repositorysqlc.CountRegressionCasesBySuiteIDParams{
		SuiteID: suiteID,
	})
	if err != nil {
		return 0, fmt.Errorf("count regression cases by suite id: %w", err)
	}
	return int(count), nil
}

func (r *Repository) latestRegressionPromotionByCaseID(ctx context.Context, caseID uuid.UUID) (*RegressionPromotion, error) {
	row, err := r.queries.GetLatestRegressionPromotionByCaseID(ctx, repositorysqlc.GetLatestRegressionPromotionByCaseIDParams{
		WorkspaceRegressionCaseID: caseID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest regression promotion by case id: %w", err)
	}

	promotion, err := mapRegressionPromotion(row)
	if err != nil {
		return nil, fmt.Errorf("map latest regression promotion: %w", err)
	}
	return &promotion, nil
}

func mapRegressionSuite(row repositorysqlc.WorkspaceRegressionSuite) (RegressionSuite, error) {
	status, err := domain.ParseRegressionSuiteStatus(row.Status)
	if err != nil {
		return RegressionSuite{}, err
	}
	defaultGateSeverity, err := domain.ParseRegressionSeverity(row.DefaultGateSeverity)
	if err != nil {
		return RegressionSuite{}, err
	}
	createdAt, err := requiredTime("workspace_regression_suites.created_at", row.CreatedAt)
	if err != nil {
		return RegressionSuite{}, err
	}
	updatedAt, err := requiredTime("workspace_regression_suites.updated_at", row.UpdatedAt)
	if err != nil {
		return RegressionSuite{}, err
	}

	return RegressionSuite{
		ID:                    row.ID,
		WorkspaceID:           row.WorkspaceID,
		SourceChallengePackID: row.SourceChallengePackID,
		Name:                  row.Name,
		Description:           row.Description,
		Status:                status,
		SourceMode:            row.SourceMode,
		DefaultGateSeverity:   defaultGateSeverity,
		CaseCount:             0,
		CreatedByUserID:       row.CreatedByUserID,
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
	}, nil
}

func mapRegressionCase(row regressionCaseFields) (RegressionCase, error) {
	status, err := domain.ParseRegressionCaseStatus(row.status)
	if err != nil {
		return RegressionCase{}, err
	}
	severity, err := domain.ParseRegressionSeverity(row.severity)
	if err != nil {
		return RegressionCase{}, err
	}
	promotionMode, err := domain.ParseRegressionPromotionMode(row.promotionMode)
	if err != nil {
		return RegressionCase{}, err
	}
	createdAt, err := requiredTime("workspace_regression_cases.created_at", row.createdAt)
	if err != nil {
		return RegressionCase{}, err
	}
	updatedAt, err := requiredTime("workspace_regression_cases.updated_at", row.updatedAt)
	if err != nil {
		return RegressionCase{}, err
	}

	return RegressionCase{
		ID:                           row.id,
		SuiteID:                      row.suiteID,
		WorkspaceID:                  row.workspaceID,
		Title:                        row.title,
		Description:                  row.description,
		Status:                       status,
		Severity:                     severity,
		PromotionMode:                promotionMode,
		SourceRunID:                  cloneUUIDPtr(row.sourceRunID),
		SourceRunAgentID:             cloneUUIDPtr(row.sourceRunAgentID),
		SourceReplayID:               cloneUUIDPtr(row.sourceReplayID),
		SourceChallengePackVersionID: row.sourceChallengePackVersionID,
		SourceChallengeInputSetID:    cloneUUIDPtr(row.sourceChallengeInputSetID),
		SourceChallengeIdentityID:    row.sourceChallengeIdentityID,
		SourceCaseKey:                row.sourceCaseKey,
		SourceItemKey:                cloneStringPtr(row.sourceItemKey),
		EvidenceTier:                 row.evidenceTier,
		FailureClass:                 row.failureClass,
		FailureSummary:               row.failureSummary,
		PayloadSnapshot:              cloneJSON(row.payloadSnapshot),
		ExpectedContract:             cloneJSON(row.expectedContract),
		ValidatorOverrides:           cloneJSON(row.validatorOverrides),
		Metadata:                     cloneJSON(row.metadata),
		LatestPromotion:              nil,
		CreatedAt:                    createdAt,
		UpdatedAt:                    updatedAt,
	}, nil
}

func mapRegressionCaseFromTableRowPartial(row repositorysqlc.WorkspaceRegressionCase) (RegressionCase, error) {
	return mapRegressionCase(regressionCaseFields{
		id:                           row.ID,
		suiteID:                      row.SuiteID,
		title:                        row.Title,
		description:                  row.Description,
		status:                       row.Status,
		severity:                     row.Severity,
		promotionMode:                row.PromotionMode,
		sourceRunID:                  row.SourceRunID,
		sourceRunAgentID:             row.SourceRunAgentID,
		sourceReplayID:               row.SourceReplayID,
		sourceChallengePackVersionID: row.SourceChallengePackVersionID,
		sourceChallengeInputSetID:    row.SourceChallengeInputSetID,
		sourceChallengeIdentityID:    row.SourceChallengeIdentityID,
		sourceCaseKey:                row.SourceCaseKey,
		sourceItemKey:                row.SourceItemKey,
		evidenceTier:                 row.EvidenceTier,
		failureClass:                 row.FailureClass,
		failureSummary:               row.FailureSummary,
		payloadSnapshot:              row.PayloadSnapshot,
		expectedContract:             row.ExpectedContract,
		validatorOverrides:           row.ValidatorOverrides,
		metadata:                     row.Metadata,
		createdAt:                    row.CreatedAt,
		updatedAt:                    row.UpdatedAt,
	})
}

func mapRegressionCaseFromJoinedRow(row repositorysqlc.GetRegressionCaseByIDRow) (RegressionCase, error) {
	return mapRegressionCase(regressionCaseFields{
		id:                           row.ID,
		suiteID:                      row.SuiteID,
		workspaceID:                  row.WorkspaceID,
		title:                        row.Title,
		description:                  row.Description,
		status:                       row.Status,
		severity:                     row.Severity,
		promotionMode:                row.PromotionMode,
		sourceRunID:                  row.SourceRunID,
		sourceRunAgentID:             row.SourceRunAgentID,
		sourceReplayID:               row.SourceReplayID,
		sourceChallengePackVersionID: row.SourceChallengePackVersionID,
		sourceChallengeInputSetID:    row.SourceChallengeInputSetID,
		sourceChallengeIdentityID:    row.SourceChallengeIdentityID,
		sourceCaseKey:                row.SourceCaseKey,
		sourceItemKey:                row.SourceItemKey,
		evidenceTier:                 row.EvidenceTier,
		failureClass:                 row.FailureClass,
		failureSummary:               row.FailureSummary,
		payloadSnapshot:              row.PayloadSnapshot,
		expectedContract:             row.ExpectedContract,
		validatorOverrides:           row.ValidatorOverrides,
		metadata:                     row.Metadata,
		createdAt:                    row.CreatedAt,
		updatedAt:                    row.UpdatedAt,
	})
}

func mapRegressionCaseFromListRow(row repositorysqlc.ListRegressionCasesBySuiteIDRow) (RegressionCase, error) {
	return mapRegressionCase(regressionCaseFields{
		id:                           row.ID,
		suiteID:                      row.SuiteID,
		workspaceID:                  row.WorkspaceID,
		title:                        row.Title,
		description:                  row.Description,
		status:                       row.Status,
		severity:                     row.Severity,
		promotionMode:                row.PromotionMode,
		sourceRunID:                  row.SourceRunID,
		sourceRunAgentID:             row.SourceRunAgentID,
		sourceReplayID:               row.SourceReplayID,
		sourceChallengePackVersionID: row.SourceChallengePackVersionID,
		sourceChallengeInputSetID:    row.SourceChallengeInputSetID,
		sourceChallengeIdentityID:    row.SourceChallengeIdentityID,
		sourceCaseKey:                row.SourceCaseKey,
		sourceItemKey:                row.SourceItemKey,
		evidenceTier:                 row.EvidenceTier,
		failureClass:                 row.FailureClass,
		failureSummary:               row.FailureSummary,
		payloadSnapshot:              row.PayloadSnapshot,
		expectedContract:             row.ExpectedContract,
		validatorOverrides:           row.ValidatorOverrides,
		metadata:                     row.Metadata,
		createdAt:                    row.CreatedAt,
		updatedAt:                    row.UpdatedAt,
	})
}

func mapRegressionPromotion(row repositorysqlc.WorkspaceRegressionPromotion) (RegressionPromotion, error) {
	createdAt, err := requiredTime("workspace_regression_promotions.created_at", row.CreatedAt)
	if err != nil {
		return RegressionPromotion{}, err
	}

	return RegressionPromotion{
		ID:                        row.ID,
		WorkspaceRegressionCaseID: row.WorkspaceRegressionCaseID,
		SourceRunID:               row.SourceRunID,
		SourceRunAgentID:          row.SourceRunAgentID,
		SourceEventRefs:           cloneJSON(row.SourceEventRefs),
		PromotedByUserID:          row.PromotedByUserID,
		PromotionReason:           row.PromotionReason,
		PromotionSnapshot:         cloneJSON(row.PromotionSnapshot),
		CreatedAt:                 createdAt,
	}, nil
}

type regressionCaseFields struct {
	id                           uuid.UUID
	suiteID                      uuid.UUID
	workspaceID                  uuid.UUID
	title                        string
	description                  string
	status                       string
	severity                     string
	promotionMode                string
	sourceRunID                  *uuid.UUID
	sourceRunAgentID             *uuid.UUID
	sourceReplayID               *uuid.UUID
	sourceChallengePackVersionID uuid.UUID
	sourceChallengeInputSetID    *uuid.UUID
	sourceChallengeIdentityID    uuid.UUID
	sourceCaseKey                string
	sourceItemKey                *string
	evidenceTier                 string
	failureClass                 string
	failureSummary               string
	payloadSnapshot              []byte
	expectedContract             []byte
	validatorOverrides           []byte
	metadata                     []byte
	createdAt                    pgtype.Timestamptz
	updatedAt                    pgtype.Timestamptz
}

func isRegressionSuiteNameConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == regressionSuiteActiveNameIndex
}

func isRegressionPromotionDuplicate(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == regressionPromotionUniquenessIndex
}
