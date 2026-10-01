package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestIntegrationScopedLocksAllowUnrelatedEditAndStop(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	a, b, c := anonSession(t, s), anonSession(t, s), anonSession(t, s)
	op, _ := submitPlan(t, s, c, testConfig(), 100)
	locked, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	defer close(release)
	go func() {
		done <- s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
			if _, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", a.ID)); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()
	select {
	case <-locked:
	case err := <-done:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("project lock wasn't acquired")
	}
	work := make(chan error, 2)
	go func() {
		work <- s.Edit(ctx, b.Actor, b.ID, b.Revision, func(v *Session) error { v.Title = "independent edit"; return nil })
	}()
	go func() { work <- s.Stop(ctx, c.Actor, op.ID) }()
	for i := 0; i < 2; i++ {
		select {
		case err := <-work:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("an unrelated edit/cancellation waited on project A")
		}
	}
}

func TestIntegrationTransactionCallbackUsesBoundedContext(t *testing.T) {
	s := integrationStore(t)
	before := time.Now()
	if err := s.transaction(context.Background(), func(ctx context.Context, tx pgx.Tx) error {
		deadline, ok := ctx.Deadline()
		if !ok || deadline.After(before.Add(5100*time.Millisecond)) {
			t.Fatal("transaction deadline didn't reach its callback")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "SELECT pg_sleep(1)")
		return err
	})
	if err == nil || time.Since(before) > time.Second {
		t.Fatalf("database query ignored transaction context: %v", err)
	}
}

func TestIntegrationConcurrentAccountingAndDeletion(t *testing.T) {
	service, v := buildService(t)
	s, ctx := service.Store, context.Background()
	op, _ := submitPlan(t, s, v, service.Config, 1000)
	if _, _, err := s.Start(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	attempts := []Attempt{}
	for _, step := range []string{"first", "second"} {
		a := Attempt{ID: uuid.New(), OperationID: op.ID, Step: step, Role: Target, Model: op.Models.Target, Policy: json.RawMessage(`{}`), RequestHash: step, InputBound: 100, MaxOutput: 100, MaxCost: 300}
		if err := s.BeginAttempt(ctx, a); err != nil {
			t.Fatal(err)
		}
		attempts = append(attempts, a)
	}
	if err := s.EndAttempt(ctx, attempts[0], "private output", json.RawMessage(`{"private":"usage"}`), nil, nil); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	race := func(fn func() error) { wg.Add(1); go func() { defer wg.Done(); errs <- fn() }() }
	race(func() error { return s.Stop(ctx, v.Actor, op.ID) })
	race(func() error { return s.Finish(ctx, op.ID, nil) })
	wg.Wait()
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	// A known first cost cannot release any hold while the second call is unknown.
	for i := 0; i < 5; i++ {
		race(func() error { return s.ReconcileCost(ctx, attempts[0].ID, 250, json.RawMessage(`{}`)) })
	}
	wg.Wait()
	for i := 0; i < 5; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	var held int
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM vibe_reservations r JOIN vibe_accounts a ON a.id=r.account_id WHERE r.operation_id=$1 AND r.settled_amount IS NULL AND a.held=r.amount`, op.ID).Scan(&held); err != nil || held != 4 {
		t.Fatalf("uncertain hold released: %d %v", held, err)
	}
	latest, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Deletion, a late completed provider response, duplicate reconciliations and
	// duplicate grants race through separate parent/operation/account boundaries.
	race(func() error { _, err := s.DeleteProject(ctx, v.Actor, v.ID, latest.Revision); return err })
	race(func() error {
		return s.EndAttempt(ctx, attempts[1], "late private output", json.RawMessage(`{"private":"late usage"}`), nil, nil)
	})
	for i := 0; i < 6; i++ {
		race(func() error { return s.ReconcileCost(ctx, attempts[1].ID, 200, json.RawMessage(`{"cost":0.0000002}`)) })
		race(func() error { return s.Grant(ctx, v.Actor, "extra:"+v.ID.String(), 100) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var exact int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM vibe_reservations r JOIN vibe_accounts a ON a.id=r.account_id JOIN vibe_grants g ON g.account_id=a.id AND g.source='initial:'||a.id WHERE r.operation_id=$1 AND r.settled_amount=450 AND a.held=0 AND a.balance=g.amount-450+CASE WHEN a.id=$2 THEN 100 ELSE 0 END`, op.ID, v.Actor).Scan(&exact); err != nil || exact != 4 {
		t.Fatalf("charge/grant wasn't exactly once: %d %v", exact, err)
	}
	for batch := 0; batch < 20; batch++ {
		if err = s.CleanupProjects(ctx); err != nil {
			t.Fatal(err)
		}
		r, e := s.DeletionStatus(ctx, v.Actor, v.ID)
		if e != nil {
			t.Fatal(e)
		}
		if r.Status == "deleted" {
			break
		}
		if batch == 19 {
			t.Fatal("deletion didn't finish")
		}
	}
	requireFault(t, s.AppendOutput(ctx, attempts[1].ID, "resurrected"), "not_found")
	requireFault(t, s.RecordDomainOutcome(ctx, op.ID, "second", DomainOutcome{Stage: "review", Status: "failed", Problems: []DomainProblem{{Code: "invalid", Message: "private detail"}}}), "not_found")
	if err = s.EndAttempt(ctx, attempts[1], "resurrected", json.RawMessage(`{"private":true}`), nil, nil); err != nil {
		t.Fatal(err)
	}
	if err = s.ReconcileCost(ctx, attempts[1].ID, 200, json.RawMessage(`{"private":true}`)); err != nil {
		t.Fatal(err)
	}
	var private int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM vibe_attempts WHERE operation_id=$1 AND (output<>'' OR usage<>'{}'::jsonb OR domain_outcome IS NOT NULL)`, op.ID).Scan(&private); err != nil || private != 0 {
		t.Fatalf("late callback resurrected content: %d %v", private, err)
	}
}

func TestIntegrationScopedCapacityAndConcurrentGrants(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	cfg := testConfig()
	actor := "anon:" + uuid.NewString()
	sessions := []Session{}
	for i := 0; i < 12; i++ {
		v, err := s.CreateSession(ctx, actor, nil, uuid.New())
		if err != nil {
			t.Fatal(err)
		}
		cleanupSession(t, s, v.ID)
		sessions = append(sessions, v)
	}
	var admitted atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, 24)
	for _, v := range sessions {
		wg.Add(1)
		go func(v Session) {
			defer wg.Done()
			sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "playground", Models: DefaultModels()}
			_, err := s.Submit(ctx, v.Actor, v.ID, sub, Plan{Submission: sub, Anonymous: true, Calls: 1, MaxCost: 100}, cfg)
			if err == nil {
				admitted.Add(1)
			} else {
				errs <- err
			}
		}(v)
	}
	wg.Wait()
	close(errs)
	if admitted.Load() != int32(LimitsFor(true).Queued) {
		t.Fatalf("actor queue ceiling breached: %d", admitted.Load())
	}
	for err := range errs {
		var f *Fault
		if !errors.As(err, &f) || f.Code != "capacity_limit" {
			t.Fatal(err)
		}
	}
	account := "test:" + uuid.NewString()
	source := "grant:" + uuid.NewString()
	errs = make(chan error, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.Grant(ctx, account, source, 1000) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	b, h, err := s.Balance(ctx, account)
	if err != nil || b != 1000 || h != 0 {
		t.Fatalf("concurrent grant credited twice: %d %d %v", b, h, err)
	}
}
