package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type AgentTryoutStatus string

const (
	AgentTryoutStatusQueued    AgentTryoutStatus = "queued"
	AgentTryoutStatusRunning   AgentTryoutStatus = "running"
	AgentTryoutStatusCompleted AgentTryoutStatus = "completed"
	AgentTryoutStatusFailed    AgentTryoutStatus = "failed"
	AgentTryoutStatusCancelled AgentTryoutStatus = "cancelled"
)

type AgentTryoutRedactionStatus string

const (
	AgentTryoutRedactionPending     AgentTryoutRedactionStatus = "pending"
	AgentTryoutRedactionPassed      AgentTryoutRedactionStatus = "passed"
	AgentTryoutRedactionFailed      AgentTryoutRedactionStatus = "failed"
	AgentTryoutRedactionNotRequired AgentTryoutRedactionStatus = "not_required"
)

// ShareReady reports whether a tryout's redaction status permits public
// exposure (share creation and public/share-token rendering). Only tryouts
// whose payloads have cleared redaction — or that never required it — may be
// shared; pending/failed tryouts must fail closed so unredacted content is
// never published.
func (s AgentTryoutRedactionStatus) ShareReady() bool {
	return s == AgentTryoutRedactionPassed || s == AgentTryoutRedactionNotRequired
}

type AgentTryout struct {
	ID                       uuid.UUID
	OrganizationID           *uuid.UUID
	WorkspaceID              *uuid.UUID
	TemplateSlug             string
	Status                   AgentTryoutStatus
	InputSnapshot            json.RawMessage
	TemplateSnapshot         json.RawMessage
	ToolPolicySnapshot       json.RawMessage
	EvaluationSpecSnapshot   json.RawMessage
	SelectedModelPolicy      json.RawMessage
	Summary                  json.RawMessage
	RedactionStatus          AgentTryoutRedactionStatus
	RunID                    *uuid.UUID
	CostLimitUSD             float64
	ActualCostUSD            *float64
	LatencyMS                *int64
	MaxDurationSeconds       int32
	AnonymousFingerprintHash *string
	CreatedByUserID          *uuid.UUID
	ParentTryoutID           *uuid.UUID
	ClaimedByUserID          *uuid.UUID
	ClaimedAt                *time.Time
	ExpiresAt                *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type AgentTryoutEvent struct {
	ID             int64
	AgentTryoutID  uuid.UUID
	SequenceNumber int64
	EventType      string
	ActorType      string
	OccurredAt     time.Time
	Payload        json.RawMessage
}

type CreateAgentTryoutParams struct {
	OrganizationID           *uuid.UUID
	WorkspaceID              *uuid.UUID
	TemplateSlug             string
	Status                   AgentTryoutStatus
	InputSnapshot            json.RawMessage
	TemplateSnapshot         json.RawMessage
	ToolPolicySnapshot       json.RawMessage
	EvaluationSpecSnapshot   json.RawMessage
	SelectedModelPolicy      json.RawMessage
	Summary                  json.RawMessage
	RedactionStatus          AgentTryoutRedactionStatus
	RunID                    *uuid.UUID
	CostLimitUSD             float64
	ActualCostUSD            *float64
	LatencyMS                *int64
	MaxDurationSeconds       int32
	AnonymousFingerprintHash *string
	CreatedByUserID          *uuid.UUID
	ParentTryoutID           *uuid.UUID
	ExpiresAt                *time.Time
}

type ClaimAgentTryoutParams struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	WorkspaceID     uuid.UUID
	ClaimedByUserID uuid.UUID
	ClaimedAt       time.Time
}

type UpdateAgentTryoutStatusParams struct {
	ID              uuid.UUID
	Status          AgentTryoutStatus
	Summary         json.RawMessage
	ActualCostUSD   *float64
	LatencyMS       *int64
	RedactionStatus *AgentTryoutRedactionStatus
}

type LinkAgentTryoutRunParams struct {
	ID      uuid.UUID
	RunID   uuid.UUID
	Status  AgentTryoutStatus
	Summary json.RawMessage
}

type RecordAgentTryoutEventParams struct {
	AgentTryoutID uuid.UUID
	EventType     string
	ActorType     string
	Payload       json.RawMessage
}

