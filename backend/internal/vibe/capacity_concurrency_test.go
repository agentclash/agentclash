package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

func TestIntegrationConcurrentDailyAndTrialCallLimits(t *testing.T) {
	for _, daily := range []bool{true, false} {
		name := "trial"
		if daily {
			name = "daily"
		}
		t.Run(name, func(t *testing.T) {
			s := integrationStore(t)
			ctx := context.Background()
			cfg := testConfig()
			if daily {
				cfg = freeConfig()
			}
			sessions := []Session{}
			ops := []Operation{}
			trial := "anon:" + uuid.NewString()
			for i := 0; i < 3; i++ {
				v := anonSession(t, s)
				if !daily {
					if _, err := s.DB.Exec(ctx, "UPDATE vibe_sessions SET trial_key=$2 WHERE id=$1", v.ID, trial); err != nil {
						t.Fatal(err)
					}
				}
				sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "playground", Models: DefaultModels()}
				if daily {
					sub.Models = cfg.DefaultModels()
				}
				cost := int64(1000)
				if daily {
					cost = 0
				}
				o, err := s.Submit(ctx, v.Actor, v.ID, sub, Plan{Submission: sub, Anonymous: true, Free: daily, Calls: 1, MaxCost: cost}, cfg)
				if err != nil {
					t.Fatal(err)
				}
				if _, _, err = s.Start(ctx, o.ID); err != nil {
					t.Fatal(err)
				}
				sessions = append(sessions, v)
				ops = append(ops, o)
			}
			// Completed history leaves exactly one call. Admission is already complete:
			// this race exercises the dispatch boundary across independent projects.
			seed := uuid.New()
			if _, err := s.DB.Exec(ctx, `INSERT INTO vibe_operations(id,session_id,actor,client_id,request_hash,kind,state,billing,models,input,max_cost,actual_cost,deadline,model_calls) VALUES($1,$2,$3,$4,'capacity-history','check','COMPLETED','RELEASED',$5,'{"anonymous":true}',0,0,now(),$6)`, seed, sessions[0].ID, sessions[0].Actor, uuid.New(), raw(ops[0].Models), TrialCalls-1); err != nil {
				t.Fatal(err)
			}
			if daily {
				var existing int
				if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM vibe_attempts WHERE max_cost=0 AND created_at>=(date_trunc('day',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')`).Scan(&existing); err != nil {
					t.Fatal(err)
				}
				if existing >= MaxFreeDailyCalls {
					t.Fatal("fixture has no remaining daily slot; run capacity tests in the isolated floor")
				}
				for i := existing; i < MaxFreeDailyCalls-1; i++ {
					if _, err := s.DB.Exec(ctx, `INSERT INTO vibe_attempts(id,operation_id,step_key,role,model,provider,policy,request_hash,input_bound,max_output,max_cost,actual_cost,state,completed_at) VALUES($1,$2,$3,'target',$4,'openrouter',$5,'history',1,1,0,0,'SUCCEEDED',now())`, uuid.New(), seed, fmt.Sprintf("history:%d", i), cfg.DefaultModel, raw(map[string]any{"profile": cfg.Profiles[cfg.DefaultModel]})); err != nil {
						t.Fatal(err)
					}
				}
				// The paid-trial model-call fixture is irrelevant for the daily subtest.
				if _, err := s.DB.Exec(ctx, "UPDATE vibe_operations SET model_calls=0 WHERE id=$1", seed); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					_, _ = s.DB.Exec(context.Background(), "DELETE FROM vibe_attempts WHERE operation_id=ANY($1)", append([]uuid.UUID{seed}, ops[0].ID, ops[1].ID, ops[2].ID))
				})
			}
			var wins atomic.Int32
			var wg sync.WaitGroup
			errs := make(chan error, 3)
			for _, o := range ops {
				wg.Add(1)
				go func(o Operation) {
					defer wg.Done()
					a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "target", Role: Target, Model: o.Models.Target, Policy: json.RawMessage(`{}`), InputBound: 1, MaxOutput: 1, MaxCost: 100}
					if daily {
						a.MaxCost = 0
						a.Policy = raw(map[string]any{"profile": cfg.Profiles[cfg.DefaultModel]})
					}
					err := s.BeginAttempt(ctx, a)
					if err == nil {
						wins.Add(1)
					} else {
						errs <- err
					}
				}(o)
			}
			wg.Wait()
			close(errs)
			if wins.Load() != 1 {
				t.Fatalf("last %s call admitted %d times", name, wins.Load())
			}
			expected := "trial_limit"
			if daily {
				expected = "free_capacity_reached"
			}
			for err := range errs {
				var f *Fault
				if !errors.As(err, &f) || f.Code != expected {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestIntegrationConcurrentHostedAndWorkspaceRunningLimits(t *testing.T) {
	for _, workspace := range []bool{false, true} {
		name, limit := "hosted", 20
		if workspace {
			name, limit = "workspace", MaxWorkspaceRunning
		}
		t.Run(name, func(t *testing.T) {
			s := integrationStore(t)
			ctx := context.Background()
			cfg := testConfig()
			ops := []Operation{}
			var destination *uuid.UUID
			var org uuid.UUID
			for i := 0; i < limit+2; i++ {
				var v Session
				if workspace {
					if destination == nil {
						v = approvalRegressionWorkspace(t, s)
						destination = v.WorkspaceID
						if err := s.DB.QueryRow(ctx, "SELECT organization_id FROM workspaces WHERE id=$1", *destination).Scan(&org); err != nil {
							t.Fatal(err)
						}
					} else {
						user := uuid.New()
						if _, err := s.DB.Exec(ctx, "INSERT INTO users(id,workos_user_id,email) VALUES($1,$2,$3)", user, user.String(), user.String()+"@example.invalid"); err != nil {
							t.Fatal(err)
						}
						if _, err := s.DB.Exec(ctx, "INSERT INTO organization_memberships(organization_id,user_id,role,membership_status) VALUES($1,$2,'org_admin','active')", org, user); err != nil {
							t.Fatal(err)
						}
						var err error
						v, err = s.CreateSession(ctx, "user:"+user.String(), destination, uuid.New())
						if err != nil {
							t.Fatal(err)
						}
						cleanupSession(t, s, v.ID)
					}
				} else {
					v = anonSession(t, s)
				}
				o, _ := submitPlan(t, s, v, cfg, 100)
				ops = append(ops, o)
			}
			var wins atomic.Int32
			var wg sync.WaitGroup
			errs := make(chan error, len(ops))
			for _, o := range ops {
				wg.Add(1)
				go func(o Operation) {
					defer wg.Done()
					_, _, err := s.Start(ctx, o.ID)
					if err == nil {
						wins.Add(1)
					} else {
						errs <- err
					}
				}(o)
			}
			wg.Wait()
			close(errs)
			if wins.Load() != int32(limit) {
				t.Fatalf("%s concurrency: want %d got %d", name, limit, wins.Load())
			}
			for err := range errs {
				var f *Fault
				if !errors.As(err, &f) || f.Code != "capacity_limit" {
					t.Fatal(err)
				}
			}
		})
	}
}
