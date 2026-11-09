package api

import (
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"net/url"
)

func (h *VibeHandler) registerCanonicalResults(r chi.Router) {
	r.Get("/canonical-results/{kind}/{resultID}/projects", h.canonicalProjects)
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

func (h *VibeHandler) canonicalProjects(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	actor, err := h.actor(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "resultID")
	if err != nil {
		vibeError(w, err)
		return
	}
	rows, err := h.Service.Store.CanonicalProjects(r.Context(), actor, vibe.ResultRef{Kind: vibe.ResultKind(chi.URLParam(r, "kind")), ID: id})
	if err != nil {
		vibeError(w, err)
		return
	}
	links := []struct {
		Binding vibe.ProjectResultBinding `json:"binding"`
		URL     string                    `json:"url"`
	}{}
	for _, binding := range rows {
		target := &url.URL{Path: "/vibe-evals", RawQuery: url.Values{"session": {binding.SessionID.String()}, "agent": {binding.ArtifactID.String()}, "view": {"results"}, "run": {string(binding.ResultRef.Kind) + ":" + binding.ResultRef.ID.String()}}.Encode()}
		if base, e := url.Parse(h.FrontendURL); e == nil && (base.Scheme == "http" || base.Scheme == "https") && base.Host != "" {
			target = base.ResolveReference(target)
		}
		links = append(links, struct {
			Binding vibe.ProjectResultBinding `json:"binding"`
			URL     string                    `json:"url"`
		}{binding, target.String()})
	}
	vibeJSON(w, 200, links)
}
