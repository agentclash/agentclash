package api

import (
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"net/http"
	"strconv"
)

func (h *VibeHandler) deleteProject(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "sessionID")
	if err != nil {
		vibeError(w, err)
		return
	}
	revision, err := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
	if err != nil || revision < 0 {
		vibeError(w, &vibe.Fault{Code: "invalid_request", Message: "A valid project revision is required."})
		return
	}
	result, err := h.Service.Store.DeleteProject(r.Context(), actor, id, revision)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 202, result)
}
func (h *VibeHandler) projectDeletion(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "sessionID")
	if err != nil {
		vibeError(w, err)
		return
	}
	result, err := h.Service.Store.DeletionStatus(r.Context(), actor, id)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, result)
}
