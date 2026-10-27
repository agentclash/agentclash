package repository_test

import (
	"context"
	"errors"
	"github.com/agentclash/agentclash/runtime/provider"
	"net/http"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/google/uuid"
)

// Transaction owner: a preview must not become differently configured children.
func TestControlledExperimentAdmissionAndContext(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	f := seedFixture(t, ctx, db)
	if _, err := db.Exec(ctx, `UPDATE challenge_pack_versions SET manifest=manifest || '{"evaluation_spec":{"name":"controlled","version_number":1,"judge_mode":"deterministic","validators":[{"key":"exact","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"scorecard":{"dimensions":["correctness"]}}}'::jsonb WHERE id=$1`, f.challengePackVersionID); err != nil {
		t.Fatal(err)
	}
	repo := repository.New(db)
	settings, err := repo.PreviewControlledSettings(ctx, f.workspaceID, f.agentDeploymentSnapshotID, f.challengePackVersionID, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := repository.CreateEvalSessionWithQueuedRunsParams{
		Session:   repository.CreateEvalSessionParams{WorkspaceID: f.workspaceID, Repetitions: 2, SchemaVersion: 1},
		RequestID: uuid.New(), RequestHash: "request-a",
		Runs: []repository.CreateQueuedRunParams{{OrganizationID: f.organizationID, WorkspaceID: f.workspaceID, ChallengePackVersionID: f.challengePackVersionID, ChallengeInputSetID: &f.challengeInputSetID, OfficialPackMode: domain.OfficialPackModeFull, CreatedByUserID: &f.userID, Name: "controlled", ExecutionMode: "single_agent", ExecutionPlan: []byte(`{}`), RunAgents: []repository.CreateQueuedRunAgentParams{{AgentDeploymentID: f.agentDeploymentID, AgentDeploymentSnapshotID: f.agentDeploymentSnapshotID, Label: "sample"}}, ExpectedSettings: map[uuid.UUID]string{f.agentDeploymentSnapshotID: settings.Hash}, ExpectedDeploymentSnapshots: map[uuid.UUID]uuid.UUID{f.agentDeploymentID: f.agentDeploymentSnapshotID}}},
	}
	request.Runs = append(request.Runs, request.Runs[0])
	if _, err = db.Exec(ctx, "UPDATE runtime_profiles SET max_tool_calls=max_tool_calls+1 WHERE id=$1", f.runtimeProfileID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.CreateEvalSessionWithQueuedRuns(ctx, request); !errors.Is(err, repository.ErrControlledSettingsChanged) {
		t.Fatalf("changed preview admitted: %v", err)
	}
	var count int
	if err = db.QueryRow(ctx, "SELECT count(*) FROM eval_sessions WHERE workspace_id=$1", f.workspaceID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial admission count=%d err=%v", count, err)
	}
	settings, err = repo.PreviewControlledSettings(ctx, f.workspaceID, f.agentDeploymentSnapshotID, f.challengePackVersionID, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Runs[0].ExpectedSettings[f.agentDeploymentSnapshotID] = settings.Hash
	admitted, err := repo.CreateEvalSessionWithQueuedRuns(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := repo.CreateEvalSessionWithQueuedRuns(ctx, request)
	if err != nil || retry.Session.ID != admitted.Session.ID || len(retry.Runs) != 2 {
		t.Fatalf("retry duplicated batch: %+v %v", retry, err)
	}
	request.RequestHash = "changed"
	if _, err = repo.CreateEvalSessionWithQueuedRuns(ctx, request); !errors.Is(err, repository.ErrExperimentRequestConflict) {
		t.Fatalf("changed idempotency body accepted: %v", err)
	}
	// Change the actual latest snapshot between resolution and the transaction.
	// The old expected snapshot cannot create new children, while a committed
	// request still recovers its original immutable execution.
	latest := uuid.New()
	if _, err = db.Exec(ctx, `INSERT INTO agent_deployment_snapshots SELECT (jsonb_populate_record(NULL::agent_deployment_snapshots,to_jsonb(s)||jsonb_build_object('id',$2::uuid,'snapshot_hash',$3::text,'created_at',now()+interval '1 second'))).* FROM agent_deployment_snapshots s WHERE id=$1`, f.agentDeploymentSnapshotID, latest, latest.String()); err != nil {
		t.Fatal(err)
	}
	request.RequestHash = "request-a"
	recovered, err := repo.CreateEvalSessionWithQueuedRuns(ctx, request)
	if err != nil || recovered.Session.ID != admitted.Session.ID {
		t.Fatal("committed source was readmitted against the latest deployment", err)
	}
	request.RequestID = uuid.New()
	if _, err = repo.CreateEvalSessionWithQueuedRuns(ctx, request); !errors.Is(err, repository.ErrControlledSettingsChanged) {
		t.Fatal("stale expected snapshot admitted", err)
	}
	if err = db.QueryRow(ctx, `SELECT count(*) FROM eval_sessions WHERE workspace_id=$1`, f.workspaceID).Scan(&count); err != nil || count != 1 {
		t.Fatal("snapshot rejection left children", count, err)
	}
	agents, err := repo.ListRunAgentsByRunID(ctx, admitted.Runs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	execution, err := repo.GetRunAgentExecutionContextByID(ctx, agents[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if execution.ValidateControlledDispatch == nil {
		t.Fatal("admitted control missing execution guard")
	}
	sent := 0
	client := provider.NewOpenAICompatibleClient(&http.Client{Transport: controlledRoundTrip(func(*http.Request) (*http.Response, error) { sent++; return nil, errors.New("unexpected transport") })}, "https://old.example/v1", controlledCredential(func(context.Context, string) (string, error) {
		_, e := db.Exec(ctx, "UPDATE provider_accounts SET base_url='https://changed.example/v1' WHERE id=$1", f.providerAccountID)
		return "rotated-live-credential", e
	}))
	req := provider.Request{ProviderKey: "openai", Model: execution.Deployment.ModelID, Messages: []provider.Message{{Role: "user", Content: "fixture"}}, ValidateBeforeDispatch: execution.ValidateControlledDispatch, OnDispatchValidated: execution.AcknowledgeControlledDispatch}
	for retry := 0; retry < 2; retry++ {
		if _, e := client.InvokeModel(ctx, req); !errors.Is(e, repository.ErrControlledSettingsChanged) {
			t.Fatal("stale credential send guard", e)
		}
	}
	var acknowledged bool
	if e := db.QueryRow(ctx, `SELECT dispatch_validated_at IS NOT NULL FROM run_agent_controlled_settings WHERE run_agent_id=$1`, agents[0].ID).Scan(&acknowledged); e != nil || acknowledged {
		t.Fatal("blocked dispatch falsely acknowledged", acknowledged, e)
	}
	if sent != 0 {
		t.Fatal("rotated live credential sent to stale origin", sent)
	}

	if err = execution.ValidateControlledDispatch(ctx); !errors.Is(err, repository.ErrControlledSettingsChanged) {
		t.Fatalf("stale origin dispatch allowed: %v", err)
	}
	if _, err = repo.GetRunAgentExecutionContextByID(ctx, agents[0].ID); !errors.Is(err, repository.ErrControlledSettingsChanged) {
		t.Fatalf("stale activity context allowed: %v", err)
	}
}

type controlledRoundTrip func(*http.Request) (*http.Response, error)

func (f controlledRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type controlledCredential func(context.Context, string) (string, error)

func (f controlledCredential) Resolve(c context.Context, s string) (string, error) { return f(c, s) }
