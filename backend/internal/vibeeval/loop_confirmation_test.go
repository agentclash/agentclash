package vibeeval

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/google/uuid"
)

var errTest = errors.New("test error")

// --- confirmation/audit fakes ---

type fakeWriteTool struct {
	name    string
	action  string
	tier    RiskTier
	called  *bool
	result  any
	execErr error
}

func (t fakeWriteTool) Name() string           { return t.name }
func (t fakeWriteTool) Phases() []string       { return []string{PhasePlan} }
func (t fakeWriteTool) RiskTier() RiskTier     { return t.tier }
func (t fakeWriteTool) RequiredAction() string { return t.action }
func (t fakeWriteTool) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{Name: t.name}
}
func (t fakeWriteTool) Execute(context.Context, Actor, Conversation, json.RawMessage) (ToolOutput, error) {
	if t.called != nil {
		*t.called = true
	}
	if t.execErr != nil {
		return ToolOutput{}, t.execErr
	}
	return ToolOutput{Result: t.result, AuditResult: map[string]any{"ok": true}}, nil
}

type fakeConfirmationStore struct {
	created   []NewPendingConfirmation
	succeeded []uuid.UUID
	failed    []uuid.UUID
	markErr   error // when set, MarkSucceeded/MarkFailed return it (simulates a finalization failure)
}

func (s *fakeConfirmationStore) Create(_ context.Context, nc NewPendingConfirmation) (PendingConfirmation, error) {
	s.created = append(s.created, nc)
	return PendingConfirmation{
		ID:             uuid.New(),
		ConversationID: nc.ConversationID,
		MessageID:      nc.MessageID,
		ToolName:       nc.ToolName,
		ToolCallID:     nc.ToolCallID,
		Action:         nc.Action,
		RiskTier:       nc.RiskTier,
		PayloadHash:    nc.PayloadHash,
		BoundArgs:      nc.BoundArgs,
		Summary:        nc.Summary,
		Status:         "pending",
		ExpiresAt:      nc.ExpiresAt,
	}, nil
}

// The loop never calls Approve/Deny/GetForResume (the api manager does); included for the interface.
func (s *fakeConfirmationStore) Approve(context.Context, uuid.UUID, string, Actor) (PendingConfirmation, error) {
	return PendingConfirmation{}, nil
}
func (s *fakeConfirmationStore) Deny(context.Context, uuid.UUID, string, Actor) (PendingConfirmation, error) {
	return PendingConfirmation{}, nil
}
func (s *fakeConfirmationStore) GetForResume(context.Context, uuid.UUID, string) (PendingConfirmation, error) {
	return PendingConfirmation{}, nil
}
func (s *fakeConfirmationStore) MarkSucceeded(_ context.Context, id uuid.UUID) error {
	s.succeeded = append(s.succeeded, id)
	return s.markErr
}
func (s *fakeConfirmationStore) MarkFailed(_ context.Context, id uuid.UUID) error {
	s.failed = append(s.failed, id)
	return s.markErr
}

type fakeAuditWriter struct{ rows []ToolInvocationAudit }

func (a *fakeAuditWriter) Append(_ context.Context, r ToolInvocationAudit) error {
	a.rows = append(a.rows, r)
	return nil
}

func newConfirmLoop(inv modelInvoker, store MessageStore, reg ToolRegistry, cs ConfirmationStore, aw AuditWriter) *AgentLoop {
	return newLoop(inv, store, reg).WithConfirmationStore(cs).WithAuditWriter(aw)
}

func toolMsgContents(msgs []Message) []string {
	var out []string
	for _, m := range msgs {
		if m.Role == RoleTool {
			out = append(out, m.Content)
		}
	}
	return out
}

func auditOutcomes(rows []ToolInvocationAudit) map[string]int {
	m := map[string]int{}
	for _, r := range rows {
		m[r.Outcome]++
	}
	return m
}

// --- tests ---

