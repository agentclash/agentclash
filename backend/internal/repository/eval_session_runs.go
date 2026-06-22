package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// jsonSemanticEqual reports whether two JSON documents are semantically equal (ignoring key order and
// insignificant whitespace), by decoding both to generic values. Used to compare a request's config
// snapshot against the jsonb form Postgres stored and re-canonicalized.
func jsonSemanticEqual(a, b json.RawMessage) bool {
	var av, bv any
	if err := json.Unmarshal(normalizeJSON(a), &av); err != nil {
		return false
	}
	if err := json.Unmarshal(normalizeJSON(b), &bv); err != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}

type BuildVersionRunnableDeployment struct {
	AgentBuildVersionID uuid.UUID
	Deployment          RunnableDeployment
}

type CreateEvalSessionWithQueuedRunsParams struct {
	// SessionID, when set, is the preallocated eval-session id (the guide confirmed path supplies it so
	// the whole session + N child runs + N reservations are idempotent across retry/re-entry on a stable
	// id). Zero ⇒ a fresh id is minted (REST/manual creates). Each child carries its own preallocated
	// RunID + optional EvalCreditReservation on CreateQueuedRunParams.
	SessionID       uuid.UUID
	Session         CreateEvalSessionParams
	Runs            []CreateQueuedRunParams
	EntitlementGate *RunEntitlementGate
}

type CreateEvalSessionWithQueuedRunsResult struct {
	Session domain.EvalSession
	Runs    []domain.Run
}

