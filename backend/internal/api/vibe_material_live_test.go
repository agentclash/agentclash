package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/backend/internal/vibe/inputs"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Explicitly opt-in: real paid calls, production limits, isolated migrated DB.
// Configure approved, unexpired profiles; this test never relaxes their checks.
func TestLiveMaterialBuildBenchmark(t *testing.T) {
	if os.Getenv("VIBE_MATERIAL_LIVE_BENCHMARK") != "yes" {
		t.Skip("paid benchmark requires explicit opt-in")
	}
	dsn := os.Getenv("VIBE_TEST_DATABASE_URL")
	if !strings.Contains(dsn, "vibe_test") {
		t.Fatal("isolated migrated test database required")
	}
	cfg, err := vibe.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LocalTesting || !cfg.Enabled || cfg.Credential == "" {
		t.Fatal("production limits and a configured model key required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer db.Close()
	rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	defer rc.Close()
	store := vibe.NewStore(db, cfg)
	if runtime := os.Getenv("VIBE_TEST_PDF_RUNTIME"); runtime != "" {
		parser, e := inputs.NewParser(ctx, runtime)
		if e != nil {
			t.Fatal(e)
		}
		blobs, e := storage.NewFilesystemStore(storage.Config{FilesystemRoot: t.TempDir(), Bucket: "benchmark"})
		if e != nil {
			t.Fatal(e)
		}
		store.Inputs.Parser, store.Inputs.Blobs = parser, blobs
		go store.Inputs.Run(ctx, slog.Default())
	}
	svc := &vibe.Service{Store: store, Config: cfg, Gate: vibe.Gate{Redis: rc}, Compiler: VibePackCompiler{}}
	runner := &vibe.Runner{Service: svc, Gateway: &vibe.Gateway{Store: store, Config: cfg, Gate: svc.Gate}}
	cases := []struct {
		name, job, text, pdf string
		want                 []string
	}{
		{"meeting-notes", "Summarize meeting notes with decisions, action owners and deadlines. Use only the supplied notes; leave missing facts unknown.", "Mira will ship the prototype on Friday. The team decided to delay advertising. No advertising owner or date was chosen.", "", []string{"Mira", "Friday"}},
		{"inline-notes", "Build an assistant that summarizes notes with decisions, owners and deadlines, leaving missing facts unknown. Summarize these notes: Lina owns the launch report due Thursday. We decided to postpone advertising; its owner and date were not chosen.", "", "", []string{"Lina", "Thursday"}},
	}
	if pdf := os.Getenv("VIBE_BENCHMARK_PDF"); pdf != "" {
		cases = append(cases, struct {
			name, job, text, pdf string
			want                 []string
		}{"invoice-pdf", "Extract invoice number, vendor, total and currency from the supplied invoice. Leave absent fields unknown. Do not pay the invoice.", "", pdf, []string{"42"}})
	}
	for _, tc := range cases {
		if only := os.Getenv("VIBE_BENCHMARK_CASE"); only != "" && tc.name != only {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			actor := "anon:" + uuid.NewString() // Each fixture measures a first guest journey; the campaign still bounds the whole benchmark.
			v, e := store.CreateAgent(ctx, actor, nil, uuid.New(), "build", cfg.DefaultModels())
			if e != nil {
				t.Fatal(e)
			}
			data, kind, name := []byte(tc.text), "text", "Material"
			if tc.pdf != "" {
				data, e = os.ReadFile(tc.pdf)
				if e != nil {
					t.Fatal(e)
				}
				kind, name = "pdf", "invoice.pdf"
				for i := 0; i < 30 && !store.Inputs.PDFAvailable(ctx); i++ {
					time.Sleep(100 * time.Millisecond)
				}
			}
			bindings := []inputs.Binding{}
			if len(data) > 0 {
				input, e := store.Inputs.Create(ctx, v.ID, actor, uuid.New(), kind, name, data)
				if e != nil {
					t.Fatal(e)
				}
				for i := 0; i < 300 && (input.Status == "uploaded" || input.Status == "extracting"); i++ {
					time.Sleep(100 * time.Millisecond)
					input, e = store.Inputs.Get(ctx, v.ID, input.ID)
					if e != nil {
						t.Fatal(e)
					}
				}
				if input.Status != "ready" || len(input.Warnings) > 0 {
					t.Fatalf("material status %s: %s", input.Status, input.Error)
				}
				bindings = append(bindings, inputs.Binding{ID: input.ID, Hash: input.Hash, Usage: "task_input"})
			}
			q, e := svc.QuoteBuild(ctx, actor, v.ID, vibe.BuildQuoteRequest{Content: tc.job, Models: cfg.DefaultModels(), Inputs: bindings})
			if e != nil {
				t.Fatal(e)
			}
			if q.MaxCost > vibe.FirstBuildSpendCeiling {
				t.Fatal("first-cycle spending bound exceeded")
			}
			_, e = svc.Prepare(ctx, actor, v.ID, vibe.Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: tc.job, Models: cfg.DefaultModels(), TestJourney: true, CycleID: &q.ID, Inputs: bindings})
			if e != nil {
				t.Fatal(e)
			}
			started := time.Now()
			for turn := 0; turn < 4; turn++ {
				v, e = store.GetSession(ctx, actor, v.ID)
				if e != nil {
					t.Fatal(e)
				}
				var queued *vibe.Operation
				for _, op := range v.Operations {
					if op.State == vibe.Queued {
						copy := op
						queued = &copy
						break
					}
				}
				if queued == nil {
					break
				}
				runErr := runner.Execute(ctx, queued.ID)
				var issue *vibe.Fault
				if runErr != nil && !errors.As(runErr, &issue) {
					issue = &vibe.Fault{Code: "benchmark_error", Message: runErr.Error()}
				}
				if e = runner.Finalize(ctx, queued.ID, issue); e != nil {
					t.Fatal(e)
				}
				if issue != nil {
					t.Errorf("stage=%s code=%s message=%s", queued.Kind, issue.Code, issue.Message)
					break
				}
			}
			v, e = store.GetSession(ctx, actor, v.ID)
			if e != nil {
				t.Fatal(e)
			}
			report, _ := json.Marshal(map[string]any{"case": tc.name, "model": cfg.DefaultModels(), "reserved_nano_usd": q.MaxCost, "token_bound": q.TokenBound, "milliseconds": time.Since(started).Milliseconds(), "usage": v.Diagnostics.BuildUsage, "build": v.Document.Build})
			t.Log(string(report))
			if dest := os.Getenv("VIBE_BENCHMARK_REPORT_DIR"); dest != "" {
				b, _ := json.MarshalIndent(v, "", "  ")
				if e = os.WriteFile(dest+"/"+tc.name+".json", b, 0600); e != nil {
					t.Fatal(e)
				}
			}
			if v.Document.Build == nil || v.Document.Build.Phase != "results" || v.Document.Build.ClarificationsUsed != 0 {
				t.Fatal("clear task did not complete output and checks")
			}
			if u := v.Diagnostics.BuildUsage; u == nil || u.Tokens == 0 || u.UnknownCost != 0 || u.UnknownTokens != 0 || u.Cost > vibe.FirstBuildSpendCeiling {
				t.Fatal("benchmark lacks complete, bounded usage accounting")
			}
			output := ""
			for _, m := range v.Document.Messages {
				if m.Origin == "playground" && m.Role == "assistant" {
					output = m.Content
				}
			}
			for _, fact := range tc.want {
				if !strings.Contains(output, fact) {
					t.Errorf("actual output missing known material fact %q", fact)
				}
			}
			if len(v.Operations) != 3 || len(v.Operations[2].Results) != 3 {
				t.Fatal("initial trial or required checks missing/duplicated")
			}
		})
	}
}
