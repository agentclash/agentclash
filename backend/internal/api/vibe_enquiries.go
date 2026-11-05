package api

import (
	"errors"
	"net"
	"net/http"

	"github.com/agentclash/agentclash/backend/internal/enquiries"
	"github.com/agentclash/agentclash/backend/internal/mutation"
	"github.com/agentclash/agentclash/backend/internal/vibe"
)

func (h *VibeHandler) createEnquiry(w http.ResponseWriter, r *http.Request) {
	v, err := h.authorizedSession(r, true)
	if err != nil {
		vibeError(w, mutation.Reject(err))
		return
	}
	if h.Enquiries == nil {
		vibeError(w, &vibe.Fault{Code: "hosted_disabled", Message: enquiries.ErrDisabled.Error()})
		return
	}
	var input enquiries.Request
	if err = vibeBody(w, r, v.Anonymous, &input); err != nil {
		vibeError(w, mutation.Reject(err))
		return
	}
	receipt, err := h.Enquiries.Create(r.Context(), v.ID, v.Actor, input)
	if err != nil {
		rejected := mutation.IsRejected(err)
		switch {
		case errors.Is(err, enquiries.ErrDisabled):
			err = &vibe.Fault{Code: "hosted_disabled", Message: err.Error()}
		case errors.Is(err, enquiries.ErrConflict):
			err = &vibe.Fault{Code: "idempotency_conflict", Message: err.Error()}
		case errors.Is(err, enquiries.ErrRevision):
			err = &vibe.Fault{Code: "revision_conflict", Message: "This project version is unavailable."}
		case errors.Is(err, enquiries.ErrUnavailable):
			err = &vibe.Fault{Code: "not_found", Message: "This project version or result is unavailable."}
		case errors.Is(err, enquiries.ErrInvalid):
			err = &vibe.Fault{Code: "invalid_request", Message: err.Error()}
		case errors.Is(err, enquiries.ErrQuota):
			err = &vibe.Fault{Code: "rate_limit", Message: err.Error()}
		}
		if rejected {
			err = mutation.Reject(err)
		}
		vibeError(w, err)
		return
	}
	vibeJSON(w, 201, receipt)
}
func (h *VibeHandler) getEnquiry(w http.ResponseWriter, r *http.Request) {
	v, err := h.authorizedSession(r, false)
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

func (h *VibeHandler) createContactEnquiry(w http.ResponseWriter, r *http.Request) {
	var input enquiries.ContactRequest
	if err := vibeBody(w, r, true, &input); err != nil {
		vibeError(w, mutation.Reject(err))
		return
	}
	actor, err := h.actor(r)
	if err != nil && r.Header.Get("Authorization") == "" {
		actor, err = h.issue(w, r)
	}
	if err != nil {
		vibeError(w, mutation.Reject(err))
		return
	}
	if !h.Enquiries.Available() {
		vibeError(w, mutation.Reject(&vibe.Fault{Code: "hosted_disabled", Message: enquiries.ErrDisabled.Error()}))
		return
	}
	// Use only the transport peer, consistently with anonymous cookie issuance.
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if err = h.Service.Gate.Check(r.Context(), "contact:"+ip, vibe.Limits{Rate: 5}); err != nil {
		vibeError(w, mutation.Reject(err))
		return
	}
	receipt, err := h.Enquiries.CreateContact(r.Context(), actor, input)
	if err != nil {
		rejected := mutation.IsRejected(err)
		switch {
		case errors.Is(err, enquiries.ErrInvalid):
			err = &vibe.Fault{Code: "invalid_request", Message: err.Error()}
		case errors.Is(err, enquiries.ErrQuota):
			err = &vibe.Fault{Code: "rate_limit", Message: err.Error()}
		case errors.Is(err, enquiries.ErrDisabled):
			err = &vibe.Fault{Code: "hosted_disabled", Message: err.Error()}
		case errors.Is(err, enquiries.ErrConflict):
			err = &vibe.Fault{Code: "idempotency_conflict", Message: err.Error()}
		}
		if rejected {
			err = mutation.Reject(err)
		}
		vibeError(w, err)
		return
	}
	vibeJSON(w, http.StatusCreated, receipt)
}
