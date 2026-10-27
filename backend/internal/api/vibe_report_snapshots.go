package api

import (
	"encoding/json"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/backend/internal/vibe/access"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"io"
	"net/http"
	"strings"
)

func (h *VibeHandler) registerReportSnapshots(r chi.Router) {
	r.Post("/sessions/{sessionID}/report-snapshots/preview", h.prepareReportSnapshot)
	r.Post("/sessions/{sessionID}/report-snapshots", h.prepareReportSnapshot)
	r.Get("/workspaces/{workspaceID}/report-snapshots", h.workspaceReportSnapshots)
	r.Get("/workspaces/{workspaceID}/report-snapshots/{snapshotID}", h.workspaceReportSnapshots)
	r.Get("/workspaces/{workspaceID}/report-snapshots/{snapshotID}/notes", h.workspaceReportNotes)
	r.Post("/workspaces/{workspaceID}/report-snapshots/{snapshotID}/notes", h.workspaceReportNotes)
	r.Get("/workspaces/{workspaceID}/report-snapshots/{snapshotID}/links", h.workspaceReportLinks)
}
func reportBody(w http.ResponseWriter, r *http.Request, out any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return &vibe.Fault{Code: "invalid_request", Message: "Use the selected report request fields."}
	}
	if d.Decode(new(any)) != io.EOF {
		return &vibe.Fault{Code: "invalid_request", Message: "Use one JSON request."}
	}
	return nil
}
func (h *VibeHandler) prepareReportSnapshot(w http.ResponseWriter, r *http.Request) {
	v, e := h.authorizedSession(r, true)
	if e != nil {
		vibeError(w, e)
		return
	}
	var in vibe.ReportSelection
	if e = reportBody(w, r, &in); e != nil {
		vibeError(w, e)
		return
	}
	create := !strings.HasSuffix(r.URL.Path, "/preview")
	out, e := h.Service.Store.PrepareReportSnapshot(r.Context(), v.Actor, v.ID, in, create)
	if e != nil {
		vibeError(w, e)
		return
	}
	status := 200
	if create {
		status = 201
	}
	vibeJSON(w, status, out)
}
func (h *VibeHandler) reportActor(r *http.Request) (string, uuid.UUID, uuid.UUID, error) {
	actor, e := h.actor(r)
	if e != nil {
		return "", uuid.Nil, uuid.Nil, e
	}
	ws, e := vibeID(r, "workspaceID")
	if e != nil {
		return "", ws, uuid.Nil, e
	}
	id := uuid.Nil
	if chi.URLParam(r, "snapshotID") != "" {
		id, e = vibeID(r, "snapshotID")
	}
	return actor, ws, id, e
}
func (h *VibeHandler) workspaceReportSnapshots(w http.ResponseWriter, r *http.Request) {
	actor, ws, id, e := h.reportActor(r)
	if e != nil {
		vibeError(w, e)
		return
	}
	if id == uuid.Nil {
		out, e := h.Service.Store.ReportSnapshots(r.Context(), actor, ws)
		if e != nil {
			vibeError(w, e)
			return
		}
		vibeJSON(w, 200, out)
		return
	}
	out, e := h.Service.Store.ReportSnapshot(r.Context(), actor, ws, id)
	if e != nil {
		vibeError(w, e)
		return
	}
	vibeJSON(w, 200, out)
}
func (h *VibeHandler) workspaceReportNotes(w http.ResponseWriter, r *http.Request) {
	actor, ws, id, e := h.reportActor(r)
	if e != nil {
		vibeError(w, e)
		return
	}
	if r.Method == "POST" {
		var in vibe.ReportNoteInput
		if e = reportBody(w, r, &in); e != nil {
			vibeError(w, e)
			return
		}
		out, e := h.Service.Store.AddReportNote(r.Context(), actor, ws, id, in)
		if e != nil {
			vibeError(w, e)
			return
		}
		vibeJSON(w, 201, out)
		return
	}
	out, e := h.Service.Store.ReportNotes(r.Context(), actor, ws, id)
	if e != nil {
		vibeError(w, e)
		return
	}
	vibeJSON(w, 200, out)
}
func (h *VibeHandler) workspaceReportLinks(w http.ResponseWriter, r *http.Request) {
	actor, ws, id, e := h.reportActor(r)
	if e != nil {
		vibeError(w, e)
		return
	}
	if e = access.Authorize(r.Context(), h.Service.Store.DB, actor, &ws, true); e != nil {
		vibeError(w, &vibe.Fault{Code: "forbidden", Message: "Members and admins manage report links."})
		return
	}
	if _, e = h.Service.Store.ReportSnapshot(r.Context(), actor, ws, id); e != nil {
		vibeError(w, e)
		return
	}
	rows, e := h.Service.Store.DB.Query(r.Context(), `SELECT id,key,expires_at,revoked_at,is_active FROM public_share_links WHERE resource_type='report_snapshot' AND resource_id=$1 AND workspace_id=$2 ORDER BY created_at DESC LIMIT 100`, id, ws)
	if e != nil {
		vibeError(w, e)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id uuid.UUID
		var key string
		var expires, revoked any
		var active bool
		if e = rows.Scan(&id, &key, &expires, &revoked, &active); e != nil {
			vibeError(w, e)
			return
		}
		out = append(out, map[string]any{"id": id, "key": key, "expires_at": expires, "revoked_at": revoked, "is_active": active})
	}
	if e = rows.Err(); e != nil {
		vibeError(w, e)
		return
	}
	vibeJSON(w, 200, out)
}
