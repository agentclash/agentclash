package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

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
