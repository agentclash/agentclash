package api

import (
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"

	"net/http"
)

func (h *VibeHandler) listSessions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	archived := query.Get("archived")
	_, cookieErr := r.Cookie(h.cookieName())
	// A visitor has no private library until creating their first trial.
	// Existing identities and invalid filters still follow normal access checks.
	if r.Header.Get("Authorization") == "" && cookieErr == http.ErrNoCookie && query.Get("workspace") == "" &&
		(archived == "" || archived == "exclude" || archived == "only" || archived == "all") {
		vibeJSON(w, http.StatusOK, []vibe.Session{})
		return
	}
	actor, err := h.actor(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	workspace := uuid.Nil
	if value := r.URL.Query().Get("workspace"); value != "" {
		workspace, err = uuid.Parse(value)
		if err != nil {
			vibeError(w, &vibe.Fault{Code: "invalid_request", Message: "Choose a workspace."})
			return
		}
	}
	items, err := h.Service.Store.LibrarySessions(r.Context(), actor, r.URL.Query().Get("archived"), workspace)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, items)
}

func (h *VibeHandler) evaluations(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	values, err := h.Service.Store.Evaluations(r.Context(), v.Actor, v.ID)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, values)
}
func (h *VibeHandler) buildQuote(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	var input vibe.BuildQuoteRequest
	if err = vibeBody(w, r, v.Anonymous, &input); err != nil {
		vibeError(w, err)
		return
	}
	quote, err := h.Service.QuoteBuild(r.Context(), v.Actor, v.ID, input)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, quote)
}

func (h *VibeHandler) runQuote(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	var sub vibe.Submission
	if err = vibeBody(w, r, v.Anonymous, &sub); err != nil {
		vibeError(w, err)
		return
	}
	q, err := h.Service.QuoteRun(r.Context(), v.Actor, v.ID, sub)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, q)
}
