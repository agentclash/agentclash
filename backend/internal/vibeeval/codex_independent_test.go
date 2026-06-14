package vibeeval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/google/uuid"
)

type inspectingInvoker struct {
	responses []provider.Response
	requests  []provider.Request
	check     func(provider.Request) error
}

func (i *inspectingInvoker) InvokeModel(_ context.Context, req provider.Request) (provider.Response, error) {
	i.requests = append(i.requests, req)
	if i.check != nil {
		if err := i.check(req); err != nil {
			return provider.Response{}, err
		}
	}
	if len(i.responses) == 0 {
		return provider.Response{}, errors.New("no scripted response")
	}
	resp := i.responses[0]
	i.responses = i.responses[1:]
	return resp, nil
}

type errStore struct {
	*memStore
	appendErrAfter int
	historyErr     error
}

func (e *errStore) Append(ctx context.Context, msg Message) (Message, error) {
	if e.appendErrAfter == 0 {
		return Message{}, errors.New("append failed")
	}
	e.appendErrAfter--
	return e.memStore.Append(ctx, msg)
}

func (e *errStore) History(context.Context, uuid.UUID) ([]Message, error) {
	if e.historyErr != nil {
		return nil, e.historyErr
	}
	return e.memStore.History(context.Background(), uuid.Nil)
}

type phaseTool struct {
	name   string
	phases []string
}

func (p phaseTool) Name() string                        { return p.name }
func (p phaseTool) Phases() []string                    { return p.phases }
func (p phaseTool) RiskTier() RiskTier                  { return ReadTier }
func (p phaseTool) RequiredAction() string              { return "read_workspace" }
func (p phaseTool) Definition() provider.ToolDefinition { return provider.ToolDefinition{Name: p.name} }
func (p phaseTool) Execute(context.Context, Actor, Conversation, json.RawMessage) (ToolOutput, error) {
	return ToolOutput{Result: "ok"}, nil
}

func TestRunTurn_ReplayedHistoryPreservesAssistantToolCalls(t *testing.T) {
	c := conv()
	called := false
	reg := NewRegistry()
	reg.Register(fakeReadTool{name: "list_packs", called: &called, result: map[string]any{"packs": []string{"p1"}}})
	store := &memStore{}

	first := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{{ID: "tc1", Name: "list_packs", Arguments: json.RawMessage(`{}`)}}},
		{OutputText: "first done"},
	}}
	if _, err := newLoop(first, store, reg).RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, "first", nil); err != nil {
		t.Fatalf("first RunTurn error: %v", err)
	}
	if !called {
		t.Fatal("first turn did not execute tool")
	}

	second := &inspectingInvoker{
		responses: []provider.Response{{OutputText: "second done"}},
		check: func(req provider.Request) error {
			return assertToolResultsHaveAssistantCalls(req.Messages)
		},
	}
	_, err := newLoop(second, store, reg).RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, "second", nil)
	if err != nil {
		t.Fatalf("second RunTurn error: %v", err)
	}
}

func TestRunTurn_ReplayedHistoryPreservesMultipleAssistantToolCalls(t *testing.T) {
	c := conv()
	firstCalled, secondCalled := false, false
	reg := NewRegistry()
	reg.Register(fakeReadTool{name: "list_packs", called: &firstCalled, result: map[string]any{"packs": []string{"p1"}}})
	reg.Register(fakeReadTool{name: "read_scorecard", called: &secondCalled, result: map[string]any{"score": 0.93}})
	store := &memStore{}

	first := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{
			{ID: "tc1", Name: "list_packs", Arguments: json.RawMessage(`{"workspace_id":"w1"}`)},
			{ID: "tc2", Name: "read_scorecard", Arguments: json.RawMessage(`{"run_agent_id":"ra1"}`)},
		}},
		{OutputText: "first done"},
	}}
	if _, err := newLoop(first, store, reg).RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, "first", nil); err != nil {
		t.Fatalf("first RunTurn error: %v", err)
	}
	if !firstCalled || !secondCalled {
		t.Fatalf("expected both tools called, got first=%v second=%v", firstCalled, secondCalled)
	}

	second := &inspectingInvoker{
		responses: []provider.Response{{OutputText: "second done"}},
		check: func(req provider.Request) error {
			if err := assertToolResultsHaveAssistantCalls(req.Messages); err != nil {
				return err
			}
			for _, msg := range req.Messages {
				if msg.Role == RoleAssistant && len(msg.ToolCalls) == 2 {
					if msg.ToolCalls[0].ID != "tc1" || msg.ToolCalls[1].ID != "tc2" {
						return fmt.Errorf("assistant tool calls replayed out of order: %+v", msg.ToolCalls)
					}
					return nil
				}
			}
			return errors.New("no replayed assistant message with both tool calls")
		},
	}
	_, err := newLoop(second, store, reg).RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, "second", nil)
	if err != nil {
		t.Fatalf("second RunTurn error: %v", err)
	}
}

