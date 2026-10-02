package api

import (
	"net/http"

	"github.com/agentclash/agentclash/backend/internal/mutation"
	"github.com/agentclash/agentclash/backend/internal/vibe/inputs"
	"github.com/google/uuid"
)

func (h *VibeHandler) bindReference(w http.ResponseWriter, r *http.Request) {
	v, err := h.authorizedSession(r, true)
	if err != nil {
		vibeError(w, mutation.Reject(err))
		return
	}
	var request struct {
		Revision   int64          `json:"revision"`
		ArtifactID uuid.UUID      `json:"artifact_id"`
		Key        string         `json:"key"`
		Input      inputs.Binding `json:"input"`
	}
	if err = vibeBody(w, r, v.Anonymous, &request); err != nil {
		vibeError(w, mutation.Reject(err))
		return
	}
	if err = h.Service.BindReference(r.Context(), v.Actor, v.ID, request.Revision, request.ArtifactID, request.Key, request.Input); err != nil {
		vibeError(w, err)
		return
	}
	h.get(w, r)
}
