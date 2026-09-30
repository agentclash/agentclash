package api

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Explicitly opted-in, four-call semantic control through the real admitted
// workflow. No automatic retries; every outcome is retained in a new report.
func TestVibeLiveBuildEntailment(t *testing.T) {
	if os.Getenv("VIBE_LIVE_BUILD_ENTAILMENT") != "true" {
		t.Skip("paid, explicitly opted-in local verification")
	}
	cfg, err := vibe.LoadConfig()
	if err != nil || !cfg.TestingLocally() || cfg.FreeOnly || cfg.Credential == "" {
		t.Fatal("requires existing paid local configuration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	dc, err := pgxpool.ParseConfig(os.Getenv("VIBE_LIVE_REVIEW_DATABASE_URL"))
	if err != nil {
		t.Fatal("database config")
	}
	if dc.ConnConfig.Host != "127.0.0.1" && dc.ConnConfig.Host != "localhost" {
		t.Fatal("local database only")
	}
	db, err := pgxpool.NewWithConfig(ctx, dc)
	if err != nil {
		t.Fatal("database connection")
	}
	defer db.Close()
	before, err := readLiveReviewBalance(ctx, db, cfg.Campaign)
	if err != nil || before.Active != 0 {
		t.Fatal("campaign unavailable or an operation is active")
	}
	path := os.Getenv("VIBE_LIVE_BUILD_REPORT")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal("a new report path is required")
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	const ceiling = int64(2 * vibe.NanoUSD)
	rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	defer rc.Close()
	models := []string{cfg.DefaultModels().Assistant, "openai/gpt-5.4-mini"}
	if models[0] == models[1] {
		t.Fatal("primary and fallback must be distinct")
	}
	maxCalls := int64(4)
	if os.Getenv("VIBE_LIVE_BUILD_CORRECTIVE") == "true" {
		models = models[1:]
		maxCalls = 2
	}
	if err = enc.Encode(map[string]any{"max_calls": maxCalls, "aggregate_max_cost_nano": ceiling, "baseline": before, "kind": "live semantic controls, not a general benchmark"}); err != nil {
		t.Fatal(err)
	}
	reserved := int64(0)
	for _, model := range models {
		cfg.DefaultModel = model
		profile, err := cfg.Profile(model)
		if err != nil {
			t.Fatal(err)
		}
		limits := cfg.Limits(true)
		limits.OutputTokens = max(limits.OutputTokens, 4096)
		bound, err := profile.BoundCost(min(limits.ContextTokens, profile.Context-limits.OutputTokens), limits.OutputTokens)
		if err != nil {
			t.Fatal(err)
		}
		for _, positive := range []bool{false, true} {
			balance, err := readLiveReviewBalance(ctx, db, cfg.Campaign)
			if err != nil || balance.Active != 0 || balance.Disabled || balance.BalanceNano < balance.HeldNano+bound || reserved+bound > ceiling || balance.TotalCalls-before.TotalCalls >= maxCalls {
				t.Fatal("bounded wave cannot safely admit another call")
			}
			// Reserve the whole bound even if an earlier call was cheap. Historical
			// account holds are never released, reset or credited by this experiment.
			reserved += bound
			store := vibe.NewStore(db, cfg)
			svc := &vibe.Service{Store: store, Config: cfg, Gate: vibe.Gate{Redis: rc}, Compiler: VibePackCompiler{}}
			actor := "anon:build-entailment:" + uuid.NewString()
			v, err := store.CreateSession(ctx, actor, nil, uuid.New(), cfg.DefaultModels())
			if err != nil {
				t.Fatal(err)
			}
			source := "Unknown senders are spam."
			if positive {
				source += " Known senders are safe."
			}
			sid, aid, pid := uuid.New(), uuid.New(), uuid.New()
			block := vibe.SourceBlock{ID: sid.String(), MessageID: sid, Text: source, Hash: vibe.Hash([]byte(source)), OriginalHash: vibe.Hash([]byte(source))}
			policy := vibe.PolicySnapshot{ID: pid, ScopeID: v.ID, SourceMessageID: sid, SourceVersion: vibe.SourcePolicyVersion, Sources: []vibe.SourceBlock{block}, Rules: []vibe.PolicyRule{{ID: "sender", Statement: source, SourceBlockIDs: []string{block.ID}, Evidence: []vibe.RuleEvidence{{SourceBlockID: block.ID, Quote: source, Kind: "requirement"}}}}}
			bp, err := svc.Compiler.Draft(vibe.DraftProposal{TestsOnly: true, Title: "Known sender entailment control", SuccessCriteria: "Follow these rules where applicable to the case:\n- " + source, Scenarios: []vibe.TestScenario{{Input: "The sender is known. Message: Hello.", Expected: "Classify this message as safe."}}}, limits)
			if err != nil {
				t.Fatal(err)
			}
			err = store.Edit(ctx, actor, v.ID, v.Revision, func(s *vibe.Session) error {
				s.Document.Evaluation = &vibe.EvaluationContext{ID: v.ID, Door: "build"}
				s.Document.TestJourney = true
				s.Document.Messages = []vibe.Message{{ID: sid, Role: "user", Content: source, CreatedAt: time.Now().UTC()}}
				s.Document.Policies = []vibe.PolicySnapshot{policy}
				s.Document.Artifacts = []vibe.Artifact{{ID: aid, Kind: "test_suite", Title: "Entailment control", PolicyID: &pid, Blueprint: bp, Provenance: "ai_generated", SourceMessageID: sid, CreatedAt: time.Now().UTC()}}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			v, err = store.GetSession(ctx, actor, v.ID)
			if err != nil {
				t.Fatal(err)
			}
			op, err := svc.PrepareSuiteEdit(ctx, actor, v.ID, v.Revision, aid, bp)
			if err != nil {
				t.Fatal(err)
			}
			var plan vibe.Plan
			if json.Unmarshal(op.Input, &plan) != nil || plan.Calls != 1 || op.MaxCost != bound || plan.Conversation.ValidatorVersion != vibe.EntailmentSuiteValidatorVersion {
				store.Stop(ctx, actor, op.ID)
				t.Fatal("unexpected admitted review contract")
			}
			if err = enc.Encode(map[string]any{"model": model, "positive_control": positive, "session": v.ID, "operation": op.ID, "reserved_nano": bound}); err != nil {
				t.Fatal(err)
			}
			f.Sync()
			ticker := time.NewTicker(time.Second)
			for !op.State.Terminal() {
				select {
				case <-ctx.Done():
					ticker.Stop()
					t.Fatal("worker deadline; do not resubmit")
				case <-ticker.C:
					op, err = store.Operation(ctx, op.ID)
					if err != nil {
						ticker.Stop()
						t.Fatal(err)
					}
				}
			}
			ticker.Stop()
			var output string
			_ = db.QueryRow(ctx, "SELECT output FROM vibe_attempts WHERE operation_id=$1 AND step_key='review'", op.ID).Scan(&output)
			committed := op.Completion != nil && op.Completion.ArtifactID != nil
			if err = enc.Encode(map[string]any{"model": model, "positive_control": positive, "operation": op.ID, "state": op.State, "billing": op.Billing, "calls": op.ModelCalls, "actual_cost_nano": op.ActualCost, "error": op.Error, "committed": committed, "review": output}); err != nil {
				t.Fatal(err)
			}
			f.Sync()
			t.Logf("model=%s positive=%v committed=%v calls=%d", model, positive, committed, op.ModelCalls)
			if op.ModelCalls != 1 || op.ActualCost == nil || op.Billing == vibe.Reconciling {
				t.Fatal("live review did not complete with certain accounting")
			}
			if op.Error != nil && op.Error.Code != "test_policy_conflict" {
				t.Errorf("review failed: %s", op.Error.Code)
				continue // The next control is independent, never a retry.
			}
			input, parseErr := vibe.BuildSuiteReviewInput(bp, *plan.Conversation.Policy, plan.Conversation.Sources, plan.Conversation.CurrentRequest, 1, *plan.ExecutionLimits)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			input.ValidatorVersion = plan.Conversation.ValidatorVersion
			input.PreviousPolicy = plan.Conversation.Policy
			validation, parseErr := vibe.ParseSuiteReview([]byte(output), input, *plan.ExecutionLimits)
			if parseErr != nil {
				t.Error("live review was invalid rather than a semantic decision", parseErr)
				continue
			}
			if (validation.Status == vibe.SuiteSupported) != positive {
				t.Errorf("wrong semantic decision: %s", validation.Status)
			}
			if committed != positive {
				t.Errorf("model=%s positive=%v: incorrect acceptance; preserved in report", model, positive)
			}
		}
	}
}
