package api

// This opt-in fixture serves the production HTTP handler and executes its outbox
// through a real Temporal server/worker. Only provider responses and Redis are
// fixtures. The opt-in auth test uses a local identity provider and real
// DevelopmentAuthenticator; no hosted identity service is contacted. Never
// construct a network-backed provider client in this file.
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/protobuf/types/known/durationpb"
)

const browserFixtureModel = "liquid/lfm-2.5-2.6b:free"

func TestVibeBrowserStack(t *testing.T) {
	if os.Getenv("VIBE_BROWSER_STACK") != "1" {
		t.Skip("opt-in server for web/playwright.vibe-stack.config.ts")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db := browserFixtureDatabase(t, ctx)
	namespace := "vibe-browser-" + uuid.NewString()
	temporalClient := browserFixtureTemporal(t, ctx, namespace)
	mini := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	defer rc.Close()
	cfg := vibe.Config{GroundedJudging: true, ReliableAuthoring: true, Enabled: true, FreeOnly: true, LocalTesting: true, Credential: "fake-no-network", DefaultModel: browserFixtureModel, Campaign: uuid.NewString(), AnonymousDaily: vibe.NanoUSD, AnonymousCampaign: 5 * vibe.NanoUSD, Profiles: map[string]vibe.ModelProfile{browserFixtureModel: {ID: browserFixtureModel, Route: "liquid/fp8", Free: true, Conformed: true, StructuredOutputs: true, Context: 65536, FramingAllowance: 4096, ExpiresAt: time.Now().Add(time.Hour)}}}
	cfg.SuiteReviewVersion = browserFixtureEnv("VIBE_BROWSER_REVIEW_VERSION", vibe.LatestSuiteValidatorVersion)
	if os.Getenv("VIBE_BROWSER_V15") == "1" {
		cfg.ConversationState, cfg.PreciseActions, cfg.ContextGuidance, cfg.InterpretedAuthoring = true, true, true, true
		cfg.SourcePolicyVersion = vibe.SourcePolicyVersion
		cfg.TwoDoor = os.Getenv("VIBE_BROWSER_TWO_DOOR") == "1"
	}
	store := vibe.NewStore(db, cfg)
	svc := &vibe.Service{Store: store, Config: cfg, Gate: vibe.Gate{Redis: rc}, Compiler: VibePackCompiler{}}
	fake := &browserFixtureProvider{}
	runner := &vibe.Runner{Service: svc, Gateway: &vibe.Gateway{Store: store, Config: cfg, Gate: svc.Gate, Client: fake}}
	worker := vibe.NewWorker(temporalClient, runner)
	if err := worker.Start(); err != nil {
		t.Fatal(err)
	}
	defer worker.Stop()
	dispatchCtx, cancelDispatch := context.WithCancel(ctx)
	dispatched := make(chan struct{})
	go func() {
		defer close(dispatched)
		vibe.DispatchOutbox(dispatchCtx, temporalClient, store, slog.Default(), svc)
	}()
	defer func() { cancelDispatch(); <-dispatched }()
	router := chi.NewRouter()
	origin := browserFixtureEnv("VIBE_BROWSER_WEB_ORIGIN", "http://127.0.0.1:53518")
	router.Use(newCORSMiddleware("dev", map[string]struct{}{origin: {}}))
	browserFixtureIdentity(t, db)
	if err := store.Grant(ctx, "org:a8000000-0000-4000-8000-000000000002", "fixture-auth-credit", vibe.NanoUSD); err != nil {
		t.Fatal(err)
	}
	auth := browserDevelopmentAuth{}
	router.Get("/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		caller, err := auth.Authenticate(r)
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		result, err := NewUserManager(repository.New(db)).GetMe(r.Context(), caller)
		if err != nil {
			http.Error(w, "fixture user unavailable", 500)
			return
		}
		browserFixtureJSON(w, result)
	})
	router.Mount("/v1/vibe", (&VibeHandler{Service: svc, Auth: auth, CookieSecret: uuid.NewString() + uuid.NewString()}).Routes())
	router.Get("/__fixture/ready", func(w http.ResponseWriter, r *http.Request) {
		browserFixtureJSON(w, map[string]any{"ready": true, "temporal_namespace": namespace, "provider": "in-process scripted fixture"})
	})
	router.Post("/__fixture/control", func(w http.ResponseWriter, r *http.Request) {
		var command struct {
			TargetDelayMS  int `json:"target_delay_ms"`
			FailEditCalls  int `json:"fail_edit_calls"`
			RateLimitCalls int `json:"rate_limit_calls"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&command); err != nil || command.FailEditCalls < 0 || command.FailEditCalls > 2 || command.RateLimitCalls < 0 || command.RateLimitCalls > 2 {
			http.Error(w, "expected fixture failure counts in [0,2]", http.StatusBadRequest)
			return
		}
		fake.mu.Lock()
		fake.targetDelayMS = min(max(command.TargetDelayMS, 0), 3000)
		fake.failEditCalls = command.FailEditCalls
		fake.rateLimitCalls = command.RateLimitCalls
		fake.mu.Unlock()
		browserFixtureJSON(w, map[string]bool{"ok": true})
	})
	router.Get("/__fixture/evidence", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.URL.Query().Get("session"))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		var actor string
		if err = db.QueryRow(r.Context(), "SELECT actor FROM vibe_sessions WHERE id=$1", id).Scan(&actor); err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		session, err := store.GetSession(r.Context(), actor, id)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		hashes := map[string]map[string]string{}
		for _, artifact := range session.Document.Artifacts {
			blueprintHash, err := vibe.CanonicalJSONHash(artifact.Blueprint)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			var blueprint map[string]json.RawMessage
			if err = json.Unmarshal(artifact.Blueprint, &blueprint); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			grading, _ := json.Marshal(map[string]json.RawMessage{"judges": blueprint["judges"], "validators": blueprint["validators"], "dimensions": blueprint["dimensions"]})
			gradingHash, err := vibe.CanonicalJSONHash(grading)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			hashes[artifact.ID.String()] = map[string]string{"blueprint": blueprintHash, "grading": gradingHash}
		}
		var attemptCount, unsettled, delivered int
		if err = db.QueryRow(r.Context(), "SELECT count(*), count(*) FILTER (WHERE actual_cost IS NULL OR actual_cost<>0 OR state<>'SUCCEEDED') FROM vibe_attempts WHERE operation_id IN (SELECT id FROM vibe_operations WHERE session_id=$1)", id).Scan(&attemptCount, &unsettled); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if err = db.QueryRow(r.Context(), "SELECT count(*) FROM vibe_outbox WHERE delivered_at IS NOT NULL AND operation_id IN (SELECT id FROM vibe_operations WHERE session_id=$1)", id).Scan(&delivered); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		fake.mu.Lock()
		calls := append([]browserFixtureCall(nil), fake.calls...)
		fake.mu.Unlock()
		browserFixtureJSON(w, map[string]any{"session": session, "hashes": hashes, "calls": calls, "attempt_count": attemptCount, "unsettled_attempts": unsettled, "delivered_operations": delivered, "temporal_namespace": namespace})
	})
	address := browserFixtureEnv("VIBE_BROWSER_API_ADDRESS", "127.0.0.1:55441")
	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" {
		t.Fatal("fixture API must bind 127.0.0.1")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: router, ReadHeaderTimeout: 10 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	defer server.Close()
	t.Logf("browser fixture ready: %s (Temporal namespace %s)", listener.Addr(), namespace)
	select {
	case <-ctx.Done():
	case err = <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatal(err)
		}
	}
}

func browserFixtureDatabase(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("VIBE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("VIBE_TEST_DATABASE_URL is required; its role must have CREATEDB")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if config.ConnConfig.Database != "vibe_test" && !strings.HasPrefix(config.ConnConfig.Database, "vibe_test_") {
		t.Fatal("refusing a non-test database URL")
	}
	admin, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	name := "vibe_test_browser_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	identifier := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanupCtx, "DROP DATABASE "+identifier+" WITH (FORCE)"); err != nil {
			t.Errorf("remove fixture database %s: %v", name, err)
		}
	})
	isolated := config.Copy()
	isolated.ConnConfig.Database = name
	db, err := pgxpool.NewWithConfig(ctx, isolated)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	files, err := filepath.Glob("../../db/migrations/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatal("could not find repository migrations", err)
	}
	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, strings.SplitN(string(body), "-- +goose Down", 2)[0]); err != nil {
			t.Fatalf("migration %s: %v", path, err)
		}
	}
	return db
}

func browserFixtureTemporal(t *testing.T, ctx context.Context, namespace string) client.Client {
	t.Helper()
	address := os.Getenv("VIBE_BROWSER_TEMPORAL_ADDRESS")
	if address == "" {
		server, err := testsuite.StartDevServer(ctx, testsuite.DevServerOptions{ExistingPath: os.Getenv("VIBE_BROWSER_TEMPORAL_CLI"), ClientOptions: &client.Options{Namespace: namespace}, LogLevel: "error"})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := server.Stop(); err != nil {
				t.Logf("Temporal shutdown: %v", err)
			}
		})
		return server.Client()
	}
	ns, err := client.NewNamespaceClient(client.Options{HostPort: address})
	if err != nil {
		t.Fatal(err)
	}
	defer ns.Close()
	if err = ns.Register(ctx, &workflowservice.RegisterNamespaceRequest{Namespace: namespace, WorkflowExecutionRetentionPeriod: durationpb.New(24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	c, err := client.Dial(client.Options{HostPort: address, Namespace: namespace})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

func browserFixtureEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func browserFixtureJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

type browserFixtureCall struct {
	Role    string            `json:"role"`
	Request *vibe.SourceBlock `json:"request,omitempty"`
}

type browserFixtureProvider struct {
	targetDelayMS  int
	mu             sync.Mutex
	failEditCalls  int
	rateLimitCalls int
	calls          []browserFixtureCall
}

func (f *browserFixtureProvider) InvokeModel(_ context.Context, req provider.Request) (provider.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if req.Model != browserFixtureModel || len(req.Messages) < 2 {
		return provider.Response{}, fmt.Errorf("unexpected model or message envelope")
	}
	var format struct {
		JSONSchema struct {
			Name string `json:"name"`
		} `json:"json_schema"`
	}
	if len(req.ResponseFormat) > 0 {
		if err := json.Unmarshal(req.ResponseFormat, &format); err != nil {
			return provider.Response{}, err
		}
	}
	name := format.JSONSchema.Name
	if name == "" && strings.Contains(req.Messages[0].Content, "DECISION EXAMPLES (illustrations") {
		name = "vibe_interpretation_v15"
	}
	var output any
	switch {
	case strings.HasPrefix(req.Messages[0].Content, "Write instructions for a bounded interactive"):
		f.calls = append(f.calls, browserFixtureCall{Role: "prototype"})
		return browserFixtureResponse(`{"instructions":"Only unopened items bought within 30 days are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund."}`), nil
	case strings.Contains(req.Messages[0].Content, "Every finding has exactly"):
		name = "judge"
		var input struct {
			Criteria struct {
				Key string `json:"key"`
			} `json:"criteria"`
			Replies []vibe.EvidenceMessage `json:"replies"`
		}
		if err := json.Unmarshal([]byte(req.Messages[1].Content), &input); err != nil || len(input.Replies) != 1 {
			return provider.Response{}, fmt.Errorf("missing saved reply: %v", err)
		}
		text := input.Replies[0].Content
		output = map[string]any{"key": input.Criteria.Key, "pass": text != "Opened items are eligible.", "reasoning": "The scripted response is compared with the unopened-only return policy.", "finding": map[string]any{"kind": "observed", "quotes": []any{map[string]any{"message_id": "output", "text": text}}, "missing": "", "covered_message_ids": []string{}}}
	case strings.HasPrefix(req.Messages[0].Content, "Evaluate the supplied output"):
		name = "judge"
		var input struct {
			Output string `json:"agent_output"`
		}
		if err := json.Unmarshal([]byte(req.Messages[1].Content), &input); err != nil {
			return provider.Response{}, err
		}
		output = map[string]any{"pass": input.Output != "Opened items are eligible.", "reasoning": "The scripted response is compared with the unopened-only return policy."}
	case name == "":
		name = "target"
		if f.targetDelayMS > 0 {
			time.Sleep(time.Duration(f.targetDelayMS) * time.Millisecond)
		}
		question := req.Messages[len(req.Messages)-1].Content
		answer := "What is the purchase age and item condition?"
		if strings.Contains(question, "unopened") {
			answer = "This unopened purchase is eligible; I cannot process a refund."
		}
		if strings.Contains(question, "I opened") {
			answer = "Opened items are ineligible."
			if strings.Contains(req.Messages[0].Content, "Opened items are eligible.") {
				answer = "Opened items are eligible."
			}
		}
		f.calls = append(f.calls, browserFixtureCall{Role: name})
		return browserFixtureResponse(answer), nil
	case name == "vibe_suite_review_v1" || name == "vibe_suite_review_v2" || name == "vibe_suite_review_v3":
		var input vibe.SuiteReviewInput
		if err := json.Unmarshal([]byte(req.Messages[1].Content), &input); err != nil {
			return provider.Response{}, err
		}
		ids, sources := []string{}, []string{}
		rules := []vibe.SuiteRuleReview{}
		for _, rule := range input.Policy.Rules {
			ids = append(ids, rule.ID)
			for _, source := range rule.SourceBlockIDs {
				if !browserFixtureContains(sources, source) {
					sources = append(sources, source)
				}
			}
			rules = append(rules, vibe.SuiteRuleReview{RuleID: rule.ID, SuiteReviewFinding: vibe.SuiteReviewFinding{Status: vibe.SuiteSupported, RuleIDs: []string{rule.ID}, SourceBlockIDs: rule.SourceBlockIDs, Reason: "This explicit rule follows its cited original source."}})
		}
		finding := vibe.SuiteReviewFinding{Status: vibe.SuiteSupported, RuleIDs: ids, SourceBlockIDs: sources, Reason: "The fixture follows the supplied policy and exact source text."}
		cases := []vibe.SuiteCaseReview{}
		for _, c := range input.Cases {
			entry := vibe.SuiteCaseReview{CaseKey: c.CaseKey, SuiteReviewFinding: finding}
			if c.Expected == "Confirm opened items are eligible." {
				entry.Status = vibe.SuiteContradicted
				entry.Reason = "The opened-item expectation contradicts the unopened-only rule."
			}
			cases = append(cases, entry)
		}
		review := map[string]any{"cases": cases, "rules": rules, "shared_criteria": finding, "policy_reconciliation": finding}
		if name != "vibe_suite_review_v1" && vibe.RequiresConsistency(input) {
			ledger, err := browserFixtureConsistency(input)
			if err != nil {
				return provider.Response{}, err
			}
			review["consistency"] = browserFixtureConsistencyPayload(ledger, name)
		}
		output = review
	default:
		var input struct {
			Request  vibe.SourceBlock     `json:"current_request"`
			Policy   *vibe.PolicySnapshot `json:"desired_rules"`
			Artifact *vibe.Artifact       `json:"selected_tests"`
		}
		if err := json.Unmarshal([]byte(req.Messages[1].Content), &input); err != nil {
			return provider.Response{}, err
		}
		if req.Messages[len(req.Messages)-1].Content != input.Request.Text {
			return provider.Response{}, fmt.Errorf("routing or repair replaced the exact current request")
		}
		f.calls = append(f.calls, browserFixtureCall{Role: name, Request: &input.Request})
		switch name {
		case "vibe_edit_tests_v13":
			var bases struct {
				Base map[string]any `json:"policy_edit_base"`
			}
			if err := json.Unmarshal([]byte(req.Messages[1].Content), &bases); err != nil {
				return provider.Response{}, err
			}
			first, second := "I bought an unopened item exactly 30 days ago. Can I return it?", "My item is unopened. Can I return it?"
			firstExpected, secondExpected := "Confirm eligibility without claiming to process a refund.", "Ask only for purchase age."
			output = map[string]any{"policy_patch": map[string]any{"base_id": bases.Base["id"], "base_hash": bases.Base["hash"], "changes": []any{}}, "case_changes": []vibe.CaseChange{{Action: "add", Input: &first, Expected: &firstExpected}, {Action: "add", Input: &second, Expected: &secondExpected}}}
		case "vibe_interpretation_v15": // JSON mode plus the server-validated typed union.
			if os.Getenv("VIBE_BROWSER_V15") != "1" {
				return provider.Response{}, fmt.Errorf("unexpected JSON-mode stage")
			}
			facts := []any{}
			var answer any
			var action any = map[string]any{"kind": "reply", "text": "Tell me what your agent should help with.", "example": nil}
			if strings.HasPrefix(input.Request.Text, "Suggest a focused") {
				action = map[string]any{"kind": "suggest_fix"}
			}
			if input.Request.Text == "Build me a returns agent for Shopify" {
				facts = append(facts, map[string]any{"kind": "job", "quote": input.Request.Text, "correction_ref": 0})
				action = map[string]any{"kind": "ask", "text": "Which returns should qualify?", "purpose": "clarify_rule", "options": []string{}}
			} else if strings.Contains(input.Request.Text, "sample policy") || strings.Contains(input.Request.Text, "don't know") {
				answer = map[string]any{"quote": input.Request.Text, "unknown": true}
			} else if strings.Contains(input.Request.Text, "Only unopened") {
				if strings.Contains(input.Request.Text, "Answer shop return questions.") {
					facts = append(facts, map[string]any{"kind": "job", "quote": "Answer shop return questions.", "correction_ref": 0})
				}
				facts = append(facts, map[string]any{"kind": "rule", "quote": input.Request.Text, "correction_ref": 0})
				var stateInput struct {
					PendingQuestion any `json:"pending_question"`
				}
				_ = json.Unmarshal([]byte(req.Messages[1].Content), &stateInput)
				if stateInput.PendingQuestion != nil {
					answer = map[string]any{"quote": input.Request.Text, "unknown": false}
				}
				action = map[string]any{"kind": "prepare_tests", "count": 3}
			}
			output = map[string]any{"observations": facts, "answer": answer, "scope_change_quote": "", "source_message_ids": []string{}, "brevity_quote": "", "action": action}
		case "vibe_route_v11":
			if f.rateLimitCalls > 0 {
				f.rateLimitCalls--
				return provider.Response{}, provider.Failure{ProviderKey: "openrouter", Code: provider.FailureCodeRateLimit, Message: "Fixture busy", RetryAfter: 20 * time.Second}
			}
			intent, count := "prepare_tests", 3
			if strings.HasPrefix(input.Request.Text, "Suggest a focused") {
				intent, count = "suggest_fix", 0
			}
			if strings.HasPrefix(input.Request.Text, "Add a test") {
				intent, count = "edit_tests", 0
			}
			if input.Request.Text == "Thanks, I am getting coffee." {
				intent, count = "chat", 0
			}
			output = map[string]any{"intent": intent, "count": count, "reply": "Your conversation is saved."}
		case "vibe_prepare_tests_v11":
			rules := []vibe.PolicyRule{}
			for _, rule := range [][2]string{{"job", "Answer shop return questions."}, {"window", "The return window is 30 days."}, {"condition", "Only unopened items are eligible."}, {"missing", "Ask only for missing purchase age or item condition."}, {"no-refund", "Never claim to process a refund."}} {
				rules = append(rules, vibe.PolicyRule{ID: rule[0], Statement: rule[1], SourceBlockIDs: []string{input.Request.ID}})
				if os.Getenv("VIBE_BROWSER_V15") == "1" {
					rules[len(rules)-1].Evidence = []vibe.RuleEvidence{{SourceBlockID: input.Request.ID, Quote: input.Request.Text, Kind: "requirement"}}
				}
			}
			output = map[string]any{"rules": rules, "tests": map[string]any{"title": "Return checks", "summary": "Eligibility and missing details.", "success_criteria": "Only unopened purchases within 30 days are eligible. Ask only for missing age or condition. Never claim to process a refund.", "scenarios": []vibe.TestScenario{{Input: "I bought an unopened item exactly 10 days ago. Can I return it?", Expected: "Confirm eligibility without claiming to process a refund."}, {Input: "I opened the item bought 10 days ago. Can I return it?", Expected: "Explain that opened items are ineligible."}, {Input: "Can I return an item?", Expected: "Ask only for purchase age and condition."}}}}
		case "vibe_edit_tests_v11":
			if f.failEditCalls > 0 {
				f.failEditCalls--
				return browserFixtureResponse(`{"invalid":"injected initial/repair author failure"}`), nil
			}
			if input.Policy == nil || input.Artifact == nil {
				return provider.Response{}, fmt.Errorf("edit lost accepted policy/tests")
			}
			rules := append([]vibe.PolicyRule(nil), input.Policy.Rules...)
			rules = append(rules, vibe.PolicyRule{ID: "off-topic", Statement: "Politely bring off-topic requests back to returns.", SourceBlockIDs: []string{input.Request.ID}})
			question, expected := "Can you recommend a vodka cocktail?", "Politely bring the customer back to shop returns."
			output = map[string]any{"rules": rules, "criteria": nil, "case_changes": []vibe.CaseChange{{Action: "add", Input: &question, Expected: &expected}}}
		case "vibe_suggest_fix_v11":
			output = map[string]any{"instruction_edits": []vibe.InstructionEdit{{Before: "Opened items are eligible.", After: "Only unopened items are eligible."}}}
		default:
			return provider.Response{}, fmt.Errorf("unexpected provider role %q", name)
		}
		name = "" // Author/router calls already carry their source in the journal.
	}
	if name != "" {
		f.calls = append(f.calls, browserFixtureCall{Role: name})
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return provider.Response{}, err
	}
	return browserFixtureResponse(string(encoded)), nil
}

func browserFixtureResponse(text string) provider.Response {
	zero := json.Number("0")
	return provider.Response{OutputText: text, Usage: provider.Usage{InputTokens: 100, OutputTokens: 200, CostUSD: &zero}}
}

func browserFixtureContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// Interpret only the exact, known browser scenarios. These scripted source and
// input spans exercise the real consistency checker; they are not an NLP parser.
func browserFixtureConsistency(input vibe.SuiteReviewInput) (vibe.ConsistencyLedger, error) {
	var sourceID string
	for _, rule := range input.Policy.Rules {
		if rule.ID == "missing" && len(rule.SourceBlockIDs) == 1 {
			sourceID = rule.SourceBlockIDs[0]
		}
	}
	span := func(text string) vibe.ConsistencySourceSpan {
		return vibe.ConsistencySourceSpan{SourceBlockID: sourceID, Text: text}
	}
	ledger := vibe.ConsistencyLedger{
		Entities: []vibe.ConsistencyEntity{{ID: "item", Source: span("items")}},
		Fields: []vibe.ConsistencyField{
			{ID: "purchase-age", EntityID: "item", Kind: "number", Aliases: []vibe.ConsistencySourceSpan{span("purchase age")}},
			{ID: "condition", EntityID: "item", Kind: "enum", Aliases: []vibe.ConsistencySourceSpan{span("item condition"), span("condition")}},
		},
		MissingOnly: []vibe.MissingOnlyConstraint{{RuleID: "missing", Source: span("Ask only for missing purchase age or item condition."), FieldIDs: []string{"purchase-age", "condition"}}},
		Cases:       []vibe.CaseFactLedger{},
	}
	for _, c := range input.Cases {
		var payload struct {
			Question string `json:"question"`
		}
		if err := json.Unmarshal(c.Input, &payload); err != nil {
			return ledger, err
		}
		age := vibe.ConsistencyFact{EntityID: "item", FieldID: "purchase-age", State: "missing", InputPointer: "/question"}
		condition := vibe.ConsistencyFact{EntityID: "item", FieldID: "condition", State: "missing", InputPointer: "/question"}
		switch payload.Question {
		case "I bought an unopened item exactly 30 days ago. Can I return it?":
			age.State, age.Evidence, age.Literal = "present", "I bought an unopened item exactly 30 days ago.", "30"
			condition.State, condition.Evidence, condition.Literal = "present", age.Evidence, "unopened"
		case "My item is unopened. Can I return it?":
			condition.State, condition.Evidence, condition.Literal = "present", "My item is unopened.", "unopened"
		case "I bought an unopened item exactly 10 days ago. Can I return it?":
			age.State, age.Evidence, age.Literal = "present", "I bought an unopened item exactly 10 days ago.", "10"
			condition.State, condition.Evidence, condition.Literal = "present", age.Evidence, "unopened"
		case "I opened the item bought 10 days ago. Can I return it?":
			age.State, age.Evidence, age.Literal = "present", "I opened the item bought 10 days ago.", "10"
			condition.State, condition.Evidence, condition.Literal = "present", age.Evidence, "opened"
		case "Can I return an item?", "Can you recommend a vodka cocktail?":
		default:
			return ledger, fmt.Errorf("unrecognized browser consistency case %q", payload.Question)
		}
		entry := vibe.CaseFactLedger{CaseKey: c.CaseKey, Facts: []vibe.ConsistencyFact{age, condition}, Obligations: []vibe.ConsistencyObligation{}}
		if c.Expected == "Ask only for purchase age and condition." {
			for _, field := range []string{"purchase-age", "condition"} {
				entry.Obligations = append(entry.Obligations, vibe.ConsistencyObligation{EntityID: "item", FieldID: field, Kind: "ask", Evidence: c.Expected})
			}
		}
		ledger.Cases = append(ledger.Cases, entry)
	}
	return ledger, nil
}

func browserFixtureConsistencyPayload(ledger vibe.ConsistencyLedger, schemaName string) any {
	if schemaName != "vibe_suite_review_v3" {
		return ledger // Frozen v2 responses include their model-extracted obligations.
	}
	// V3 asks the server to derive obligations from the actual expected text.
	// Omit the model field completely, including empty/null obligation arrays.
	cases := make([]map[string]any, 0, len(ledger.Cases))
	for _, c := range ledger.Cases {
		cases = append(cases, map[string]any{"case_key": c.CaseKey, "facts": c.Facts})
	}
	return map[string]any{"entities": ledger.Entities, "fields": ledger.Fields, "missing_only": ledger.MissingOnly, "cases": cases}
}

// Only the opt-in localhost test server maps its local identity-provider token
// into the production development authenticator's header contract.
type browserDevelopmentAuth struct{}

func (browserDevelopmentAuth) Authenticate(r *http.Request) (Caller, error) {
	parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), ".")
	if len(parts) != 3 {
		return Caller{}, ErrUnauthenticated
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Caller{}, ErrUnauthenticated
	}
	var claims struct {
		Subject string `json:"sub"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Subject != "a8000000-0000-4000-8000-000000000001" {
		return Caller{}, ErrUnauthenticated
	}
	copy := r.Clone(r.Context())
	copy.Header.Set(headerUserID, claims.Subject)
	return NewDevelopmentAuthenticator().Authenticate(copy)
}
func browserFixtureIdentity(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	for _, sql := range []string{
		`INSERT INTO users(id,workos_user_id,email) VALUES('a8000000-0000-4000-8000-000000000001','a8000000-0000-4000-8000-000000000001','vibe-browser@example.invalid')`,
		`INSERT INTO organizations(id,name,slug) VALUES('a8000000-0000-4000-8000-000000000002','Browser tests','vibe-browser')`,
		`INSERT INTO workspaces(id,organization_id,name,slug) VALUES('a8000000-0000-4000-8000-000000000003','a8000000-0000-4000-8000-000000000002','Browser workspace','vibe-browser')`,
		`INSERT INTO organization_memberships(organization_id,user_id,role,membership_status) VALUES('a8000000-0000-4000-8000-000000000002','a8000000-0000-4000-8000-000000000001','org_admin','active')`,
	} {
		if _, err := db.Exec(context.Background(), sql); err != nil {
			t.Fatal(err)
		}
	}
}
