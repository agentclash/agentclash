package vibe

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func TestIntegrationAuthoringIncludesActiveAcceptedArtifact(t *testing.T) {
	store := integrationStore(t)
	session := anonSession(t, store)
	ctx := context.Background()
	accepted := Artifact{ID: uuid.New(), Title: "Accepted agent", AgentPrompt: "PRESERVE_ACTIVE_AGENT: only use verified product facts.", Blueprint: json.RawMessage(`{"pinned":"original coverage"}`), Accepted: true, CreatedAt: timestamp()}
	proposal := Artifact{ID: uuid.New(), Title: "Unrelated imported draft", AgentPrompt: "Unrelated instructions, not accepted.", Blueprint: json.RawMessage(`{"pinned":"different coverage"}`), CreatedAt: timestamp()}
	if err := store.Edit(ctx, session.Actor, session.ID, session.Revision, func(v *Session) error {
		v.Document.Artifacts = []Artifact{accepted, proposal}
		v.Document.ActiveArtifactID = &accepted.ID
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	session, err := store.GetSession(ctx, session.Actor, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	cfg := freeConfig()
	gate := testGate(t)
	svc := &Service{Store: store, Config: cfg, Gate: gate, Compiler: repairCompiler{}}
	var requests []provider.Request
	fake := callFunc(func(_ context.Context, request provider.Request) (provider.Response, error) {
		requests = append(requests, request)
		output := `{"reply":"Review the improvement.","proposed_requirements":[],"assumptions":[],"draft":{"title":"Improved agent","agent_prompt":"Only use supplied facts. Add a clear CTA.","examples":[],"success_criteria":""}}`
		if len(requests) == 1 {
			output = `{"reply":"Invalid shape","proposed_requirements":[{}],"assumptions":[],"draft":null}`
		}
		zero := json.Number("0")
		return provider.Response{OutputText: output, Usage: provider.Usage{InputTokens: 100, OutputTokens: 100, CostUSD: &zero}}, nil
	})
	runner := Runner{Service: svc, Gateway: &Gateway{Store: store, Config: cfg, Gate: gate, Client: fake}}
	op, err := svc.Prepare(ctx, session.Actor, session.ID, Submission{ClientID: uuid.New(), Revision: session.Revision, Kind: "message", Content: "Improve the accepted agent's CTA.", Models: cfg.DefaultModels()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := store.DB.Exec(ctx, "DELETE FROM vibe_attempts WHERE operation_id=$1", op.ID); err != nil {
			t.Errorf("cleanup fixture attempts: %v", err)
		}
	})
	err = runner.Execute(ctx, op.ID)
	if finishErr := store.Finish(ctx, op.ID, issueFrom(err)); finishErr != nil {
		t.Fatal(finishErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 {
		t.Fatalf("want initial and one repair, got %d", len(requests))
	}
	for i, request := range requests {
		if !strings.Contains(request.Messages[1].Content, accepted.AgentPrompt) {
			t.Errorf("authoring call %d omitted the active accepted agent instructions", i)
		}
		var data struct {
			AcceptedAgent *Artifact `json:"accepted_agent"`
		}
		if err := json.Unmarshal([]byte(request.Messages[1].Content), &data); err != nil {
			t.Fatal(err)
		}
		if data.AcceptedAgent == nil || data.AcceptedAgent.ID != accepted.ID || !data.AcceptedAgent.Accepted {
			t.Errorf("authoring call %d did not identify the accepted agent separately from the proposal", i)
		}
		if _, err := CountContext(request, cfg.Profiles[cfg.DefaultModel], LimitsFor(true)); err != nil {
			t.Errorf("call %d escaped complete context bounds: %v", i, err)
		}
	}
	current, err := store.GetSession(ctx, session.Actor, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	improved := current.Document.Artifacts[len(current.Document.Artifacts)-1]
	var got, want any
	if json.Unmarshal(improved.Blueprint, &got) != nil || json.Unmarshal(accepted.Blueprint, &want) != nil || !reflect.DeepEqual(got, want) || improved.ParentID == nil || *improved.ParentID != accepted.ID {
		t.Fatal("improvement lost its accepted parent or changed pinned coverage")
	}
}
