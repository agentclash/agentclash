package api

import (
	"context"
	"errors"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
	"testing"
)

type lostControlledChildAck struct {
	next EvalSessionCreator
	lost bool
}

func (s *lostControlledChildAck) CreateEvalSession(ctx context.Context, c Caller, in CreateEvalSessionInput) (CreateEvalSessionResult, error) {
	v, e := s.next.CreateEvalSession(ctx, c, in)
	if e == nil && !s.lost {
		s.lost = true
		return CreateEvalSessionResult{}, errors.New("lost child acknowledgment")
	}
	return v, e
}
func TestControlledFleetRecoversAdmittedChild(t *testing.T) {
	h, ws, artifact, run, agent := canonicalHarness(t)
	useDeterministicCanonicalEvaluator(t, h, run)
	repo := repository.New(h.db)
	var pack, inputSet, snapshot, deployment uuid.UUID
	if err := h.db.QueryRow(h.ctx, `SELECT r.challenge_pack_version_id,r.challenge_input_set_id,a.agent_deployment_snapshot_id,a.agent_deployment_id FROM runs r JOIN run_agents a ON a.run_id=r.id WHERE a.id=$1`, agent).Scan(&pack, &inputSet, &snapshot, &deployment); err != nil {
		t.Fatal(err)
	}
	binding, err := repo.GetConnector(h.ctx, ws, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ApproveConnector(h.ctx, binding, h.user); err != nil {
		t.Fatal(err)
	}
	manager := NewRunCreationManager(allowWorkspaceAuthorizer{}, repo, nil, nil)
	caller := Caller{UserID: h.user}
	request := CreateEvalSessionInput{WorkspaceID: ws, ChallengePackVersionID: pack, ChallengeInputSetID: &inputSet, Participants: []EvalSessionParticipantInput{{AgentDeploymentID: &deployment, Label: "application"}}, Name: "fixed Fleet", ExecutionMode: "single_agent", DeploymentSnapshotIDs: map[uuid.UUID]uuid.UUID{deployment: snapshot}, PreviewOnly: true, EvalSession: CreateEvalSessionConfigInput{Repetitions: 2, SchemaVersion: 1, Aggregation: EvalSessionAggregationInput{Method: "mean", ReportVariance: true, ConfidenceInterval: .95}}}
	preview, err := manager.CreateEvalSession(h.ctx, caller, request)
	if err != nil {
		t.Fatal("preview", err)
	}
	request.PreviewOnly = false
	request.AdmissionRequestID = uuid.New()
	request.ControlledSettings = preview.Preview.ControlledSettings
	lost := &lostControlledChildAck{next: manager}
	starter := &fakeEvalSetStarter{}
	fleet := NewEvalSetManager(allowWorkspaceAuthorizer{}).WithPersistence(repo, lost, starter)
	first, ids, err := fleet.CreateControlled(h.ctx, caller, request)
	if err == nil || first.ID == uuid.Nil || len(ids) != 0 {
		t.Fatal("expected persisted outer with lost child ack", first, ids, err)
	}
	var links int
	if err = h.db.QueryRow(h.ctx, `SELECT count(*) FROM eval_set_sessions WHERE eval_set_id=$1`, first.ID).Scan(&links); err != nil || links != 1 {
		t.Fatal("child and parent link were not atomic", links, err)
	}
	var children int
	if err = h.db.QueryRow(h.ctx, `SELECT count(*) FROM eval_sessions WHERE workspace_id=$1`, ws).Scan(&children); err != nil || children != 1 {
		t.Fatal(children, err)
	}
	second, ids, err := fleet.CreateControlled(h.ctx, caller, request)
	if err != nil || second.ID != first.ID || len(ids) != 1 {
		t.Fatal("failed durable recovery", second, ids, err)
	}
	third, again, err := fleet.CreateControlled(h.ctx, caller, request)
	if err != nil || third.ID != first.ID || len(again) != 1 || again[0] != ids[0] {
		t.Fatal("duplicated linked child", third, again, err)
	}
	started := len(starter.started)
	if _, err = fleet.Cancel(h.ctx, caller, first.ID); err != nil {
		t.Fatal(err)
	}
	cancelled, retained, err := fleet.CreateControlled(h.ctx, caller, request)
	if err != nil || cancelled.Status != "cancelled" || len(retained) != 1 || retained[0] != ids[0] || len(starter.started) != started {
		t.Fatal("cancelled same-request recovery did not retain the committed child", cancelled, retained, err)
	}
	var runs, receipts int
	if err = h.db.QueryRow(h.ctx, `SELECT count(*) FROM runs WHERE eval_session_id=$1`, ids[0]).Scan(&runs); err != nil || runs != 2 {
		t.Fatal(runs, err)
	}
	if err = h.db.QueryRow(h.ctx, `SELECT count(*) FROM run_agent_controlled_settings c JOIN run_agents a ON a.id=c.run_agent_id JOIN runs r ON r.id=a.run_id WHERE r.eval_session_id=$1`, ids[0]).Scan(&receipts); err != nil || receipts != 2 {
		t.Fatal(receipts, err)
	}
	projection, err := h.svc.Store.CanonicalResult(h.ctx, h.actor, h.session.ID, vibe.ResultRef{Kind: vibe.ResultEvalSession, ID: ids[0]}, &artifact.ID)
	if err != nil || projection.Fleet == nil || projection.Fleet.ID != first.ID || len(projection.Fleet.SessionIDs) != 1 {
		t.Fatal("bound Fleet source unavailable", projection.Fleet, err)
	}
	request.Name = "different body"
	if _, _, err = fleet.CreateControlled(h.ctx, caller, request); !errors.Is(err, repository.ErrExperimentRequestConflict) {
		t.Fatal("changed outer receipt accepted", err)
	}
	_ = run
}

type cancelBeforeControlledChild struct {
	next   EvalSessionCreator
	cancel func() error
}

func (s cancelBeforeControlledChild) CreateEvalSession(ctx context.Context, c Caller, in CreateEvalSessionInput) (CreateEvalSessionResult, error) {
	if err := s.cancel(); err != nil {
		return CreateEvalSessionResult{}, err
	}
	return s.next.CreateEvalSession(ctx, c, in)
}
func TestControlledFleetCancellationPreventsLateChildAdmission(t *testing.T) {
	h, ws, _, run, agent := canonicalHarness(t)
	useDeterministicCanonicalEvaluator(t, h, run)
	repo := repository.New(h.db)
	var pack, inputs, snapshot, deployment uuid.UUID
	if err := h.db.QueryRow(h.ctx, `SELECT r.challenge_pack_version_id,r.challenge_input_set_id,a.agent_deployment_snapshot_id,a.agent_deployment_id FROM runs r JOIN run_agents a ON a.run_id=r.id WHERE a.id=$1`, agent).Scan(&pack, &inputs, &snapshot, &deployment); err != nil {
		t.Fatal(err)
	}
	binding, err := repo.GetConnector(h.ctx, ws, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ApproveConnector(h.ctx, binding, h.user); err != nil {
		t.Fatal(err)
	}
	manager := NewRunCreationManager(allowWorkspaceAuthorizer{}, repo, nil, nil)
	caller := Caller{UserID: h.user}
	in := CreateEvalSessionInput{WorkspaceID: ws, ChallengePackVersionID: pack, ChallengeInputSetID: &inputs, Participants: []EvalSessionParticipantInput{{AgentDeploymentID: &deployment}}, ExecutionMode: "single_agent", PreviewOnly: true, DeploymentSnapshotIDs: map[uuid.UUID]uuid.UUID{deployment: snapshot}, EvalSession: CreateEvalSessionConfigInput{Repetitions: 2, SchemaVersion: 1}}
	preview, err := manager.CreateEvalSession(h.ctx, caller, in)
	if err != nil {
		t.Fatal(err)
	}
	in.PreviewOnly = false
	in.AdmissionRequestID = uuid.New()
	in.ControlledSettings = preview.Preview.ControlledSettings
	starter := &fakeEvalSetStarter{}
	fleet := NewEvalSetManager(allowWorkspaceAuthorizer{}).WithPersistence(repo, cancelBeforeControlledChild{next: manager, cancel: func() error {
		_, e := h.db.Exec(h.ctx, `UPDATE eval_sets SET status='cancelled' WHERE workspace_id=$1`, ws)
		return e
	}}, starter)
	_, _, _ = fleet.CreateControlled(h.ctx, caller, in)
	var count int
	if err = h.db.QueryRow(h.ctx, `SELECT count(*) FROM eval_sessions WHERE workspace_id=$1`, ws).Scan(&count); err != nil || count != 0 {
		t.Fatalf("late child admitted after parent cancellation: %d %v", count, err)
	}
	if len(starter.started) != 0 {
		t.Fatal("cancelled Fleet started")
	}
}

func TestControlledPreviewAndAdmissionRespectReadableParticipantBound(t *testing.T) {
	h, ws, _, run, agent := canonicalHarness(t)
	useDeterministicCanonicalEvaluator(t, h, run)
	repo := repository.New(h.db)
	var pack, inputs, snapshot, deployment uuid.UUID
	if err := h.db.QueryRow(h.ctx, `SELECT r.challenge_pack_version_id,r.challenge_input_set_id,a.agent_deployment_snapshot_id,a.agent_deployment_id FROM runs r JOIN run_agents a ON a.run_id=r.id WHERE a.id=$1`, agent).Scan(&pack, &inputs, &snapshot, &deployment); err != nil {
		t.Fatal(err)
	}
	binding, err := repo.GetConnector(h.ctx, ws, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ApproveConnector(h.ctx, binding, h.user); err != nil {
		t.Fatal(err)
	}
	manager := NewRunCreationManager(allowWorkspaceAuthorizer{}, repo, nil, nil)
	caller := Caller{UserID: h.user}
	in := CreateEvalSessionInput{WorkspaceID: ws, ChallengePackVersionID: pack, ChallengeInputSetID: &inputs, Participants: []EvalSessionParticipantInput{{AgentDeploymentID: &deployment, Label: "A"}, {AgentDeploymentID: &deployment, Label: "B"}}, ExecutionMode: "comparison", PreviewOnly: true, DeploymentSnapshotIDs: map[uuid.UUID]uuid.UUID{deployment: snapshot}, EvalSession: CreateEvalSessionConfigInput{Repetitions: 50, SchemaVersion: 1}}
	preview, err := manager.CreateEvalSession(h.ctx, caller, in)
	if err != nil || preview.Preview.ParticipantSamples != 100 {
		t.Fatal("readable boundary unavailable", preview.Preview, err)
	}
	in.EvalSession.Repetitions = 51
	check := func(err error) {
		t.Helper()
		var invalid RunCreationValidationError
		if !errors.As(err, &invalid) || invalid.Code != "experiment_scope_limit" {
			t.Fatalf("unreadable scope not rejected by resolved participant owner: %v", err)
		}
	}
	_, err = manager.CreateEvalSession(h.ctx, caller, in)
	check(err)
	in.PreviewOnly = false
	in.AdmissionRequestID = uuid.New()
	in.ControlledSettings = preview.Preview.ControlledSettings
	_, err = manager.CreateEvalSession(h.ctx, caller, in)
	check(err)
	fleet := NewEvalSetManager(allowWorkspaceAuthorizer{}).WithPersistence(repo, manager, &fakeEvalSetStarter{})
	_, _, err = fleet.CreateControlled(h.ctx, caller, in)
	check(err)
	var count int
	if err = h.db.QueryRow(h.ctx, `SELECT count(*) FROM eval_sessions WHERE workspace_id=$1`, ws).Scan(&count); err != nil || count != 0 {
		t.Fatal("oversize scope partially admitted", count, err)
	}
}
