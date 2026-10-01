package vibe

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestVibeIntegrationStarterNeedsBriefBeforeAdmission(t *testing.T) {
	store := integrationStore(t)
	cfg := testConfig()
	// No Redis gate or provider: intake must be resolved before either is used.
	svc := &Service{Store: store, Config: cfg}
	ctx := context.Background()
	for _, label := range []string{
		"Help me build an agent",
		"I have an agent that needs testing",
		"I’m figuring out what AI could do for us",
		"  HELP ME\nBUILD AN AGENT!  ",
		"I'm figuring out what AI could do for us.",
	} {
		for _, kind := range []string{"message", "build"} {
			t.Run(kind+"/"+label, func(t *testing.T) {
				session := anonSession(t, store)
				_, err := svc.Prepare(ctx, session.Actor, session.ID, Submission{ClientID: uuid.New(), Revision: session.Revision, Kind: kind, Content: label, Models: cfg.DefaultModels()})
				if issue := issueFrom(err); issue == nil || issue.Code != "invalid_message" || !strings.Contains(issue.Message, "?") {
					t.Fatal("starter did not produce an intake question", err)
				}
				saved, err := store.GetSession(ctx, session.Actor, session.ID)
				if err != nil {
					t.Fatal(err)
				}
				if saved.Revision != session.Revision || !reflect.DeepEqual(saved.Document, session.Document) || len(saved.Operations) != 0 {
					t.Fatal("starter changed the conversation or admitted an operation")
				}
				var attempts, reservations int
				err = store.DB.QueryRow(ctx, `SELECT
					(SELECT count(*) FROM vibe_attempts WHERE operation_id IN (SELECT id FROM vibe_operations WHERE session_id=$1)),
					(SELECT count(*) FROM vibe_reservations WHERE operation_id IN (SELECT id FROM vibe_operations WHERE session_id=$1))`, session.ID).Scan(&attempts, &reservations)
				if err != nil || attempts != 0 || reservations != 0 {
					t.Fatal("starter consumed provider or reservation allowance", attempts, reservations, err)
				}
			})
		}
	}
}

func TestVibeBriefContext(t *testing.T) {
	for _, tc := range []struct {
		name    string
		doc     Document
		content string
		ask     bool
	}{
		{name: "specific task", content: "Help me build an agent that answers refund questions."},
		{name: "existing stack", content: "My support agent runs in Python; I can share logs."},
		{name: "legacy starter history", doc: Document{Messages: []Message{{Role: "user", Content: "Help me build an agent"}, {Role: "assistant", Content: "What should it do?"}}}, content: "Help me build an agent", ask: true},
		{name: "prior user brief", doc: Document{Messages: []Message{{Role: "user", Content: "We need help triaging customer refunds."}}}, content: "Help me build an agent"},
		{name: "trial is not design context", doc: Document{Messages: []Message{{Role: "user", Content: "I want a refund.", Origin: "playground"}}}, content: "Help me build an agent", ask: true},
		{name: "imported draft", doc: Document{Artifacts: []Artifact{{AgentPrompt: "Answer supplied refund questions"}}}, content: "Help me build an agent"},
		{name: "known requirements", doc: Document{Requirements: []Requirement{{Statement: "Triage refund requests", Status: "accepted"}}}, content: "Help me build an agent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := briefQuestion(tc.doc, tc.content) != ""; got != tc.ask {
				t.Fatalf("ask = %v, want %v", got, tc.ask)
			}
		})
	}
}

func TestVibeIntegrationLegacyStarterRecoversOriginalOperation(t *testing.T) {
	store := integrationStore(t)
	session := anonSession(t, store)
	cfg := testConfig()
	ctx := context.Background()
	sub := Submission{ClientID: uuid.New(), Revision: session.Revision, Kind: "message", Content: "Help me build an agent", Models: cfg.DefaultModels()}
	op, err := store.Submit(ctx, session.Actor, session.ID, sub, Plan{Submission: sub, Anonymous: true, Calls: 2, MaxCost: 10000000}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{Store: store, Config: cfg, Gate: testGate(t)}
	again, err := svc.Prepare(ctx, session.Actor, session.ID, sub)
	if err != nil || again.ID != op.ID {
		t.Fatal("legacy starter lost its original receipt", err)
	}
	sub.Content = "I have an agent that needs testing"
	_, err = svc.Prepare(ctx, session.Actor, session.ID, sub)
	if issue := issueFrom(err); issue == nil || issue.Code != "idempotency_conflict" {
		t.Fatal("changed starter lost the idempotency conflict", err)
	}
	saved, err := store.GetSession(ctx, session.Actor, session.ID)
	if err != nil || len(saved.Operations) != 1 || saved.Operations[0].ModelCalls != 0 {
		t.Fatal("receipt recovery repeated work", err)
	}
}

func TestVibeCapabilityInstructionsMatchEffectivePreview(t *testing.T) {
	for _, c := range Capabilities() {
		if c.ID == "text_preview" {
			if c.Instructions == "" || c.Instructions+"User instructions" != PreviewPrompt("User instructions") {
				t.Fatal("read-only UI rules diverged from effective preview instructions")
			}
			return
		}
	}
	t.Fatal("missing text preview capability")
}
