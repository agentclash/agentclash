package vibeeval

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agentclash/agentclash/backend/internal/provider"
)

// modelInvoker is the slice of the provider Router the loop needs. provider.Router satisfies
// it. Kept small so the loop is unit-testable with a fake (accept interfaces).
type modelInvoker interface {
	InvokeModel(ctx context.Context, request provider.Request) (provider.Response, error)
}

// GuideModel is the AgentClash-owned, server-side model config for the guide agent
// (VIBEEVAL_GUIDE_*). Credential reference is a secret://-/env:// server ref, never BYOK.
type GuideModel struct {
	ProviderKey         string
	Model               string
	CredentialReference string
}

// AgentLimits bound a single turn.
type AgentLimits struct {
	MaxSteps     int
	MaxToolCalls int
	WallClock    time.Duration
}

// DefaultLimits is a conservative bound for the read-only Step 2 loop.
func DefaultLimits() AgentLimits {
	return AgentLimits{MaxSteps: 8, MaxToolCalls: 16, WallClock: 2 * time.Minute}
}

// EventType identifies a streamed turn event (consumed by the api SSE handler).
type EventType string

const (
	EventAssistantText EventType = "assistant.text"
	EventToolCall      EventType = "tool.call"
	EventToolResult    EventType = "tool.result"
	EventTurnCompleted EventType = "turn.completed"
	EventError         EventType = "error"
)

