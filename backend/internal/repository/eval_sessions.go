package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	repositorysqlc "github.com/Atharva-Kanherkar/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CreateEvalSessionParams struct {
	Repetitions            int32
	AggregationConfig      json.RawMessage
	SuccessThresholdConfig json.RawMessage
	RoutingTaskSnapshot    json.RawMessage
	SchemaVersion          int32
}

type TransitionEvalSessionStatusParams struct {
	EvalSessionID uuid.UUID
	ToStatus      domain.EvalSessionStatus
}

type EvalSessionWithRuns struct {
	Session domain.EvalSession
	Runs    []domain.Run
}

func (r *Repository) CreateEvalSession(ctx context.Context, params CreateEvalSessionParams) (domain.EvalSession, error) {
	if params.Repetitions < 1 {
		return domain.EvalSession{}, ErrEvalSessionRepetitionsInvalid
	}
	if params.SchemaVersion < 1 {
		return domain.EvalSession{}, ErrEvalSessionSchemaVersion
	}

	row, err := r.queries.CreateEvalSession(ctx, repositorysqlc.CreateEvalSessionParams{
		Status:                 string(domain.EvalSessionStatusQueued),
		Repetitions:            params.Repetitions,
		AggregationConfig:      normalizeJSON(params.AggregationConfig),
		SuccessThresholdConfig: normalizeJSON(params.SuccessThresholdConfig),
		RoutingTaskSnapshot:    normalizeJSON(params.RoutingTaskSnapshot),
		SchemaVersion:          params.SchemaVersion,
	})
	if err != nil {
		return domain.EvalSession{}, fmt.Errorf("create eval session: %w", err)
	}

	session, err := mapEvalSession(row)
	if err != nil {
		return domain.EvalSession{}, fmt.Errorf("map eval session: %w", err)
	}

	return session, nil
}

func (r *Repository) GetEvalSessionByID(ctx context.Context, id uuid.UUID) (domain.EvalSession, error) {
	row, err := r.queries.GetEvalSessionByID(ctx, repositorysqlc.GetEvalSessionByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EvalSession{}, ErrEvalSessionNotFound
		}
		return domain.EvalSession{}, fmt.Errorf("get eval session by id: %w", err)
	}

	session, err := mapEvalSession(row)
	if err != nil {
		return domain.EvalSession{}, fmt.Errorf("map eval session: %w", err)
	}

	return session, nil
}

func (r *Repository) GetEvalSessionWithRuns(ctx context.Context, id uuid.UUID) (EvalSessionWithRuns, error) {
	session, err := r.GetEvalSessionByID(ctx, id)
	if err != nil {
		return EvalSessionWithRuns{}, err
	}

	rows, err := r.queries.ListRunsByEvalSessionID(ctx, repositorysqlc.ListRunsByEvalSessionIDParams{
		EvalSessionID: &id,
	})
	if err != nil {
		return EvalSessionWithRuns{}, fmt.Errorf("list runs by eval session id: %w", err)
	}

	runs := make([]domain.Run, 0, len(rows))
	for _, row := range rows {
		run, mapErr := mapRun(row)
		if mapErr != nil {
			return EvalSessionWithRuns{}, fmt.Errorf("map run %s: %w", row.ID, mapErr)
		}
		runs = append(runs, run)
	}

	return EvalSessionWithRuns{
		Session: session,
		Runs:    runs,
	}, nil
}

