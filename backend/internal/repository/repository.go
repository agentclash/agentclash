package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	repositorysqlc "github.com/Atharva-Kanherkar/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db      *pgxpool.Pool
	queries *repositorysqlc.Queries
}

type SetRunTemporalIDsParams struct {
	RunID              uuid.UUID
	TemporalWorkflowID string
	TemporalRunID      string
}

type TransitionRunStatusParams struct {
	RunID           uuid.UUID
	ToStatus        domain.RunStatus
	Reason          *string
	ChangedByUserID *uuid.UUID
}

type TransitionRunAgentStatusParams struct {
	RunAgentID    uuid.UUID
	ToStatus      domain.RunAgentStatus
	Reason        *string
	FailureReason *string
}

type InsertRunStatusHistoryParams struct {
	RunID           uuid.UUID
	FromStatus      *domain.RunStatus
	ToStatus        domain.RunStatus
	Reason          *string
	ChangedByUserID *uuid.UUID
}

type InsertRunAgentStatusHistoryParams struct {
	RunAgentID uuid.UUID
	FromStatus *domain.RunAgentStatus
	ToStatus   domain.RunAgentStatus
	Reason     *string
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{
		db:      db,
		queries: repositorysqlc.New(db),
	}
}

func (r *Repository) GetRunByID(ctx context.Context, id uuid.UUID) (domain.Run, error) {
	row, err := r.queries.GetRunByID(ctx, repositorysqlc.GetRunByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Run{}, ErrRunNotFound
		}
		return domain.Run{}, fmt.Errorf("get run by id: %w", err)
	}

	run, err := mapRun(row)
	if err != nil {
		return domain.Run{}, fmt.Errorf("map run: %w", err)
	}

	return run, nil
}

func (r *Repository) ListRunAgentsByRunID(ctx context.Context, runID uuid.UUID) ([]domain.RunAgent, error) {
	rows, err := r.queries.ListRunAgentsByRunID(ctx, repositorysqlc.ListRunAgentsByRunIDParams{RunID: runID})
	if err != nil {
		return nil, fmt.Errorf("list run agents by run id: %w", err)
	}

	runAgents := make([]domain.RunAgent, 0, len(rows))
	for _, row := range rows {
		runAgent, mapErr := mapRunAgent(row)
		if mapErr != nil {
			return nil, fmt.Errorf("map run agent %s: %w", row.ID, mapErr)
		}
		runAgents = append(runAgents, runAgent)
	}

	return runAgents, nil
}

