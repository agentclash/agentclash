package vibe

import (
	"context"
	"encoding/json"
	"github.com/agentclash/agentclash/backend/internal/email"
	"github.com/agentclash/agentclash/backend/internal/enquiries"
	"github.com/agentclash/agentclash/backend/internal/vibe/inputs"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"strings"
	"testing"
	"time"
)

func materialService(t *testing.T) (*Service, Session) {
	s, v := buildService(t)
	s.Config.MaterialBuild = true
	s.Store.Inputs = &inputs.Repository{DB: s.Store.DB}
	t.Cleanup(func() {
		s.Store.DB.Exec(context.Background(), `DELETE FROM vibe_enquiries WHERE session_id=$1`, v.ID)
		s.Store.DB.Exec(context.Background(), `DELETE FROM vibe_inputs WHERE session_id=$1`, v.ID)
	})
	return s, v
}
func addText(t *testing.T, s *Service, v Session, text string) inputs.Binding {
	t.Helper()
	r, err := s.Store.Inputs.Create(context.Background(), v.ID, v.Actor, uuid.New(), "text", "Notes", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return inputs.Binding{ID: r.ID, Hash: r.Hash, Usage: "task_input"}
}
func TestIntegrationMaterialBuildRunsActualInputBeforeChecksOnce(t *testing.T) {
	s, v := materialService(t)
	ctx := context.Background()
	binding := addText(t, s, v, "At the planning meeting, Mira agreed to ship the prototype on Friday.")
	reference := addText(t, s, v, "The project codename is KESTREL.")
	reference.Usage = "reference"
	job := "Summarize meeting notes, including decisions and action owners."
	quote, err := s.QuoteBuild(ctx, v.Actor, v.ID, BuildQuoteRequest{Content: job, Models: DefaultModels(), Inputs: []inputs.Binding{binding, reference}})
	if err != nil {
		t.Fatal(err)
	}
	sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: job, Models: DefaultModels(), TestJourney: true, CycleID: &quote.ID, Inputs: []inputs.Binding{binding, reference}}
	o, err := s.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	v = executeBuildFixture(t, s, o, func(req provider.Request) any {
		calls++
		all := string(raw(req.Messages))
		if calls > 1 && (!strings.Contains(all, "KESTREL") || strings.Contains(all, "Mira")) {
			t.Fatal("author/reviewer must see factual references, not initial-trial data")
		}

		switch calls {
		case 1:
			return interpretationFixture(prepareAction{Kind: "prepare_tests", Count: 3}, factObservation{Kind: "job", Quote: job})
		case 2:
			var in taskInput
			json.Unmarshal([]byte(req.Messages[1].Content), &in)
			id := in.CurrentRequest.ID
			return createSuiteCommand{Rules: []PolicyRule{{ID: "summary", Statement: job, SourceBlockIDs: []string{id}, Evidence: []RuleEvidence{{SourceBlockID: id, Quote: job, Kind: "requirement"}}}}, Tests: testSuiteProposal{Title: "Meeting notes", Summary: job, SuccessCriteria: job, Scenarios: []TestScenario{{Input: "Ana owns the report, due Tuesday.", Expected: "Summarize that Ana owns the report due Tuesday."}, {Input: "A deadline was discussed but not decided.", Expected: "Do not invent a deadline."}, {Input: "The team decided to pause. No owner was chosen.", Expected: "Summarize the pause without inventing an owner."}}}}
		case 3:
			var in SuiteReviewInput
			json.Unmarshal([]byte(req.Messages[1].Content), &in)
			return supportedSuiteReview(in)
		default:
			t.Fatal("unexpected authoring call")
			return nil
		}
	})
	if v.Document.Build.Phase != "trying" || v.Document.Build.ClarificationsUsed != 0 {
		t.Fatalf("wrong first continuation: %+v", v.Document.Build)
	}
	var trial Operation
	for _, operation := range v.Operations {
		if operation.Kind == "check" {
			t.Fatal("checks started before actual output")
		}
		if operation.Kind == "playground" {
			trial = operation
		}
	}
	if trial.ID == uuid.Nil {
		t.Fatal("missing actual trial")
	}
	v = executeBuildFixture(t, s, trial, func(req provider.Request) any {
		all := string(raw(req.Messages))
		if !strings.Contains(all, "Mira") || !strings.Contains(all, "KESTREL") || strings.Contains(all, "Ana owns") {
			t.Fatal("missing material or leaked generated answer key")
		}
		return "Mira will ship the prototype on Friday."
	})
	if v.Document.Build.Phase != "checking" || v.Document.Build.TrialID == nil || *v.Document.Build.TrialID != trial.ID {
		t.Fatalf("trial lost: %+v", v.Document.Build)
	}
	ResumeBuilds(ctx, s)
	ResumeBuilds(ctx, s)
	current, err := s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	trials, checks := 0, 0
	for _, op := range current.Operations {
		if op.Kind == "playground" {
			trials++
		}
		if op.Kind == "check" {
			checks++
			stored, err := s.Store.Operation(ctx, op.ID)
			if err != nil {
				t.Fatal(err)
			}
			var frozen Plan
			if err := json.Unmarshal(stored.Input, &frozen); err != nil {
				t.Fatal(err)
			}
			refs := referenceInputs(frozen)
			if len(refs) != 1 || refs[0].ID != reference.ID {
				t.Fatal("check reference version lost")
			}
			messages, err := materialMessages(ctx, s.Store.DB, v.ID, nil, refs)
			if err != nil || strings.Contains(string(raw(messages)), "Mira") || !strings.Contains(string(raw(messages)), "KESTREL") {
				t.Fatal("check inherited trial data or lost reference", err)
			}
			if len(op.Results) != 3 {
				t.Fatal("required checks disappeared")
			}
		}
	}
	if trials != 1 || checks != 1 {
		t.Fatalf("duplicate continuation: %d %d", trials, checks)
	}
	if current.Document.Artifacts[0].InputContract == nil {
		t.Fatal("missing truthful input contract")
	}
}