// tryoutRowQuerier is satisfied by both *pgxpool.Pool and pgx.Tx, letting the
// hand-written anonymous quota queries run either standalone or inside the
// advisory-locked transaction used by WithinAnonymousAgentTryoutQuotaLock.
type tryoutRowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *Repository) CreateAgentTryout(ctx context.Context, params CreateAgentTryoutParams) (AgentTryout, error) {
	return createAgentTryout(ctx, r.queries, params)
}

func (r *Repository) RecordAgentTryoutEvent(ctx context.Context, params RecordAgentTryoutEventParams) (AgentTryoutEvent, error) {
	row, err := r.queries.RecordAgentTryoutEvent(ctx, repositorysqlc.RecordAgentTryoutEventParams{
		AgentTryoutID: params.AgentTryoutID,
		EventType:     params.EventType,
		ActorType:     params.ActorType,
		Payload:       normalizeJSONObject(params.Payload),
	})
	if err != nil {
		return AgentTryoutEvent{}, fmt.Errorf("record agent tryout event: %w", err)
	}
	return mapAgentTryoutEvent(row)
}

func (r *Repository) ListAgentTryoutEventsAfter(ctx context.Context, tryoutID uuid.UUID, afterID int64, limit int32) ([]AgentTryoutEvent, error) {
	rows, err := r.queries.ListAgentTryoutEventsAfter(ctx, repositorysqlc.ListAgentTryoutEventsAfterParams{
		AgentTryoutID: tryoutID,
		AfterID:       afterID,
		LimitCount:    limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list agent tryout events: %w", err)
	}
	events := make([]AgentTryoutEvent, 0, len(rows))
	for _, row := range rows {
		event, err := mapAgentTryoutEvent(row)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func createAgentTryout(ctx context.Context, queries *repositorysqlc.Queries, params CreateAgentTryoutParams) (AgentTryout, error) {
	costLimit, err := numericFromFloat(&params.CostLimitUSD)
	if err != nil {
		return AgentTryout{}, fmt.Errorf("encode agent tryout cost limit: %w", err)
	}
	actualCost, err := numericFromFloat(params.ActualCostUSD)
	if err != nil {
		return AgentTryout{}, fmt.Errorf("encode agent tryout actual cost: %w", err)
	}
	row, err := queries.CreateAgentTryout(ctx, repositorysqlc.CreateAgentTryoutParams{
		OrganizationID:           cloneUUIDPtr(params.OrganizationID),
		WorkspaceID:              cloneUUIDPtr(params.WorkspaceID),
		TemplateSlug:             params.TemplateSlug,
		Status:                   string(params.Status),
		InputSnapshot:            normalizeJSONObject(params.InputSnapshot),
		TemplateSnapshot:         normalizeJSONObject(params.TemplateSnapshot),
		ToolPolicySnapshot:       normalizeJSONObject(params.ToolPolicySnapshot),
		EvaluationSpecSnapshot:   normalizeJSONObject(params.EvaluationSpecSnapshot),
		SelectedModelPolicy:      normalizeJSONObject(params.SelectedModelPolicy),
		Summary:                  normalizeJSONObject(params.Summary),
		RedactionStatus:          string(params.RedactionStatus),
		RunID:                    cloneUUIDPtr(params.RunID),
		CostLimitUsd:             costLimit,
		ActualCostUsd:            actualCost,
		LatencyMs:                cloneInt64Ptr(params.LatencyMS),
		MaxDurationSeconds:       params.MaxDurationSeconds,
		AnonymousFingerprintHash: cloneStringPtr(params.AnonymousFingerprintHash),
		CreatedByUserID:          cloneUUIDPtr(params.CreatedByUserID),
		ParentTryoutID:           cloneUUIDPtr(params.ParentTryoutID),
		ExpiresAt:                toPGTimestamp(params.ExpiresAt),
	})
	if err != nil {
		return AgentTryout{}, fmt.Errorf("create agent tryout: %w", err)
	}
	return mapAgentTryout(row)
}

func (r *Repository) CountAnonymousAgentTryoutsByFingerprint(ctx context.Context, fingerprintHash string, since time.Time) (int64, error) {
	return countAnonymousAgentTryoutsByFingerprint(ctx, r.db, fingerprintHash, since)
}

func countAnonymousAgentTryoutsByFingerprint(ctx context.Context, q tryoutRowQuerier, fingerprintHash string, since time.Time) (int64, error) {
	var count int64
	err := q.QueryRow(ctx, `
SELECT COUNT(*)
FROM agent_tryouts
WHERE organization_id IS NULL
  AND workspace_id IS NULL
  AND anonymous_fingerprint_hash = $1
  AND created_at >= $2`, fingerprintHash, since.UTC()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count anonymous agent tryouts by fingerprint: %w", err)
	}
	return count, nil
}

func (r *Repository) SumAnonymousAgentTryoutCostLimitUSD(ctx context.Context, windowStart, windowEnd time.Time) (float64, error) {
	return sumAnonymousAgentTryoutCostLimitUSD(ctx, r.db, windowStart, windowEnd)
}

func sumAnonymousAgentTryoutCostLimitUSD(ctx context.Context, q tryoutRowQuerier, windowStart, windowEnd time.Time) (float64, error) {
	var total pgtype.Numeric
	err := q.QueryRow(ctx, `
SELECT COALESCE(SUM(cost_limit_usd), 0)
FROM agent_tryouts
WHERE organization_id IS NULL
  AND workspace_id IS NULL
  AND anonymous_fingerprint_hash IS NOT NULL
  AND created_at >= $1
  AND created_at < $2`, windowStart.UTC(), windowEnd.UTC()).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum anonymous agent tryout cost limits: %w", err)
	}
	return derefFloat64(numericPtr(total)), nil
}

// AnonymousAgentTryoutQuotaTx exposes the anonymous-tryout reads and the create
// write as a single transactional unit. Implementations run every call inside
// one transaction that holds a global advisory lock, so the per-fingerprint
// quota count, the hosted daily-spend sum, and the insert are atomic with
// respect to other anonymous-tryout creations — closing the check-then-create
// TOCTOU window.
type AnonymousAgentTryoutQuotaTx interface {
	CountAnonymousAgentTryoutsByFingerprint(ctx context.Context, fingerprintHash string, since time.Time) (int64, error)
	SumAnonymousAgentTryoutCostLimitUSD(ctx context.Context, windowStart, windowEnd time.Time) (float64, error)
	CreateAgentTryout(ctx context.Context, params CreateAgentTryoutParams) (AgentTryout, error)
}

type anonymousAgentTryoutQuotaTx struct {
	repo *Repository
	tx   pgx.Tx
}

func (q anonymousAgentTryoutQuotaTx) CountAnonymousAgentTryoutsByFingerprint(ctx context.Context, fingerprintHash string, since time.Time) (int64, error) {
	return countAnonymousAgentTryoutsByFingerprint(ctx, q.tx, fingerprintHash, since)
}

func (q anonymousAgentTryoutQuotaTx) SumAnonymousAgentTryoutCostLimitUSD(ctx context.Context, windowStart, windowEnd time.Time) (float64, error) {
	return sumAnonymousAgentTryoutCostLimitUSD(ctx, q.tx, windowStart, windowEnd)
}

func (q anonymousAgentTryoutQuotaTx) CreateAgentTryout(ctx context.Context, params CreateAgentTryoutParams) (AgentTryout, error) {
	return createAgentTryout(ctx, q.repo.queries.WithTx(q.tx), params)
}

// WithinAnonymousAgentTryoutQuotaLock runs fn inside a transaction that first
// takes a global advisory lock shared by all anonymous tryout creations. Both
// the per-fingerprint quota and the global hosted daily-spend cap are protected:
// concurrent anonymous creations serialize on the lock, so reads observe every
// previously committed tryout before the next insert. If fn returns an error the
// transaction rolls back and no tryout is created.
func (r *Repository) WithinAnonymousAgentTryoutQuotaLock(ctx context.Context, fn func(AnonymousAgentTryoutQuotaTx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin anonymous agent tryout quota transaction: %w", err)
	}
	defer rollback(ctx, tx)

	// Global serialization point for anonymous tryout creation. Keyed on a fixed
	// string (not the fingerprint) because the hosted daily-spend cap is a single
	// global counter that every anonymous creation contends for.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('anonymous_agent_tryout_quota', 451))`); err != nil {
		return fmt.Errorf("lock anonymous agent tryout quota: %w", err)
	}

	if err := fn(anonymousAgentTryoutQuotaTx{repo: r, tx: tx}); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit anonymous agent tryout quota transaction: %w", err)
	}
	return nil
}

