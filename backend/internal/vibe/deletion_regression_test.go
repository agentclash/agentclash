package vibe

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIntegrationQuotePersistenceCannotResurrectDeletedContent(t *testing.T) {
	s, v := buildService(t)
	ctx := context.Background()
	q := quoteRecord{uuid.New(), "request", raw(map[string]string{"content": "private request"}), 1, timestamp().Add(time.Hour)}
	if err := s.Store.persistQuote(ctx, v.Actor, v.ID, v.Revision, q); err != nil {
		t.Fatal(err)
	}
	// Model preparation has finished but its persistence is paused. Deletion
	// finishes before the prepared quote is allowed to reach the Store.
	ready, resume := make(chan struct{}), make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(ready)
		<-resume
		q.ID = uuid.New()
		result <- s.Store.persistQuote(ctx, v.Actor, v.ID, v.Revision, q)
	}()
	<-ready
	if _, err := s.Store.DeleteProject(ctx, v.Actor, v.ID, v.Revision); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.CleanupProjects(ctx); err != nil {
		t.Fatal(err)
	}
	close(resume)
	requireFault(t, <-result, "not_found")
	var total, contents int
	if err := s.Store.DB.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE specification<>'{}'::jsonb) FROM vibe_cycle_quotes WHERE session_id=$1`, v.ID).Scan(&total, &contents); err != nil {
		t.Fatal(err)
	}
	if total != 1 || contents != 0 {
		t.Fatalf("late quote survived deletion: rows=%d content=%d", total, contents)
	}
}

func TestIntegrationQuotePersistenceRechecksOwnershipAndRevision(t *testing.T) {
	s, v := buildService(t)
	ctx := context.Background()
	q := quoteRecord{uuid.New(), "request", raw(map[string]string{"content": "private"}), 1, timestamp().Add(time.Hour)}
	requireFault(t, s.Store.persistQuote(ctx, "anon:other", v.ID, v.Revision, q), "not_found")
	if err := s.Store.Edit(ctx, v.Actor, v.ID, v.Revision, func(*Session) error { return nil }); err != nil {
		t.Fatal(err)
	}
	requireFault(t, s.Store.persistQuote(ctx, v.Actor, v.ID, v.Revision, q), "revision_conflict")
}

func TestIntegrationCleanupSkipsBlockedProjectsBeforeBatchLimit(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	var eligible Session
	var ids []uuid.UUID
	for i := 0; i < 21; i++ {
		// These deliberately unresolved fixtures must stay unresolved. The
		// ordinary session helper finishes synthetic operations during cleanup,
		// making them eligible and polluting a later run's 20-row batch.
		v, err := s.CreateSession(ctx, "anon:"+uuid.NewString(), nil, uuid.New())
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, v.ID)
		if _, err := s.DB.Exec(ctx, `UPDATE vibe_sessions SET deleted_at=now()-interval '1 day'+$2*interval '1 second',document='{}' WHERE id=$1`, v.ID, i); err != nil {
			t.Fatal(err)
		}
		if i == 20 {
			eligible = v
			continue
		}
		_, err = s.DB.Exec(ctx, `INSERT INTO vibe_operations(id,session_id,actor,client_id,request_hash,kind,state,billing,models,input,max_cost,deadline) VALUES($1,$2,$3,$4,'blocked','message','CANCELLED','RECONCILING','{}','{}',1,now())`, uuid.New(), v.ID, v.Actor, uuid.New())
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CleanupProjects(ctx); err != nil {
		t.Fatal(err)
	}
	r, err := s.DeletionStatus(ctx, eligible.Actor, eligible.ID)
	if err != nil || r.Status != "deleted" {
		t.Fatalf("eligible project starved: %+v %v", r, err)
	}
	var blocked int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM vibe_sessions s JOIN vibe_operations o ON o.session_id=s.id WHERE s.id=ANY($1) AND o.request_hash='blocked' AND s.cleanup_finished_at IS NOT NULL`, ids).Scan(&blocked); err != nil {
		t.Fatal(err)
	}
	if blocked != 0 {
		t.Fatal("uncertain projects scrubbed prematurely")
	}
}
