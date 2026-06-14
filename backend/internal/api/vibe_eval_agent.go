package api

import (
	"context"
	"os"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibeeval"
	"github.com/google/uuid"
)

// This file wires the boundary-clean internal/vibeeval guide agent into the api layer
// (#875 §11.1): the agent core never imports api; identity, authorization, message
// persistence, and tools arrive through small interfaces that api implements here.

// VibeEvalGuideConfig is the AgentClash-owned, server-side model for the guide agent
// (Q4). The credential reference is a secret://-/env:// server ref — never BYOK.
type VibeEvalGuideConfig struct {
	ProviderKey         string
	Model               string
	CredentialReference string
}

// VibeEvalGuideConfigFromEnv reads VIBEEVAL_GUIDE_{PROVIDER_KEY,MODEL,CREDENTIAL_REFERENCE}.
// Returns ok=false when unset so the agent endpoints can be disabled (noop) like other
// optional services.
func VibeEvalGuideConfigFromEnv() (VibeEvalGuideConfig, bool) {
	cfg := VibeEvalGuideConfig{
		ProviderKey:         os.Getenv("VIBEEVAL_GUIDE_PROVIDER_KEY"),
		Model:               os.Getenv("VIBEEVAL_GUIDE_MODEL"),
		CredentialReference: os.Getenv("VIBEEVAL_GUIDE_CREDENTIAL_REFERENCE"),
	}
	ok := cfg.ProviderKey != "" && cfg.Model != "" && cfg.CredentialReference != ""
	return cfg, ok
}

// vibeEvalMessageStore adapts repository persistence to vibeeval.MessageStore.
type vibeEvalMessageStore struct {
	repo *repository.Repository
}

func (s vibeEvalMessageStore) Append(ctx context.Context, msg vibeeval.Message) (vibeeval.Message, error) {
	row, err := s.repo.AppendVibeEvalMessage(ctx, repository.AppendVibeEvalMessageParams{
		ConversationID: msg.ConversationID,
		Role:           msg.Role,
		Content:        msg.Content,
		RedactionState: msg.RedactionState,
		ToolCallID:     msg.ToolCallID,
		ToolName:       msg.ToolName,
		ToolArgs:       msg.ToolArgs,
		ToolCalls:      msg.ToolCalls,
		Usage:          msg.Usage,
	})
	if err != nil {
		return vibeeval.Message{}, err
	}
	return toVibeEvalMessage(row), nil
}

func (s vibeEvalMessageStore) History(ctx context.Context, conversationID uuid.UUID) ([]vibeeval.Message, error) {
	rows, err := s.repo.ListVibeEvalMessagesByConversationID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	out := make([]vibeeval.Message, 0, len(rows))
	for _, row := range rows {
		out = append(out, toVibeEvalMessage(row))
	}
	return out, nil
}

func toVibeEvalMessage(row repository.VibeEvalMessage) vibeeval.Message {
	return vibeeval.Message{
		ID:             row.ID,
		ConversationID: row.ConversationID,
		Seq:            row.Seq,
		Role:           row.Role,
		Content:        row.Content,
		RedactionState: row.RedactionState,
		ToolCallID:     row.ToolCallID,
		ToolName:       row.ToolName,
		ToolArgs:       row.ToolArgs,
		ToolCalls:      row.ToolCalls,
		Usage:          row.Usage,
		CreatedAt:      row.CreatedAt,
	}
}

// vibeEvalAuthorizer bridges a per-turn authenticated api.Caller to the
// vibeeval.WorkspaceAuthorizer contract. The guide loop calls Authorize with an action
// NAME (string); we map it to api.Action and run the authoritative
// AuthorizeWorkspaceAction. The action constants + role matrix stay in api (§11.1).
type vibeEvalAuthorizer struct {
	authorizer WorkspaceAuthorizer
	caller     Caller
}

func (a vibeEvalAuthorizer) Authorize(ctx context.Context, workspaceID uuid.UUID, action string) error {
	return AuthorizeWorkspaceAction(ctx, a.authorizer, a.caller, workspaceID, Action(action))
}
