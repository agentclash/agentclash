package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxWorkspaceSecretValueBytes = 64 << 10 // 64 KiB per value

type WorkspaceSecretsRepository interface {
	ListWorkspaceSecrets(ctx context.Context, workspaceID uuid.UUID) ([]repository.WorkspaceSecretMetadata, error)
	UpsertWorkspaceSecret(ctx context.Context, params repository.UpsertWorkspaceSecretParams) error
	DeleteWorkspaceSecret(ctx context.Context, workspaceID uuid.UUID, key string) error
}

type WorkspaceSecretsService interface {
	ListSecrets(ctx context.Context) ([]WorkspaceSecretSummary, error)
	UpsertSecret(ctx context.Context, key string, value string) error
	DeleteSecret(ctx context.Context, key string) error
}

type WorkspaceSecretSummary struct {
	Key       string     `json:"key"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	CreatedBy *uuid.UUID `json:"created_by,omitempty"`
	UpdatedBy *uuid.UUID `json:"updated_by,omitempty"`
}

type WorkspaceSecretsManager struct {
	repo WorkspaceSecretsRepository
}

func NewWorkspaceSecretsManager(repo WorkspaceSecretsRepository) *WorkspaceSecretsManager {
	return &WorkspaceSecretsManager{repo: repo}
}

func (m *WorkspaceSecretsManager) ListSecrets(ctx context.Context) ([]WorkspaceSecretSummary, error) {
	workspaceID, err := WorkspaceIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := m.repo.ListWorkspaceSecrets(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list workspace secrets: %w", err)
	}
	out := make([]WorkspaceSecretSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, WorkspaceSecretSummary{
			Key:       row.Key,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
			CreatedBy: row.CreatedBy,
			UpdatedBy: row.UpdatedBy,
		})
	}
	return out, nil
}

func (m *WorkspaceSecretsManager) UpsertSecret(ctx context.Context, key string, value string) error {
	workspaceID, err := WorkspaceIDFromContext(ctx)
	if err != nil {
		return err
	}
	caller, err := CallerFromContext(ctx)
	if err != nil {
		return err
	}
	actorID := caller.UserID
	return m.repo.UpsertWorkspaceSecret(ctx, repository.UpsertWorkspaceSecretParams{
		WorkspaceID: workspaceID,
		Key:         key,
		Value:       value,
		ActorUserID: &actorID,
	})
}

func (m *WorkspaceSecretsManager) DeleteSecret(ctx context.Context, key string) error {
	workspaceID, err := WorkspaceIDFromContext(ctx)
	if err != nil {
		return err
	}
	return m.repo.DeleteWorkspaceSecret(ctx, workspaceID, key)
}

type listWorkspaceSecretsResponse struct {
	Items []WorkspaceSecretSummary `json:"items"`
}

type upsertWorkspaceSecretRequest struct {
	Value *string `json:"value"`
}

func listWorkspaceSecretsHandler(logger *slog.Logger, service WorkspaceSecretsService, authorizer WorkspaceAuthorizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		wsID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		if err := AuthorizeWorkspaceAction(r.Context(), authorizer, caller, wsID, ActionManageSecrets); err != nil {
			writeAuthzError(w, err)
			return
		}

		secrets, err := service.ListSecrets(r.Context())
		if err != nil {
			logger.Error("list workspace secrets request failed",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, listWorkspaceSecretsResponse{Items: secrets})
	}
}

func upsertWorkspaceSecretHandler(logger *slog.Logger, service WorkspaceSecretsService, authorizer WorkspaceAuthorizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		wsID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		if err := AuthorizeWorkspaceAction(r.Context(), authorizer, caller, wsID, ActionManageSecrets); err != nil {
			writeAuthzError(w, err)
			return
		}

		key := chi.URLParam(r, "secretKey")
		if !repository.IsValidSecretKey(key) {
			writeError(w, http.StatusBadRequest, "invalid_secret_key", "secret key must match [A-Za-z_][A-Za-z0-9_]* and be 1..128 characters")
			return
		}

		defer r.Body.Close()
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWorkspaceSecretValueBytes))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_body", fmt.Sprintf("request body must be valid json and at most %d bytes", maxWorkspaceSecretValueBytes))
			return
		}

		var payload upsertWorkspaceSecretRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_body", "request body must be valid json")
			return
		}
		if payload.Value == nil {
			writeError(w, http.StatusBadRequest, "invalid_request_body", "value is required")
			return
		}

		if err := service.UpsertSecret(r.Context(), key, *payload.Value); err != nil {
			switch {
			case errors.Is(err, repository.ErrInvalidSecretKey):
				writeError(w, http.StatusBadRequest, "invalid_secret_key", err.Error())
				return
			case errors.Is(err, repository.ErrSecretsCipherUnset):
				logger.Error("workspace secrets cipher is not configured",
					"method", r.Method,
					"path", r.URL.Path,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				return
			default:
				logger.Error("upsert workspace secret request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteWorkspaceSecretHandler(logger *slog.Logger, service WorkspaceSecretsService, authorizer WorkspaceAuthorizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		wsID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		if err := AuthorizeWorkspaceAction(r.Context(), authorizer, caller, wsID, ActionManageSecrets); err != nil {
			writeAuthzError(w, err)
			return
		}

		key := chi.URLParam(r, "secretKey")
		if !repository.IsValidSecretKey(key) {
			writeError(w, http.StatusBadRequest, "invalid_secret_key", "secret key must match [A-Za-z_][A-Za-z0-9_]* and be 1..128 characters")
			return
		}
		if err := service.DeleteSecret(r.Context(), key); err != nil {
			switch {
			case errors.Is(err, repository.ErrWorkspaceSecretNotFound):
				writeError(w, http.StatusNotFound, "secret_not_found", "secret does not exist")
				return
			default:
				logger.Error("delete workspace secret request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type noopWorkspaceSecretsService struct{}

func (noopWorkspaceSecretsService) ListSecrets(context.Context) ([]WorkspaceSecretSummary, error) {
	return nil, errors.New("workspace secrets service is not configured")
}

func (noopWorkspaceSecretsService) UpsertSecret(context.Context, string, string) error {
	return errors.New("workspace secrets service is not configured")
}

func (noopWorkspaceSecretsService) DeleteSecret(context.Context, string) error {
	return errors.New("workspace secrets service is not configured")
}
