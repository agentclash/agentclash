package vibeeval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// PendingConfirmation is a proposed workspace_write+ action awaiting user approval (#875 §5.3).
// Boundary-clean: no api types; identity flows via Actor, authorization via action string.
type PendingConfirmation struct {
	ID             uuid.UUID
	ConversationID uuid.UUID
	MessageID      *uuid.UUID // the assistant tool-call message that proposed this
	ToolName       string
	ToolCallID     string // provider tool_use id, for resume pairing
	Action         string
	RiskTier       RiskTier
	PayloadHash    string
	BoundArgs      json.RawMessage // exact args to execute on approve
	Summary        string
	Status         string
	ExpiresAt      time.Time
}

// NewPendingConfirmation is the input to ConfirmationStore.Create.
type NewPendingConfirmation struct {
	ConversationID   uuid.UUID
	MessageID        *uuid.UUID
	ProposedByUserID uuid.UUID
	ToolName         string
	ToolCallID       string
	Action           string
	RiskTier         RiskTier
	PayloadHash      string
	BoundArgs        json.RawMessage
	Summary          string
	ExpiresAt        time.Time
}

// ConfirmationStore persists and transitions pending confirmations. The api layer backs this with
// the repository's NAMED transition methods (Approve/Deny/MarkSucceeded/MarkFailed) — never the
// generic ones. Resolve atomicity and crash-safe re-entry live in the repo (Step 3a).
type ConfirmationStore interface {
	Create(ctx context.Context, nc NewPendingConfirmation) (PendingConfirmation, error)
	Approve(ctx context.Context, id uuid.UUID, presentedHash string, by Actor) (PendingConfirmation, error)
	Deny(ctx context.Context, id uuid.UUID, presentedHash string, by Actor) (PendingConfirmation, error)
	GetForResume(ctx context.Context, id uuid.UUID, presentedHash string) (PendingConfirmation, error)
	MarkSucceeded(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID) error
}

// Audit outcomes (mirror vibe_eval_tool_invocations.outcome). Nuance (e.g. one_confirmation_per_turn)
// goes in the result-payload metadata, not the outcome enum.
const (
	AuditOutcomeOK                   = "ok"
	AuditOutcomeDenied               = "denied"
	AuditOutcomeError                = "error"
	AuditOutcomeConfirmationRequired = "confirmation_required"
)

// ToolInvocationAudit is one metadata-only audit record (#875 §6). It never carries secrets or raw
// artifact contents; the backing AuditWriter scrubs request/result payloads to structured metadata.
type ToolInvocationAudit struct {
	ConversationID uuid.UUID
	MessageID      *uuid.UUID
	Actor          Actor
	ToolName       string
	Action         string
	RiskTier       RiskTier
	PayloadHash    string
	ConfirmationID *uuid.UUID
	RequestPayload json.RawMessage
	ResultPayload  json.RawMessage
	Outcome        string
}

// AuditWriter appends tool-invocation audit rows. The default is a no-op until the api adapter
// wires the repo-backed, scrubbing writer (Step 3b-2).
type AuditWriter interface {
	Append(ctx context.Context, a ToolInvocationAudit) error
}

// NoopAuditWriter discards audit rows.
type NoopAuditWriter struct{}

// Append discards the record.
func (NoopAuditWriter) Append(context.Context, ToolInvocationAudit) error { return nil }

// requiresConfirmation reports whether a tool's risk tier needs a propose→confirm step
// (workspace_write and above). read/draft execute inline (Phase 0 matrix).
func requiresConfirmation(tier RiskTier) bool {
	switch tier {
	case WorkspaceWriteTier, CostIncurringTier, AdminSensitiveTier, DestructiveTier:
		return true
	default:
		return false
	}
}

// payloadHash binds a confirmation to exactly the tool + normalized args shown to the user: sha256
// over canonical JSON, so re-ordered keys hash identically (anti bait-and-switch, #875 §5.3).
func payloadHash(toolName string, args json.RawMessage) string {
	h := sha256.Sum256(append([]byte(toolName+"\x00"), canonicalJSON(args)...))
	return hex.EncodeToString(h[:])
}

// canonicalJSON normalizes JSON so semantically-equal payloads hash identically (Go's json.Marshal
// emits object keys in sorted order). Non-JSON input is hashed verbatim.
func canonicalJSON(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("null")
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	b, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return b
}
