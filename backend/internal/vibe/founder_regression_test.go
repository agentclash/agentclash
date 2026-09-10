package vibe

import (
	"context"
	"encoding/json"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestVibeIntegrationOverflowAndPlanExecution(t *testing.T) {
	store := integrationStore(t)
	session := anonSession(t, store)
	ctx := context.Background()
	cfg := testConfig()
	gate := testGate(t)
	svc := &Service{Store: store, Config: cfg, Gate: gate, Compiler: repairCompiler{}}
	accepted := Artifact{ID: uuid.New(), Title: "Accepted", AgentPrompt: strings.Repeat("exact accepted instructions ", 1000), Blueprint: json.RawMessage(`{"fixed":"coverage"}`), Accepted: true}
	plan := Artifact{ID: uuid.New(), Kind: "test_plan", Title: "Not executable", TestPlan: &TestPlan{Objective: "Plan"}, Accepted: true}
	if err := store.Edit(ctx, session.Actor, session.ID, session.Revision, func(v *Session) error {
		v.Document.Artifacts = []Artifact{accepted, plan}
		v.Document.ActiveArtifactID = &accepted.ID
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	session, err := store.GetSession(ctx, session.Actor, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"playground", "check", "retest"} {
		_, err = svc.Prepare(ctx, session.Actor, session.ID, Submission{ClientID: uuid.New(), Revision: session.Revision, Kind: kind, ArtifactID: &plan.ID, Content: "test", Models: cfg.DefaultModels()})
		if issue := issueFrom(err); issue == nil || issue.Code != "artifact_required" {
			t.Fatal("test plan admitted as executable", kind, err)
		}
	}
	runner := Runner{Service: svc, Gateway: &Gateway{Store: store, Config: cfg, Gate: gate, Client: callFunc(func(context.Context, provider.Request) (provider.Response, error) {
		t.Fatal("oversized context dispatched")
		return provider.Response{}, nil
	})}}
	op, err := svc.Prepare(ctx, session.Actor, session.ID, Submission{ClientID: uuid.New(), Revision: session.Revision, Kind: "message", Content: "Keep my accepted work", Models: cfg.DefaultModels()})
	if err != nil {
		t.Fatal(err)
	}
	err = runner.Execute(ctx, op.ID)
	issue := issueFrom(err)
	if issue == nil || issue.Code != "context_limit" || issue.Context == nil {
		t.Fatal("missing persistent limit diagnostic", err)
	}
	if err = store.Finish(ctx, op.ID, issue); err != nil {
		t.Fatal(err)
	}
	var attempts int
	if err = store.DB.QueryRow(ctx, "SELECT count(*) FROM vibe_attempts WHERE operation_id=$1", op.ID).Scan(&attempts); err != nil || attempts != 0 {
		t.Fatal("overflow created a provider attempt", err)
	}
	saved, err := store.GetSession(ctx, session.Actor, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Document.Artifacts[0].AgentPrompt != accepted.AgentPrompt || string(saved.Document.Artifacts[0].Blueprint) != string(accepted.Blueprint) { // JSONB whitespace is immaterial.
		var got, want any
		_ = json.Unmarshal(saved.Document.Artifacts[0].Blueprint, &got)
		_ = json.Unmarshal(accepted.Blueprint, &want)
		if saved.Document.Artifacts[0].AgentPrompt != accepted.AgentPrompt || string(raw(got)) != string(raw(want)) {
			t.Fatal("accepted work changed")
		}
	}
	if saved.Operations[len(saved.Operations)-1].Error.Context == nil {
		t.Fatal("context diagnostic did not survive persistence")
	}
}

// Sanitized recorded inputs, not scripted good model replies. No network or
// provider keys are used; a separate capped live check measures authoring.
func TestVibeFounderContext(t *testing.T) {
	content, err := os.ReadFile("testdata/founder_context.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name string
		Plan Plan
	}
	if err = json.Unmarshal(content, &fixtures); err != nil {
		t.Fatal(err)
	}
	profile := ModelProfile{Free: true, FramingAllowance: 4096, Context: 512000, StructuredOutputs: true}
	l := LimitsFor(true)
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			f.Plan.AuthoringVersion = 3
			original := string(raw(f.Plan))
			messages := authoringMessages(f.Plan, repairCompiler{}, profile)
			req := provider.Request{Messages: messages, ResponseFormat: authoringFormatVersion(profile, 3), MaxOutputTokens: l.OutputTokens}
			count, err := CountContext(req, profile, l)
			if err != nil {
				t.Fatalf("initial bound %d: %v", count.UpperBound, err)
			}
			t.Logf("initial bound %d", count.UpperBound)
			if _, err = authoringRepairMessages(messages, strings.Repeat("bad", 10000), "Correct the unsupported action promise; preserve all requested coverage.", profile, l, 3); err != nil {
				t.Fatal("no repair headroom", err)
			}
			if _, err = authoringRepairMessages(messages, strings.Repeat("bad", 10000), strings.Repeat("invalid draft diagnostic ", 100), profile, l, 3); err != nil {
				t.Fatal("long diagnostic prevented the bounded repair", err)
			}
			var projected map[string]json.RawMessage
			if json.Unmarshal([]byte(messages[1].Content), &projected) != nil {
				t.Fatal("lost data boundary")
			}
			var doc struct {
				Requirements []Requirement
				Latest       *Artifact `json:"latest_proposal"`
			}
			if json.Unmarshal(projected["conversation_data"], &doc) != nil {
				t.Fatal("invalid compact document")
			}
			if len(doc.Requirements) != len(f.Plan.Document.Requirements) {
				t.Fatal("active requirements were removed to fit")
			}
			for i, q := range doc.Requirements {
				old := f.Plan.Document.Requirements[i]
				if q.Statement != old.Statement || q.Status != old.Status || q.ID != old.ID {
					t.Fatal("requirement meaning or identity changed")
				}
			}
			// Reconstruct the only allowed deduplication and compare every field,
			// including validators, judges, cases and arbitrary source evidence.
			compareArtifact := func(projected json.RawMessage, want Artifact) {
				var got struct {
					AgentPrompt     string         `json:"agent_prompt"`
					Blueprint       map[string]any `json:"blueprint"`
					InstructionsRef string         `json:"blueprint_instructions_ref"`
				}
				if err := json.Unmarshal(projected, &got); err != nil {
					t.Fatal(err)
				}
				if got.InstructionsRef == "agent_prompt" {
					got.Blueprint["instructions"] = got.AgentPrompt
				}
				var expected map[string]any
				if err := json.Unmarshal(want.Blueprint, &expected); err != nil {
					t.Fatal(err)
				}
				if got.AgentPrompt != want.AgentPrompt || !reflect.DeepEqual(got.Blueprint, expected) {
					t.Fatal("artifact coverage or instructions changed")
				}
			}
			if f.Plan.Artifact != nil {
				compareArtifact(projected["accepted_agent"], *f.Plan.Artifact)
			}
			if n := len(f.Plan.Document.Artifacts); n > 0 {
				latest := f.Plan.Document.Artifacts[n-1]
				if f.Plan.Artifact == nil || latest.ID != f.Plan.Artifact.ID {
					var data map[string]json.RawMessage
					_ = json.Unmarshal(projected["conversation_data"], &data)
					compareArtifact(data["latest_proposal"], latest)
				}
			}
			if string(raw(f.Plan)) != original {
				t.Fatal("context building mutated persisted state")
			}
		})
	}
}

func TestVibeSingleAuthoringArtifact(t *testing.T) {
	for _, response := range []string{
		`{"artifact":{"kind":"agent_draft","title":"Preview","agent_prompt":"Use supplied facts","examples":["No facts supplied"],"success_criteria":"State uncertainty"}}`,
		`{"artifact":{"kind":"test_plan","title":"Plan","objective":"Test our invocation","scenarios":[{"input":"Timeout","expected":"No success claim"}],"evidence_needed":["Observed output"],"next_steps":["Configure invocation timeout"],"local_test_code":""}}`,
	} {
		var a assistantReply
		if err := json.Unmarshal([]byte(response), &a); err != nil {
			t.Fatal(err)
		}
		if err := a.unpackArtifact(2); err != nil {
			t.Fatal(err)
		}
		if (a.Draft == nil) == (a.TestPlan == nil) {
			t.Fatal("artifact union did not choose exactly one representation")
		}
	}
	a := assistantReply{Artifact: &AuthoringArtifact{Kind: "test_plan", AgentPrompt: "Execute this"}}
	if a.unpackArtifact(3) == nil {
		t.Fatal("test plan included executable instructions")
	}
	var schema struct{ Properties map[string]any }
	if err := json.Unmarshal(authoringV3Schema, &schema); err != nil {
		t.Fatal(err)
	}
	if schema.Properties["artifact"] == nil || schema.Properties["draft"] != nil || schema.Properties["test_plan"] != nil {
		t.Fatal("ambiguous authoring shape")
	}
	for _, response := range []string{
		`{"draft":{"title":"Legacy draft"}}`,
		`{"artifact":{"kind":"agent_draft","positive_example":"Supported facts only","negative_example":"Unsupported action request"}}`,
		`{"artifact":{"kind":"agent_draft","examples":["A single happy path"]}}`,
	} {
		var reply assistantReply
		if err := json.Unmarshal([]byte(response), &reply); err != nil {
			t.Fatal(err)
		}
		if err := reply.unpackArtifact(3); err == nil {
			t.Fatal("current authoring contract accepted missing coverage", response)
		}
	}
}

func TestVibeIntegrationSelectedExistingJourneyPrecedesAuthoring(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	cfg := freeConfig()
	svc := &Service{Store: s, Config: cfg, Gate: testGate(t), Compiler: repairCompiler{}}
	o, err := svc.Prepare(context.Background(), v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: "Test my existing agent", JourneyMode: "existing", Models: cfg.DefaultModels()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = s.Finish(context.Background(), o.ID, &Fault{Code: "fixture_complete", Message: "No provider used"})
	}()
	saved, err := s.GetSession(context.Background(), v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Document.Journey.Mode != "existing" || saved.Document.Journey.PreviewConsent {
		t.Fatal("journey choice not saved before authoring, or consent invented")
	}
	var plan Plan
	if err := json.Unmarshal(o.Input, &plan); err != nil {
		t.Fatal(err)
	}
	format := string(authoringSchemaForPlan(plan))
	if strings.Contains(format, "agent_draft") || !strings.Contains(format, "test_plan") {
		t.Fatal("existing-agent schema permits an unsolicited surrogate")
	}
	plan.Document.Journey.PreviewConsent = true
	if !strings.Contains(string(authoringSchemaForPlan(plan)), "agent_draft") {
		t.Fatal("explicit preview choice did not allow a draft")
	}
}

func TestVibePendingRequirementRevisionAndManualOverride(t *testing.T) {
	for _, manual := range []bool{false, true} {
		id := uuid.New()
		v := Session{Actor: "user:reviewer", Document: Document{Requirements: []Requirement{{ID: id, Statement: "Original confirmed rule", Status: "accepted"}}}}
		if err := ReconcileRequirements(&v.Document, []RequirementChange{{Action: "replace", RequirementID: id.String(), Statement: "First proposal"}}, uuid.New(), uuid.New()); err != nil {
			t.Fatal(err)
		}
		pending := v.Document.Requirements[1].ID
		if manual {
			text := "User's direct replacement"
			if err := DecideRequirement(&v, id, "superseded", &text); err != nil {
				t.Fatal(err)
			}
			if err := DecideRequirement(&v, pending, "accepted", nil); err == nil {
				t.Fatal("obsolete pending proposal revived")
			}
		} else {
			if err := ReconcileRequirements(&v.Document, []RequirementChange{{Action: "replace", RequirementID: pending.String(), Statement: "Revised proposal"}}, uuid.New(), uuid.New()); err != nil {
				t.Fatal(err)
			}
			if v.Document.Requirements[0].Status != "accepted" {
				t.Fatal("pending revision silently changed confirmation")
			}
			if err := DecideRequirement(&v, v.Document.Requirements[2].ID, "accepted", nil); err != nil {
				t.Fatal(err)
			}
		}
		accepted := 0
		for _, q := range v.Document.Requirements {
			if q.Status == "accepted" {
				accepted++
			}
		}
		if accepted != 1 {
			t.Fatal("conflicting confirmed requirements", v.Document.Requirements)
		}
	}
}

func TestVibeIntegrationConcurrentRequirementDecisions(t *testing.T) {
	store := integrationStore(t)
	v := anonSession(t, store)
	ctx := context.Background()
	id := uuid.New()
	if err := store.Edit(ctx, v.Actor, v.ID, v.Revision, func(s *Session) error {
		s.Document.Requirements = []Requirement{{ID: id, Statement: "Review me", Status: "proposed"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	v, err := store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for _, status := range []string{"accepted", "rejected"} {
		go func(status string) {
			results <- store.Edit(ctx, v.Actor, v.ID, v.Revision, func(s *Session) error { return DecideRequirement(s, id, status, nil) })
		}(status)
	}
	success := 0
	for range 2 {
		if err := <-results; err == nil {
			success++
		} else if f := issueFrom(err); f.Code != "revision_conflict" {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal("concurrent decisions were both committed")
	}
}

func TestVibeRequirementReconciliation(t *testing.T) {
	id, source, reply := uuid.New(), uuid.New(), uuid.New()
	v := Session{Actor: "user:fixture", Document: Document{Requirements: []Requirement{{ID: id, Statement: "Use verified facts", Status: "accepted"}}}}
	if err := ReconcileRequirements(&v.Document, []RequirementChange{{Action: "add", Statement: " Use  verified facts "}}, source, reply); err != nil || len(v.Document.Requirements) != 1 {
		t.Fatal("duplicate added", err)
	}
	change := RequirementChange{Action: "replace", RequirementID: id.String(), Statement: "Use supplied evidence and state uncertainty"}
	if err := ReconcileRequirements(&v.Document, []RequirementChange{change, change}, source, reply); err != nil {
		t.Fatal(err)
	}
	if len(v.Document.Requirements) != 2 || v.Document.Requirements[0].Status != "accepted" {
		t.Fatal("proposal changed confirmation or duplicated replacement")
	}
	replacement := v.Document.Requirements[1]
	if replacement.SourceMessageID != source || replacement.AcceptedBy != "" || replacement.SupersedesID == nil {
		t.Fatal("lost proposal provenance")
	}
	if err := DecideRequirement(&v, replacement.ID, "accepted", nil); err != nil {
		t.Fatal(err)
	}
	if v.Document.Requirements[0].Status != "superseded" || v.Document.Requirements[1].AcceptedBy != v.Actor {
		t.Fatal("explicit confirmation was not applied")
	}
	if err := ReconcileRequirements(&v.Document, []RequirementChange{{Action: "remove", RequirementID: replacement.ID.String()}}, source, reply); err != nil {
		t.Fatal(err)
	}
	removal := v.Document.Requirements[2]
	if v.Document.Requirements[1].Status != "accepted" {
		t.Fatal("removal bypassed confirmation")
	}
	if err := DecideRequirement(&v, removal.ID, "rejected", nil); err != nil || v.Document.Requirements[1].Status != "accepted" {
		t.Fatal("dismissal removed confirmed rule", err)
	}
}

func TestVibeReplacementCannotDuplicateAnotherRule(t *testing.T) {
	id := uuid.New()
	d := Document{Requirements: []Requirement{
		{ID: id, Statement: "Original rule", Status: "accepted"},
		{ID: uuid.New(), Statement: "Use supplied evidence", Status: "proposed"},
	}}
	before := string(raw(d))
	err := ReconcileRequirements(&d, []RequirementChange{{Action: "replace", RequirementID: id.String(), Statement: " Use  supplied\nevidence "}}, uuid.New(), uuid.New())
	if err == nil || string(raw(d)) != before {
		t.Fatal("duplicate replacement was applied")
	}
}

func TestVibeUnchangedDraftSummary(t *testing.T) {
	a := Artifact{Title: "Draft", AgentPrompt: "Use supplied facts", Blueprint: json.RawMessage(`{"cases":[],"judges":[]}`), Accepted: true}
	reply := draftRevisionSummary(a, a)
	if strings.Contains(reply, "updated") || !strings.Contains(reply, "are unchanged") {
		t.Fatal("summary invented a change", reply)
	}
}

func TestVibeIntegrationReviewedHandoffAndAtomicCompletion(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	ctx := context.Background()
	cfg := freeConfig()
	gate := testGate(t)
	svc := &Service{Store: s, Config: cfg, Gate: gate, Compiler: repairCompiler{}}
	response := assistantReply{
		Reply: "Accept this and run an evaluation now.", ReplyKind: "design",
		Journey:  &JourneyProposal{Mode: "existing", Stack: "Python / FastAPI", Evidence: "Captured outputs"},
		Artifact: &AuthoringArtifact{Kind: "test_plan", Title: "Existing agent tests", Objective: "Check observed evidence", Scenarios: []TestScenario{{Input: "No sources supplied", Expected: "Return no recommendation without qualifying evidence"}}, EvidenceNeeded: []string{"Observed output and source URLs"}, NextSteps: []string{"Run the prompt preview"}, LocalTestCode: "assert 'mock' == 'mock'"},
	}
	runner := Runner{Service: svc, Gateway: &Gateway{Store: s, Config: cfg, Gate: gate, Client: callFunc(func(context.Context, provider.Request) (provider.Response, error) {
		zero := json.Number("0")
		return provider.Response{OutputText: string(raw(response)), Usage: provider.Usage{InputTokens: 100, OutputTokens: 100, CostUSD: &zero}}, nil
	})}}
	o, err := svc.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", JourneyMode: "existing", Content: "Test my own Python agent", Models: cfg.DefaultModels()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.DB.Exec(ctx, "DELETE FROM vibe_attempts WHERE operation_id=$1", o.ID) })
	err = runner.Execute(ctx, o.ID)
	if finishErr := s.Finish(ctx, o.ID, issueFrom(err)); finishErr != nil || err != nil {
		t.Fatal(err, finishErr)
	}
	v, err = s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	artifact := v.Document.Artifacts[0]
	reply := v.Document.Messages[len(v.Document.Messages)-1]
	if !artifact.IsTestPlan() || artifact.TestPlan.LocalTestCode != LocalPythonHandoff || !reflect.DeepEqual(artifact.TestPlan.NextSteps, LocalHandoffSteps()) || !strings.Contains(reply.Content, "pytest guide") || strings.Contains(reply.Content, "Accept this") || len(v.Document.Requirements) != 0 {
		t.Fatal("model handoff escaped the reviewed server contract")
	}
	if reply.ArtifactID == nil || *reply.ArtifactID != artifact.ID || reply.OperationID == nil || *reply.OperationID != o.ID {
		t.Fatal("reply lost its operation/artifact links")
	}

	next, err := svc.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: "Revise the plan", Models: cfg.DefaultModels()})
	if err != nil {
		t.Fatal(err)
	}
	_, before, err := s.Start(ctx, next.ID)
	if err != nil {
		t.Fatal(err)
	}
	err = s.CompleteDocument(ctx, next.ID, "Uncommitted reply", &Artifact{ID: uuid.New(), Kind: "test_plan", TestPlan: artifact.TestPlan}, nil, AuthoringCompletion{Journey: &JourneyProposal{Mode: "idea", Stack: "Uncommitted stack"}, Changes: []RequirementChange{{Action: "replace", RequirementID: uuid.New().String(), Statement: "Missing predecessor"}}})
	if err == nil {
		t.Fatal("invalid linked requirement change committed")
	}
	if finishErr := s.Finish(ctx, next.ID, issueFrom(err)); finishErr != nil {
		t.Fatal(finishErr)
	}
	after, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil || string(raw(before.Document)) != string(raw(after.Document)) {
		t.Fatal("failed completion partially changed reply, artifact or journey", err)
	}
}

func TestVibeExistingAgentAndSupportBoundaries(t *testing.T) {
	p := Plan{Document: Document{Journey: Journey{Mode: "existing"}}}
	a := assistantReply{Reply: "Review this", ReplyKind: "design", Journey: &JourneyProposal{Mode: "existing"}, Draft: &DraftProposal{Title: "Replacement", AgentPrompt: "Write text"}}
	if err := a.validateJourney(p, LimitsFor(true)); err == nil {
		t.Fatal("replacement prompt escaped existing-agent fork")
	}
	p.Document.Journey.PreviewConsent = true
	if err := a.validateJourney(p, LimitsFor(true)); err != nil {
		t.Fatal(err)
	}
	a.Draft = nil
	a.ReplyKind = "support"
	a.Changes = []RequirementChange{{Action: "add", Statement: "Provide API documentation"}}
	if a.validateJourney(p, LimitsFor(true)) == nil {
		t.Fatal("support request became agent requirement")
	}
	a.Changes = nil
	a.Journey = &JourneyProposal{Mode: "existing", Stack: "Python / FastAPI / LangGraph", Evidence: "Example runs only"}
	a.TestPlan = &TestPlan{Title: "Existing research agent", Objective: "Review evidence quality", Scenarios: []TestScenario{{Input: "No relevant sources", Expected: "Return no qualifying recommendation without evidence"}}, EvidenceNeeded: []string{"Captured output and sources"}, NextSteps: []string{"Run pytest locally after configuring the invocation"}}
	if err := a.validateJourney(p, LimitsFor(true)); err != nil {
		t.Fatal(err)
	}
}

func TestVibePreviewPromisesAndCriteria(t *testing.T) {
	for _, text := range []string{"Confirm the booking back to them.", "Say you will confirm it later.", "Escalate to a human immediately."} {
		if validatePreviewText(text) == nil {
			t.Fatalf("accepted action promise %q", text)
		}
	}
	for _, text := range []string{"Do not confirm the booking.", "Explain that booking is unavailable.", "Describe a hypothetical booking as simulated."} {
		if err := validatePreviewText(text); err != nil {
			t.Fatal(text, err)
		}
	}
	for _, criteria := range []string{"At least two signals must be rated High confidence.", "Output at least 3 companies with buying signals.", "Maintain natural pacing without dead air."} {
		if validatePreviewProposal(assistantReply{Draft: &DraftProposal{SuccessCriteria: criteria}}) == nil {
			t.Fatal("unsupported criterion accepted")
		}
	}
}

func TestVibeContextDiagnostics(t *testing.T) {
	req := provider.Request{Messages: []provider.Message{{Role: "user", Content: strings.Repeat("x", 20000)}}, MaxOutputTokens: 2048}
	_, err := CountContext(req, ModelProfile{Free: true, Context: 512000, FramingAllowance: 4096}, LimitsFor(true))
	f := issueFrom(err)
	if f == nil || f.Code != "context_limit" || f.Context == nil || f.Context.UpperBound <= f.Context.Limit || f.Context.LargestSection != "user_messages" {
		t.Fatalf("missing diagnostic: %+v", f)
	}
}
