package vibeeval

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/google/uuid"
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
	MaxSteps        int
	MaxToolCalls    int
	WallClock       time.Duration
	ConfirmationTTL time.Duration // how long a proposed confirmation stays resolvable
}

// DefaultLimits is a conservative bound for the guide loop.
func DefaultLimits() AgentLimits {
	return AgentLimits{MaxSteps: 8, MaxToolCalls: 16, WallClock: 2 * time.Minute, ConfirmationTTL: 15 * time.Minute}
}

// EventType identifies a streamed turn event (consumed by the api SSE handler).
type EventType string

const (
	EventAssistantText        EventType = "assistant.text"
	EventToolCall             EventType = "tool.call"
	EventToolResult           EventType = "tool.result"
	EventConfirmationRequired EventType = "confirmation.required"
	EventTurnCompleted        EventType = "turn.completed"
	EventError                EventType = "error"
)

// Event is a streamed turn event. The confirmation.required card carries metadata only (id, action,
// summary) — never the bound args or any secret/content.
type Event struct {
	Type           EventType `json:"type"`
	Text           string    `json:"text,omitempty"`
	ToolName       string    `json:"tool_name,omitempty"`
	ToolCallID     string    `json:"tool_call_id,omitempty"`
	ConfirmationID string    `json:"confirmation_id,omitempty"`
	Action         string    `json:"action,omitempty"`
	Summary        string    `json:"summary,omitempty"`
	PayloadHash    string    `json:"payload_hash,omitempty"` // the client echoes this on resolve (hash of args, not the args)
	ExpiresAt      string    `json:"expires_at,omitempty"`   // RFC3339, on confirmation.required
	StopReason     string    `json:"stop_reason,omitempty"`
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
	StopReason      string // completed | limit | error | confirmation_required
	// PendingConfirmation is set when StopReason == confirmation_required: the proposed action the
	// caller must approve/deny via POST .../confirmations/{id} to resume the turn.
	PendingConfirmation *PendingConfirmation
	Usage               provider.Usage
}

// AgentLoop runs one bounded, tool-calling guide turn. read/draft tools execute inline;
// workspace_write+ tools propose a confirmation and end the turn (Step 3, end+resume). Credit
// (Step 4) is not wired here.
type AgentLoop struct {
	invoker       modelInvoker
	registry      ToolRegistry
	messages      MessageStore
	redactor      EvidenceRedactor
	confirmations ConfirmationStore // nil until WithConfirmationStore; required for write tools
	audit         AuditWriter
	model         GuideModel
	limits        AgentLimits
}

// NewAgentLoop constructs the loop. invoker is typically a provider.Router. Audit defaults to a
// no-op and confirmations to nil; the api layer injects repo-backed implementations via the
// With* setters (Step 3b-2).
func NewAgentLoop(invoker modelInvoker, reg ToolRegistry, store MessageStore, redactor EvidenceRedactor, model GuideModel, limits AgentLimits) *AgentLoop {
	// Normalize each defaultable bound independently — a caller that sets only MaxSteps must still
	// get a non-zero ConfirmationTTL, else a proposed confirmation would be expired at creation.
	def := DefaultLimits()
	if limits.MaxSteps == 0 {
		limits.MaxSteps = def.MaxSteps
	}
	if limits.MaxToolCalls == 0 {
		limits.MaxToolCalls = def.MaxToolCalls
	}
	if limits.WallClock == 0 {
		limits.WallClock = def.WallClock
	}
	if limits.ConfirmationTTL == 0 {
		limits.ConfirmationTTL = def.ConfirmationTTL
	}
	return &AgentLoop{invoker: invoker, registry: reg, messages: store, redactor: redactor, audit: NoopAuditWriter{}, model: model, limits: limits}
}

// WithConfirmationStore injects the confirmation persistence (enables workspace_write+ tools).
func (l *AgentLoop) WithConfirmationStore(s ConfirmationStore) *AgentLoop {
	l.confirmations = s
	return l
}

