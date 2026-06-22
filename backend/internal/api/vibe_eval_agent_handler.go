package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	billingpkg "github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibeeval"
	"github.com/google/uuid"
)

// createVibeEvalTurnHandler runs one bounded guide turn and streams its events over SSE.
// Authorization happens before the switch to SSE so a 403 returns as a normal HTTP error.
func createVibeEvalTurnHandler(logger *slog.Logger, mgr *VibeEvalAgentManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := vibeEvalCallerAndWorkspace(w, r)
		if !ok {
			return
		}
		conversationID, ok := parseVibeEvalURLUUID(w, "conversationID", "invalid_conversation_id", r)
		if !ok {
			return
		}
		var req struct {
			Message string `json:"message"`
		}
		if !decodeVibeEvalJSON(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.Message) == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "message is required")
			return
		}
		if err := mgr.AuthorizeTurn(r.Context(), caller, workspaceID); err != nil {
			writeAuthzError(w, err)
			return
		}
		// Validate the conversation BEFORE metering (4e): a missing or cross-workspace conversation must
		// NOT burn a guide-agent turn — it is not a genuine turn and makes no model call.
		if err := mgr.RequireConversation(r.Context(), workspaceID, conversationID); err != nil {
			handleVibeEvalError(w, logger, err)
			return
		}
		// Meter the guide-agent turn allowance BEFORE the SSE switch (4e): an exhausted allowance is a
		// clean 402 with no model call. A fresh user turn always consumes one.
		if err := mgr.MeterFreshTurn(r.Context(), workspaceID); err != nil {
			var gateErr billingpkg.GateError
			if errors.As(err, &gateErr) {
				writeBillingGateError(w, gateErr.Decision)
				return
			}
			handleVibeEvalError(w, logger, err)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "streaming_unsupported", "streaming is not supported")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		writeFrame := func(eventType string, payload any) {
			data, err := json.Marshal(payload)
			if err != nil {
				return
			}
			fmt.Fprintf(w, "event: %s\n", eventType)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		sink := func(e vibeeval.Event) { writeFrame(string(e.Type), e) }

		if _, err := mgr.RunTurn(r.Context(), caller, workspaceID, conversationID, req.Message, sink); err != nil {
			logger.Error("vibe eval turn failed", "error", err)
			// Headers are already sent; surface a generic error event (no internal detail).
			writeFrame(string(vibeeval.EventError), map[string]string{
				"type":  string(vibeeval.EventError),
				"error": "turn failed",
			})
		}
	}
}

// resolveVibeEvalConfirmationHandler approves or denies a pending confirmation and streams the
// continuation turn over SSE. Ownership + bound-action authorization happen BEFORE the SSE switch
// (so not-found/forbidden return as normal HTTP errors); the atomic resolve + execution stream.
func resolveVibeEvalConfirmationHandler(logger *slog.Logger, mgr *VibeEvalAgentManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := vibeEvalCallerAndWorkspace(w, r)
		if !ok {
			return
		}
		conversationID, ok := parseVibeEvalURLUUID(w, "conversationID", "invalid_conversation_id", r)
		if !ok {
			return
		}
		confirmationID, ok := parseVibeEvalURLUUID(w, "confirmationID", "invalid_confirmation_id", r)
		if !ok {
			return
		}
		var req struct {
			Approve     *bool  `json:"approve"`
			PayloadHash string `json:"payload_hash"`
		}
		if !decodeVibeEvalJSON(w, r, &req) {
			return
		}
		// approve must be explicit — a missing field must NOT silently deny this high-impact action.
		if req.Approve == nil {
			writeError(w, http.StatusBadRequest, "validation_error", "approve is required")
			return
		}
		if strings.TrimSpace(req.PayloadHash) == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "payload_hash is required")
			return
		}

		conv, pc, err := mgr.LoadConfirmationForResolve(r.Context(), caller, workspaceID, conversationID, confirmationID)
		if err != nil {
			handleVibeEvalError(w, logger, err)
			return
		}
		// Meter the guide-agent turn allowance BEFORE the SSE switch (4e): only a genuine fresh APPROVE
		// resume consumes a turn. Deny is always allowed and uncounted; an approve for a non-resolvable
		// confirmation is uncounted (no model call). An exhausted allowance is a clean 402.
		if err := mgr.MeterConfirmationResolve(r.Context(), workspaceID, pc, req.PayloadHash, *req.Approve); err != nil {
			var gateErr billingpkg.GateError
			if errors.As(err, &gateErr) {
				writeBillingGateError(w, gateErr.Decision)
				return
			}
			handleVibeEvalError(w, logger, err)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "streaming_unsupported", "streaming is not supported")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		writeFrame := func(eventType string, payload any) {
			data, err := json.Marshal(payload)
			if err != nil {
				return
			}
			fmt.Fprintf(w, "event: %s\n", eventType)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		sink := func(e vibeeval.Event) { writeFrame(string(e.Type), e) }

		if _, err := mgr.ResolveConfirmation(r.Context(), caller, conv, pc, *req.Approve, req.PayloadHash, sink); err != nil {
			// Headers already sent — surface as an error event. The not-resolvable case (already
			// resolved / expired / payload-hash mismatch) is a normal client outcome, not a 500.
			msg := "resolve failed"
			if errors.Is(err, repository.ErrVibeEvalConfirmationNotResolvable) {
				msg = "confirmation not resolvable (already resolved, expired, or payload hash mismatch)"
			} else {
				logger.Error("vibe eval confirmation resolve failed", "error", err)
			}
			writeFrame(string(vibeeval.EventError), map[string]string{"type": string(vibeeval.EventError), "error": msg})
		}
	}
}

type vibeEvalMessageResponse struct {
	ID             uuid.UUID `json:"id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Seq            int64     `json:"seq"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	RedactionState string    `json:"redaction_state"`
	ToolCallID     string    `json:"tool_call_id,omitempty"`
	ToolName       string    `json:"tool_name,omitempty"`
	CreatedAt      string    `json:"created_at"`
}

func listVibeEvalMessagesHandler(logger *slog.Logger, mgr *VibeEvalAgentManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := vibeEvalCallerAndWorkspace(w, r)
		if !ok {
			return
		}
		conversationID, ok := parseVibeEvalURLUUID(w, "conversationID", "invalid_conversation_id", r)
		if !ok {
			return
		}
		if err := mgr.AuthorizeRead(r.Context(), caller, workspaceID); err != nil {
			writeAuthzError(w, err)
			return
		}
		items, err := mgr.ListMessages(r.Context(), workspaceID, conversationID)
		if err != nil {
			handleVibeEvalError(w, logger, err)
			return
		}
		out := make([]vibeEvalMessageResponse, 0, len(items))
		for _, m := range items {
			out = append(out, mapVibeEvalMessageResponse(m))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": out})
	}
}

func mapVibeEvalMessageResponse(m repository.VibeEvalMessage) vibeEvalMessageResponse {
	return vibeEvalMessageResponse{
		ID:             m.ID,
		ConversationID: m.ConversationID,
		Seq:            m.Seq,
		Role:           m.Role,
		Content:        m.Content,
		RedactionState: m.RedactionState,
		ToolCallID:     m.ToolCallID,
		ToolName:       m.ToolName,
		CreatedAt:      m.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
