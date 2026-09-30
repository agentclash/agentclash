package vibe

import (
	"context"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"

	"strings"
	"testing"
)

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

func TestVibeContextDiagnostics(t *testing.T) {
	req := provider.Request{Messages: []provider.Message{{Role: "user", Content: strings.Repeat("x", 20000)}}, MaxOutputTokens: 2048}
	_, err := CountContext(req, ModelProfile{Free: true, Context: 512000, FramingAllowance: 4096}, LimitsFor(true))
	f := issueFrom(err)
	if f == nil || f.Code != "context_limit" || f.Context == nil || f.Context.UpperBound <= f.Context.Limit || f.Context.LargestSection != "user_messages" {
		t.Fatalf("missing diagnostic: %+v", f)
	}
}