// WithAuditWriter injects the tool-invocation audit writer (defaults to no-op).
func (l *AgentLoop) WithAuditWriter(a AuditWriter) *AgentLoop {
	if a != nil {
		l.audit = a
	}
	return l
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

	return l.runLoop(ctx, actor, authorizer, conv, pmsgs, result, 0, sink)
}

// runLoop drives the model/tool iteration over an already-built provider message list. Shared by
// RunTurn (fresh user turn) and ResumeConfirmedTurn (after a confirmation resolves). toolCalls is
// the running tool-call count carried across the resume boundary.
func (l *AgentLoop) runLoop(ctx context.Context, actor Actor, authorizer WorkspaceAuthorizer, conv Conversation, pmsgs []provider.Message, result TurnResult, toolCalls int, sink EventSink) (TurnResult, error) {
	defs := l.registry.Definitions(conv.Phase)
	steps := 0

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
		assistantMsg, err := l.messages.Append(ctx, Message{
			ConversationID: conv.ID,
			Role:           RoleAssistant,
			Content:        resp.OutputText,
			RedactionState: RedactionNone,
			ToolCalls:      toolCallsRaw,
			Usage:          usageJSON(resp.Usage),
		})
		if err != nil {
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

		// Confirmation gating (#875 §5.3): if the assistant batch contains any workspace_write+
		// tool call, defer the WHOLE batch — execute none, propose one confirmation for the first
		// such call, end the turn. The batch is executed atomically on resume so tool_use/
		// tool_result pairing stays valid for siblings before and after the confirmed call.
		if idx := l.firstConfirmationTierCall(resp.ToolCalls); idx >= 0 {
			tc := resp.ToolCalls[idx]
			tool, _ := l.registry.Resolve(tc.Name)
			// Authorize the action BEFORE proposing — never show an approval card for an action the
			// caller cannot perform. The api endpoint re-authorizes on resolve (defense in depth).
			if err := authorizer.Authorize(ctx, conv.WorkspaceID, tool.RequiredAction()); err != nil {
				// Forbidden: propose nothing; block the whole batch with synthetic results (history
				// stays paired), then let the model react on the next step.
				for _, btc := range resp.ToolCalls {
					action, tier := l.actionAndTier(btc.Name)
					content, reason := "not executed because a required confirmation is not authorized", "confirmation_forbidden"
					if btc.ID == tc.ID {
						content = "error: not authorized for " + btc.Name
					}
					pmsgs = append(pmsgs, l.appendSyntheticToolResult(ctx, conv, actor, &assistantMsg.ID, nil, btc, action, tier, content, AuditOutcomeError, reason, sink))
				}
				continue
			}
			pc, err := l.requireConfirmation(ctx, actor, conv, assistantMsg.ID, tc)
			if err != nil {
				sink(Event{Type: EventError, Text: err.Error()})
				result.StopReason = "error"
				return result, err
			}
			result.StopReason = "confirmation_required"
			result.PendingConfirmation = &pc
			sink(Event{Type: EventConfirmationRequired, ToolName: pc.ToolName, ToolCallID: pc.ToolCallID, ConfirmationID: pc.ID.String(), Action: pc.Action, Summary: pc.Summary, PayloadHash: pc.PayloadHash, ExpiresAt: pc.ExpiresAt.UTC().Format(time.RFC3339)})
			return l.finish(result, sink), nil
		}

		for _, tc := range resp.ToolCalls {
			if toolCalls >= l.limits.MaxToolCalls {
				result.StopReason = "limit"
				return l.finish(result, sink), nil
			}
			toolCalls++
			rec, toolMsg := l.executeTool(ctx, actor, authorizer, conv, tc, &assistantMsg.ID, nil, sink)
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

// firstConfirmationTierCall returns the index of the first resolved, confirmation-tier tool call in
// the batch, or -1. Unknown/unloaded tool names are not confirmation-tier (executeTool errors them).
func (l *AgentLoop) firstConfirmationTierCall(calls []provider.ToolCall) int {
	for i, tc := range calls {
		if tool, ok := l.registry.Resolve(tc.Name); ok && requiresConfirmation(tool.RiskTier()) {
			return i
		}
	}
	return -1
}

// requireConfirmation persists a pending confirmation for one workspace_write+ tool call and audits
// the propose as outcome=confirmation_required. The caller (runLoop) has already authorized the
// action before reaching here, so a card is never shown for a forbidden action; the api endpoint
// re-authorizes again on resolve (defense in depth).
func (l *AgentLoop) requireConfirmation(ctx context.Context, actor Actor, conv Conversation, messageID uuid.UUID, tc provider.ToolCall) (PendingConfirmation, error) {
	if l.confirmations == nil {
		return PendingConfirmation{}, fmt.Errorf("confirmation store not configured for tool %s", tc.Name)
	}
	tool, _ := l.registry.Resolve(tc.Name)
	mid := messageID
	hash := payloadHash(tc.Name, tc.Arguments)
	pc, err := l.confirmations.Create(ctx, NewPendingConfirmation{
		OrganizationID:   conv.OrganizationID,
		WorkspaceID:      conv.WorkspaceID,
		ConversationID:   conv.ID,
		MessageID:        &mid,
		ProposedByUserID: actor.UserID,
		ToolName:         tc.Name,
		ToolCallID:       tc.ID,
		Action:           tool.RequiredAction(),
		RiskTier:         tool.RiskTier(),
		PayloadHash:      hash,
		BoundArgs:        append(json.RawMessage(nil), tc.Arguments...),
		Summary:          confirmationSummary(tool),
		ExpiresAt:        time.Now().Add(l.limits.ConfirmationTTL),
	})
	if err != nil {
		return PendingConfirmation{}, err
	}
	l.auditInvocation(ctx, conv, actor, &mid, &pc.ID, tc.Name, tool.RequiredAction(), tool.RiskTier(), hash, tc.Arguments, nil, AuditOutcomeConfirmationRequired)
	return pc, nil
}

func confirmationSummary(tool Tool) string {
	return "Confirm " + tool.Name() + " (" + string(tool.RiskTier()) + ")"
}

// executeTool resolves, authorizes, runs, redacts, and persists one tool call. It returns the
// audit record and the provider tool-result message to append. A draft+ tier call writes one
// audit row (outcome ok/error) regardless of exit path; messageID links the assistant tool-call
// row and confirmationID is set when this call was resolved from a confirmation.
func (l *AgentLoop) executeTool(ctx context.Context, actor Actor, authorizer WorkspaceAuthorizer, conv Conversation, tc provider.ToolCall, messageID, confirmationID *uuid.UUID, sink EventSink) (rec ToolInvocationRecord, msg provider.Message) {
	rec = ToolInvocationRecord{ToolName: tc.Name}
	sink(Event{Type: EventToolCall, ToolName: tc.Name, ToolCallID: tc.ID})

	outcome := AuditOutcomeError
	var resultMeta json.RawMessage
	defer func() {
		if requiresAudit(rec.RiskTier) {
			l.auditInvocation(ctx, conv, actor, messageID, confirmationID, tc.Name, rec.Action, rec.RiskTier, payloadHash(tc.Name, tc.Arguments), tc.Arguments, resultMeta, outcome)
		}
	}()

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
	outcome = AuditOutcomeOK
	resultMeta = auditResultJSON(out.AuditResult)
	return rec, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: evidence}
}

// requiresAudit reports whether a tool call writes an audit row: every draft+ tier call. read-tier
// calls are audited only when they touch sensitive evidence (Phase 0) — a later refinement.
func requiresAudit(tier RiskTier) bool {
	return tier == DraftTier || requiresConfirmation(tier)
}

// auditInvocation appends one metadata-only audit row, best-effort (audit failure never fails the
// turn). request is the raw tool args (the AuditWriter scrubs); result is metadata-only.
func (l *AgentLoop) auditInvocation(ctx context.Context, conv Conversation, actor Actor, messageID, confirmationID *uuid.UUID, toolName, action string, tier RiskTier, hash string, request, result json.RawMessage, outcome string) {
	_ = l.audit.Append(ctx, ToolInvocationAudit{
		OrganizationID: conv.OrganizationID,
		WorkspaceID:    conv.WorkspaceID,
		ConversationID: conv.ID,
		MessageID:      messageID,
		Actor:          actor,
		ToolName:       toolName,
		Action:         action,
		RiskTier:       tier,
		PayloadHash:    hash,
		ConfirmationID: confirmationID,
		RequestPayload: request,
		ResultPayload:  result,
		Outcome:        outcome,
	})
}

func auditResultJSON(meta map[string]any) json.RawMessage {
	if len(meta) == 0 {
		return nil
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return nil
	}
	return b
}

// ResumeConfirmedTurn continues a turn that was suspended for a confirmation. pc is the ALREADY-
// resolved confirmation (the api manager performed the atomic Approve/Deny + crash-safe re-entry
// before calling this). The whole deferred assistant batch is resolved here so tool_use/
// tool_result pairing stays valid, then the model loop continues (Step 3b, end+resume).
func (l *AgentLoop) ResumeConfirmedTurn(ctx context.Context, actor Actor, authorizer WorkspaceAuthorizer, conv Conversation, pc PendingConfirmation, approve bool, sink EventSink) (TurnResult, error) {
	if sink == nil {
		sink = func(Event) {}
	}
	var result TurnResult

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

	batch, ok := l.deferredBatch(history, pc)
	if !ok {
		return result, fmt.Errorf("deferred assistant batch for confirmation %s not found", pc.ID)
	}

	// Validate the confirmed call is present (and is the recorded tool) BEFORE executing anything,
	// so a stale/corrupt confirmation never causes siblings to run without resolving the action.
	if err := assertConfirmedCallPresent(batch, pc); err != nil {
		return result, err
	}

	toolCalls := 0
	for _, tc := range batch {
		var toolMsg provider.Message
		switch {
		case tc.ID == pc.ToolCallID:
			// The confirmed call: execute with the BOUND args (authoritative), or deny.
			if approve {
				bound := provider.ToolCall{ID: pc.ToolCallID, Name: pc.ToolName, Arguments: pc.BoundArgs}
				var rec ToolInvocationRecord
				rec, toolMsg = l.executeTool(ctx, actor, authorizer, conv, bound, pc.MessageID, &pc.ID, sink)
				result.ToolInvocations = append(result.ToolInvocations, rec)
				toolCalls++
				// Surface finalization failures: a swallowed MarkSucceeded leaves the row 'executing',
				// which the retry path would treat as resumable and could re-run the effect.
				if rec.OK {
					if merr := l.confirmations.MarkSucceeded(ctx, pc.ID); merr != nil {
						return result, fmt.Errorf("finalize confirmation %s succeeded: %w", pc.ID, merr)
					}
				} else {
					if merr := l.confirmations.MarkFailed(ctx, pc.ID); merr != nil {
						return result, fmt.Errorf("finalize confirmation %s failed: %w", pc.ID, merr)
					}
				}
			} else {
				toolMsg = l.appendSyntheticToolResult(ctx, conv, actor, pc.MessageID, &pc.ID, tc, pc.Action, pc.RiskTier,
					"confirmation denied by user", AuditOutcomeDenied, "denied_by_user", sink)
			}
		case l.isConfirmationTierName(tc.Name):
			// An additional confirmation-tier sibling: never executed — one confirmation per turn.
			action, tier := l.actionAndTier(tc.Name)
			content, reason := "not executed: only one confirmation can be resolved per turn; please re-propose this action", "one_confirmation_per_turn"
			if !approve {
				content, reason = "not executed because the confirmation batch was denied", "batch_denied"
			}
			toolMsg = l.appendSyntheticToolResult(ctx, conv, actor, pc.MessageID, nil, tc, action, tier, content, AuditOutcomeError, reason, sink)
		default:
			// read/draft sibling: execute on approve, synthetic on deny.
			if approve {
				var rec ToolInvocationRecord
				rec, toolMsg = l.executeTool(ctx, actor, authorizer, conv, tc, pc.MessageID, nil, sink)
				result.ToolInvocations = append(result.ToolInvocations, rec)
				toolCalls++
			} else {
				action, tier := l.actionAndTier(tc.Name)
				toolMsg = l.appendSyntheticToolResult(ctx, conv, actor, pc.MessageID, nil, tc, action, tier,
					"not executed because the confirmation batch was denied", AuditOutcomeError, "batch_denied", sink)
			}
		}
		pmsgs = append(pmsgs, toolMsg)
	}

	return l.runLoop(ctx, actor, authorizer, conv, pmsgs, result, toolCalls, sink)
}

// assertConfirmedCallPresent verifies the deferred batch contains exactly the recorded confirmed
// call (id + tool name), so a stale/corrupt confirmation never resolves against the wrong call or
// silently runs siblings without resolving the action.
func assertConfirmedCallPresent(batch []provider.ToolCall, pc PendingConfirmation) error {
	for _, tc := range batch {
		if tc.ID == pc.ToolCallID {
			if tc.Name != pc.ToolName {
				return fmt.Errorf("confirmation %s tool %q does not match batch call %q", pc.ID, pc.ToolName, tc.Name)
			}
			return nil
		}
	}
	return fmt.Errorf("confirmation %s tool_call %q not found in deferred batch", pc.ID, pc.ToolCallID)
}

// ContinueTurn replays the conversation and continues the model loop WITHOUT appending a user
// message or executing any deferred batch. The api manager uses it for the crash-safe finalize
// path: a confirmed effect already executed (its tool-result is persisted) but finalization
// failed; on retry we must NOT re-execute — just let the model respond over the existing history.
func (l *AgentLoop) ContinueTurn(ctx context.Context, actor Actor, authorizer WorkspaceAuthorizer, conv Conversation, sink EventSink) (TurnResult, error) {
	if sink == nil {
		sink = func(Event) {}
	}
	var result TurnResult
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
	return l.runLoop(ctx, actor, authorizer, conv, pmsgs, result, 0, sink)
}

// deferredBatch returns the tool-call array of the assistant message that proposed pc.
func (l *AgentLoop) deferredBatch(history []Message, pc PendingConfirmation) ([]provider.ToolCall, bool) {
	if pc.MessageID == nil {
		return nil, false
	}
	for _, m := range history {
		if m.Role == RoleAssistant && m.ID == *pc.MessageID {
			var calls []provider.ToolCall
			if len(m.ToolCalls) > 0 {
				if err := json.Unmarshal(m.ToolCalls, &calls); err != nil {
					return nil, false
				}
			}
			return calls, true
		}
	}
	return nil, false
}

func (l *AgentLoop) isConfirmationTierName(name string) bool {
	tool, ok := l.registry.Resolve(name)
	return ok && requiresConfirmation(tool.RiskTier())
}

func (l *AgentLoop) actionAndTier(name string) (string, RiskTier) {
	if tool, ok := l.registry.Resolve(name); ok {
		return tool.RequiredAction(), tool.RiskTier()
	}
	return "", RiskTier("")
}

// appendSyntheticToolResult appends a system-generated tool-result (not untrusted evidence) to keep
// tool_use/tool_result pairing valid for a call that was deferred/denied/superseded, audits it for
// draft+ tiers, and returns the provider message (marked as an error result).
func (l *AgentLoop) appendSyntheticToolResult(ctx context.Context, conv Conversation, actor Actor, messageID, confirmationID *uuid.UUID, tc provider.ToolCall, action string, tier RiskTier, content, outcome, reason string, sink EventSink) provider.Message {
	sink(Event{Type: EventToolResult, ToolName: tc.Name, ToolCallID: tc.ID})
	_, _ = l.messages.Append(ctx, Message{ConversationID: conv.ID, Role: RoleTool, Content: content, RedactionState: RedactionNotApplicable, ToolCallID: tc.ID, ToolName: tc.Name})
	if requiresAudit(tier) {
		reasonMeta, _ := json.Marshal(map[string]string{"reason": reason})
		l.auditInvocation(ctx, conv, actor, messageID, confirmationID, tc.Name, action, tier, payloadHash(tc.Name, tc.Arguments), tc.Arguments, reasonMeta, outcome)
	}
	return provider.Message{Role: "tool", ToolCallID: tc.ID, Content: content, IsError: true}
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