func TestRunTurn_ConfirmationTierProposesAndStops(t *testing.T) {
	called := false
	reg := NewRegistry()
	reg.Register(fakeWriteTool{name: "publish_draft", action: "publish_challenge_pack", tier: WorkspaceWriteTier, called: &called, result: "published"})

	inv := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{{ID: "tc1", Name: "publish_draft", Arguments: json.RawMessage(`{"draft_id":"d1"}`)}}},
	}}
	store := &memStore{}
	cs := &fakeConfirmationStore{}
	aw := &fakeAuditWriter{}

	var events []Event
	res, err := newConfirmLoop(inv, store, reg, cs, aw).RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, conv(), "publish my draft", func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatalf("RunTurn error: %v", err)
	}
	if called {
		t.Fatal("workspace_write tool must NOT execute before confirmation")
	}
	if res.StopReason != "confirmation_required" {
		t.Fatalf("StopReason = %q, want confirmation_required", res.StopReason)
	}
	if res.PendingConfirmation == nil || res.PendingConfirmation.ToolName != "publish_draft" {
		t.Fatalf("PendingConfirmation = %+v, want publish_draft", res.PendingConfirmation)
	}
	if len(cs.created) != 1 || cs.created[0].PayloadHash == "" {
		t.Fatalf("expected one created confirmation with a payload hash, got %+v", cs.created)
	}
	// Only user + assistant persisted — NO tool-result yet.
	if len(store.msgs) != 2 || store.msgs[1].Role != RoleAssistant {
		t.Fatalf("persisted %v, want [user, assistant] only", roles(store.msgs))
	}
	if !hasEvent(events, EventConfirmationRequired) {
		t.Fatalf("missing confirmation.required event: %v", events)
	}
	if got := auditOutcomes(aw.rows); got[AuditOutcomeConfirmationRequired] != 1 {
		t.Fatalf("audit outcomes = %v, want one confirmation_required", got)
	}
}

func TestResumeConfirmedTurn_ApproveExecutesAndContinues(t *testing.T) {
	called := false
	reg := NewRegistry()
	reg.Register(fakeWriteTool{name: "publish_draft", action: "publish_challenge_pack", tier: WorkspaceWriteTier, called: &called, result: "published"})

	inv := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{{ID: "tc1", Name: "publish_draft", Arguments: json.RawMessage(`{"draft_id":"d1"}`)}}},
		{OutputText: "Published your draft."},
	}}
	store := &memStore{}
	cs := &fakeConfirmationStore{}
	aw := &fakeAuditWriter{}
	loop := newConfirmLoop(inv, store, reg, cs, aw)
	c := conv()

	res, err := loop.RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, "publish", nil)
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	pc := res.PendingConfirmation
	if pc == nil {
		t.Fatal("no pending confirmation")
	}

	res2, err := loop.ResumeConfirmedTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, *pc, true, nil)
	if err != nil {
		t.Fatalf("ResumeConfirmedTurn: %v", err)
	}
	if !called {
		t.Fatal("confirmed tool must execute on approve")
	}
	if res2.StopReason != "completed" || res2.AssistantText != "Published your draft." {
		t.Fatalf("resume result = %+v, want completed final text", res2)
	}
	if len(cs.succeeded) != 1 {
		t.Fatalf("MarkSucceeded calls = %d, want 1", len(cs.succeeded))
	}
	if got := auditOutcomes(aw.rows); got[AuditOutcomeOK] != 1 {
		t.Fatalf("audit outcomes = %v, want one ok for the executed publish", got)
	}
}

func TestResumeConfirmedTurn_DenyAppendsSyntheticAndContinues(t *testing.T) {
	called := false
	reg := NewRegistry()
	reg.Register(fakeWriteTool{name: "publish_draft", action: "publish_challenge_pack", tier: WorkspaceWriteTier, called: &called, result: "x"})

	inv := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{{ID: "tc1", Name: "publish_draft", Arguments: json.RawMessage(`{"draft_id":"d1"}`)}}},
		{OutputText: "Okay, I did not make any changes."},
	}}
	store := &memStore{}
	cs := &fakeConfirmationStore{}
	aw := &fakeAuditWriter{}
	loop := newConfirmLoop(inv, store, reg, cs, aw)
	c := conv()

	res, _ := loop.RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, "publish", nil)
	res2, err := loop.ResumeConfirmedTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, *res.PendingConfirmation, false, nil)
	if err != nil {
		t.Fatalf("ResumeConfirmedTurn deny: %v", err)
	}
	if called {
		t.Fatal("confirmed tool must NOT execute on deny")
	}
	if res2.AssistantText != "Okay, I did not make any changes." {
		t.Fatalf("AssistantText = %q", res2.AssistantText)
	}
	tools := toolMsgContents(store.msgs)
	if len(tools) != 1 || tools[0] != "confirmation denied by user" {
		t.Fatalf("tool messages = %v, want one synthetic denial", tools)
	}
	if got := auditOutcomes(aw.rows); got[AuditOutcomeDenied] != 1 {
		t.Fatalf("audit outcomes = %v, want one denied", got)
	}
}

