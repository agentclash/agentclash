package api

import (
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
	"net/http"
)

func (h *VibeHandler) gradeReviews(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	if r.Method == http.MethodPost {
		var in vibe.GradeReviewInput
		if err = vibeBody(w, r, v.Anonymous, &in); err != nil {
			vibeError(w, err)
			return
		}
		review, e := h.Service.Store.AddGradeReview(r.Context(), v.Actor, v.ID, in)
		if e != nil {
			vibeError(w, e)
			return
		}
		vibeJSON(w, 201, review)
		return
	}
	var result *uuid.UUID
	if id := r.URL.Query().Get("result_id"); id != "" {
		parsed, e := uuid.Parse(id)
		if e != nil {
			vibeError(w, &vibe.Fault{Code: "invalid_request", Message: "Choose a valid result."})
			return
		}
		result = &parsed
	}
	reviews, err := h.Service.Store.GradeReviews(r.Context(), v.Actor, v.ID, result)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, reviews)
}