func TestIntegrationDocumentPolicySourceCannotOutliveItsGrantOrInput(t *testing.T) {
	s, v := materialService(t)
	ctx := context.Background()
	b := addText(t, s, v, "Refunds require approval. This other paragraph is not policy.")
	source := DocumentSource{InputID: b.ID, Hash: b.Hash, Page: 1, Quote: "Refunds require approval."}
	job := "Build a refund decision assistant using the selected policy."
	q, err := s.QuoteBuild(ctx, v.Actor, v.ID, BuildQuoteRequest{Content: job, Models: DefaultModels(), AdoptRules: []DocumentSource{source}})
	if err != nil {
		t.Fatal(err)
	}
	sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: job, Models: DefaultModels(), TestJourney: true, CycleID: &q.ID, AdoptRules: []DocumentSource{source}}
	op, err := s.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	var p Plan
	if err = json.Unmarshal(op.Input, &p); err != nil {
		t.Fatal(err)
	}
	if p.Conversation.SourceVersion != DocumentSourceVersion {
		t.Fatal("document policy wasn't versioned")
	}
	found := false
	for _, block := range p.Conversation.Sources {
		if block.Document != nil {
			found = true
			if block.Text != source.Quote || block.Document.AdoptionMessageID != sub.ClientID {
				t.Fatal("wrong quote or adoption grant")
			}
		}
	}
	if !found {
		t.Fatal("explicit document policy missing")
	}
	if err = s.Store.Inputs.Delete(ctx, v.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if err = validateBoundSources(ctx, s.Store.DB, v.ID, p); err == nil {
		t.Fatal("deleted policy source still authorized execution")
	}
}