// Codex-required: [read, publish, read] — initial turn creates one pending confirmation and NO tool
// results; approve appends all three results in order and continues.
func TestResumeConfirmedTurn_MixedBatchApproveExecutesAllInOrder(t *testing.T) {
	read1, pub, read2 := false, false, false
	reg := NewRegistry()
	reg.Register(fakeReadTool{name: "read_a", called: &read1, result: "a"})
	reg.Register(fakeWriteTool{name: "publish_draft", action: "publish_challenge_pack", tier: WorkspaceWriteTier, called: &pub, result: "p"})
	reg.Register(fakeReadTool{name: "read_b", called: &read2, result: "b"})

	inv := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{
			{ID: "r1", Name: "read_a", Arguments: json.RawMessage(`{}`)},
			{ID: "p1", Name: "publish_draft", Arguments: json.RawMessage(`{"draft_id":"d1"}`)},
			{ID: "r2", Name: "read_b", Arguments: json.RawMessage(`{}`)},
		}},
		{OutputText: "Done."},
	}}
	store := &memStore{}
	cs := &fakeConfirmationStore{}
	aw := &fakeAuditWriter{}
	loop := newConfirmLoop(inv, store, reg, cs, aw)
	c := conv()

	res, _ := loop.RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, "go", nil)
	if read1 || pub || read2 {
		t.Fatal("no tool in the batch may execute before confirmation")
	}
	if len(store.msgs) != 2 {
		t.Fatalf("persisted %v, want [user, assistant] only", roles(store.msgs))
	}
	if res.PendingConfirmation == nil || res.PendingConfirmation.ToolCallID != "p1" {
		t.Fatalf("pending confirmation = %+v, want for tool_call p1", res.PendingConfirmation)
	}

	if _, err := loop.ResumeConfirmedTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, *res.PendingConfirmation, true, nil); err != nil {
		t.Fatalf("resume approve: %v", err)
	}
	if !read1 || !pub || !read2 {
		t.Fatalf("approve must execute the whole batch: read_a=%v publish=%v read_b=%v", read1, pub, read2)
	}
	// tool-result order must match the original tool-call order r1, p1, r2.
	var order []string
	for _, m := range store.msgs {
		if m.Role == RoleTool {
			order = append(order, m.ToolCallID)
		}
	}
	if len(order) != 3 || order[0] != "r1" || order[1] != "p1" || order[2] != "r2" {
		t.Fatalf("tool-result order = %v, want [r1 p1 r2]", order)
	}
}

// Codex-required: [publish_a, publish_b] — one pending confirmation; approve executes publish_a and
// synthesizes a non-executed result for publish_b; deny synthesizes results for both.
func TestResumeConfirmedTurn_TwoConfirmTierOnePerTurn(t *testing.T) {
	a, b := false, false
	reg := NewRegistry()
	reg.Register(fakeWriteTool{name: "publish_a", action: "publish_challenge_pack", tier: WorkspaceWriteTier, called: &a, result: "a"})
	reg.Register(fakeWriteTool{name: "publish_b", action: "publish_challenge_pack", tier: WorkspaceWriteTier, called: &b, result: "b"})

	batch := provider.Response{ToolCalls: []provider.ToolCall{
		{ID: "pa", Name: "publish_a", Arguments: json.RawMessage(`{}`)},
		{ID: "pb", Name: "publish_b", Arguments: json.RawMessage(`{}`)},
	}}

	// Approve path.
	inv := &scriptedInvoker{responses: []provider.Response{batch, {OutputText: "done"}}}
	store := &memStore{}
	cs := &fakeConfirmationStore{}
	loop := newConfirmLoop(inv, store, reg, cs, &fakeAuditWriter{})
	c := conv()
	res, _ := loop.RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, "go", nil)
	if res.PendingConfirmation.ToolCallID != "pa" {
		t.Fatalf("pending confirmation tool_call = %q, want pa (first confirm-tier)", res.PendingConfirmation.ToolCallID)
	}
	if _, err := loop.ResumeConfirmedTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, *res.PendingConfirmation, true, nil); err != nil {
		t.Fatalf("resume approve: %v", err)
	}
	if !a || b {
		t.Fatalf("approve must execute publish_a only: a=%v b=%v", a, b)
	}

	// Deny path: neither executes; both get synthetic results.
	a, b = false, false
	inv2 := &scriptedInvoker{responses: []provider.Response{batch, {OutputText: "no changes"}}}
	store2 := &memStore{}
	loop2 := newConfirmLoop(inv2, store2, reg, &fakeConfirmationStore{}, &fakeAuditWriter{})
	c2 := conv()
	res2, _ := loop2.RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c2, "go", nil)
	if _, err := loop2.ResumeConfirmedTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c2, *res2.PendingConfirmation, false, nil); err != nil {
		t.Fatalf("resume deny: %v", err)
	}
	if a || b {
		t.Fatalf("deny must execute nothing: a=%v b=%v", a, b)
	}
	if got := len(toolMsgContents(store2.msgs)); got != 2 {
		t.Fatalf("deny synthetic tool messages = %d, want 2 (both calls paired)", got)
	}
}

