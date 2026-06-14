package vibeeval

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/google/uuid"
)

// --- fakes ---

type scriptedInvoker struct {
	responses []provider.Response
	calls     int
}

func (s *scriptedInvoker) InvokeModel(_ context.Context, _ provider.Request) (provider.Response, error) {
	if s.calls >= len(s.responses) {
		return provider.Response{}, errors.New("no scripted response")
	}
	r := s.responses[s.calls]
	s.calls++
	return r, nil
}

type memStore struct {
	msgs []Message
	seq  int64
}

func (m *memStore) Append(_ context.Context, msg Message) (Message, error) {
	m.seq++
	msg.Seq = m.seq
	msg.ID = uuid.New()
	m.msgs = append(m.msgs, msg)
	return msg, nil
}

func (m *memStore) History(_ context.Context, _ uuid.UUID) ([]Message, error) {
	return append([]Message(nil), m.msgs...), nil
}

type allowAll struct{}

func (allowAll) Authorize(context.Context, uuid.UUID, string) error { return nil }

type denyAll struct{}

func (denyAll) Authorize(context.Context, uuid.UUID, string) error { return errors.New("forbidden") }

type fakeReadTool struct {
	name   string
	called *bool
	result any
}

func (t fakeReadTool) Name() string           { return t.name }
func (t fakeReadTool) Phases() []string       { return []string{PhasePlan} }
func (t fakeReadTool) RiskTier() RiskTier     { return ReadTier }
func (t fakeReadTool) RequiredAction() string { return "read_workspace" }
func (t fakeReadTool) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{Name: t.name, Description: "test tool"}
}
func (t fakeReadTool) Execute(context.Context, Actor, Conversation, json.RawMessage) (ToolOutput, error) {
	*t.called = true
	return ToolOutput{Result: t.result, AuditResult: map[string]any{"ok": true}}, nil
}

func newLoop(inv modelInvoker, store MessageStore, reg ToolRegistry) *AgentLoop {
	return NewAgentLoop(inv, reg, store, NewEvidenceRedactor(),
		GuideModel{ProviderKey: "anthropic", Model: "claude-sonnet-4-6", CredentialReference: "secret://anthropic"},
		DefaultLimits())
}

func conv() Conversation {
	return Conversation{ID: uuid.New(), WorkspaceID: uuid.New(), OrganizationID: uuid.New(), Phase: PhasePlan}
}

// --- tests ---

func TestRunTurn_ToolCallThenFinalAnswer(t *testing.T) {
	called := false
	reg := NewRegistry()
	reg.Register(fakeReadTool{name: "list_packs", called: &called, result: map[string]any{"packs": []string{"p1"}}})

	inv := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{{ID: "tc1", Name: "list_packs", Arguments: json.RawMessage(`{}`)}}},
		{OutputText: "You have 1 pack: p1."},
	}}
	store := &memStore{}

	var events []Event
	res, err := newLoop(inv, store, reg).RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, conv(), "list my packs", func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatalf("RunTurn error: %v", err)
	}
	if !called {
		t.Fatal("tool was not executed")
	}
	if res.StopReason != "completed" {
		t.Fatalf("StopReason = %q, want completed", res.StopReason)
	}
	if res.AssistantText != "You have 1 pack: p1." {
		t.Fatalf("AssistantText = %q", res.AssistantText)
	}
	// user, assistant(tool call), tool(result), assistant(final) = 4 persisted
	if len(store.msgs) != 4 {
		t.Fatalf("persisted %d messages, want 4: %+v", len(store.msgs), roles(store.msgs))
	}
	if store.msgs[2].Role != RoleTool || !strings.Contains(store.msgs[2].Content, "UNTRUSTED EVIDENCE") {
		t.Fatalf("tool message not wrapped as evidence: %q", store.msgs[2].Content)
	}
	if !hasEvent(events, EventToolResult) || !hasEvent(events, EventTurnCompleted) {
		t.Fatalf("missing expected events: %v", events)
	}
}

func TestRunTurn_AuthorizerDenyProducesErrorEvidence(t *testing.T) {
	called := false
	reg := NewRegistry()
	reg.Register(fakeReadTool{name: "list_packs", called: &called, result: "x"})

	inv := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{{ID: "tc1", Name: "list_packs", Arguments: json.RawMessage(`{}`)}}},
		{OutputText: "Sorry, you can't do that."},
	}}
	store := &memStore{}

	res, err := newLoop(inv, store, reg).RunTurn(context.Background(), Actor{UserID: uuid.New()}, denyAll{}, conv(), "list", nil)
	if err != nil {
		t.Fatalf("RunTurn error: %v", err)
	}
	if called {
		t.Fatal("tool should NOT execute when authorizer denies")
	}
	if len(res.ToolInvocations) != 1 || res.ToolInvocations[0].Error != "forbidden" {
		t.Fatalf("expected one forbidden invocation, got %+v", res.ToolInvocations)
	}
}

func TestRunTurn_UnknownToolDoesNotPanic(t *testing.T) {
	reg := NewRegistry()
	inv := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{{ID: "tc1", Name: "does_not_exist", Arguments: json.RawMessage(`{}`)}}},
		{OutputText: "ok"},
	}}
	res, err := newLoop(inv, &memStore{}, reg).RunTurn(context.Background(), Actor{}, allowAll{}, conv(), "go", nil)
	if err != nil {
		t.Fatalf("RunTurn error: %v", err)
	}
	if len(res.ToolInvocations) != 1 || res.ToolInvocations[0].Error == "" {
		t.Fatalf("expected one errored invocation for unknown tool, got %+v", res.ToolInvocations)
	}
}

func roles(msgs []Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Role
	}
	return out
}

func hasEvent(events []Event, t EventType) bool {
	for _, e := range events {
		if e.Type == t {
			return true
		}
	}
	return false
}