func (r *Repository) ListRunnableDeploymentsByBuildVersionID(
	ctx context.Context,
	workspaceID uuid.UUID,
	buildVersionIDs []uuid.UUID,
) ([]BuildVersionRunnableDeployment, error) {
	if len(buildVersionIDs) == 0 {
		return []BuildVersionRunnableDeployment{}, nil
	}

	// Mirror ListRunnableDeploymentsWithLatestSnapshot's frozen billing identity (4d-1): the build-version
	// path MUST surface source_provider_account_id (BYOK signal), provider/model identity, and the frozen
	// output rate — otherwise build-version eval-session participants are misclassified managed/BYOK or
	// priced at zero. LEFT JOINs so a lane with no managed alias still returns one row.
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (agent_deployments.id)
			agent_deployments.current_build_version_id,
			agent_deployments.id,
			agent_deployments.organization_id,
			agent_deployments.workspace_id,
			agent_deployments.name,
			agent_deployment_snapshots.id AS agent_deployment_snapshot_id,
			agent_deployments.spend_policy_id,
			agent_deployments.routing_policy_id,
			agent_deployment_snapshots.source_provider_account_id,
			model_catalog_entries.provider_key,
			model_catalog_entries.provider_model_id,
			model_aliases.output_cost_per_million_tokens
		FROM agent_deployments
		JOIN agent_deployment_snapshots
		  ON agent_deployment_snapshots.agent_deployment_id = agent_deployments.id
		 AND agent_deployment_snapshots.organization_id = agent_deployments.organization_id
		 AND agent_deployment_snapshots.workspace_id = agent_deployments.workspace_id
		LEFT JOIN model_aliases
		  ON model_aliases.id = agent_deployment_snapshots.source_model_alias_id
		LEFT JOIN model_catalog_entries
		  ON model_catalog_entries.id = model_aliases.model_catalog_entry_id
		WHERE agent_deployments.workspace_id = $1
		  AND agent_deployments.current_build_version_id = ANY($2::uuid[])
		  AND agent_deployments.status = 'active'
		  AND agent_deployments.archived_at IS NULL
		ORDER BY agent_deployments.id, agent_deployment_snapshots.created_at DESC, agent_deployment_snapshots.id DESC
	`, workspaceID, buildVersionIDs)
	if err != nil {
		return nil, fmt.Errorf("list runnable deployments by build version id: %w", err)
	}
	defer rows.Close()

	deployments := make([]BuildVersionRunnableDeployment, 0)
	for rows.Next() {
		var item BuildVersionRunnableDeployment
		var providerKey, providerModelID *string
		var outputRate pgtype.Numeric
		if err := rows.Scan(
			&item.AgentBuildVersionID,
			&item.Deployment.ID,
			&item.Deployment.OrganizationID,
			&item.Deployment.WorkspaceID,
			&item.Deployment.Name,
			&item.Deployment.AgentDeploymentSnapshotID,
			&item.Deployment.SpendPolicyID,
			&item.Deployment.RoutingPolicyID,
			&item.Deployment.SourceProviderAccountID,
			&providerKey,
			&providerModelID,
			&outputRate,
		); err != nil {
			return nil, fmt.Errorf("scan runnable deployment by build version id: %w", err)
		}
		// Convert the frozen rate carefully: NULL → 0 (the managed estimate blocks on a non-positive
		// rate), but an INVALID numeric is an error — never a silent 0 (same rule as the deployment path).
		rate, err := numericRateToFloat64(outputRate)
		if err != nil {
			return nil, fmt.Errorf("convert frozen output rate for deployment %s: %w", item.Deployment.ID, err)
		}
		item.Deployment.ProviderKey = derefString(providerKey)
		item.Deployment.ProviderModelID = derefString(providerModelID)
		item.Deployment.OutputCostPerMillionTokens = rate
		deployments = append(deployments, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runnable deployments by build version id: %w", err)
	}

	return deployments, nil
}

func (r *Repository) CreateEvalSessionWithQueuedRuns(
	ctx context.Context,
	params CreateEvalSessionWithQueuedRunsParams,
) (CreateEvalSessionWithQueuedRunsResult, error) {
	// Validate every child up front, before any side effect, so a malformed child fails the whole
	// session deterministically rather than mid-transaction.
	for _, runParams := range params.Runs {
		if err := validateCreateQueuedRunParams(runParams); err != nil {
			return CreateEvalSessionWithQueuedRunsResult{}, err
		}
	}

	// Stable session id: the guide confirmed path preallocates it (so the session + its child runs +
	// reservations are idempotent across retry); REST/manual creates mint a fresh one.
	preallocated := params.SessionID != uuid.Nil
	sessionID := params.SessionID
	if !preallocated {
		sessionID = uuid.New()
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, fmt.Errorf("begin eval session create transaction: %w", err)
	}
	defer rollback(ctx, tx)

	queries := r.queries.WithTx(tx)

	// Idempotency on the preallocated session id: if the session already exists, a prior attempt
	// committed the session + child runs + reservations — return them rather than failing on a
	// duplicate primary key (or double-debiting). But verify the SAME requested effect first: returning
	// a session created for different config/children/reservations would mask a corrupt or colliding
	// request.
	if preallocated {
		if existingRow, err := queries.GetEvalSessionByID(ctx, repositorysqlc.GetEvalSessionByIDParams{ID: sessionID}); err == nil {
			existingSession, mapErr := mapEvalSession(existingRow)
			if mapErr != nil {
				return CreateEvalSessionWithQueuedRunsResult{}, fmt.Errorf("map existing eval session %s: %w", sessionID, mapErr)
			}
			existingRuns, verifyErr := verifyExistingEvalSessionMatchesRequest(ctx, queries, tx, existingSession, params)
			if verifyErr != nil {
				return CreateEvalSessionWithQueuedRunsResult{}, verifyErr
			}
			return CreateEvalSessionWithQueuedRunsResult{Session: existingSession, Runs: existingRuns}, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return CreateEvalSessionWithQueuedRunsResult{}, fmt.Errorf("check existing eval session %s: %w", sessionID, err)
		}
	}

	if params.EntitlementGate != nil && len(params.Runs) > 0 {
		workspaceID := params.Runs[0].WorkspaceID
		if err := enforceRunEntitlementGate(ctx, tx, workspaceID, params.EntitlementGate); err != nil {
			return CreateEvalSessionWithQueuedRunsResult{}, err
		}
	}
	sessionRow, err := queries.CreateEvalSession(ctx, repositorysqlc.CreateEvalSessionParams{
		ID:                     sessionID,
		Status:                 string(domain.EvalSessionStatusQueued),
		Repetitions:            params.Session.Repetitions,
		AggregationConfig:      normalizeJSON(params.Session.AggregationConfig),
		SuccessThresholdConfig: normalizeJSON(params.Session.SuccessThresholdConfig),
		RoutingTaskSnapshot:    normalizeJSON(params.Session.RoutingTaskSnapshot),
		SchemaVersion:          params.Session.SchemaVersion,
	})
	if err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, fmt.Errorf("create eval session: %w", err)
	}

	session, err := mapEvalSession(sessionRow)
	if err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, fmt.Errorf("map eval session: %w", err)
	}

	queuedAt := time.Now().UTC()
	runs := make([]domain.Run, 0, len(params.Runs))
	for idx, runParams := range params.Runs {
		// Reserve each managed child's eval credit in the SAME tx as session+run creation: insufficient
		// credit on ANY child rolls back the whole session — no session, no child runs, no reservations
		// (4d-3, pin #5). BYOK children carry no reservation. Mirrors CreateQueuedRun's reserve-before-create.
		if runParams.EvalCreditReservation != nil && runParams.EvalCreditReservation.AmountMicros > 0 {
			if runParams.RunID == uuid.Nil {
				return CreateEvalSessionWithQueuedRunsResult{}, fmt.Errorf("child run %d: a managed eval-credit reservation requires a preallocated run id", idx+1)
			}
			childRunID := runParams.RunID
			if _, err := r.reserveEvalCreditInTx(ctx, tx, runParams.OrganizationID, runParams.EvalCreditReservation.Key,
				runParams.EvalCreditReservation.AmountMicros, CreditRef{RunID: &childRunID, Reason: "vibe eval session child run reservation"}); err != nil {
				return CreateEvalSessionWithQueuedRunsResult{}, err
			}
		}

		result, err := createQueuedRunWithQueries(ctx, queries, runParams, queuedAt)
		if err != nil {
			return CreateEvalSessionWithQueuedRunsResult{}, fmt.Errorf("create queued child run %d: %w", idx+1, err)
		}

		runRow, err := queries.AttachRunToEvalSession(ctx, repositorysqlc.AttachRunToEvalSessionParams{
			EvalSessionID: &session.ID,
			ID:            result.Run.ID,
		})
		if err != nil {
			return CreateEvalSessionWithQueuedRunsResult{}, fmt.Errorf("attach child run %d to eval session: %w", idx+1, err)
		}

		run, err := mapRun(runRow)
		if err != nil {
			return CreateEvalSessionWithQueuedRunsResult{}, fmt.Errorf("map attached child run %d: %w", idx+1, err)
		}
		runs = append(runs, run)
	}

	if err := tx.Commit(ctx); err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, fmt.Errorf("commit eval session create transaction: %w", err)
	}

	return CreateEvalSessionWithQueuedRunsResult{
		Session: session,
		Runs:    runs,
	}, nil
}

// ErrEvalSessionIdempotencyMismatch means a preallocated eval-session id already exists but for a
// DIFFERENT requested effect (session config, child run set, or per-child reservation) — the retry is
// not the same operation, so returning the existing session would mask a corrupt or colliding request.
var ErrEvalSessionIdempotencyMismatch = errors.New("preallocated eval session id exists for a different request")

// verifyExistingEvalSessionMatchesRequest proves a pre-existing eval session (found on the idempotent
// retry path) is the same effect the caller is requesting — session config, the exact child run set
// (matched by preallocated run id), each child's run-shaping fields + reservation, and that BYOK
// children carry no stray managed reservation — so returning it as success is safe (4d-3 idempotency).
func verifyExistingEvalSessionMatchesRequest(
	ctx context.Context,
	qtx *repositorysqlc.Queries,
	tx pgx.Tx,
	session domain.EvalSession,
	params CreateEvalSessionWithQueuedRunsParams,
) ([]domain.Run, error) {
	if session.Repetitions != params.Session.Repetitions || session.SchemaVersion != params.Session.SchemaVersion {
		return nil, fmt.Errorf("%w: session %s config differs (repetitions/schema_version)", ErrEvalSessionIdempotencyMismatch, session.ID)
	}
	// Compare config snapshots SEMANTICALLY: the stored values are jsonb, which Postgres re-canonicalizes
	// (key order / whitespace), so a raw byte compare against re-normalized params would spuriously differ.
	if !jsonSemanticEqual(params.Session.AggregationConfig, session.AggregationConfig.Document) ||
		!jsonSemanticEqual(params.Session.SuccessThresholdConfig, session.SuccessThresholdConfig.Document) ||
		!jsonSemanticEqual(params.Session.RoutingTaskSnapshot, session.RoutingTaskSnapshot.Document) {
		return nil, fmt.Errorf("%w: session %s config snapshot differs", ErrEvalSessionIdempotencyMismatch, session.ID)
	}

	rows, err := qtx.ListRunsByEvalSessionID(ctx, repositorysqlc.ListRunsByEvalSessionIDParams{EvalSessionID: &session.ID})
	if err != nil {
		return nil, fmt.Errorf("load existing child runs for session %s: %w", session.ID, err)
	}
	if len(rows) != len(params.Runs) {
		return nil, fmt.Errorf("%w: session %s has %d child runs, request has %d", ErrEvalSessionIdempotencyMismatch, session.ID, len(rows), len(params.Runs))
	}
	existingByID := make(map[uuid.UUID]domain.Run, len(rows))
	orderedRuns := make([]domain.Run, 0, len(rows))
	for _, row := range rows {
		run, mapErr := mapRun(row)
		if mapErr != nil {
			return nil, fmt.Errorf("map existing child run %s: %w", row.ID, mapErr)
		}
		existingByID[run.ID] = run
	}

	for _, childParams := range params.Runs {
		if childParams.RunID == uuid.Nil {
			return nil, fmt.Errorf("%w: session %s idempotency requires preallocated child run ids", ErrEvalSessionIdempotencyMismatch, session.ID)
		}
		existing, ok := existingByID[childParams.RunID]
		if !ok {
			return nil, fmt.Errorf("%w: session %s has no child run %s", ErrEvalSessionIdempotencyMismatch, session.ID, childParams.RunID)
		}
		if err := verifyExistingRunMatchesRequest(ctx, qtx, tx, existing, childParams); err != nil {
			return nil, err
		}
		// A BYOK child (no reservation requested) must not carry a stray managed reservation for its run
		// id — otherwise a corrupt prior attempt could silently hold credit against a BYOK run.
		if childParams.EvalCreditReservation == nil {
			var count int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM org_eval_credit_reservations WHERE run_id = $1`, childParams.RunID).Scan(&count); err != nil {
				return nil, fmt.Errorf("check stray reservation for byok child %s: %w", childParams.RunID, err)
			}
			if count != 0 {
				return nil, fmt.Errorf("%w: byok child run %s has %d unexpected reservation(s)", ErrEvalSessionIdempotencyMismatch, childParams.RunID, count)
			}
		}
		orderedRuns = append(orderedRuns, existing)
	}
	return orderedRuns, nil
}