func TestRunTurn_MalformedPersistedToolCallsReturnsHistoryError(t *testing.T) {
	c := conv()
	store := &memStore{msgs: []Message{
		{
			ID:             uuid.New(),
			ConversationID: c.ID,
			Seq:            1,
			Role:           RoleAssistant,
			Content:        "I will call a tool",
			ToolCalls:      json.RawMessage(`{"not":"an array"`),
		},
	}}
	inv := &scriptedInvoker{responses: []provider.Response{{OutputText: "should not invoke"}}}

	_, err := newLoop(inv, store, NewRegistry()).RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, "continue", nil)
	if err == nil {
		t.Fatal("RunTurn error = nil, want malformed tool_calls error")
	}
	if !strings.Contains(err.Error(), "rebuild history") || !strings.Contains(err.Error(), "unmarshal tool_calls") {
		t.Fatalf("RunTurn error = %v, want wrapped tool_calls history error", err)
	}
	if inv.calls != 0 {
		t.Fatalf("invocations = %d, want 0 when history is corrupt", inv.calls)
	}
}

func TestRunTurn_StopsAtMaxSteps(t *testing.T) {
	called := false
	reg := NewRegistry()
	reg.Register(fakeReadTool{name: "list_packs", called: &called, result: "ok"})
	inv := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{{ID: "tc1", Name: "list_packs", Arguments: json.RawMessage(`{}`)}}},
		{OutputText: "should not be reached"},
	}}
	store := &memStore{}
	loop := NewAgentLoop(inv, reg, store, NewEvidenceRedactor(),
		GuideModel{ProviderKey: "anthropic", Model: "claude-sonnet-4-6", CredentialReference: "secret://anthropic"},
		AgentLimits{MaxSteps: 1, MaxToolCalls: 4})

	res, err := loop.RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, conv(), "go", nil)
	if err != nil {
		t.Fatalf("RunTurn error: %v", err)
	}
	if res.StopReason != "limit" {
		t.Fatalf("StopReason = %q, want limit", res.StopReason)
	}
	if inv.calls != 1 {
		t.Fatalf("invocations = %d, want 1", inv.calls)
	}
}

func TestRunTurn_StopsAtMaxToolCalls(t *testing.T) {
	called := false
	reg := NewRegistry()
	reg.Register(fakeReadTool{name: "list_packs", called: &called, result: "ok"})
	inv := &scriptedInvoker{responses: []provider.Response{{
		ToolCalls: []provider.ToolCall{
			{ID: "tc1", Name: "list_packs", Arguments: json.RawMessage(`{}`)},
			{ID: "tc2", Name: "list_packs", Arguments: json.RawMessage(`{}`)},
		},
	}}}
	store := &memStore{}
	loop := NewAgentLoop(inv, reg, store, NewEvidenceRedactor(),
		GuideModel{ProviderKey: "anthropic", Model: "claude-sonnet-4-6", CredentialReference: "secret://anthropic"},
		AgentLimits{MaxSteps: 4, MaxToolCalls: 1})

	res, err := loop.RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, conv(), "go", nil)
	if err != nil {
		t.Fatalf("RunTurn error: %v", err)
	}
	if res.StopReason != "limit" {
		t.Fatalf("StopReason = %q, want limit", res.StopReason)
	}
	if len(res.ToolInvocations) != 1 {
		t.Fatalf("ToolInvocations = %d, want 1", len(res.ToolInvocations))
	}
}

