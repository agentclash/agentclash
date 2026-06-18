package api

import (
	"context"
	"encoding/json"

	"github.com/agentclash/agentclash/backend/internal/redaction"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibeeval"
	"github.com/google/uuid"
)

// vibeEvalConfirmationStore adapts the boundary-clean vibeeval.ConfirmationStore to the repository's
// NAMED transition methods only (Approve/Deny/MarkSucceeded/MarkFailed) — never the generic sqlc
// ones — so the engine path cannot move a confirmation to an illegal state.
type vibeEvalConfirmationStore struct {
	repo *repository.Repository
}

func (s vibeEvalConfirmationStore) Create(ctx context.Context, nc vibeeval.NewPendingConfirmation) (vibeeval.PendingConfirmation, error) {
	row, err := s.repo.CreateVibeEvalPendingConfirmation(ctx, repository.CreateVibeEvalPendingConfirmationParams{
		OrganizationID:   nc.OrganizationID,
		WorkspaceID:      nc.WorkspaceID,
		ConversationID:   nc.ConversationID,
		MessageID:        nc.MessageID,
		ProposedByUserID: nc.ProposedByUserID,
		ToolName:         nc.ToolName,
		ToolCallID:       nc.ToolCallID,
		Action:           nc.Action,
		RiskTier:         string(nc.RiskTier),
		PayloadHash:      nc.PayloadHash,
		BoundArgs:        nc.BoundArgs,
		Summary:          nc.Summary,
		Estimate:         nc.Estimate,
		ExpiresAt:        nc.ExpiresAt,
	})
	if err != nil {
		return vibeeval.PendingConfirmation{}, err
	}
	return toVibeevalPendingConfirmation(row), nil
}

func (s vibeEvalConfirmationStore) Approve(ctx context.Context, id uuid.UUID, presentedHash string, by vibeeval.Actor) (vibeeval.PendingConfirmation, error) {
	row, err := s.repo.ApproveVibeEvalPendingConfirmation(ctx, id, presentedHash, by.UserID)
	if err != nil {
		return vibeeval.PendingConfirmation{}, err
	}
	return toVibeevalPendingConfirmation(row), nil
}

func (s vibeEvalConfirmationStore) Deny(ctx context.Context, id uuid.UUID, presentedHash string, by vibeeval.Actor) (vibeeval.PendingConfirmation, error) {
	row, err := s.repo.DenyVibeEvalPendingConfirmation(ctx, id, presentedHash, by.UserID)
	if err != nil {
		return vibeeval.PendingConfirmation{}, err
	}
	return toVibeevalPendingConfirmation(row), nil
}

func (s vibeEvalConfirmationStore) GetForResume(ctx context.Context, id uuid.UUID, presentedHash string) (vibeeval.PendingConfirmation, error) {
	row, err := s.repo.GetVibeEvalPendingConfirmationForResume(ctx, id, presentedHash)
	if err != nil {
		return vibeeval.PendingConfirmation{}, err
	}
	return toVibeevalPendingConfirmation(row), nil
}

func (s vibeEvalConfirmationStore) MarkSucceeded(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.MarkVibeEvalPendingConfirmationSucceeded(ctx, id)
	return err
}

func (s vibeEvalConfirmationStore) MarkFailed(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.MarkVibeEvalPendingConfirmationFailed(ctx, id)
	return err
}

func toVibeevalPendingConfirmation(row repository.VibeEvalPendingConfirmation) vibeeval.PendingConfirmation {
	return vibeeval.PendingConfirmation{
		ID:             row.ID,
		ConversationID: row.ConversationID,
		MessageID:      row.MessageID,
		ToolName:       row.ToolName,
		ToolCallID:     row.ToolCallID,
		Action:         row.Action,
		RiskTier:       vibeeval.RiskTier(row.RiskTier),
		PayloadHash:    row.PayloadHash,
		BoundArgs:      row.BoundArgs,
		Summary:        row.Summary,
		Estimate:       row.Estimate,
		Status:         row.Status,
		ExpiresAt:      row.ExpiresAt,
	}
}

// vibeEvalAuditWriter persists tool-invocation audit rows, structured-scrubbing the request/result
// payloads (metadata only — never secrets/contents) before the repo write (#875 §6).
type vibeEvalAuditWriter struct {
	repo *repository.Repository
}

func (w vibeEvalAuditWriter) Append(ctx context.Context, a vibeeval.ToolInvocationAudit) error {
	_, err := w.repo.AppendVibeEvalToolInvocation(ctx, repository.AppendVibeEvalToolInvocationParams{
		OrganizationID: a.OrganizationID,
		WorkspaceID:    a.WorkspaceID,
		ConversationID: a.ConversationID,
		MessageID:      a.MessageID,
		ActorUserID:    a.Actor.UserID,
		ToolName:       a.ToolName,
		Action:         a.Action,
		RiskTier:       string(a.RiskTier),
		PayloadHash:    a.PayloadHash,
		ConfirmationID: a.ConfirmationID,
		RequestPayload: scrubStructuredJSON(a.RequestPayload),
		ResultPayload:  scrubStructuredJSON(a.ResultPayload),
		Outcome:        a.Outcome,
	})
	return err
}

// scrubStructuredJSON recursively scrubs known-secret-shaped strings out of a structured JSON value
// using the shared Step-1 redactor, so audit payloads keep their shape (ids/keys/state) while never
// persisting credentials or signed URLs.
func scrubStructuredJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		b, _ := json.Marshal(redaction.ScrubHeaderSecrets(string(raw)))
		return b
	}
	b, err := json.Marshal(scrubJSONValue(v))
	if err != nil {
		return raw
	}
	return b
}

func scrubJSONValue(v any) any {
	switch t := v.(type) {
	case string:
		return redaction.ScrubHeaderSecrets(t)
	case map[string]any:
		for k, val := range t {
			t[k] = scrubJSONValue(val)
		}
		return t
	case []any:
		for i, val := range t {
			t[i] = scrubJSONValue(val)
		}
		return t
	default:
		return v
	}
}
