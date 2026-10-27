package api

import (
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
)

func (h *VibeHandler) registerCanonicalResults(r chi.Router) {
	r.Get("/sessions/{sessionID}/canonical-results", h.listCanonicalResults)
	r.Get("/sessions/{sessionID}/results/{kind}/{resultID}/observed-evidence", h.getObservedEvidence)
	r.Get("/sessions/{sessionID}/canonical-results/{kind}/{resultID}", h.getCanonicalResult)
}
func (h *VibeHandler) listCanonicalResults(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	v, e := h.authorizedSession(r, false)
	if e != nil {
		vibeError(w, e)
		return
	}
	rows, e := h.Service.Store.CanonicalResults(r.Context(), v.Actor, v.ID)
	if e != nil {
		vibeError(w, e)
		return
	}
	vibeJSON(w, 200, rows)
}
func (h *VibeHandler) getCanonicalResult(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	v, e := h.authorizedSession(r, false)
	if e != nil {
		vibeError(w, e)
		return
	}
	id, e := vibeID(r, "resultID")
	if e != nil {
		vibeError(w, e)
		return
	}
	var artifact *uuid.UUID
	if value := r.URL.Query().Get("artifact_id"); value != "" {
		a, err := uuid.Parse(value)
		if err != nil {
			vibeError(w, &vibe.Fault{Code: "invalid_request", Message: "Choose an exact test version."})
			return
		}
		artifact = &a
	}
	result, e := h.Service.Store.CanonicalResult(r.Context(), v.Actor, v.ID, vibe.ResultRef{Kind: vibe.ResultKind(chi.URLParam(r, "kind")), ID: id}, artifact)
	if e != nil {
		vibeError(w, e)
		return
	}
	if hash := r.URL.Query().Get("hash"); hash != "" && hash != result.EvidenceHash {
		vibeError(w, &vibe.Fault{Code: "evidence_changed", Message: "Canonical evidence changed after preparation. Prepare a fresh export."})
		return
	}
	vibeJSON(w, 200, result)
}

func (h *VibeHandler) getObservedEvidence(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	v, e := h.authorizedSession(r, false)
	if e != nil {
		vibeError(w, e)
		return
	}
	id, e := vibeID(r, "resultID")
	if e != nil {
		vibeError(w, e)
		return
	}
	artifact, e := uuid.Parse(r.URL.Query().Get("artifact_id"))
	if e != nil {
		vibeError(w, &vibe.Fault{Code: "invalid_request", Message: "Choose an exact version."})
		return
	}
	out, e := h.Service.Store.ObservedEvidence(r.Context(), v.Actor, v.ID, vibe.ResultRef{Kind: vibe.ResultKind(chi.URLParam(r, "kind")), ID: id}, &artifact)
	if e != nil {
		vibeError(w, e)
		return
	}
	if expected := r.URL.Query().Get("hash"); expected != "" && expected != out.EvidenceHash {
		vibeError(w, &vibe.Fault{Code: "evidence_changed", Message: "Observed evidence changed. Prepare the export again."})
		return
	}
	vibeJSON(w, http.StatusOK, out)
}