// ExpireAnonymousAgentTryoutsParams configures one retention sweep batch.
type ExpireAnonymousAgentTryoutsParams struct {
	// Now is the comparison instant; tryouts with expires_at <= Now are eligible.
	Now time.Time
	// Limit bounds the rows processed per call so the sweep stays incremental.
	Limit int32
}

// ExpireAnonymousAgentTryouts deletes one batch of expired, unclaimed anonymous
// tryouts and schedules their run artifacts for deletion, returning the number
// of tryouts removed. Claimed tryouts have expires_at cleared (see
// ClaimAgentTryout) and are never matched, so claimed/workspace tryouts are
// retained. Artifacts are soft-expired (retention_status -> scheduled_for_deletion)
// rather than hard-deleted so downstream storage cleanup can reclaim objects.
// Rows are locked FOR UPDATE SKIP LOCKED so concurrent sweeps don't contend.
func (r *Repository) ExpireAnonymousAgentTryouts(ctx context.Context, params ExpireAnonymousAgentTryoutsParams) (int64, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 500
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin agent tryout retention transaction: %w", err)
	}
	defer rollback(ctx, tx)

	// Collect the eligible rows in a scoped closure that defers rows.Close():
	// the cursor MUST be fully closed before the UPDATE/DELETE below, because a
	// single transaction can only have one query in flight at a time. Scoping
	// the deferred close here guarantees that ordering by construction.
	tryoutIDs, runIDs, err := func() ([]uuid.UUID, []uuid.UUID, error) {
		rows, err := tx.Query(ctx, `
			SELECT id, run_id
			FROM agent_tryouts
			WHERE expires_at IS NOT NULL
			  AND expires_at <= $1
			  AND claimed_by_user_id IS NULL
			  AND workspace_id IS NULL
			ORDER BY expires_at ASC, id ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		`, params.Now.UTC(), limit)
		if err != nil {
			return nil, nil, fmt.Errorf("select expired anonymous agent tryouts: %w", err)
		}
		defer rows.Close()

		var (
			tryoutIDs []uuid.UUID
			runIDs    []uuid.UUID
		)
		for rows.Next() {
			var (
				id    uuid.UUID
				runID *uuid.UUID
			)
			if err := rows.Scan(&id, &runID); err != nil {
				return nil, nil, fmt.Errorf("scan expired anonymous agent tryout: %w", err)
			}
			tryoutIDs = append(tryoutIDs, id)
			if runID != nil {
				runIDs = append(runIDs, *runID)
			}
		}
		if err := rows.Err(); err != nil {
			return nil, nil, fmt.Errorf("iterate expired anonymous agent tryouts: %w", err)
		}
		return tryoutIDs, runIDs, nil
	}()
	if err != nil {
		return 0, err
	}

	if len(tryoutIDs) == 0 {
		if err := tx.Commit(ctx); err != nil {
			return 0, fmt.Errorf("commit agent tryout retention transaction: %w", err)
		}
		return 0, nil
	}

	if len(runIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE artifacts
			SET retention_status = 'scheduled_for_deletion', updated_at = now()
			WHERE run_id = ANY($1::uuid[])
			  AND retention_status = 'active'
		`, runIDs); err != nil {
			return 0, fmt.Errorf("schedule expired anonymous artifacts for deletion: %w", err)
		}
	}

	tag, err := tx.Exec(ctx, `DELETE FROM agent_tryouts WHERE id = ANY($1::uuid[])`, tryoutIDs)
	if err != nil {
		return 0, fmt.Errorf("delete expired anonymous agent tryouts: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit agent tryout retention transaction: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *Repository) GetAgentTryoutByID(ctx context.Context, id uuid.UUID) (AgentTryout, error) {
	row, err := r.queries.GetAgentTryoutByID(ctx, repositorysqlc.GetAgentTryoutByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AgentTryout{}, ErrAgentTryoutNotFound
		}
		return AgentTryout{}, fmt.Errorf("get agent tryout by id: %w", err)
	}
	return mapAgentTryout(row)
}

func (r *Repository) ListAgentTryoutsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) ([]AgentTryout, error) {
	rows, err := r.queries.ListAgentTryoutsByWorkspaceID(ctx, repositorysqlc.ListAgentTryoutsByWorkspaceIDParams{
		WorkspaceID: &workspaceID,
		LimitCount:  limit,
		OffsetCount: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list agent tryouts by workspace id: %w", err)
	}
	items := make([]AgentTryout, 0, len(rows))
	for _, row := range rows {
		item, err := mapAgentTryout(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if items == nil {
		items = []AgentTryout{}
	}
	return items, nil
}

func (r *Repository) ClaimAgentTryout(ctx context.Context, params ClaimAgentTryoutParams) (AgentTryout, error) {
	row, err := r.queries.ClaimAgentTryout(ctx, repositorysqlc.ClaimAgentTryoutParams{
		ID:              params.ID,
		OrganizationID:  &params.OrganizationID,
		WorkspaceID:     &params.WorkspaceID,
		ClaimedByUserID: &params.ClaimedByUserID,
		ClaimedAt:       pgtype.Timestamptz{Time: params.ClaimedAt.UTC(), Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, lookupErr := r.GetAgentTryoutByID(ctx, params.ID); errors.Is(lookupErr, ErrAgentTryoutNotFound) {
				return AgentTryout{}, ErrAgentTryoutNotFound
			}
			return AgentTryout{}, ErrAgentTryoutAlreadyClaimed
		}
		return AgentTryout{}, fmt.Errorf("claim agent tryout: %w", err)
	}
	return mapAgentTryout(row)
}

func (r *Repository) UpdateAgentTryoutStatus(ctx context.Context, params UpdateAgentTryoutStatusParams) (AgentTryout, error) {
	actualCost, err := numericFromFloat(params.ActualCostUSD)
	if err != nil {
		return AgentTryout{}, fmt.Errorf("encode agent tryout actual cost: %w", err)
	}
	var redactionStatus *string
	if params.RedactionStatus != nil {
		value := string(*params.RedactionStatus)
		redactionStatus = &value
	}
	row, err := r.queries.UpdateAgentTryoutStatus(ctx, repositorysqlc.UpdateAgentTryoutStatusParams{
		ID:              params.ID,
		Status:          string(params.Status),
		Summary:         nullableJSON(params.Summary),
		ActualCostUsd:   actualCost,
		LatencyMs:       cloneInt64Ptr(params.LatencyMS),
		RedactionStatus: redactionStatus,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AgentTryout{}, ErrAgentTryoutNotFound
		}
		return AgentTryout{}, fmt.Errorf("update agent tryout status: %w", err)
	}
	return mapAgentTryout(row)
}

func (r *Repository) SetAgentTryoutRunID(ctx context.Context, id uuid.UUID, runID uuid.UUID) (AgentTryout, error) {
	row, err := r.queries.SetAgentTryoutRunID(ctx, repositorysqlc.SetAgentTryoutRunIDParams{
		ID:    id,
		RunID: &runID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AgentTryout{}, ErrAgentTryoutNotFound
		}
		return AgentTryout{}, fmt.Errorf("set agent tryout run id: %w", err)
	}
	return mapAgentTryout(row)
}

func (r *Repository) LinkAgentTryoutRunIfUnset(ctx context.Context, params LinkAgentTryoutRunParams) (AgentTryout, error) {
	status := params.Status
	if status == "" {
		status = AgentTryoutStatusRunning
	}
	row := r.db.QueryRow(ctx, `
UPDATE agent_tryouts
SET
    run_id = COALESCE(run_id, $2),
    status = CASE
        WHEN run_id IS NULL AND status = 'queued' THEN $3
        ELSE status
    END,
    summary = CASE
        WHEN run_id IS NULL AND $4::jsonb IS NOT NULL THEN $4::jsonb
        ELSE summary
    END
WHERE id = $1
RETURNING id, organization_id, workspace_id, template_slug, status, input_snapshot,
    template_snapshot, tool_policy_snapshot, evaluation_spec_snapshot, selected_model_policy,
    summary, redaction_status, run_id, cost_limit_usd, actual_cost_usd, latency_ms,
    max_duration_seconds, anonymous_fingerprint_hash, created_by_user_id,
    claimed_by_user_id, claimed_at, expires_at, created_at, updated_at`,
		params.ID, params.RunID, string(status), nullableJSON(params.Summary),
	)
	tryout, err := scanAgentTryoutRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AgentTryout{}, ErrAgentTryoutNotFound
		}
		return AgentTryout{}, fmt.Errorf("link agent tryout run: %w", err)
	}
	return tryout, nil
}

func (r *Repository) GetAgentHarnessExecutionByRunID(ctx context.Context, runID uuid.UUID) (AgentHarnessExecution, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, organization_id, workspace_id, agent_harness_id, created_by_user_id,
    run_id, run_agent_id, evaluation_spec_id, temporal_workflow_id, temporal_run_id, retry_of_execution_id, retry_idempotency_key,
    status, harness_snapshot, execution_config_snapshot, evaluation_config_snapshot,
    error_message, started_at, completed_at, cancelled_at, created_at, updated_at
FROM agent_harness_executions
WHERE run_id = $1
ORDER BY created_at ASC
LIMIT 1`, runID)
	return scanAgentHarnessExecution(row)
}