func TestEvidenceRedactor_EdgeCasesAndSecretScrub(t *testing.T) {
	redactor := NewEvidenceRedactor()

	cases := []struct {
		name string
		raw  any
		want string
	}{
		{name: "nil", raw: nil, want: "BEGIN UNTRUSTED EVIDENCE"},
		{name: "bytes", raw: []byte("Authorization: Bearer secret\nok"), want: "[redacted]\nok"},
		{name: "struct", raw: struct {
			OK bool `json:"ok"`
		}{OK: true}, want: `{"ok":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := redactor.Wrap("tool", tc.raw)
			if err != nil {
				t.Fatalf("Wrap error: %v", err)
			}
			if !strings.Contains(got, tc.want) {
				t.Fatalf("wrapped evidence %q does not contain %q", got, tc.want)
			}
		})
	}

	if _, err := redactor.Wrap("bad", make(chan int)); err == nil {
		t.Fatal("Wrap(marshal-failing value) error = nil")
	}
}

func TestRegistry_ForPhaseStableDefinitionsAndUnknownPhase(t *testing.T) {
	reg := NewRegistry()
	reg.Register(phaseTool{name: "zeta", phases: []string{PhasePlan, PhaseAnalyze}})
	reg.Register(phaseTool{name: "alpha", phases: []string{PhasePlan}})

	tools := reg.ForPhase(PhasePlan)
	if len(tools) != 2 || tools[0].Name() != "alpha" || tools[1].Name() != "zeta" {
		t.Fatalf("ForPhase order = %v, want alpha,zeta", toolNames(tools))
	}
	if len(reg.ForPhase(PhasePublish)) != 0 {
		t.Fatalf("unknown/unregistered phase returned tools: %v", toolNames(reg.ForPhase(PhasePublish)))
	}
	if _, ok := reg.Resolve("zeta"); !ok {
		t.Fatal("Resolve(zeta) = false")
	}
	defs := reg.Definitions(PhasePlan)
	if len(defs) != 2 || defs[0].Name != "alpha" || defs[1].Name != "zeta" {
		t.Fatalf("Definitions order = %+v, want alpha,zeta", defs)
	}
	analyze := reg.ForPhase(PhaseAnalyze)
	if len(analyze) != 1 || analyze[0].Name() != "zeta" {
		t.Fatalf("multi-phase registration missing from analyze: %v", toolNames(analyze))
	}
}

func TestRunTurn_PropagatesStoreAppendError(t *testing.T) {
	store := &errStore{memStore: &memStore{}, appendErrAfter: 0}
	inv := &scriptedInvoker{responses: []provider.Response{{OutputText: "unused"}}}

	_, err := newLoop(inv, store, NewRegistry()).RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, conv(), "go", nil)
	if err == nil || !strings.Contains(err.Error(), "append failed") {
		t.Fatalf("RunTurn error = %v, want append failed", err)
	}
	if inv.calls != 0 {
		t.Fatalf("invoker calls = %d, want 0 when user append fails", inv.calls)
	}
}

func toolNames(tools []Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name())
	}
	return names
}

func assertToolResultsHaveAssistantCalls(messages []provider.Message) error {
	var lastAssistant provider.Message
	for _, msg := range messages {
		switch msg.Role {
		case RoleAssistant:
			lastAssistant = msg
		case RoleTool:
			if len(lastAssistant.ToolCalls) == 0 {
				return fmt.Errorf("tool message %q replayed without preceding assistant ToolCalls", msg.ToolCallID)
			}
			found := false
			for _, call := range lastAssistant.ToolCalls {
				if call.ID == msg.ToolCallID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("tool message %q replayed without matching assistant ToolCalls; calls=%+v", msg.ToolCallID, lastAssistant.ToolCalls)
			}
		}
	}
	return nil
}