func (r *Repository) SetRunTemporalIDs(ctx context.Context, params SetRunTemporalIDsParams) (domain.Run, error) {
	if params.TemporalWorkflowID == "" {
		return domain.Run{}, ErrTemporalWorkflowID
	}
	if params.TemporalRunID == "" {
		return domain.Run{}, ErrTemporalRunID
	}

	row, err := r.queries.SetRunTemporalIDs(ctx, repositorysqlc.SetRunTemporalIDsParams{
		ID:                 params.RunID,
		TemporalWorkflowID: stringPtr(params.TemporalWorkflowID),
		TemporalRunID:      stringPtr(params.TemporalRunID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			currentRow, getErr := r.queries.GetRunByID(ctx, repositorysqlc.GetRunByIDParams{ID: params.RunID})
			if getErr != nil {
				if errors.Is(getErr, pgx.ErrNoRows) {
					return domain.Run{}, ErrRunNotFound
				}
				return domain.Run{}, fmt.Errorf("load run after temporal id write miss: %w", getErr)
			}

			if temporalIDsMatch(currentRow, params) {
				run, mapErr := mapRun(currentRow)
				if mapErr != nil {
					return domain.Run{}, fmt.Errorf("map run: %w", mapErr)
				}
				return run, nil
			}

			return domain.Run{}, TemporalIDConflictError{
				RunID:                params.RunID,
				ExistingWorkflowID:   currentRow.TemporalWorkflowID,
				ExistingTemporalRun:  currentRow.TemporalRunID,
				RequestedWorkflowID:  params.TemporalWorkflowID,
				RequestedTemporalRun: params.TemporalRunID,
			}
		}
		return domain.Run{}, fmt.Errorf("set run temporal ids: %w", err)
	}

	run, err := mapRun(row)
	if err != nil {
		return domain.Run{}, fmt.Errorf("map run: %w", err)
	}

	return run, nil
}

func (r *Repository) TransitionRunStatus(ctx context.Context, params TransitionRunStatusParams) (domain.Run, error) {
	if !params.ToStatus.Valid() {
		return domain.Run{}, fmt.Errorf("%w: %q", domain.ErrInvalidRunStatus, params.ToStatus)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Run{}, fmt.Errorf("begin run status transition transaction: %w", err)
	}
	defer rollback(ctx, tx)

	queries := r.queries.WithTx(tx)
	currentRow, err := queries.GetRunByID(ctx, repositorysqlc.GetRunByIDParams{ID: params.RunID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Run{}, ErrRunNotFound
		}
		return domain.Run{}, fmt.Errorf("load run for transition: %w", err)
	}

	currentStatus, err := domain.ParseRunStatus(currentRow.Status)
	if err != nil {
		return domain.Run{}, fmt.Errorf("load run status for transition: %w", err)
	}
	if !currentStatus.CanTransitionTo(params.ToStatus) {
		return domain.Run{}, InvalidTransitionError{
			Entity: "run",
			From:   string(currentStatus),
			To:     string(params.ToStatus),
		}
	}

	updatedRow, err := queries.UpdateRunStatus(ctx, repositorysqlc.UpdateRunStatusParams{
		ID:         params.RunID,
		FromStatus: string(currentStatus),
		ToStatus:   string(params.ToStatus),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Run{}, TransitionConflictError{
				Entity:   "run",
				ID:       params.RunID,
				Expected: string(currentStatus),
			}
		}
		return domain.Run{}, fmt.Errorf("update run status: %w", err)
	}

	_, err = queries.InsertRunStatusHistory(ctx, repositorysqlc.InsertRunStatusHistoryParams{
		RunID:           params.RunID,
		FromStatus:      stringPtr(string(currentStatus)),
		ToStatus:        string(params.ToStatus),
		Reason:          cloneStringPtr(params.Reason),
		ChangedByUserID: cloneUUIDPtr(params.ChangedByUserID),
	})
	if err != nil {
		return domain.Run{}, fmt.Errorf("insert run status history: %w", err)
	}

	run, err := mapRun(updatedRow)
	if err != nil {
		return domain.Run{}, fmt.Errorf("map run: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Run{}, fmt.Errorf("commit run status transition: %w", err)
	}

	return run, nil
}

func (r *Repository) TransitionRunAgentStatus(ctx context.Context, params TransitionRunAgentStatusParams) (domain.RunAgent, error) {
	if !params.ToStatus.Valid() {
		return domain.RunAgent{}, fmt.Errorf("%w: %q", domain.ErrInvalidRunAgentStatus, params.ToStatus)
	}
	if params.ToStatus != domain.RunAgentStatusFailed && params.FailureReason != nil {
		return domain.RunAgent{}, ErrUnexpectedFailureCause
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.RunAgent{}, fmt.Errorf("begin run-agent status transition transaction: %w", err)
	}
	defer rollback(ctx, tx)

	queries := r.queries.WithTx(tx)
	currentRow, err := queries.GetRunAgentByID(ctx, repositorysqlc.GetRunAgentByIDParams{ID: params.RunAgentID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RunAgent{}, ErrRunAgentNotFound
		}
		return domain.RunAgent{}, fmt.Errorf("load run agent for transition: %w", err)
	}

	currentStatus, err := domain.ParseRunAgentStatus(currentRow.Status)
	if err != nil {
		return domain.RunAgent{}, fmt.Errorf("load run-agent status for transition: %w", err)
	}
	if !currentStatus.CanTransitionTo(params.ToStatus) {
		return domain.RunAgent{}, InvalidTransitionError{
			Entity: "run_agent",
			From:   string(currentStatus),
			To:     string(params.ToStatus),
		}
	}

	failureReason := cloneStringPtr(params.FailureReason)
	if params.ToStatus == domain.RunAgentStatusFailed && failureReason == nil {
		failureReason = cloneStringPtr(params.Reason)
	}
	historyReason := cloneStringPtr(params.Reason)
	if params.ToStatus == domain.RunAgentStatusFailed && historyReason == nil {
		historyReason = cloneStringPtr(failureReason)
	}

	updatedRow, err := queries.UpdateRunAgentStatus(ctx, repositorysqlc.UpdateRunAgentStatusParams{
		ID:            params.RunAgentID,
		FromStatus:    string(currentStatus),
		ToStatus:      string(params.ToStatus),
		FailureReason: failureReason,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RunAgent{}, TransitionConflictError{
				Entity:   "run_agent",
				ID:       params.RunAgentID,
				Expected: string(currentStatus),
			}
		}
		return domain.RunAgent{}, fmt.Errorf("update run-agent status: %w", err)
	}

	_, err = queries.InsertRunAgentStatusHistory(ctx, repositorysqlc.InsertRunAgentStatusHistoryParams{
		RunAgentID: params.RunAgentID,
		FromStatus: stringPtr(string(currentStatus)),
		ToStatus:   string(params.ToStatus),
		Reason:     historyReason,
	})
	if err != nil {
		return domain.RunAgent{}, fmt.Errorf("insert run-agent status history: %w", err)
	}

	runAgent, err := mapRunAgent(updatedRow)
	if err != nil {
		return domain.RunAgent{}, fmt.Errorf("map run agent: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RunAgent{}, fmt.Errorf("commit run-agent status transition: %w", err)
	}

	return runAgent, nil
}

func (r *Repository) InsertRunStatusHistory(ctx context.Context, params InsertRunStatusHistoryParams) (domain.RunStatusHistory, error) {
	if !params.ToStatus.Valid() {
		return domain.RunStatusHistory{}, fmt.Errorf("%w: %q", domain.ErrInvalidRunStatus, params.ToStatus)
	}
	if params.FromStatus != nil && !params.FromStatus.Valid() {
		return domain.RunStatusHistory{}, fmt.Errorf("%w: %q", domain.ErrInvalidRunStatus, *params.FromStatus)
	}
	if params.FromStatus != nil && !params.FromStatus.CanTransitionTo(params.ToStatus) {
		return domain.RunStatusHistory{}, InvalidTransitionError{
			Entity: "run",
			From:   string(*params.FromStatus),
			To:     string(params.ToStatus),
		}
	}

	row, err := r.queries.InsertRunStatusHistory(ctx, repositorysqlc.InsertRunStatusHistoryParams{
		RunID:           params.RunID,
		FromStatus:      runStatusPtr(params.FromStatus),
		ToStatus:        string(params.ToStatus),
		Reason:          cloneStringPtr(params.Reason),
		ChangedByUserID: cloneUUIDPtr(params.ChangedByUserID),
	})
	if err != nil {
		return domain.RunStatusHistory{}, fmt.Errorf("insert run status history: %w", err)
	}

	history, err := mapRunStatusHistory(row)
	if err != nil {
		return domain.RunStatusHistory{}, fmt.Errorf("map run status history: %w", err)
	}

	return history, nil
}

func (r *Repository) InsertRunAgentStatusHistory(ctx context.Context, params InsertRunAgentStatusHistoryParams) (domain.RunAgentStatusHistory, error) {
	if !params.ToStatus.Valid() {
		return domain.RunAgentStatusHistory{}, fmt.Errorf("%w: %q", domain.ErrInvalidRunAgentStatus, params.ToStatus)
	}
	if params.FromStatus != nil && !params.FromStatus.Valid() {
		return domain.RunAgentStatusHistory{}, fmt.Errorf("%w: %q", domain.ErrInvalidRunAgentStatus, *params.FromStatus)
	}
	if params.FromStatus != nil && !params.FromStatus.CanTransitionTo(params.ToStatus) {
		return domain.RunAgentStatusHistory{}, InvalidTransitionError{
			Entity: "run_agent",
			From:   string(*params.FromStatus),
			To:     string(params.ToStatus),
		}
	}

	row, err := r.queries.InsertRunAgentStatusHistory(ctx, repositorysqlc.InsertRunAgentStatusHistoryParams{
		RunAgentID: params.RunAgentID,
		FromStatus: runAgentStatusPtr(params.FromStatus),
		ToStatus:   string(params.ToStatus),
		Reason:     cloneStringPtr(params.Reason),
	})
	if err != nil {
		return domain.RunAgentStatusHistory{}, fmt.Errorf("insert run-agent status history: %w", err)
	}

	history, err := mapRunAgentStatusHistory(row)
	if err != nil {
		return domain.RunAgentStatusHistory{}, fmt.Errorf("map run-agent status history: %w", err)
	}

	return history, nil
}

func mapRun(row repositorysqlc.Run) (domain.Run, error) {
	status, err := domain.ParseRunStatus(row.Status)
	if err != nil {
		return domain.Run{}, err
	}

	createdAt, err := requiredTime("runs.created_at", row.CreatedAt)
	if err != nil {
		return domain.Run{}, err
	}
	updatedAt, err := requiredTime("runs.updated_at", row.UpdatedAt)
	if err != nil {
		return domain.Run{}, err
	}

	return domain.Run{
		ID:                     row.ID,
		OrganizationID:         row.OrganizationID,
		WorkspaceID:            row.WorkspaceID,
		ChallengePackVersionID: row.ChallengePackVersionID,
		ChallengeInputSetID:    cloneUUIDPtr(row.ChallengeInputSetID),
		CreatedByUserID:        cloneUUIDPtr(row.CreatedByUserID),
		Name:                   row.Name,
		Status:                 status,
		ExecutionMode:          row.ExecutionMode,
		TemporalWorkflowID:     cloneStringPtr(row.TemporalWorkflowID),
		TemporalRunID:          cloneStringPtr(row.TemporalRunID),
		ExecutionPlan:          cloneJSON(row.ExecutionPlan),
		QueuedAt:               optionalTime(row.QueuedAt),
		StartedAt:              optionalTime(row.StartedAt),
		FinishedAt:             optionalTime(row.FinishedAt),
		CancelledAt:            optionalTime(row.CancelledAt),
		FailedAt:               optionalTime(row.FailedAt),
		CreatedAt:              createdAt,
		UpdatedAt:              updatedAt,
	}, nil
}

func mapRunAgent(row repositorysqlc.RunAgent) (domain.RunAgent, error) {
	status, err := domain.ParseRunAgentStatus(row.Status)
	if err != nil {
		return domain.RunAgent{}, err
	}

	createdAt, err := requiredTime("run_agents.created_at", row.CreatedAt)
	if err != nil {
		return domain.RunAgent{}, err
	}
	updatedAt, err := requiredTime("run_agents.updated_at", row.UpdatedAt)
	if err != nil {
		return domain.RunAgent{}, err
	}

	return domain.RunAgent{
		ID:                        row.ID,
		OrganizationID:            row.OrganizationID,
		WorkspaceID:               row.WorkspaceID,
		RunID:                     row.RunID,
		AgentDeploymentID:         row.AgentDeploymentID,
		AgentDeploymentSnapshotID: row.AgentDeploymentSnapshotID,
		LaneIndex:                 row.LaneIndex,
		Label:                     row.Label,
		Status:                    status,
		QueuedAt:                  optionalTime(row.QueuedAt),
		StartedAt:                 optionalTime(row.StartedAt),
		FinishedAt:                optionalTime(row.FinishedAt),
		FailureReason:             cloneStringPtr(row.FailureReason),
		CreatedAt:                 createdAt,
		UpdatedAt:                 updatedAt,
	}, nil
}

func mapRunStatusHistory(row repositorysqlc.RunStatusHistory) (domain.RunStatusHistory, error) {
	toStatus, err := domain.ParseRunStatus(row.ToStatus)
	if err != nil {
		return domain.RunStatusHistory{}, err
	}

	fromStatus, err := parseOptionalRunStatus(row.FromStatus)
	if err != nil {
		return domain.RunStatusHistory{}, err
	}

	changedAt, err := requiredTime("run_status_history.changed_at", row.ChangedAt)
	if err != nil {
		return domain.RunStatusHistory{}, err
	}

	return domain.RunStatusHistory{
		ID:              row.ID,
		RunID:           row.RunID,
		FromStatus:      fromStatus,
		ToStatus:        toStatus,
		Reason:          cloneStringPtr(row.Reason),
		ChangedByUserID: cloneUUIDPtr(row.ChangedByUserID),
		ChangedAt:       changedAt,
	}, nil
}

func mapRunAgentStatusHistory(row repositorysqlc.RunAgentStatusHistory) (domain.RunAgentStatusHistory, error) {
	toStatus, err := domain.ParseRunAgentStatus(row.ToStatus)
	if err != nil {
		return domain.RunAgentStatusHistory{}, err
	}

	fromStatus, err := parseOptionalRunAgentStatus(row.FromStatus)
	if err != nil {
		return domain.RunAgentStatusHistory{}, err
	}

	changedAt, err := requiredTime("run_agent_status_history.changed_at", row.ChangedAt)
	if err != nil {
		return domain.RunAgentStatusHistory{}, err
	}

	return domain.RunAgentStatusHistory{
		ID:         row.ID,
		RunAgentID: row.RunAgentID,
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		Reason:     cloneStringPtr(row.Reason),
		ChangedAt:  changedAt,
	}, nil
}

func parseOptionalRunStatus(raw *string) (*domain.RunStatus, error) {
	if raw == nil {
		return nil, nil
	}

	status, err := domain.ParseRunStatus(*raw)
	if err != nil {
		return nil, err
	}

	return &status, nil
}

func parseOptionalRunAgentStatus(raw *string) (*domain.RunAgentStatus, error) {
	if raw == nil {
		return nil, nil
	}

	status, err := domain.ParseRunAgentStatus(*raw)
	if err != nil {
		return nil, err
	}

	return &status, nil
}

func requiredTime(field string, value pgtype.Timestamptz) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, fmt.Errorf("%s is null", field)
	}
	return value.Time, nil
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return timePtr(value.Time)
}

func cloneJSON(value []byte) json.RawMessage {
	if value == nil {
		return nil
	}
	return append(json.RawMessage(nil), value...)
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	return stringPtr(*value)
}

func cloneUUIDPtr(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func runStatusPtr(status *domain.RunStatus) *string {
	if status == nil {
		return nil
	}
	return stringPtr(string(*status))
}

func runAgentStatusPtr(status *domain.RunAgentStatus) *string {
	if status == nil {
		return nil
	}
	return stringPtr(string(*status))
}

func stringPtr(value string) *string {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}

func temporalIDsMatch(row repositorysqlc.Run, params SetRunTemporalIDsParams) bool {
	if row.TemporalWorkflowID == nil || row.TemporalRunID == nil {
		return false
	}
	return *row.TemporalWorkflowID == params.TemporalWorkflowID &&
		*row.TemporalRunID == params.TemporalRunID
}