func TestIntegrationMaterialOwnershipExpiryIdempotencyAndDeletion(t *testing.T) {
	s, v := materialService(t)
	ctx := context.Background()
	id := uuid.New()
	a, err := s.Store.Inputs.Create(ctx, v.ID, v.Actor, id, "text", "Data", []byte("Invoice 42"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Store.Inputs.Create(ctx, v.ID, v.Actor, id, "text", "Data", []byte("Invoice 42"))
	if err != nil || a.ID != b.ID {
		t.Fatal("duplicate upload", err)
	}
	if a.ExpiresAt == nil || time.Until(*a.ExpiresAt) < 6*24*time.Hour {
		t.Fatal("guest retention missing")
	}
	if _, err = s.Store.Inputs.Create(ctx, v.ID, v.Actor, id, "text", "Data", []byte("Changed")); err != inputs.ErrConflict {
		t.Fatal("idempotency collision accepted", err)
	}
	binding := inputs.Binding{ID: a.ID, Hash: a.Hash, Usage: "task_input"}
	if _, err = inputs.Resolve(ctx, s.Store.DB, uuid.New(), binding); err == nil {
		t.Fatal("cross-project material accepted")
	}
	if err = s.Store.Inputs.Delete(ctx, v.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = inputs.Resolve(ctx, s.Store.DB, v.ID, binding); err == nil {
		t.Fatal("deleted material reusable")
	}
	other := addText(t, s, v, "Old notes")
	s.Store.DB.Exec(ctx, `UPDATE vibe_inputs SET expires_at=now()-interval '1 second' WHERE id=$1`, other.ID)
	if _, err = inputs.Resolve(ctx, s.Store.DB, v.ID, other); err == nil {
		t.Fatal("expired source reusable before sweeper")
	}
	if err = s.Store.Inputs.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	r, err := s.Store.Inputs.Get(ctx, v.ID, other.ID)
	if err != nil || r.Status != "expired" || len(r.Pages) != 0 {
		t.Fatal("expiry retained raw text", r, err)
	}
}

type localEnquirySender struct{}

func (localEnquirySender) SendMessage(context.Context, email.Message) (string, error) {
	return "test-provider", nil
}
func TestIntegrationEnquiryAndProjectDeletion(t *testing.T) {
	s, v := materialService(t)
	ctx := context.Background()
	addText(t, s, v, "Private project text")
	delivery := &enquiries.Store{DB: s.Store.DB}
	request := enquiries.Request{ClientID: uuid.New(), Source: enquiries.Source{Revision: v.Revision}, Email: "person@example.test", Summary: "Discuss building a notes assistant."}
	if _, err := delivery.Create(ctx, v.ID, v.Actor, request); err != enquiries.ErrDisabled {
		t.Fatal("unconfigured email accepted", err)
	}
	delivery.Recipient = "team@example.test"
	delivery.Sender = localEnquirySender{}
	first, err := delivery.Create(ctx, v.ID, v.Actor, request)
	if err != nil {
		t.Fatal(err)
	}
	delivery.Sender = nil
	again, err := delivery.Create(ctx, v.ID, v.Actor, request)
	if err != nil || again.ID != first.ID || again.Status != "received" {
		t.Fatal("lost acknowledgement wasn't recoverable", err)
	}
	request.Summary = "different"
	if _, err = delivery.Create(ctx, v.ID, v.Actor, request); err != enquiries.ErrConflict {
		t.Fatal("changed enquiry reused receipt", err)
	}
	sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "playground", Models: DefaultModels()}
	op, err := s.Store.Submit(ctx, v.Actor, v.ID, sub, Plan{Submission: sub, Anonymous: true, Calls: 1, MaxCost: 1}, s.Config)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Store.Start(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	attempt := Attempt{ID: uuid.New(), OperationID: op.ID, Step: "target", Role: Target, Model: op.Models.Target, Policy: raw(map[string]any{}), RequestHash: "test", InputBound: 1, MaxOutput: 1, MaxCost: 1}
	if err = s.Store.BeginAttempt(ctx, attempt); err != nil {
		t.Fatal(err)
	}
	cost := int64(1)
	if err = s.Store.EndAttempt(ctx, attempt, "private reply", raw(provider.Response{OutputText: "private reply", RawResponse: raw(map[string]string{"text": "private raw reply"})}), &cost, nil); err != nil {
		t.Fatal(err)
	}
	if err = s.Store.Finish(ctx, op.ID, nil); err != nil {
		t.Fatal(err)
	}
	v, err = s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := s.Store.DeleteProject(ctx, v.Actor, v.ID, v.Revision)
	if err != nil || receipt.Status != "deleting" {
		t.Fatal(receipt, err)
	}
	if _, err = s.Store.GetSession(ctx, v.Actor, v.ID); err == nil {
		t.Fatal("deleted project still readable")
	}
	if _, err = s.Store.Inputs.Create(ctx, v.ID, v.Actor, uuid.New(), "text", "late", []byte("late write")); err == nil {
		t.Fatal("late input resurrected project")
	}
	if err = s.Store.CleanupProjects(ctx); err != nil {
		t.Fatal(err)
	}
	receipt, err = s.Store.DeletionStatus(ctx, v.Actor, v.ID)
	if err != nil || receipt.Status != "deleted" {
		t.Fatal(receipt, err)
	}
	var content string
	s.Store.DB.QueryRow(ctx, `SELECT content::text FROM vibe_enquiries WHERE id=$1`, first.ID).Scan(&content)
	if content != "{}" {
		t.Fatal("enquiry content survived deletion")
	}
	var output, usage string
	if err = s.Store.DB.QueryRow(ctx, `SELECT output,usage::text,actual_cost FROM vibe_attempts WHERE id=$1`, attempt.ID).Scan(&output, &usage, &cost); err != nil {
		t.Fatal(err)
	}
	if output != "" || usage != "{}" || cost != 1 {
		t.Fatal("deletion retained raw response or lost accounting")
	}
}

func TestDocumentRulesRequireExplicitAdoptionAndOriginalQuote(t *testing.T) {
	id := uuid.New()
	source := DocumentSource{InputID: uuid.New(), Hash: "original-file-hash", Page: 1, Quote: "Refunds require approval.", AdoptionMessageID: id}
	block := SourceBlock{ID: source.sourceID(), MessageID: id, Document: &source, Text: source.Quote, Hash: Hash([]byte(source.Quote)), OriginalHash: source.Hash}
	rule := PolicyRule{ID: "refund", Statement: source.Quote, SourceBlockIDs: []string{block.ID}, Evidence: []RuleEvidence{{SourceBlockID: block.ID, Quote: source.Quote, Kind: "requirement"}}}
	policy := PolicySnapshot{SourceVersion: DocumentSourceVersion, Rules: []PolicyRule{rule}, Sources: projectedRuleSources([]PolicyRule{rule}, []SourceBlock{block})}
	doc := Document{Messages: []Message{{ID: id, Role: "user", Content: "Use this paragraph as a rule", AdoptedSources: []DocumentSource{source}}}}
	if _, err := verifiedPolicySources(doc, policy); err != nil {
		t.Fatal(err)
	}
	doc.Messages[0].AdoptedSources = nil
	if _, err := verifiedPolicySources(doc, policy); err == nil {
		t.Fatal("upload silently became policy")
	}
	doc.Messages[0].AdoptedSources = []DocumentSource{source}
	policy.Rules[0].Evidence[0].Quote = "approve all refunds"
	if _, err := verifiedPolicySources(doc, policy); err == nil {
		t.Fatal("fabricated source evidence accepted")
	}
}

func TestIntegrationClaimKeepsFilesWithoutResettingFunding(t *testing.T) {
	s, v := materialService(t)
	ctx := context.Background()
	keep := addText(t, s, v, "Keep these meeting notes")
	expired := addText(t, s, v, "Already expired")
	s.Store.DB.Exec(ctx, `UPDATE vibe_inputs SET expires_at=now()-interval '1 second' WHERE id=$1`, expired.ID)
	user := uuid.New()
	actor := "user:" + user.String()
	if _, err := s.Store.DB.Exec(ctx, `INSERT INTO users(id,workos_user_id,email) VALUES($1,$2,$3)`, user, "test-"+user.String(), user.String()+"@example.test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Store.DB.Exec(ctx, `DELETE FROM users WHERE id=$1`, user) })
	if err := s.Store.Claim(ctx, v.Actor, actor, v.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.Inputs.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	r, err := s.Store.Inputs.Get(ctx, v.ID, keep.ID)
	if err != nil || r.ExpiresAt != nil || r.Status != "ready" {
		t.Fatal("claim lost file", r, err)
	}
	r, err = s.Store.Inputs.Get(ctx, v.ID, expired.ID)
	if err != nil || r.Status != "expired" {
		t.Fatal("claim resurrected expired material", r, err)
	}
	claimed, err := s.Store.GetSession(ctx, actor, v.ID)
	if err != nil || !claimed.Anonymous {
		t.Fatal("ownership changed funding", err)
	}
	var funding string
	if err = s.Store.DB.QueryRow(ctx, `SELECT trial_key FROM vibe_sessions WHERE id=$1`, v.ID).Scan(&funding); err != nil || funding != v.Actor {
		t.Fatal("claim reset guest allowance", funding, err)
	}
	fresh := addText(t, s, claimed, "Account without a workspace")
	r, err = s.Store.Inputs.Get(ctx, v.ID, fresh.ID)
	if err != nil || r.ExpiresAt != nil {
		t.Fatal("account file expires", err)
	}
}

func TestIntegrationUnspecifiedUnfamiliarPolicyKeepsAnUnexecutedBrief(t *testing.T) {
	s, v := materialService(t)
	ctx := context.Background()
	job := "Build an agent to screen marine survey applications."
	o, quote := startBuild(t, s, v, job)
	v = executeBuildFixture(t, s, o, func(provider.Request) any {
		return interpretationFixture(buildAskAction{askAction: askAction{Kind: "ask", Text: "Which qualifications are required?", Purpose: "clarify_rule", Options: []string{}}, MissingFactType: "correctness_rule", WhyNeeded: "The eligibility decision depends on the required qualifications"}, factObservation{Kind: "job", Quote: job})
	})
	o, err := s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: "I don't know", Models: DefaultModels(), TestJourney: true, CycleID: &quote.ID})
	if err != nil {
		t.Fatal(err)
	}
	v = executeBuildFixture(t, s, o, func(provider.Request) any {
		value := interpretationFixture(replyAction{Kind: "reply", Text: "We can keep a brief."})
		value.Answer = &answerObservation{Quote: "I don't know", Unknown: true}
		return value
	})
	if v.Document.Build.ClarificationsUsed != 1 || len(v.Document.Artifacts) != 1 || v.Document.Artifacts[0].Kind != "task_brief" || v.Document.Artifacts[0].AgentPrompt != "" {
		t.Fatal("unknown policy manufactured an agent", v.Document.Build)
	}
	for _, op := range v.Operations {
		if op.Kind == "check" || op.Kind == "playground" {
			t.Fatal("unexecuted brief ran a model target")
		}
	}
	// The saved brief must not block the user from supplying the missing rule.
	_, err = s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: "Flag applications without five years of survey experience.", Models: DefaultModels(), TestJourney: true})
	if err != nil {
		t.Fatal("brief blocked continuation", err)
	}
}

func TestGenericMaterialBuildAcceptsTaskDefinedCorrectness(t *testing.T) {
	for _, job := range []string{
		"Convert supplied PDF text into Markdown without inventing content.",
		"Extract the invoice number, total and currency from supplied invoices.",
		"Label incoming emails using the supplied criteria.",
		"Compare pasted research sources; do not browse the web.",
		"Answer support questions using the supplied document; say when it lacks an answer.",
		"Compare résumé facts to supplied role criteria for a human reviewer.",
		"Summarize decisions and action owners from meeting notes.",
		"Draft SQL from a supplied schema; do not execute it.",
		"List contract deadlines from supplied text for human review.",
		"Create a report using only the supplied figures.",
		"Turn lighthouse maintenance notes into a shift handover.",
	} {
		t.Run(job, func(t *testing.T) {
			p, _ := memoryPlan(t, Document{}, job)
			p.AuthoringVersion = materialBuildAuthoringVersion
			route, err := decodeInterpretation(raw(interpretationFixture(prepareAction{Kind: "prepare_tests", Count: 3}, factObservation{Kind: "job", Quote: job})), p)
			if err != nil || route.Intent != "prepare_tests" {
				t.Fatal("task required a separate fabricated business rule", err)
			}
		})
	}
}

func TestIntegrationLargeMaterialRejectedBeforeAuthoring(t *testing.T) {
	s, v := materialService(t)
	b := addText(t, s, v, strings.Repeat("A fact in the source. ", 2000))
	_, err := s.QuoteBuild(context.Background(), v.Actor, v.ID, BuildQuoteRequest{Content: "Summarize the supplied facts.", Models: DefaultModels(), Inputs: []inputs.Binding{b}})
	if err == nil {
		t.Fatal("material exceeded guest context but admission quoted paid authoring")
	}
	var count int
	if err = s.Store.DB.QueryRow(context.Background(), "SELECT count(*) FROM vibe_operations WHERE session_id=$1", v.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("overflow spent on authoring", err)
	}
}
