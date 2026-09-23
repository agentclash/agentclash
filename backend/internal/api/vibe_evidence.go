package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
)

func (h *VibeHandler) addEvidence(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	var input vibe.EvidenceInput
	if err = decodeEvidenceInput(w, r, v.Anonymous, &input); err != nil {
		vibeError(w, err)
		return
	}
	if err = h.Service.AddEvidence(r.Context(), v.Actor, v.ID, input); err != nil {
		vibeError(w, err)
		return
	}
	h.get(w, r)
}

func decodeEvidenceInput(w http.ResponseWriter, r *http.Request, anonymous bool, input *vibe.EvidenceInput) error {
	invalid := &vibe.Fault{Code: "invalid_evidence", Message: "The chat upload is invalid or exceeds the size limit. Send one uncompressed JSON request."}
	if (r.Header.Get("Content-Encoding") != "" && r.Header.Get("Content-Encoding") != "identity") || strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return invalid
	}
	l := vibe.LimitsFor(anonymous)
	// The attachment envelope can contain a whole transcript; ordinary chat
	// message limits stay unchanged. Strict decoding preserves source text.
	l.StringBytes = l.FileBytes
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(l.FileBytes)))
	if err != nil {
		return invalid
	}
	if err = vibe.Decode(b, l, input); err != nil {
		return invalid
	}
	return nil
}

func (h *VibeHandler) saveCheck(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	var input struct {
		Revision    int64     `json:"revision"`
		WorkspaceID uuid.UUID `json:"workspace_id"`
		BaselineID  uuid.UUID `json:"baseline_operation_id"`
	}
	if err = vibeBody(w, r, v.Anonymous, &input); err != nil {
		vibeError(w, err)
		return
	}
	saved, err := h.Service.Store.SaveCheck(r.Context(), v.Actor, v.ID, input.Revision, input.WorkspaceID, input.BaselineID)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, saved)
}
func (h *VibeHandler) savedChecks(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	ws := uuid.Nil
	if value := r.URL.Query().Get("workspace"); value != "" {
		ws, err = uuid.Parse(value)
	}
	if err != nil {
		vibeError(w, &vibe.Fault{Code: "invalid_request", Message: "Choose a workspace."})
		return
	}
	items, err := h.Service.Store.ListChecks(r.Context(), actor, ws)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, items)
}

func (h *VibeHandler) saveBrief(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	var input struct {
		Revision    int64     `json:"revision"`
		WorkspaceID uuid.UUID `json:"workspace_id"`
		ArtifactID  uuid.UUID `json:"artifact_id"`
	}
	if err = vibeBody(w, r, v.Anonymous, &input); err != nil {
		vibeError(w, err)
		return
	}
	saved, err := h.Service.Store.SaveBrief(r.Context(), v.Actor, v.ID, input.Revision, input.WorkspaceID, input.ArtifactID)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, http.StatusOK, saved)
}