// Finding 1: a forbidden confirmation-tier proposal creates NO pending row and leaves history
// paired (synthetic forbidden result), then the model continues.
func TestRunTurn_ForbiddenConfirmationTierProposesNothing(t *testing.T) {
	called := false
	reg := NewRegistry()
	reg.Register(fakeWriteTool{name: "publish_draft", action: "publish_challenge_pack", tier: WorkspaceWriteTier, called: &called, result: "x"})

	inv := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{{ID: "tc1", Name: "publish_draft", Arguments: json.RawMessage(`{}`)}}},
		{OutputText: "You are not allowed to publish here."},
	}}
	store := &memStore{}
	cs := &fakeConfirmationStore{}
	loop := newConfirmLoop(inv, store, reg, cs, &fakeAuditWriter{})

	res, err := loop.RunTurn(context.Background(), Actor{UserID: uuid.New()}, denyAll{}, conv(), "publish", nil)
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if called {
		t.Fatal("forbidden tool must not execute")
	}
	if len(cs.created) != 0 {
		t.Fatalf("forbidden action must not create a pending confirmation, got %d", len(cs.created))
	}
	if res.StopReason != "completed" {
		t.Fatalf("StopReason = %q, want completed (model reacted after the block)", res.StopReason)
	}
	if got := toolMsgContents(store.msgs); len(got) != 1 {
		t.Fatalf("want one synthetic tool result to keep history paired, got %v", got)
	}
}

// Finding 2/asks: confirmed Execute errors on resume → MarkFailed (not MarkSucceeded).
func TestResumeConfirmedTurn_ExecuteErrorMarksFailed(t *testing.T) {
	reg := NewRegistry()
	reg.Register(fakeWriteTool{name: "publish_draft", action: "publish_challenge_pack", tier: WorkspaceWriteTier, execErr: errTest})

	inv := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{{ID: "tc1", Name: "publish_draft", Arguments: json.RawMessage(`{}`)}}},
		{OutputText: "That failed."},
	}}
	store := &memStore{}
	cs := &fakeConfirmationStore{}
	loop := newConfirmLoop(inv, store, reg, cs, &fakeAuditWriter{})
	c := conv()

	res, _ := loop.RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, "publish", nil)
	if _, err := loop.ResumeConfirmedTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, *res.PendingConfirmation, true, nil); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if len(cs.failed) != 1 || len(cs.succeeded) != 0 {
		t.Fatalf("want MarkFailed once and no MarkSucceeded, got failed=%d succeeded=%d", len(cs.failed), len(cs.succeeded))
	}
}

// Finding 2: a MarkSucceeded error must be surfaced (not swallowed) so the api can retry rather
// than leave an 'executing' row that the retry path would re-enter.
func TestResumeConfirmedTurn_MarkErrorIsSurfaced(t *testing.T) {
	reg := NewRegistry()
	reg.Register(fakeWriteTool{name: "publish_draft", action: "publish_challenge_pack", tier: WorkspaceWriteTier, result: "ok"})

	inv := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{{ID: "tc1", Name: "publish_draft", Arguments: json.RawMessage(`{}`)}}},
		{OutputText: "done"},
	}}
	store := &memStore{}
	cs := &fakeConfirmationStore{markErr: errTest}
	loop := newConfirmLoop(inv, store, reg, cs, &fakeAuditWriter{})
	c := conv()

	res, _ := loop.RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, "publish", nil)
	if _, err := loop.ResumeConfirmedTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, *res.PendingConfirmation, true, nil); err == nil {
		t.Fatal("expected MarkSucceeded failure to be surfaced as an error")
	}
}

