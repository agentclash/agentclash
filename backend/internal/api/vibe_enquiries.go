package api

import (
	"errors"
	"net/http"

	"github.com/agentclash/agentclash/backend/internal/enquiries"
	"github.com/agentclash/agentclash/backend/internal/vibe"
)

func (h *VibeHandler) createEnquiry(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	if h.Enquiries == nil {
		vibeError(w, &vibe.Fault{Code: "hosted_disabled", Message: enquiries.ErrDisabled.Error()})
		return
	}
	if err = h.Service.Store.Authorize(r.Context(), v, true); err != nil {
		vibeError(w, err)
		return
	}
	var input enquiries.Request
	if err = vibeBody(w, r, v.Anonymous, &input); err != nil {
		vibeError(w, err)
		return
	}
	// A receipt lookup precedes current-revision checks so a lost response is
	// recoverable after more chat messages or a configuration change.
	if _, err = h.Enquiries.Get(r.Context(), v.ID, input.ClientID); err != nil {
		if input.Source.Revision > v.Revision {
			vibeError(w, &vibe.Fault{Code: "revision_conflict", Message: "This project version is unavailable."})
			return
		}
		if input.Source.ArtifactID != nil {
			found := false
			for _, a := range v.Document.Artifacts {
				found = found || a.ID == *input.Source.ArtifactID
			}
			if !found {
				vibeError(w, &vibe.Fault{Code: "not_found", Message: "This version is unavailable."})
				return
			}
		}
		if input.Source.OperationID != nil {
			found := false
			for _, o := range v.Operations {
				found = found || o.ID == *input.Source.OperationID
			}
			if !found {
				vibeError(w, &vibe.Fault{Code: "not_found", Message: "These results are unavailable."})
				return
			}
		}
	}
	receipt, err := h.Enquiries.Create(r.Context(), v.ID, v.Actor, input)
	if err != nil {
		switch {
		case errors.Is(err, enquiries.ErrDisabled):
			err = &vibe.Fault{Code: "hosted_disabled", Message: err.Error()}
		case errors.Is(err, enquiries.ErrConflict):
			err = &vibe.Fault{Code: "idempotency_conflict", Message: err.Error()}
		case errors.Is(err, enquiries.ErrInvalid):
			err = &vibe.Fault{Code: "invalid_request", Message: err.Error()}
		case errors.Is(err, enquiries.ErrQuota):
			err = &vibe.Fault{Code: "rate_limit", Message: err.Error()}
		}
		vibeError(w, err)
		return
	}
	vibeJSON(w, 201, receipt)
}
func (h *VibeHandler) getEnquiry(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "enquiryID")
	if err != nil {
		vibeError(w, err)
		return
	}
	if h.Enquiries == nil {
		vibeError(w, &vibe.Fault{Code: "not_found", Message: "Enquiry unavailable."})
		return
	}
	receipt, err := h.Enquiries.Get(r.Context(), v.ID, id)
	if err != nil {
		vibeError(w, &vibe.Fault{Code: "not_found", Message: "Enquiry unavailable."})
		return
	}
	vibeJSON(w, 200, receipt)
}
