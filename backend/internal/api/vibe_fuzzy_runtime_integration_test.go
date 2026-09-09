package api

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/agentclash/agentclash/runtime/scoring"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type vibeFuzzyFixtureClient struct{ output string }

func (f vibeFuzzyFixtureClient) InvokeModel(context.Context, provider.Request) (provider.Response, error) {
	zero := json.Number("0")
	return provider.Response{OutputText: f.output, Usage: provider.Usage{InputTokens: 100, OutputTokens: 100, CostUSD: &zero}}, nil
}

func TestVibeIntegrationFuzzyBoundsUseOperandsNotPayload(t *testing.T) {
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
		name, output    string
		passed, unknown int
	}{
		{"large unrelated metadata", "done", 1, 0},
		{"oversized output remains unknown", strings.Repeat("x", 2049), 0, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := &vibe.Store{DB: db}
			models := vibe.DefaultModels()
			session, err := store.CreateSession(ctx, "anon:"+uuid.NewString(), nil, uuid.New(), models)
			if err != nil {
				t.Fatal(err)
			}
			rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
			t.Cleanup(func() { _ = rc.Close() })
			cfg := vibe.Config{Enabled: true, Credential: "fixture-only-no-network", Campaign: uuid.NewString(), AnonymousDaily: vibe.NanoUSD, AnonymousCampaign: 5 * vibe.NanoUSD, Profiles: map[string]vibe.ModelProfile{}}
			for _, id := range []string{models.Assistant, models.Target, models.Evaluator} {
				cfg.Profiles[id] = vibe.ModelProfile{ID: id, Route: "openai", InputNanoPerToken: 400, OutputNanoPerToken: 1600, Context: 128000, FramingAllowance: 2048, Conformed: true, ExpiresAt: time.Now().Add(time.Hour)}
			}
			svc := &vibe.Service{Store: store, Config: cfg, Gate: vibe.Gate{Redis: rc}, Compiler: VibePackCompiler{}}
			bundle := vibeReliabilityBundle(t)
			// This fixture deliberately requests only deterministic fuzzy scoring.
			bundle.Version.EvaluationSpec.LLMJudges = nil
			dimensions := bundle.Version.EvaluationSpec.Scorecard.Dimensions[:0]
			for _, dimension := range bundle.Version.EvaluationSpec.Scorecard.Dimensions {
				if dimension.Source == scoring.DimensionSourceValidators {
					dimensions = append(dimensions, dimension)
				}
			}
			bundle.Version.EvaluationSpec.Scorecard.Dimensions = dimensions
			bundle.InputSets[0].Cases = bundle.InputSets[0].Cases[:1]
			bundle.InputSets[0].Cases[0].Payload = map[string]any{"question": "Say done", "expected": "done", "metadata": strings.Repeat("m", 3000)}
			bundle.Version.EvaluationSpec.Validators[0].Type = scoring.ValidatorTypeFuzzyMatch
			bundle.Version.EvaluationSpec.Validators[0].ExpectedFrom = "case.payload.expected"
			if err := svc.Import(ctx, session.Actor, session.ID, session.Revision, vibeReliabilityJSON(t, bundle)); err != nil {
				t.Fatal(err)
			}
			session, err = store.GetSession(ctx, session.Actor, session.ID)
			if err != nil {
				t.Fatal(err)
			}
			artifact := session.Document.Artifacts[0]
			if err := store.Edit(ctx, session.Actor, session.ID, session.Revision, func(v *vibe.Session) error {
				v.Document.Artifacts[0].Accepted = true
				v.Document.ActiveArtifactID = &artifact.ID
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			session, err = store.GetSession(ctx, session.Actor, session.ID)
			if err != nil {
				t.Fatal(err)
			}
			op, err := svc.Prepare(ctx, session.Actor, session.ID, vibe.Submission{ClientID: uuid.New(), Revision: session.Revision, Kind: "check", ArtifactID: &artifact.ID, Models: models})
			if err != nil {
				t.Fatal(err)
			}
			runner := vibe.Runner{Service: svc, Gateway: &vibe.Gateway{Store: store, Config: cfg, Gate: svc.Gate, Client: vibeFuzzyFixtureClient{output: tt.output}}}
			if err := runner.Execute(ctx, op.ID); err != nil {
				t.Fatal(err)
			}
			if err := store.Finish(ctx, op.ID, nil); err != nil {
				t.Fatal(err)
			}
			session, err = store.GetSession(ctx, session.Actor, session.ID)
			if err != nil {
				t.Fatal(err)
			}
			result := session.Operations[len(session.Operations)-1]
			if result.Scorecard == nil || result.Scorecard.Passed != tt.passed || result.Scorecard.Unknown != tt.unknown {
				t.Fatalf("fuzzy operand guard misclassified real persisted result: %+v", result)
			}
		})
	}
}