type agentTryoutScanner interface {
	Scan(dest ...any) error
}

func scanAgentTryoutRow(scanner agentTryoutScanner) (AgentTryout, error) {
	var row repositorysqlc.AgentTryout
	err := scanner.Scan(
		&row.ID,
		&row.OrganizationID,
		&row.WorkspaceID,
		&row.TemplateSlug,
		&row.Status,
		&row.InputSnapshot,
		&row.TemplateSnapshot,
		&row.ToolPolicySnapshot,
		&row.EvaluationSpecSnapshot,
		&row.SelectedModelPolicy,
		&row.Summary,
		&row.RedactionStatus,
		&row.RunID,
		&row.CostLimitUsd,
		&row.ActualCostUsd,
		&row.LatencyMs,
		&row.MaxDurationSeconds,
		&row.AnonymousFingerprintHash,
		&row.CreatedByUserID,
		&row.ClaimedByUserID,
		&row.ClaimedAt,
		&row.ExpiresAt,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		return AgentTryout{}, err
	}
	return mapAgentTryout(row)
}

func mapAgentTryout(row repositorysqlc.AgentTryout) (AgentTryout, error) {
	createdAt, err := requiredTime("agent_tryouts.created_at", row.CreatedAt)
	if err != nil {
		return AgentTryout{}, err
	}
	updatedAt, err := requiredTime("agent_tryouts.updated_at", row.UpdatedAt)
	if err != nil {
		return AgentTryout{}, err
	}
	return AgentTryout{
		ID:                       row.ID,
		OrganizationID:           cloneUUIDPtr(row.OrganizationID),
		WorkspaceID:              cloneUUIDPtr(row.WorkspaceID),
		TemplateSlug:             row.TemplateSlug,
		Status:                   AgentTryoutStatus(row.Status),
		InputSnapshot:            cloneJSON(row.InputSnapshot),
		TemplateSnapshot:         cloneJSON(row.TemplateSnapshot),
		ToolPolicySnapshot:       cloneJSON(row.ToolPolicySnapshot),
		EvaluationSpecSnapshot:   cloneJSON(row.EvaluationSpecSnapshot),
		SelectedModelPolicy:      cloneJSON(row.SelectedModelPolicy),
		Summary:                  cloneJSON(row.Summary),
		RedactionStatus:          AgentTryoutRedactionStatus(row.RedactionStatus),
		RunID:                    cloneUUIDPtr(row.RunID),
		CostLimitUSD:             derefFloat64(numericPtr(row.CostLimitUsd)),
		ActualCostUSD:            numericPtr(row.ActualCostUsd),
		LatencyMS:                cloneInt64Ptr(row.LatencyMs),
		MaxDurationSeconds:       row.MaxDurationSeconds,
		AnonymousFingerprintHash: cloneStringPtr(row.AnonymousFingerprintHash),
		CreatedByUserID:          cloneUUIDPtr(row.CreatedByUserID),
		ParentTryoutID:           cloneUUIDPtr(row.ParentTryoutID),
		ClaimedByUserID:          cloneUUIDPtr(row.ClaimedByUserID),
		ClaimedAt:                optionalTime(row.ClaimedAt),
		ExpiresAt:                optionalTime(row.ExpiresAt),
		CreatedAt:                createdAt,
		UpdatedAt:                updatedAt,
	}, nil
}

func mapAgentTryoutEvent(row repositorysqlc.AgentTryoutEvent) (AgentTryoutEvent, error) {
	occurredAt, err := requiredTime("agent_tryout_events.occurred_at", row.OccurredAt)
	if err != nil {
		return AgentTryoutEvent{}, err
	}
	return AgentTryoutEvent{
		ID:             row.ID,
		AgentTryoutID:  row.AgentTryoutID,
		SequenceNumber: row.SequenceNumber,
		EventType:      row.EventType,
		ActorType:      row.ActorType,
		OccurredAt:     occurredAt,
		Payload:        cloneJSON(row.Payload),
	}, nil
}
