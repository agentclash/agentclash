package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type testsJourneyClient struct {
	t             *testing.T
	authorReply   string
	inputs        []string
	policies      []string
	fixed         bool
	lastInput     string
	authorInputs  []string
	authorReplies []string
}

func (f *testsJourneyClient) InvokeModel(_ context.Context, r provider.Request) (provider.Response, error) {
	output := f.authorReply
	if strings.HasPrefix(r.Messages[0].Content, "You are Vibe Evals.") {
		f.authorInputs = append(f.authorInputs, r.Messages[1].Content)
		if len(f.authorReplies) > 0 {
			output, f.authorReplies = f.authorReplies[0], f.authorReplies[1:]
		}
	}
	if strings.Contains(r.Messages[0].Content, "Evaluate the supplied output") {
		output = `{"pass":true,"reasoning":"The answer follows the stated rule."}`
		if f.lastInput == "Return an unopened item bought 45 days ago." && !f.fixed {
			output = `{"pass":false,"reasoning":"The agent allowed a return after the deadline."}`
		}
	} else if !strings.HasPrefix(r.Messages[0].Content, "You are Vibe Evals.") {
		f.lastInput = r.Messages[1].Content
		f.inputs = append(f.inputs, f.lastInput)
		f.policies = append(f.policies, r.Messages[0].Content)
		output = "Please provide the purchase age and item condition."
		if strings.Contains(f.lastInput, "10 days") {
			output = "The unopened item is eligible within 30 days."
		}
		if strings.Contains(f.lastInput, "45 days") {
			output = "You can return it."
			if f.fixed {
				output = "It is past 30 days and is not eligible."
			}
		}
	}
	if bytes.Contains(r.ResponseFormat, []byte("vibe_route_v10")) {
		var full map[string]any
		if json.Unmarshal([]byte(output), &full) == nil {
			output = string(mustJSON(f.t, map[string]any{"intent": full["intent"], "reply": full["reply"]}))
		}
	}
	zero := json.Number("0")
	return provider.Response{OutputText: output, Usage: provider.Usage{InputTokens: 100, OutputTokens: 200, CostUSD: &zero}}, nil
}

