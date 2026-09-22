package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/vibe"
)

func (h *VibeHandler) interact(w http.ResponseWriter, r *http.Request) {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		vibeError(w, &vibe.Fault{Code: "invalid_request", Message: "Send an application/json request."})
		return
	}
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
	if err != nil {
		vibeError(w, err)
		return
	}
	a, err := vibe.DecodeInteraction(b)
	if err != nil {
		vibeError(w, err)
		return
	}
	if err = h.Service.Interact(r.Context(), v.Actor, v.ID, a); err != nil {
		vibeError(w, err)
		return
	}
	h.get(w, r)
}