// Event is a streamed turn event.
type Event struct {
	Type       EventType `json:"type"`
	Text       string    `json:"text,omitempty"`
	ToolName   string    `json:"tool_name,omitempty"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
	StopReason string    `json:"stop_reason,omitempty"`
}

// EventSink receives turn events as they happen. nil is tolerated.
type EventSink func(Event)

// ToolInvocationRecord summarizes one tool call within a turn.
type ToolInvocationRecord struct {
	ToolName string
	Action   string
	RiskTier RiskTier
	OK       bool
	Error    string
}

// TurnResult is the outcome of one bounded turn.
type TurnResult struct {
	AssistantText   string
	ToolInvocations []ToolInvocationRecord
	StopReason      string // completed | limit | error
	Usage           provider.Usage
}

// AgentLoop runs one bounded, tool-calling guide turn. Step 2 exercises read-only tools
// only — confirmation (Step 3) and credit (Step 4) are not wired here.
type AgentLoop struct {
	invoker  modelInvoker
	registry ToolRegistry
	messages MessageStore
	redactor EvidenceRedactor
	model    GuideModel
	limits   AgentLimits
}

// NewAgentLoop constructs the loop. invoker is typically a provider.Router.
func NewAgentLoop(invoker modelInvoker, reg ToolRegistry, store MessageStore, redactor EvidenceRedactor, model GuideModel, limits AgentLimits) *AgentLoop {
	if limits.MaxSteps == 0 {
		limits = DefaultLimits()
	}
	return &AgentLoop{invoker: invoker, registry: reg, messages: store, redactor: redactor, model: model, limits: limits}
}

// RunTurn runs one bounded turn for the given actor/conversation. actor is audit identity;
// authorization goes through authorizer (never api.Caller). sink receives live events.
func (l *AgentLoop) RunTurn(ctx context.Context, actor Actor, authorizer WorkspaceAuthorizer, conv Conversation, userMessage string, sink EventSink) (TurnResult, error) {
	if sink == nil {
		sink = func(Event) {}
	}
	var result TurnResult

	// Persist the user message (verbatim for Step 2; the narrow chat scrub lands with its
	// consumer per §11.6).
	if _, err := l.messages.Append(ctx, Message{ConversationID: conv.ID, Role: RoleUser, Content: userMessage, RedactionState: RedactionNone}); err != nil {
		return result, err
	}

	// Build the in-memory provider message list (system + prior transcript + this user turn).
	// In-memory keeps tool_use/tool_result pairing correct within the turn; for cross-turn
	// replay, toProviderMessage reconstructs each assistant row's persisted tool_calls (and
	// errors on a corrupt row) so the tool_use/tool_result pairing survives prior turns.
	pmsgs := []provider.Message{{Role: "system", Content: systemPrompt(conv.Phase)}}
	history, err := l.messages.History(ctx, conv.ID)
	if err != nil {
		return result, err
	}
	for _, m := range history {
		pm, err := toProviderMessage(m)
		if err != nil {
			return result, fmt.Errorf("rebuild history for conversation %s: %w", conv.ID, err)
		}
		pmsgs = append(pmsgs, pm)
	}

	defs := l.registry.Definitions(conv.Phase)
	steps, toolCalls := 0, 0

	for {
		if steps >= l.limits.MaxSteps {
			result.StopReason = "limit"
			break
		}
		steps++

		resp, err := l.invoker.InvokeModel(ctx, provider.Request{
			ProviderKey:         l.model.ProviderKey,
			Model:               l.model.Model,
			CredentialReference: l.model.CredentialReference,
			Messages:            pmsgs,
			Tools:               defs,
		})
		if err != nil {
			sink(Event{Type: EventError, Text: err.Error()})
			result.StopReason = "error"
			return result, err
		}
		result.Usage = addUsage(result.Usage, resp.Usage)

		// Record + persist the assistant turn. The full tool-call array is stored so a later
		// turn replaying this row reconstructs the assistant tool_use/tool_result pairing
		// (provider history is otherwise invalid — a tool message with no preceding call).
		pmsgs = append(pmsgs, provider.Message{Role: "assistant", Content: resp.OutputText, ToolCalls: resp.ToolCalls})
		toolCallsRaw, err := toolCallsJSON(resp.ToolCalls)
		if err != nil {
			return result, err
		}
		if _, err := l.messages.Append(ctx, Message{
			ConversationID: conv.ID,
			Role:           RoleAssistant,
			Content:        resp.OutputText,
			RedactionState: RedactionNone,
			ToolCalls:      toolCallsRaw,
			Usage:          usageJSON(resp.Usage),
		}); err != nil {
			return result, err
		}
		if resp.OutputText != "" {
			sink(Event{Type: EventAssistantText, Text: resp.OutputText})
			result.AssistantText = resp.OutputText
		}

		if len(resp.ToolCalls) == 0 {
			result.StopReason = "completed"
			break
		}

		for _, tc := range resp.ToolCalls {
			if toolCalls >= l.limits.MaxToolCalls {
				result.StopReason = "limit"
				return l.finish(result, sink), nil
			}
			toolCalls++
			rec, toolMsg := l.executeTool(ctx, actor, authorizer, conv, tc, sink)
			result.ToolInvocations = append(result.ToolInvocations, rec)
			pmsgs = append(pmsgs, toolMsg)
		}
	}

	return l.finish(result, sink), nil
}

func (l *AgentLoop) finish(result TurnResult, sink EventSink) TurnResult {
	sink(Event{Type: EventTurnCompleted, StopReason: result.StopReason})
	return result
}

// executeTool resolves, authorizes, runs, redacts, and persists one tool call. It returns
// the audit record and the provider tool-result message to append to the conversation.
func (l *AgentLoop) executeTool(ctx context.Context, actor Actor, authorizer WorkspaceAuthorizer, conv Conversation, tc provider.ToolCall, sink EventSink) (ToolInvocationRecord, provider.Message) {
	rec := ToolInvocationRecord{ToolName: tc.Name}
	sink(Event{Type: EventToolCall, ToolName: tc.Name, ToolCallID: tc.ID})

	fail := func(content string) provider.Message {
		evidence, _ := l.redactor.Wrap(tc.Name, content)
		_, _ = l.messages.Append(ctx, Message{ConversationID: conv.ID, Role: RoleTool, Content: evidence, RedactionState: RedactionApplied, ToolCallID: tc.ID, ToolName: tc.Name})
		sink(Event{Type: EventToolResult, ToolName: tc.Name, ToolCallID: tc.ID})
		return provider.Message{Role: "tool", ToolCallID: tc.ID, Content: evidence, IsError: true}
	}

	tool, ok := l.registry.Resolve(tc.Name)
	if !ok {
		rec.Error = "unknown or not-loaded tool"
		return rec, fail("error: unknown or not-loaded tool " + tc.Name)
	}
	rec.Action, rec.RiskTier = tool.RequiredAction(), tool.RiskTier()

	if err := authorizer.Authorize(ctx, conv.WorkspaceID, tool.RequiredAction()); err != nil {
		rec.Error = "forbidden"
		return rec, fail("error: not authorized for " + tc.Name)
	}

	out, err := tool.Execute(ctx, actor, conv, tc.Arguments)
	if err != nil {
		rec.Error = err.Error()
		return rec, fail("error executing " + tc.Name + ": " + err.Error())
	}

	evidence, err := l.redactor.Wrap(tc.Name, out.Result)
	if err != nil {
		rec.Error = err.Error()
		return rec, fail("error rendering " + tc.Name + " output")
	}
	if _, err := l.messages.Append(ctx, Message{ConversationID: conv.ID, Role: RoleTool, Content: evidence, RedactionState: RedactionApplied, ToolCallID: tc.ID, ToolName: tc.Name}); err != nil {
		// A dropped tool-result row breaks the NEXT turn's replay (assistant tool_use
		// with no following tool_result). Surface it instead of reporting success.
		rec.Error = err.Error()
		sink(Event{Type: EventToolResult, ToolName: tc.Name, ToolCallID: tc.ID})
		return rec, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: evidence}
	}
	sink(Event{Type: EventToolResult, ToolName: tc.Name, ToolCallID: tc.ID})
	rec.OK = true
	return rec, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: evidence}
}

func toProviderMessage(m Message) (provider.Message, error) {
	pm := provider.Message{Role: m.Role, Content: m.Content}
	switch m.Role {
	case RoleTool:
		pm.ToolCallID = m.ToolCallID
	case RoleAssistant:
		// Reconstruct the tool-call array so a replayed assistant row keeps its
		// tool_use/tool_result pairing with the following tool message(s). A corrupt
		// row must surface as an error, not silently drop the calls — dropping them
		// reproduces the unpaired-tool-message bug this persistence fix prevents.
		if len(m.ToolCalls) > 0 {
			var calls []provider.ToolCall
			if err := json.Unmarshal(m.ToolCalls, &calls); err != nil {
				return provider.Message{}, fmt.Errorf("unmarshal tool_calls for message %s: %w", m.ID, err)
			}
			if len(calls) > 0 { // '[]' default decodes to an empty slice; leave ToolCalls nil
				pm.ToolCalls = calls
			}
		}
	}
	return pm, nil
}

// toolCallsJSON marshals the assistant tool-call array for persistence. Returns nil on the
// no-call case so the repo layer keeps the column's '[]' default rather than storing "null".
// A marshal failure is returned so the assistant row is never persisted with its tool calls
// silently dropped (which would corrupt cross-turn replay).
func toolCallsJSON(calls []provider.ToolCall) (json.RawMessage, error) {
	if len(calls) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(calls)
	if err != nil {
		return nil, fmt.Errorf("marshal tool_calls: %w", err)
	}
	return b, nil
}

func addUsage(a, b provider.Usage) provider.Usage {
	a.InputTokens += b.InputTokens
	a.OutputTokens += b.OutputTokens
	a.TotalTokens += b.TotalTokens
	return a
}

func usageJSON(u provider.Usage) json.RawMessage {
	b, err := json.Marshal(u)
	if err != nil {
		return nil
	}
	return b
}
