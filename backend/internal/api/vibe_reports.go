package api

import (
	"encoding/json"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/go-chi/chi/v5"
	"io"
	"mime"
	"net/http"
	"strings"
)

// RegisterSDKReports is mounted by the existing Vibe router at integration.
// Its mutation body cap also applies when composed without the shell middleware.
func (h *VibeHandler) RegisterSDKReports(r chi.Router) {
	r.Post("/sessions/{sessionID}/reports", h.importSDKReport)
	r.Post("/sessions/{sessionID}/reports/{reportID}/promote", h.promoteSDKExpectation)
	r.Get("/sessions/{sessionID}/reports", h.listSDKReports)
	r.Get("/sessions/{sessionID}/reports/{reportID}", h.getSDKReport)
	r.Get("/sessions/{sessionID}/reports/{reportID}/original", h.originalSDKReport)
	r.Get("/sessions/{sessionID}/reports/{reportID}/case", h.sdkReportCase)
}
func (h *VibeHandler) importSDKReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	v, err := h.authorizedSession(r, true)
	if err != nil {
		vibeError(w, err)
		return
	}
	if r.Header.Get("Content-Encoding") != "" && r.Header.Get("Content-Encoding") != "identity" || strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		vibeError(w, &vibe.Fault{Code: "invalid_request", Message: "Upload an uncompressed JSON report envelope."})
		return
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, vibe.SDKReportRequestBytes))
	if err != nil {
		vibeError(w, &vibe.Fault{Code: "invalid_import", Message: "SDK report envelope exceeds its upload limit."})
		return
	}
	report, err := h.Service.Store.ImportSDKReport(r.Context(), v.Actor, v.ID, b)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, http.StatusCreated, report)
}
func (h *VibeHandler) listSDKReports(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	v, err := h.authorizedSession(r, false)
	if err != nil {
		vibeError(w, err)
		return
	}
	reports, err := h.Service.Store.SDKReports(r.Context(), v.Actor, v.ID)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, reports)
}
func (h *VibeHandler) getSDKReport(w http.ResponseWriter, r *http.Request) {
	h.readSDKReport(w, r, false)
}
func (h *VibeHandler) originalSDKReport(w http.ResponseWriter, r *http.Request) {
	h.readSDKReport(w, r, true)
}
func (h *VibeHandler) readSDKReport(w http.ResponseWriter, r *http.Request, original bool) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	v, err := h.authorizedSession(r, false)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "reportID")
	if err != nil {
		vibeError(w, err)
		return
	}
	report, err := h.Service.Store.SDKReport(r.Context(), v.Actor, v.ID, id)
	if err != nil {
		vibeError(w, err)
		return
	}
	if hash := r.URL.Query().Get("hash"); hash != "" && hash != report.ContentHash {
		vibeError(w, &vibe.Fault{Code: "evidence_changed", Message: "SDK source identity differs from this selection."})
		return
	}
	if original {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "sdk-report-" + id.String() + ".json"}))
		w.Header().Set("X-Content-SHA256", report.ContentHash)
		_, _ = w.Write([]byte(report.SourceJSON))
		return
	}
	vibeJSON(w, 200, report)
}

func (h *VibeHandler) sdkReportCase(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	v, err := h.authorizedSession(r, false)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "reportID")
	if err != nil {
		vibeError(w, err)
		return
	}
	report, err := h.Service.Store.SDKReport(r.Context(), v.Actor, v.ID, id)
	if err != nil {
		vibeError(w, err)
		return
	}
	c, err := vibe.SDKReportCase(report, r.URL.Query().Get("source_case_id"), r.URL.Query().Get("key"))
	if err != nil {
		vibeError(w, err)
		return
	}
	b, _ := json.Marshal(c)
	c.EvidenceHash = vibe.Hash(b)
	vibeJSON(w, 200, c)
}

func (h *VibeHandler) promoteSDKExpectation(w http.ResponseWriter, r *http.Request) {
	v, err := h.authorizedSession(r, true)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "reportID")
	if err != nil {
		vibeError(w, err)
		return
	}
	var in vibe.SDKPromotion
	if err = vibeBody(w, r, false, &in); err != nil {
		vibeError(w, err)
		return
	}
	out, err := h.Service.PromoteSDKExpectation(r.Context(), v.Actor, v.ID, id, in)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, out)
}
