package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/google/uuid"
)

// Durable admission retries recover the original children rather than charging twice.
func TestEvalSessionAdmissionRecoversOriginalChildren(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	f := seedFixture(t, ctx, db)
	repo := repository.New(db)
	request := repository.CreateEvalSessionWithQueuedRunsParams{
		Session:   repository.CreateEvalSessionParams{WorkspaceID: f.workspaceID, Repetitions: 2, SchemaVersion: 1},
		RequestID: uuid.New(), RequestHash: "request-a",
		Runs: []repository.CreateQueuedRunParams{{OrganizationID: f.organizationID, WorkspaceID: f.workspaceID, ChallengePackVersionID: f.challengePackVersionID, ChallengeInputSetID: &f.challengeInputSetID, OfficialPackMode: domain.OfficialPackModeFull, CreatedByUserID: &f.userID, Name: "repetitions", ExecutionMode: "single_agent", ExecutionPlan: []byte(`{}`), RunAgents: []repository.CreateQueuedRunAgentParams{{AgentDeploymentID: f.agentDeploymentID, AgentDeploymentSnapshotID: f.agentDeploymentSnapshotID, Label: "sample"}}}},
	}
	request.Runs = append(request.Runs, request.Runs[0])
	admitted, err := repo.CreateEvalSessionWithQueuedRuns(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := repo.CreateEvalSessionWithQueuedRuns(ctx, request)
	if err != nil || retry.Session.ID != admitted.Session.ID || len(retry.Runs) != 2 {
		t.Fatalf("retry duplicated batch: %+v %v", retry, err)
	}
	request.RequestHash = "changed"
	if _, err = repo.CreateEvalSessionWithQueuedRuns(ctx, request); !errors.Is(err, repository.ErrEvalSessionAdmissionConflict) {
		t.Fatalf("changed idempotency body accepted: %v", err)
	}
	// Mutable deployment updates cannot cause a committed request to be readmitted.
	latest := uuid.New()
	if _, err = db.Exec(ctx, `INSERT INTO agent_deployment_snapshots SELECT (jsonb_populate_record(NULL::agent_deployment_snapshots,to_jsonb(s)||jsonb_build_object('id',$2::uuid,'snapshot_hash',$3::text,'created_at',now()+interval '1 second'))).* FROM agent_deployment_snapshots s WHERE id=$1`, f.agentDeploymentSnapshotID, latest, latest.String()); err != nil {
		t.Fatal(err)
	}
	request.RequestHash = "request-a"
	recovered, err := repo.CreateEvalSessionWithQueuedRuns(ctx, request)
	if err != nil || recovered.Session.ID != admitted.Session.ID {
		t.Fatal("committed source was readmitted against the latest deployment", err)
	}
	var count int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM eval_sessions WHERE workspace_id=$1`, f.workspaceID).Scan(&count); err != nil || count != 1 {
		t.Fatal("retry created duplicate children", count, err)
	}
	agents, err := repo.ListRunAgentsByRunID(ctx, admitted.Runs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	// Historical controlled jobs must not be resumed with the retired guards removed.
	if _, err = db.Exec(ctx, `INSERT INTO run_agent_controlled_settings(run_agent_id,snapshot_id,settings_hash,settings) VALUES($1,$2,'retired','{}')`, agents[0].ID, f.agentDeploymentSnapshotID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.GetRunAgentExecutionContextByID(ctx, agents[0].ID); !errors.Is(err, repository.ErrFrozenExecutionContext) {
		t.Fatalf("retired controlled job allowed to execute: %v", err)
	}
}
