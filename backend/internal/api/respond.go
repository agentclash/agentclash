package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/agentclash/agentclash/backend/internal/billing"
)

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code          string  `json:"code"`
	Message       string  `json:"message"`
	PlanKey       string  `json:"plan_key,omitempty"`
	UpgradeTarget string  `json:"upgrade_target,omitempty"`
	Limit         *int    `json:"limit,omitempty"`
	Used          *int    `json:"used,omitempty"`
	Remaining     *int    `json:"remaining,omitempty"`
	ResetAt       *string `json:"reset_at,omitempty"`
	ExpiresAt     *string `json:"expires_at,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if payload == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Default().Error("failed to encode JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, errorEnvelope{
		Error: apiError{
			Code:    code,
			Message: message,
		},
	})
}

func writeBillingGateError(w http.ResponseWriter, status int, decision billing.GateDecision) {
	used := decision.Used
	var resetAt *string
	if decision.ResetAt != nil {
		formatted := decision.ResetAt.UTC().Format("2006-01-02T15:04:05Z")
		resetAt = &formatted
	}
	var expiresAt *string
	if decision.ExpiresAt != nil {
		formatted := decision.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
		expiresAt = &formatted
	}
	writeJSON(w, status, errorEnvelope{
		Error: apiError{
			Code:          decision.Code,
			Message:       decision.Message,
			PlanKey:       decision.PlanKey,
			UpgradeTarget: decision.UpgradeTarget,
			Limit:         decision.Limit,
			Used:          &used,
			Remaining:     decision.Remaining,
			ResetAt:       resetAt,
			ExpiresAt:     expiresAt,
		},
	})
}