// Finding 3: a confirmation whose tool_call_id is absent from the deferred batch must error and
// not execute siblings.
func TestResumeConfirmedTurn_ConfirmedCallMissingErrors(t *testing.T) {
	sib := false
	reg := NewRegistry()
	reg.Register(fakeWriteTool{name: "publish_draft", action: "publish_challenge_pack", tier: WorkspaceWriteTier, result: "x"})
	reg.Register(fakeReadTool{name: "read_a", called: &sib, result: "a"})

	inv := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{
			{ID: "p1", Name: "publish_draft", Arguments: json.RawMessage(`{}`)},
			{ID: "r1", Name: "read_a", Arguments: json.RawMessage(`{}`)},
		}},
	}}
	store := &memStore{}
	loop := newConfirmLoop(inv, store, reg, &fakeConfirmationStore{}, &fakeAuditWriter{})
	c := conv()

	res, _ := loop.RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, "go", nil)
	stale := *res.PendingConfirmation
	stale.ToolCallID = "does-not-exist"
	if _, err := loop.ResumeConfirmedTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, c, stale, true, nil); err == nil {
		t.Fatal("expected error when the confirmed tool_call_id is missing from the batch")
	}
	if sib {
		t.Fatal("siblings must not execute when the confirmed call is absent")
	}
}

// Finding 4: the confirmation.required event carries expiry metadata.
func TestRunTurn_ConfirmationEventIncludesExpiry(t *testing.T) {
	reg := NewRegistry()
	reg.Register(fakeWriteTool{name: "publish_draft", action: "publish_challenge_pack", tier: WorkspaceWriteTier, result: "x"})
	inv := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{{ID: "tc1", Name: "publish_draft", Arguments: json.RawMessage(`{}`)}}},
	}}
	var events []Event
	_, _ = newConfirmLoop(inv, &memStore{}, reg, &fakeConfirmationStore{}, &fakeAuditWriter{}).
		RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, conv(), "publish", func(e Event) { events = append(events, e) })
	for _, e := range events {
		if e.Type == EventConfirmationRequired {
			if e.ExpiresAt == "" {
				t.Fatal("confirmation.required event missing expires_at")
			}
			return
		}
	}
	t.Fatal("no confirmation.required event emitted")
}

// Finding 5: partial AgentLimits must still yield a non-zero ConfirmationTTL (else a confirmation
// is born expired). Verified via the proposed ExpiresAt being well in the future.
func TestRunTurn_PartialLimitsStillGiveConfirmationTTL(t *testing.T) {
	reg := NewRegistry()
	reg.Register(fakeWriteTool{name: "publish_draft", action: "publish_challenge_pack", tier: WorkspaceWriteTier, result: "x"})
	inv := &scriptedInvoker{responses: []provider.Response{
		{ToolCalls: []provider.ToolCall{{ID: "tc1", Name: "publish_draft", Arguments: json.RawMessage(`{}`)}}},
	}}
	cs := &fakeConfirmationStore{}
	loop := NewAgentLoop(inv, reg, &memStore{}, NewEvidenceRedactor(),
		GuideModel{ProviderKey: "anthropic", Model: "claude-sonnet-4-6", CredentialReference: "secret://anthropic"},
		AgentLimits{MaxSteps: 3, MaxToolCalls: 5}).WithConfirmationStore(cs).WithAuditWriter(&fakeAuditWriter{})

	if _, err := loop.RunTurn(context.Background(), Actor{UserID: uuid.New()}, allowAll{}, conv(), "publish", nil); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cs.created) != 1 {
		t.Fatalf("want one created confirmation, got %d", len(cs.created))
	}
	if !cs.created[0].ExpiresAt.After(time.Now().Add(time.Minute)) {
		t.Fatalf("ExpiresAt = %v, want a non-zero TTL well in the future (ConfirmationTTL defaulted)", cs.created[0].ExpiresAt)
	}
}
