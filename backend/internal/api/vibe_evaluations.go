package api

import (
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
	"net/http"
)

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
func (h *VibeHandler) createEvaluation(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	if !h.Service.Config.TwoDoor {
		vibeError(w, &vibe.Fault{Code: "hosted_disabled", Message: "The new entry flow is not enabled."})
		return
	}
	var input struct {
		ClientID uuid.UUID `json:"client_id"`
		Door     string    `json:"door"`
	}
	if err = vibeBody(w, r, v.Anonymous, &input); err != nil {
		vibeError(w, err)
		return
	}
	value, err := h.Service.Store.CreateEvaluation(r.Context(), v.Actor, v.ID, input.ClientID, input.Door)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 201, value)
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