func (r *Repository) ListEvalSessions(ctx context.Context, limit int32, offset int32) ([]domain.EvalSession, error) {
	if limit < 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.queries.ListEvalSessions(ctx, repositorysqlc.ListEvalSessionsParams{
		ResultLimit:  limit,
		ResultOffset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list eval sessions: %w", err)
	}

	sessions := make([]domain.EvalSession, 0, len(rows))
	for _, row := range rows {
		session, mapErr := mapEvalSession(row)
		if mapErr != nil {
			return nil, fmt.Errorf("map eval session %s: %w", row.ID, mapErr)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (r *Repository) TransitionEvalSessionStatus(ctx context.Context, params TransitionEvalSessionStatusParams) (domain.EvalSession, error) {
	if !params.ToStatus.Valid() {
		return domain.EvalSession{}, fmt.Errorf("%w: %q", domain.ErrInvalidEvalSessionStatus, params.ToStatus)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.EvalSession{}, fmt.Errorf("begin eval session status transition transaction: %w", err)
	}
	defer rollback(ctx, tx)

	queries := r.queries.WithTx(tx)
	currentRow, err := queries.GetEvalSessionByID(ctx, repositorysqlc.GetEvalSessionByIDParams{ID: params.EvalSessionID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EvalSession{}, ErrEvalSessionNotFound
		}
		return domain.EvalSession{}, fmt.Errorf("load eval session for transition: %w", err)
	}

	currentStatus, err := domain.ParseEvalSessionStatus(currentRow.Status)
	if err != nil {
		return domain.EvalSession{}, fmt.Errorf("load eval session status for transition: %w", err)
	}
	if !currentStatus.CanTransitionTo(params.ToStatus) {
		return domain.EvalSession{}, IllegalSessionTransitionError{
			From: string(currentStatus),
			To:   string(params.ToStatus),
		}
	}

	updatedRow, err := queries.UpdateEvalSessionStatus(ctx, repositorysqlc.UpdateEvalSessionStatusParams{
		ID:         params.EvalSessionID,
		FromStatus: string(currentStatus),
		ToStatus:   string(params.ToStatus),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EvalSession{}, TransitionConflictError{
				Entity:   "eval session",
				ID:       params.EvalSessionID,
				Expected: string(currentStatus),
			}
		}
		return domain.EvalSession{}, fmt.Errorf("update eval session status: %w", err)
	}

	session, err := mapEvalSession(updatedRow)
	if err != nil {
		return domain.EvalSession{}, fmt.Errorf("map eval session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.EvalSession{}, fmt.Errorf("commit eval session status transition: %w", err)
	}

	return session, nil
}

func (r *Repository) AttachRunToEvalSession(ctx context.Context, runID uuid.UUID, evalSessionID uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin eval session attachment transaction: %w", err)
	}
	defer rollback(ctx, tx)

	queries := r.queries.WithTx(tx)

	if _, err := queries.GetEvalSessionByID(ctx, repositorysqlc.GetEvalSessionByIDParams{ID: evalSessionID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEvalSessionNotFound
		}
		return fmt.Errorf("load eval session for attachment: %w", err)
	}

	runRow, err := queries.GetRunByID(ctx, repositorysqlc.GetRunByIDParams{ID: runID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRunNotFound
		}
		return fmt.Errorf("load run for eval session attachment: %w", err)
	}

	if runRow.EvalSessionID != nil {
		if *runRow.EvalSessionID == evalSessionID {
			if err := tx.Commit(ctx); err != nil {
				return fmt.Errorf("commit idempotent eval session attachment: %w", err)
			}
			return nil
		}
		return RunAlreadyAttachedToSessionError{
			RunID:                  runID,
			ExistingEvalSessionID:  *runRow.EvalSessionID,
			RequestedEvalSessionID: evalSessionID,
		}
	}

	if _, err := queries.AttachRunToEvalSession(ctx, repositorysqlc.AttachRunToEvalSessionParams{
		EvalSessionID: &evalSessionID,
		ID:            runID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			currentRow, getErr := queries.GetRunByID(ctx, repositorysqlc.GetRunByIDParams{ID: runID})
			if getErr != nil {
				if errors.Is(getErr, pgx.ErrNoRows) {
					return ErrRunNotFound
				}
				return fmt.Errorf("reload run after attachment miss: %w", getErr)
			}
			if currentRow.EvalSessionID != nil {
				return RunAlreadyAttachedToSessionError{
					RunID:                  runID,
					ExistingEvalSessionID:  *currentRow.EvalSessionID,
					RequestedEvalSessionID: evalSessionID,
				}
			}
			return AttachmentConflictError{
				Entity: "run",
				ID:     runID,
			}
		}
		return fmt.Errorf("attach run to eval session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit eval session attachment: %w", err)
	}

	return nil
}

func mapEvalSession(row repositorysqlc.EvalSession) (domain.EvalSession, error) {
	status, err := domain.ParseEvalSessionStatus(row.Status)
	if err != nil {
		return domain.EvalSession{}, err
	}

	createdAt, err := requiredTime("eval_sessions.created_at", row.CreatedAt)
	if err != nil {
		return domain.EvalSession{}, err
	}
	updatedAt, err := requiredTime("eval_sessions.updated_at", row.UpdatedAt)
	if err != nil {
		return domain.EvalSession{}, err
	}

	return domain.EvalSession{
		ID:          row.ID,
		Status:      status,
		Repetitions: row.Repetitions,
		AggregationConfig: domain.EvalSessionSnapshot{
			Document: cloneJSON(row.AggregationConfig),
		},
		SuccessThresholdConfig: domain.EvalSessionSnapshot{
			Document: cloneJSON(row.SuccessThresholdConfig),
		},
		RoutingTaskSnapshot: domain.EvalSessionSnapshot{
			Document: cloneJSON(row.RoutingTaskSnapshot),
		},
		SchemaVersion: row.SchemaVersion,
		CreatedAt:     createdAt,
		StartedAt:     optionalTime(row.StartedAt),
		FinishedAt:    optionalTime(row.FinishedAt),
		UpdatedAt:     updatedAt,
	}, nil
}
