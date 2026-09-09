package api

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/agentclash/agentclash/runtime/scoring"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Records the real gateway's wire requests; no provider adapter or service runs.
type vibeNormalizedReferenceClient struct {
	output   string
	requests []provider.Request
}

func (f *vibeNormalizedReferenceClient) InvokeModel(_ context.Context, request provider.Request) (provider.Response, error) {
	f.requests = append(f.requests, request)
	output := f.output
	if len(request.ResponseFormat) > 0 {
		output = `{"pass":true,"reasoning":"Fixture evaluator observed the response."}`
	}
	zero := json.Number("0")
	return provider.Response{OutputText: output, Usage: provider.Usage{InputTokens: 100, OutputTokens: 100, CostUSD: &zero}}, nil
}

func TestVibeIntegrationNormalizedReferenceEvidence(t *testing.T) {
	dsn := os.Getenv("VIBE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated migrated VIBE_TEST_DATABASE_URL")
	}
	if !strings.Contains(dsn, "vibe_test") {
		t.Fatal("refusing a non-test database")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, tt := range []struct {
		name, ref, expected, decoy, output string
		fuzzy, reject, array               bool
	}{
		{name: "trailing_expected", ref: "case.payload.item.expected ", expected: "SECRET", decoy: "decoy", output: "SECRET"},
		{name: "trailing_decoy", ref: "case.payload.item.expected ", expected: "SECRET", decoy: "decoy", output: "decoy"},
		{name: "surrounding_expected", ref: "\t case.payload.item.expected \n", expected: "SECRET", decoy: "decoy", output: "SECRET"},
		{name: "surrounding_decoy", ref: "\t case.payload.item.expected \n", expected: "SECRET", decoy: "decoy", output: "decoy"},
		{name: "ordinary_nested", ref: "case.payload.item.expected", expected: "SECRET", decoy: "decoy", output: "SECRET"},
		{name: "ordinary_array", ref: "case.payload.items.0.expected", expected: "SECRET", decoy: "decoy", output: "SECRET", array: true},
		{name: "surrounding_array", ref: "\tcase.payload.items.0.expected \n", expected: "SECRET", decoy: "decoy", output: "SECRET", array: true},
		{name: "fuzzy_trailing_actual_3000", ref: "case.payload.item.expected ", expected: strings.Repeat("s", 3000), decoy: "decoy", output: "decoy", fuzzy: true, reject: true},
		{name: "fuzzy_surrounding_actual_3000", ref: "\tcase.payload.item.expected \n", expected: strings.Repeat("s", 3000), decoy: "decoy", output: "decoy", fuzzy: true, reject: true},
		{name: "fuzzy_trailing_actual_2049", ref: "case.payload.item.expected ", expected: strings.Repeat("s", 2049), decoy: "decoy", output: "decoy", fuzzy: true, reject: true},
		{name: "fuzzy_trailing_actual_2048", ref: "case.payload.item.expected ", expected: strings.Repeat("s", 2048), decoy: "decoy", output: strings.Repeat("s", 2048), fuzzy: true},
		{name: "fuzzy_trailing_decoy_3000", ref: "case.payload.item.expected ", expected: "SECRET", decoy: strings.Repeat("d", 3000), output: "SECRET", fuzzy: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := vibeReliabilityBundle(t)
			validator := &b.Version.EvaluationSpec.Validators[0]
			validator.Type, validator.ExpectedFrom = scoring.ValidatorTypeExactMatch, tt.ref
			if tt.fuzzy {
				validator.Type = scoring.ValidatorTypeFuzzyMatch
			}
			// The suffixed key is deliberately a different value, not another
			// copy of the answer. Only the normalized path is scored.
			decoyKey := "expected" + tt.ref[len(strings.TrimRight(tt.ref, " \t\n")):]
			if decoyKey == "expected" {
				decoyKey += " "
			}
			for i := range b.InputSets[0].Cases {
				item := map[string]any{"prompt": "Answer", "expected": tt.expected, decoyKey: tt.decoy}
				payload := map[string]any{"prompt": "Answer", "item": item}
				if tt.array {
					payload = map[string]any{"prompt": "Answer", "items": []any{item, map[string]any{"prompt": "Retain this sibling"}}}
				}
				b.InputSets[0].Cases[i].Payload = payload
			}
			blueprint := vibeReliabilityJSON(t, b)
			artifact := vibe.Artifact{ID: uuid.New(), Title: "Normalized reference fixture", AgentPrompt: "Answer the supplied prompt.", Blueprint: blueprint, Accepted: true, CreatedAt: time.Now()}
			compiled, compileErr := (VibePackCompiler{}).Compile(blueprint, "openai/gpt-4.1-mini", artifact.ID, vibe.LimitsFor(true))
			if tt.reject {
				if compileErr == nil || !strings.Contains(compileErr.Error(), "resolved fuzzy") {
					t.Errorf("compiler must bound the actual %d-byte scored operand, not the decoy: %v", len(tt.expected), compileErr)
				}
			} else if compileErr != nil {
				t.Errorf("bounded normalized reference rejected: %v", compileErr)
			} else {
				if !reflect.DeepEqual(compiled.Cases, b.InputSets[0].Cases) || !reflect.DeepEqual(compiled.Bundle.Version.EvaluationSpec.Validators, b.Version.EvaluationSpec.Validators) {
					t.Error("compiler mutated original case or validator declarations")
				}
				var composition challengepack.Composition
				if err := json.Unmarshal(compiled.Composition, &composition); err != nil {
					t.Fatal(err)
				}
				rebuilt, err := challengepack.ComposeBundle(composition, challengepack.ResolvedPieces{})
				if err != nil || !reflect.DeepEqual(rebuilt.Version.EvaluationSpec, compiled.Bundle.Version.EvaluationSpec) || !reflect.DeepEqual(rebuilt.InputSets, compiled.Bundle.InputSets) {
					t.Fatalf("saved composition/coverage differs from original compiled bundle: %v", err)
				}
			}
			if !bytes.Equal(blueprint, vibeReliabilityJSON(t, b)) {
				t.Fatal("compilation changed source evidence")
			}

			store := &vibe.Store{DB: db}
			model := "openai/gpt-4.1-mini"
			models := vibe.Models{Assistant: model, Target: model, Evaluator: model}
			session, err := store.CreateSession(ctx, "anon:"+uuid.NewString(), nil, uuid.New(), models)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if _, err := db.Exec(ctx, "DELETE FROM vibe_attempts WHERE operation_id IN (SELECT id FROM vibe_operations WHERE session_id=$1)", session.ID); err != nil {
					t.Errorf("cleanup only fixture attempts: %v", err)
				}
			})
			rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
			t.Cleanup(func() { _ = rc.Close() })
			cfg := vibe.Config{Enabled: true, Credential: "fixture-only-no-network", DefaultModel: model, Campaign: uuid.NewString(), AnonymousDaily: vibe.NanoUSD, AnonymousCampaign: 5 * vibe.NanoUSD, Profiles: map[string]vibe.ModelProfile{model: {ID: model, Route: "openai", InputNanoPerToken: 400, OutputNanoPerToken: 1600, Context: 128000, FramingAllowance: 2048, Conformed: true, ExpiresAt: time.Now().Add(time.Hour)}}}
			svc := &vibe.Service{Store: store, Config: cfg, Gate: vibe.Gate{Redis: rc}, Compiler: VibePackCompiler{}}
			if err := store.Edit(ctx, session.Actor, session.ID, session.Revision, func(v *vibe.Session) error {
				v.Document.Artifacts = []vibe.Artifact{artifact}
				v.Document.ActiveArtifactID = &artifact.ID
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			session, err = store.GetSession(ctx, session.Actor, session.ID)
			if err != nil {
				t.Fatal(err)
			}
			beforeArtifact := vibeReliabilityJSON(t, session.Document.Artifacts)
			sub := vibe.Submission{ClientID: uuid.New(), Revision: session.Revision, Kind: "check", ArtifactID: &artifact.ID, Models: models}
			plan := vibe.Plan{Submission: sub, Artifact: &artifact, ChecksPerCase: 2, Anonymous: true, MaxCost: vibe.NanoUSD / 10}
			for _, c := range b.InputSets[0].Cases {
				plan.Cases = append(plan.Cases, c.CaseKey)
			}
			plan.Calls = len(plan.Cases) * 2
			// Submit the historical plan directly, bypassing today's import and
			// Prepare compiler. Execute must independently protect queued plans.
			op, err := store.Submit(ctx, session.Actor, session.ID, sub, plan, cfg)
			if err != nil {
				t.Fatal(err)
			}
			persisted, err := store.Operation(ctx, op.ID)
			if err != nil {
				t.Fatal(err)
			}
			beforePlan := append([]byte(nil), persisted.Input...)
			fake := &vibeNormalizedReferenceClient{output: tt.output}
			runner := vibe.Runner{Service: svc, Gateway: &vibe.Gateway{Store: store, Config: cfg, Gate: svc.Gate, Client: fake}}
			execErr := runner.Execute(ctx, op.ID)
			var issue *vibe.Fault
			if execErr != nil {
				issue = &vibe.Fault{Code: "fixture_execution_error", Message: execErr.Error()}
			}
			if err := store.Finish(ctx, op.ID, issue); err != nil {
				t.Fatal(err)
			}
			if tt.reject {
				if execErr == nil || !strings.Contains(execErr.Error(), "resolved fuzzy") {
					t.Errorf("persisted plan scored an oversized normalized expected operand: %v", execErr)
				}
			} else if execErr != nil {
				t.Errorf("persisted bounded plan failed: %v", execErr)
			}
			wantCalls := plan.Calls
			if tt.reject {
				wantCalls = 0
			}
			if len(fake.requests) != wantCalls {
				t.Errorf("wire calls = %d, want %d (no unchecked sends, retries or dropped judges)", len(fake.requests), wantCalls)
			}
			targets, judges := 0, 0
			for _, request := range fake.requests {
				if len(request.Tools) != 0 || len(request.Messages) != 2 {
					t.Fatal("request escaped the fixed text-only graph")
				}
				if len(request.ResponseFormat) > 0 {
					judges++
					if !strings.Contains(request.Messages[1].Content, tt.expected) {
						t.Error("judge lost original expected evidence")
					}
					continue
				}
				targets++
				var sent map[string]any
				if err := json.Unmarshal([]byte(request.Messages[1].Content), &sent); err != nil {
					t.Fatal(err)
				}
				wantItem := map[string]any{"prompt": "Answer", decoyKey: tt.decoy}
				want := map[string]any{"prompt": "Answer", "item": wantItem}
				if tt.array {
					want = map[string]any{"prompt": "Answer", "items": []any{wantItem, map[string]any{"prompt": "Retain this sibling"}}}
				}
				if strings.Contains(request.Messages[1].Content, tt.expected) || !reflect.DeepEqual(sent, want) {
					t.Errorf("actual scored expected reached target or safe input was lost; reference %q", tt.ref)
				}
			}
			if targets != wantCalls/2 || judges != wantCalls/2 {
				t.Errorf("target/judge graph changed: %d/%d, want %d each", targets, judges, wantCalls/2)
			}
			session, err = store.GetSession(ctx, session.Actor, session.ID)
			if err != nil {
				t.Fatal(err)
			}
			result := session.Operations[len(session.Operations)-1]
			if result.ModelCalls != wantCalls || result.Scorecard == nil || result.Scorecard.Total != len(plan.Cases) || result.Scorecard.ChecksExpected != plan.Calls || len(result.Results) != len(plan.Cases) {
				t.Fatalf("persisted graph lost coverage or retried: %+v", result)
			}
			if tt.reject {
				if result.Scorecard.Unknown != len(plan.Cases) || result.Scorecard.ChecksEvaluated != 0 || result.State != vibe.Partial {
					t.Errorf("unsafe plan must retain all checks as unevaluated UNKNOWN: %+v", result.Scorecard)
				}
			} else {
				wantPass, wantFail := len(plan.Cases), 0
				if tt.output != tt.expected {
					wantPass, wantFail = 0, len(plan.Cases)
				}
				if result.Scorecard.Passed != wantPass || result.Scorecard.Failed != wantFail || result.Scorecard.Unknown != 0 || result.Scorecard.ChecksEvaluated != plan.Calls || result.State != vibe.Completed {
					t.Errorf("scorer used decoy or lost checks: %+v", result.Scorecard)
				}
				for _, c := range b.InputSets[0].Cases {
					evidence, err := store.GetCase(ctx, session.Actor, op.ID, c.CaseKey)
					if err != nil {
						t.Fatal(err)
					}
					var input map[string]any
					if err := json.Unmarshal(evidence.Input, &input); err != nil || !reflect.DeepEqual(input, c.Payload) || len(evidence.Checks) != plan.ChecksPerCase {
						t.Fatalf("persisted case evidence/checks changed: %v", err)
					}
				}
			}
			afterPlan, err := store.Operation(ctx, op.ID)
			if err != nil || !bytes.Equal(afterPlan.Input, beforePlan) || !bytes.Equal(beforeArtifact, vibeReliabilityJSON(t, session.Document.Artifacts)) {
				t.Fatalf("execution mutated persisted accepted contract or plan: %v", err)
			}
			var attempts int
			if err := db.QueryRow(ctx, "SELECT count(*) FROM vibe_attempts WHERE operation_id=$1", op.ID).Scan(&attempts); err != nil || attempts != wantCalls {
				t.Errorf("persisted attempts = %d, want %d, err %v", attempts, wantCalls, err)
			}
		})
	}
}
