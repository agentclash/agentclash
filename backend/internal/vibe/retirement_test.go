package vibe

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestIntegrationRetirementUsesDocumentFormatAndIsRepeatable(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	old := anonSession(t, s)
	other := anonSession(t, s)
	if _, err := s.DB.Exec(ctx, `UPDATE vibe_sessions SET document='{"format_version":2,"messages":[{"content":"private old chat"}]}' WHERE id=$1`, other.ID); err != nil {
		t.Fatal(err)
	}
	current, err := s.CreateAgent(ctx, old.Actor, nil, uuid.New(), "test", DefaultModels())
	if err != nil {
		t.Fatal(err)
	}
	cleanupSession(t, s, current.ID)
	_, err = s.DB.Exec(ctx, `INSERT INTO vibe_operations(id,session_id,actor,client_id,request_hash,kind,state,billing,models,input,max_cost,actual_cost,deadline) VALUES($1,$2,$3,$4,'kept-improve','message','COMPLETED','SETTLED','{}','{"authoring_version":15,"content":"current instructions"}',1,1,now())`, uuid.New(), current.ID, current.Actor, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	// A zero-cost attempt must retain the accounting-only free marker when
	// its inference policy/content is scrubbed (the database enforces this).
	freeID := uuid.New()
	_, err = s.DB.Exec(ctx, `INSERT INTO vibe_operations(id,session_id,actor,client_id,request_hash,kind,state,billing,models,input,max_cost,actual_cost,deadline) VALUES($1,$2,$3,$4,'free-old','message','COMPLETED','RELEASED','{}','{}',0,0,now())`, freeID, old.ID, old.Actor, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.Exec(ctx, `INSERT INTO vibe_attempts(id,operation_id,step_key,role,model,provider,input_bound,max_output,state,request_hash,max_cost,actual_cost,policy,output,completed_at) VALUES($1,$2,'route','assistant','fake','fake',1,1,'SUCCEEDED','free-attempt',0,0,'{"profile":{"free":true},"private":"old instructions"}','old output',now())`, uuid.New(), freeID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.RetirePreV1(ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uuid.UUID{old.ID, other.ID} {
		r, e := s.deletionReceipt(ctx, id)
		if e != nil || r.Status != "deleted" {
			t.Fatal("old container not scrubbed", r, e)
		}
	}
	if _, err = s.GetSession(ctx, current.Actor, current.ID); err != nil {
		t.Fatal("current Improve 15 deleted", err)
	}
	var content string
	if err = s.DB.QueryRow(ctx, `SELECT input->>'content' FROM vibe_operations WHERE session_id=$1`, current.ID).Scan(&content); err != nil || content != "current instructions" {
		t.Fatal("V1 content lost", err)
	}
	count, err := s.RetirePreV1(ctx)
	if err != nil || count != 0 {
		t.Fatal("retirement not idempotent", count, err)
	}
}
func TestIntegrationRetirementRequiresDrainedExecution(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	o, _ := submitPlan(t, s, v, testConfig(), 1)
	_, err := s.RetirePreV1(context.Background())
	requireFault(t, err, "retirement_pending")
	if _, err = s.GetSession(context.Background(), v.Actor, v.ID); err != nil {
		t.Fatal("pending project deleted")
	}
	if err = s.Stop(context.Background(), v.Actor, o.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RetirePreV1(context.Background()); err != nil {
		t.Fatal(err)
	}
}
