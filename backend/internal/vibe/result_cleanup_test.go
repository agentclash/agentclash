package vibe

import (
	"context"
	"github.com/google/uuid"
	"testing"
)

func TestIntegrationResultEvidenceCleanup(t *testing.T) {
	s := integrationStore(t)
	root := anonSession(t, s)
	ctx := context.Background()
	v, err := s.CreateAgent(ctx, root.Actor, nil, uuid.New(), "test", DefaultModels())
	if err != nil {
		t.Fatal(err)
	}
	cleanupSession(t, s, v.ID)
	sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: "saved", Models: DefaultModels()}
	op, err := s.Submit(ctx, v.Actor, v.ID, sub, Plan{Submission: sub, Anonymous: true, Calls: 1, MaxCost: 1}, testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Stop(ctx, v.Actor, op.ID); err != nil {
		t.Fatal(err)
	}
	// At the deletion boundary the payload format is immaterial: every content
	// table must be purged, while the existing accounting journal remains.
	for i, query := range []string{
		`INSERT INTO vibe_evidence_receipts(session_id,client_id,request_hash,evidence_id,artifact_id) VALUES($1,$2,'receipt',$2,$2)`,
		`INSERT INTO vibe_grade_reviews(id,session_id,client_id,request_hash,result_kind,result_id,body) VALUES($2,$1,$2,'review','vibe_operation',$3,'{"private":"review"}')`,
		`INSERT INTO vibe_validation_reviews(id,session_id,operation_id,stage_key,candidate_hash,policy_hash,body) VALUES($2,$1,$3,'review','candidate','policy','{"private":"candidate"}')`,
	} {
		args := []any{v.ID, uuid.New()}
		if i > 0 {
			args = append(args, op.ID)
		}
		if _, err = s.DB.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	v, err = s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DeleteProject(ctx, v.Actor, v.ID, v.Revision); err != nil {
		t.Fatal(err)
	}
	if err = s.CleanupProjects(ctx); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"vibe_evidence_receipts", "vibe_grade_reviews", "vibe_validation_reviews"} {
		var n int
		if err = s.DB.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE session_id=$1", v.ID).Scan(&n); err != nil || n != 0 {
			t.Fatal("project content retained", table, n, err)
		}
	}
}
