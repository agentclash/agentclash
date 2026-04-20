package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	repositorysqlc "github.com/Atharva-Kanherkar/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
)

type BuildVersionRunnableDeployment struct {
	AgentBuildVersionID uuid.UUID
	Deployment          RunnableDeployment
}

type CreateEvalSessionWithQueuedRunsParams struct {
	Session CreateEvalSessionParams
	Runs    []CreateQueuedRunParams
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

	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (agent_deployments.id)
			agent_deployments.current_build_version_id,
			agent_deployments.id,
			agent_deployments.organization_id,
			agent_deployments.workspace_id,
			agent_deployments.name,
			agent_deployment_snapshots.id AS agent_deployment_snapshot_id,
			agent_deployments.spend_policy_id,
			agent_deployments.routing_policy_id
		FROM agent_deployments
		JOIN agent_deployment_snapshots
		  ON agent_deployment_snapshots.agent_deployment_id = agent_deployments.id
		 AND agent_deployment_snapshots.organization_id = agent_deployments.organization_id
		 AND agent_deployment_snapshots.workspace_id = agent_deployments.workspace_id
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
		if err := rows.Scan(
			&item.AgentBuildVersionID,
			&item.Deployment.ID,
			&item.Deployment.OrganizationID,
			&item.Deployment.WorkspaceID,
			&item.Deployment.Name,
			&item.Deployment.AgentDeploymentSnapshotID,
			&item.Deployment.SpendPolicyID,
			&item.Deployment.RoutingPolicyID,
		); err != nil {
			return nil, fmt.Errorf("scan runnable deployment by build version id: %w", err)
		}
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
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, fmt.Errorf("begin eval session create transaction: %w", err)
	}
	defer rollback(ctx, tx)

	queries := r.queries.WithTx(tx)
	sessionRow, err := queries.CreateEvalSession(ctx, repositorysqlc.CreateEvalSessionParams{
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
		if err := validateCreateQueuedRunParams(runParams); err != nil {
			return CreateEvalSessionWithQueuedRunsResult{}, err
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