func TestVibeIntegrationCoreJourneyKeepsRunnableTests(t *testing.T) {
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
	model := "liquid/lfm-2.5-2.6b:free"
	cfg := vibe.Config{Enabled: true, FreeOnly: true, LocalTesting: true, Credential: "fake-no-network", DefaultModel: model, Profiles: map[string]vibe.ModelProfile{model: {ID: model, Route: "liquid/fp8", Free: true, Conformed: true, StructuredOutputs: true, Context: 65536, FramingAllowance: 4096, ExpiresAt: time.Now().Add(time.Hour)}}}
	store := vibe.NewStore(db, cfg)
	cfg.Campaign, cfg.AnonymousDaily, cfg.AnonymousCampaign = uuid.NewString(), vibe.NanoUSD, 5*vibe.NanoUSD
	rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	defer rc.Close()
	svc := &vibe.Service{Store: store, Config: cfg, Gate: vibe.Gate{Redis: rc}, Compiler: VibePackCompiler{}}
	actor := "anon:" + uuid.NewString()
	v, err := store.CreateSession(ctx, actor, nil, uuid.New(), cfg.DefaultModels())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM vibe_attempts WHERE operation_id IN (SELECT id FROM vibe_operations WHERE session_id=$1)", v.ID)
	}()
	inputs := []string{"Return an unopened item bought 10 days ago.", "Return an unopened item bought 45 days ago.", "I want to return an item."}
	proposal := map[string]any{"intent": "prepare_tests", "case_changes": []any{}, "criteria": nil, "instruction_edits": []any{}, "remember": []any{}, "reply": "I prepared tests for eligible returns, late purchases and missing details.", "tests": map[string]any{
		"title": "Returns tests", "summary": "Eligible returns, late purchases and missing details.", "success_criteria": "Allow unopened returns within 30 days; decline opened or older items; ask for missing purchase age or condition; never claim to process a refund.",
		"scenarios": []vibe.TestScenario{{Input: inputs[0], Expected: "Explain that this item is eligible. Never claim a refund was processed."}, {Input: inputs[1], Expected: "Explain that this item is past the 30-day deadline and is ineligible."}, {Input: inputs[2], Expected: "Ask for the purchase age and whether the item is unopened."}},
	}}
	b, _ := json.Marshal(proposal)
	fake := &testsJourneyClient{t: t, authorReply: string(b)}
	runner := vibe.Runner{Service: svc, Gateway: &vibe.Gateway{Store: store, Config: cfg, Gate: svc.Gate, Client: fake}}
	reload := func() {
		t.Helper()
		v, err = store.GetSession(ctx, actor, v.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	execute := func(sub vibe.Submission) vibe.Operation {
		t.Helper()
		sub.ClientID = uuid.New()
		sub.Revision = v.Revision
		sub.TestJourney = true
		sub.EvaluationFirst = true
		sub.Models = cfg.DefaultModels()
		o, e := svc.Prepare(ctx, actor, v.ID, sub)
		if e != nil {
			t.Fatal(e)
		}
		if e = runner.Execute(ctx, o.ID); e != nil {
			t.Fatal(e)
		}
		if e = store.Finish(ctx, o.ID, nil); e != nil {
			t.Fatal(e)
		}
		reload()
		return v.Operations[len(v.Operations)-1]
	}
	first := execute(vibe.Submission{Kind: "message", Content: "My agent handles returns. Unopened items are eligible within 30 days. Ask for missing age or condition. Never claim to process refunds."})
	if first.ModelCalls != 2 || len(fake.inputs) != 0 || len(v.Document.Artifacts) != 1 || !v.Document.TestJourney {
		t.Fatal("description did not prepare tests without a target call")
	}
	a := v.Document.Artifacts[0]
	if !a.IsTestSuite() || a.AgentPrompt != "" {
		t.Fatal("description manufactured an agent")
	}
	before := append([]byte(nil), a.Blueprint...)
	if _, err = svc.Prepare(ctx, actor, v.ID, vibe.Submission{TestJourney: true, ClientID: uuid.New(), Revision: v.Revision, Kind: "check", ArtifactID: &a.ID, ApproveArtifact: true, Models: cfg.DefaultModels()}); err == nil || !strings.Contains(err.Error(), "agent") {
		t.Fatal("unbound tests ran", err)
	}
	// Bind through the actual HTTP edit contract, preserving the supplied policy
	// and all tests. Authentication is a local fixture, not real WorkOS login.
	org, ws, user := uuid.New(), uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO organizations(id,name,slug) VALUES($1,'Test journey',$2)", []any{org, org.String()}},
		{"INSERT INTO workspaces(id,organization_id,name,slug) VALUES($1,$2,'Test journey',$3)", []any{ws, org, ws.String()}},
		{"INSERT INTO users(id,workos_user_id,email) VALUES($1,$2,$3)", []any{user, user.String(), user.String() + "@example.invalid"}},
		{"INSERT INTO organization_memberships(organization_id,user_id,role,membership_status) VALUES($1,$2,'org_admin','active')", []any{org, user}},
	} {
		if _, err = db.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	owner := "user:" + user.String()
	if err = store.Claim(ctx, actor, owner, v.ID); err != nil {
		t.Fatal(err)
	}
	actor = owner
	reload()
	policy := "Answer customer return questions. Allow returns of unopened items. Ask for missing details."
	handler := (&VibeHandler{Service: svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	payload, _ := json.Marshal(map[string]any{"revision": v.Revision, "artifact_id": a.ID, "agent_prompt": policy})
	req := httptest.NewRequest(http.MethodPatch, "/sessions/"+v.ID.String(), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer fixture")
	req.Header.Set(headerUserID, user.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	reload()
	a = v.Document.Artifacts[len(v.Document.Artifacts)-1]
	if a.AgentPrompt != policy || !bytes.Equal(before, a.Blueprint) {
		t.Fatal("binding rewrote the agent or the tests")
	}
	baseline := execute(vibe.Submission{Kind: "check", ArtifactID: &a.ID, ApproveArtifact: true})
	if baseline.ModelCalls != 6 || baseline.Scorecard.Passed != 2 || baseline.Scorecard.Failed != 1 || !reflect.DeepEqual(fake.inputs, inputs) {
		t.Fatalf("run skipped inputs or fabricated score: %+v inputs=%v", baseline.Scorecard, fake.inputs)
	}
	for _, p := range fake.policies {
		if p != policy {
			t.Fatal("target policy was modified")
		}
	}
	fix := policy + " Decline purchases older than 30 days."
	b, _ = json.Marshal(map[string]any{"intent": "suggest_fix", "case_changes": []any{}, "criteria": nil, "remember": []any{}, "reply": "Add the missing 30-day deadline.", "tests": nil, "instruction_edits": []vibe.InstructionEdit{{Before: "Ask for missing details.", After: "Ask for missing details. Decline purchases older than 30 days."}}})
	fake.authorReply = string(b)
	execute(vibe.Submission{Kind: "message", Content: "Help me fix this", Purpose: "suggest_change", ArtifactID: &a.ID, BaselineID: &baseline.ID})
	a = v.Document.Artifacts[len(v.Document.Artifacts)-1]
	if !bytes.Equal(before, a.Blueprint) || a.AgentPrompt != fix || a.Accepted {
		t.Fatal("suggestion changed tests or applied itself")
	}
	// Reading results is separate from permission to change/run anything. A new
	// conversational reply owns no proposal; the existing fix remains unchanged.
	proposalCount := len(v.Document.Artifacts)
	for _, chat := range []struct{ message, intent, reply string }{
		{"let's drink vodka", "chat", "Ha, taking a break? Your tests will be here when you're ready."},
		{"thanks bro", "chat", "You're welcome."},
		{"Why did the second one fail?", "explain_results", "It allowed a 45-day purchase even though your rule says 30 days."},
	} {
		fake.authorReply = string(mustJSON(t, map[string]any{"intent": chat.intent, "reply": chat.reply, "tests": nil, "case_changes": []any{}, "criteria": nil, "instruction_edits": []any{}, "remember": []any{}}))
		op := execute(vibe.Submission{Kind: "message", Content: chat.message, ArtifactID: &a.ID, ViewedRunID: &baseline.ID})
		if op.ModelCalls != 1 || op.Decision == nil || op.Decision.Intent != chat.intent || len(fake.inputs) != 3 || len(v.Document.Artifacts) != proposalCount || v.Document.Messages[len(v.Document.Messages)-1].ArtifactID != nil {
			t.Fatal("chat changed evaluation state or claimed proposal ownership")
		}
		var contextMap map[string]json.RawMessage
		_ = json.Unmarshal([]byte(fake.authorInputs[len(fake.authorInputs)-1]), &contextMap)
		var observed vibe.Artifact
		_ = json.Unmarshal(contextMap["viewed_agent"], &observed)
		if !bytes.Contains(contextMap["viewed_results"], []byte("45 days")) || bytes.Contains(contextMap["viewed_agent"], []byte(fix)) {
			t.Fatal("discussion lost the displayed results or mixed in newer instructions")
		}
	}
	if a.ProposalMessageID == nil || a.SourceMessageID == v.Document.Artifacts[0].SourceMessageID {
		t.Fatal("fix inherited old proposal ownership")
	}
	foreign, err := store.CreateSession(ctx, actor, nil, uuid.New(), cfg.DefaultModels())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Prepare(ctx, actor, foreign.ID, vibe.Submission{Kind: "message", Content: "Explain this", TestJourney: true, ClientID: uuid.New(), Revision: foreign.Revision, Models: cfg.DefaultModels(), ViewedRunID: &baseline.ID}); err == nil {
		t.Fatal("accepted a result from another conversation")
	}
	if _, err = svc.Prepare(ctx, actor, v.ID, vibe.Submission{Kind: "message", Content: "Explain this", TestJourney: true, ClientID: uuid.New(), Revision: v.Revision, Models: cfg.DefaultModels(), ViewedRunID: &baseline.ID, ViewedCaseKey: "invented"}); err == nil {
		t.Fatal("accepted an unknown case")
	}
	fake.fixed = true
	compared := execute(vibe.Submission{Kind: "retest", ArtifactID: &a.ID, BaselineID: &baseline.ID, ApproveArtifact: true})
	if compared.Scorecard.Passed != 3 || !reflect.DeepEqual(fake.inputs[3:], inputs) {
		t.Fatal("rerun did not use the same tests")
	}
	draftID, err := svc.Save(ctx, actor, v.ID, v.Revision, a.ID, ws, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	reload()
	var composition []byte
	var noBuild bool
	if err = db.QueryRow(ctx, `SELECT d.composition,a.build_id IS NULL FROM challenge_pack_drafts d JOIN vibe_saved_artifacts a ON a.draft_id=d.id WHERE d.id=$1`, draftID).Scan(&composition, &noBuild); err != nil {
		t.Fatal(err)
	}
	var saved challengepack.Composition
	if err = json.Unmarshal(composition, &saved); err != nil || !noBuild {
		t.Fatal("tests were not saved as a canonical independent pack", err)
	}
	compiled, err := challengepack.ComposeBundle(saved, challengepack.ResolvedPieces{})
	if err != nil {
		t.Fatal(err)
	}
	if len(compiled.InputSets) != 1 || len(compiled.InputSets[0].Cases) != 3 {
		t.Fatal("saved pack lost cases")
	}
	for i, c := range compiled.InputSets[0].Cases {
		if c.Payload["question"] != inputs[i] || vibe.ExpectedBehavior(c) == "" {
			t.Fatal("saved pack changed test input or lost expected behavior")
		}
	}
	kept, err := store.ListChecks(ctx, actor, ws)
	if err != nil || len(kept) != 1 || kept[0].DraftID == nil || *kept[0].DraftID != draftID || kept[0].ArtifactID != a.ID {
		t.Fatal("saved tests cannot be reopened", err, kept)
	}
	if _, err = store.GetSession(ctx, "anon:someone-else", v.ID); err == nil {
		t.Fatal("saved test session lost privacy")
	}
	reopened := execute(vibe.Submission{Kind: "retest", ArtifactID: &kept[0].ArtifactID, BaselineID: &compared.ID, ApproveArtifact: true})
	if reopened.Scorecard.Passed != 3 || !reflect.DeepEqual(fake.inputs[6:], inputs) {
		t.Fatal("reopened tests were regenerated or did not execute every input")
	}
	// Keeping the same version twice creates no duplicate pack.
	again, err := svc.Save(ctx, actor, v.ID, v.Revision, a.ID, ws, nil, true)
	if err != nil || again != draftID {
		t.Fatal("saving the same tests was not idempotent", err)
	}
	reload()
	// Changing a test is supported, but its score cannot be presented as an
	// improvement against the old suite. Check the actual edit/admission boundary.
	payload, _ = json.Marshal(map[string]any{
		"revision": v.Revision, "artifact_id": a.ID,
		"evaluation": map[string]any{
			"scenarios": []vibe.TestScenario{
				{Input: inputs[0], Expected: "Explain that the unopened item is eligible."},
				{Input: "Return an unopened item bought 60 days ago.", Expected: "Decline the late return."},
				{Input: inputs[2], Expected: "Ask for purchase age and item condition."},
			},
			"success_criteria": "Only unopened items within 30 days are eligible.",
		},
	})
	req = httptest.NewRequest(http.MethodPatch, "/sessions/"+v.ID.String(), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer fixture")
	req.Header.Set(headerUserID, user.String())
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	reload()
	changed := v.Document.Artifacts[len(v.Document.Artifacts)-1]
	_, err = svc.Prepare(ctx, actor, v.ID, vibe.Submission{Kind: "retest", TestJourney: true, ClientID: uuid.New(), Revision: v.Revision, Models: cfg.DefaultModels(), ArtifactID: &changed.ID, BaselineID: &reopened.ID, ApproveArtifact: true})
	if err == nil || !strings.Contains(err.Error(), "tests changed") {
		t.Fatal("changed tests were admitted as a fair comparison", err)
	}
	if len(fake.inputs) != 9 {
		t.Fatal("saving or rejecting changed tests invoked the target")
	}
	// Five-case imported suites stay in the core journey through editing and
	// saving, with original case keys and all retained cases executed.
	var imported map[string]any
	_ = json.Unmarshal(before, &imported)
	cases := imported["cases"].([]any)
	for i := 3; i < 5; i++ {
		var extra map[string]any
		_ = json.Unmarshal(mustJSON(t, cases[0]), &extra)
		extra["key"] = fmt.Sprintf("imported-%d", i)
		cases = append(cases, extra)
	}
	imported["cases"] = cases
	if err = svc.Import(ctx, actor, v.ID, v.Revision, mustJSON(t, map[string]any{"format": "agentclash-vibe-v1", "agent_prompt": fix, "evaluation": imported})); err != nil {
		t.Fatal(err)
	}
	reload()
	importedArtifact := v.Document.Artifacts[len(v.Document.Artifacts)-1]
	if !importedArtifact.IsTestSuite() || importedArtifact.AgentPrompt != fix {
		t.Fatal("import left the test journey or lost target")
	}
	payload = mustJSON(t, map[string]any{"revision": v.Revision, "artifact_id": importedArtifact.ID, "case_changes": []any{map[string]any{"action": "update", "case_key": "imported-3", "expected": "Explain eligibility without processing a refund."}}})
	req = httptest.NewRequest(http.MethodPatch, "/sessions/"+v.ID.String(), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer fixture")
	req.Header.Set(headerUserID, user.String())
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	reload()
	edited := v.Document.Artifacts[len(v.Document.Artifacts)-1]
	compiledEdit, err := svc.Compiler.Compile(edited.Blueprint, model, edited.ID, cfg.Limits(v.Anonymous))
	if err != nil || len(compiledEdit.Cases) != 5 {
		t.Fatal("editing dropped imported cases", err)
	}
	fiveRun := execute(vibe.Submission{Kind: "check", ArtifactID: &edited.ID, ApproveArtifact: true})
	if fiveRun.ModelCalls != 10 || fiveRun.Scorecard.Total != 5 || len(fake.inputs) != 14 {
		t.Fatal("import run did not execute all five cases", fiveRun)
	}
	if _, err = svc.Save(ctx, actor, v.ID, v.Revision, edited.ID, ws, nil, true); err != nil {
		t.Fatal(err)
	}
	reload()
	// An empty route is never accepted. A full handler payload is also not a
	// valid repaired route, so neither response can mutate or lock a decision.
	fake.authorReplies = []string{
		string(mustJSON(t, map[string]any{"intent": "chat", "reply": ""})),
		string(b),
	}
	sub := vibe.Submission{Kind: "message", Content: "thanks", TestJourney: true, ClientID: uuid.New(), Revision: v.Revision, Models: cfg.DefaultModels()}
	bad, err := svc.Prepare(ctx, actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	if err = runner.Execute(ctx, bad.ID); err == nil {
		t.Fatal("invalid repaired route was accepted")
	}
	invalidRoute, readErr := store.Operation(ctx, bad.ID)
	if readErr != nil || invalidRoute.Decision != nil {
		t.Fatal("invalid route committed a decision", readErr, invalidRoute.Decision)
	}
	if err = store.Finish(ctx, bad.ID, &vibe.Fault{Code: "invalid_response", Message: "Fixture route is invalid"}); err != nil {
		t.Fatal(err)
	}
	reload()
	if len(fake.inputs) != 14 || v.Document.Artifacts[len(v.Document.Artifacts)-1].ID != edited.ID {
		t.Fatal("failed repair changed tests")
	}
}
