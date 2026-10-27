package api

import (
	"encoding/json"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"io"
	"mime"
	"net/http"
)

func (h *VibeHandler) reimport(w http.ResponseWriter, r *http.Request) {
	v, err := h.authorizedSession(r, true)
	if err != nil {
		vibeError(w, err)
		return
	}
	var in vibe.ReimportRequest
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 6*int64(vibe.LimitsFor(v.Anonymous).FileBytes)+4096))
	if err == nil {
		err = json.Unmarshal(b, &in)
	}
	if err != nil {
		vibeError(w, &vibe.Fault{Code: "invalid_import", Message: "Upload a bounded JSON evaluation export."})
		return
	}
	out, err := h.Service.Reimport(r.Context(), v.Actor, v.ID, in)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, out)
}

func (h *VibeHandler) metadata(w http.ResponseWriter, r *http.Request) {
	v, err := h.authorizedSession(r, false)
	if err != nil {
		vibeError(w, err)
		return
	}
	var in vibe.MetadataPatch
	if err = vibeBody(w, r, v.Anonymous, &in); err != nil {
		vibeError(w, err)
		return
	}
	out, err := h.Service.Store.PatchMetadata(r.Context(), v.Actor, v.ID, in)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, out)
}

func (h *VibeHandler) original(w http.ResponseWriter, r *http.Request) {
	v, err := h.authorizedSession(r, false)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "artifactID")
	if err != nil {
		vibeError(w, err)
		return
	}
	o, err := h.Service.Store.GetOriginal(r.Context(), v.Actor, v.ID, id)
	if err != nil {
		vibeError(w, err)
		return
	}
	w.Header().Set("Content-Type", o.MediaType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": o.Filename}))
	w.Header().Set("X-Content-SHA256", o.Hash)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Write(o.Content)
}

func (h *VibeHandler) prepareExport(w http.ResponseWriter, r *http.Request) {
	v, err := h.authorizedSession(r, false)
	if err != nil {
		vibeError(w, err)
		return
	}
	var in vibe.ExportRequest
	if err = vibeBody(w, r, v.Anonymous, &in); err != nil {
		vibeError(w, err)
		return
	}
	out, err := h.Service.Store.PrepareExport(r.Context(), v.Actor, v.ID, in)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, out)
}
func (h *VibeHandler) exportReview(w http.ResponseWriter, r *http.Request) {
	v, err := h.authorizedSession(r, false)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "reviewID")
	if err != nil {
		vibeError(w, err)
		return
	}
	out, err := h.Service.Store.ExportReview(r.Context(), v.Actor, v.ID, id, r.URL.Query().Get("kind"), r.URL.Query().Get("hash"))
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, out)
}

func (h *VibeHandler) advanced(w http.ResponseWriter, r *http.Request) {
	v, err := h.authorizedSession(r, false)
	if err != nil {
		vibeError(w, err)
		return
	}
	var in vibe.AdvancedRequest
	if err = vibeBody(w, r, v.Anonymous, &in); err != nil {
		vibeError(w, err)
		return
	}
	draft, err := h.Service.SaveAdvanced(r.Context(), v.Actor, v.ID, in)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, map[string]any{"draft_id": draft, "workspace_id": in.WorkspaceID, "artifact_id": in.ArtifactID})
}
func (h *VibeHandler) savedOriginal(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "draftID")
	if err != nil {
		vibeError(w, err)
		return
	}
	o, err := h.Service.Store.SavedOriginal(r.Context(), actor, id)
	if err != nil {
		vibeError(w, err)
		return
	}
	if r.URL.Query().Get("metadata") == "1" {
		vibeJSON(w, 200, o)
		return
	}
	w.Header().Set("Content-Type", o.MediaType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": o.Filename}))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Write(o.Content)
}

func (h *VibeHandler) caseMapping(w http.ResponseWriter, r *http.Request) {
	v, err := h.authorizedSession(r, false)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "artifactID")
	if err != nil {
		vibeError(w, err)
		return
	}
	p, err := h.Service.Store.PrepareExport(r.Context(), v.Actor, v.ID, vibe.ExportRequest{Scope: "selected", ArtifactID: id})
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, p.CaseMappings)
}
