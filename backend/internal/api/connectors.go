package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/connectors"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"io"
	"net/http"
	"time"
)

type ConnectorRepository interface {
	CreateConnector(context.Context, repository.CreateConnectorParams) (connectors.Binding, error)
	GetConnector(context.Context, uuid.UUID, uuid.UUID) (connectors.Binding, error)
	ListConnectors(context.Context, uuid.UUID) ([]connectors.Binding, error)
	ApproveConnector(context.Context, connectors.Binding, uuid.UUID) (connectors.Binding, error)
	RevokeConnector(context.Context, uuid.UUID, uuid.UUID) error
}

// MountConnectorRoutes belongs inside the existing authenticated router.
// Configuration and approval are admin-only; execution stays on canonical runs.
func MountConnectorRoutes(router chi.Router, repo ConnectorRepository, authorizer WorkspaceAuthorizer, resolver provider.EndpointResolver) {
	handler := func(action Action, fn func(http.ResponseWriter, *http.Request, Caller, uuid.UUID)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			caller, err := CallerFromContext(r.Context())
			if err != nil {
				writeAuthzError(w, err)
				return
			}
			ws, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
			if err != nil {
				writeError(w, 400, "invalid_workspace", "invalid workspace")
				return
			}
			if err = AuthorizeWorkspaceAction(r.Context(), authorizer, caller, ws, action); err != nil {
				writeAuthzError(w, err)
				return
			}
			fn(w, r, caller, ws)
		}
	}
	router.Get("/workspaces/{workspaceID}/connectors", handler(ActionReadWorkspace, func(w http.ResponseWriter, r *http.Request, _ Caller, ws uuid.UUID) {
		items, err := repo.ListConnectors(r.Context(), ws)
		if err != nil {
			writeError(w, 503, "connectors_unavailable", "connector registry unavailable")
			return
		}
		writeJSON(w, 200, map[string]any{"connectors": items})
	}))
	router.Post("/workspaces/{workspaceID}/connectors", handler(ActionManageInfrastructure, func(w http.ResponseWriter, r *http.Request, caller Caller, ws uuid.UUID) {
		var body struct {
			Name   string            `json:"name"`
			Config connectors.Config `json:"config"`
		}
		if err := decodeConnectorBody(w, r, &body); err != nil {
			writeError(w, 400, "invalid_connector", err.Error())
			return
		}
		if err := body.Config.Validate(); err != nil {
			writeError(w, 400, "invalid_connector", err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		normalized, err := provider.ValidateBaseURLWithResolver(ctx, body.Config.EndpointURL, resolver)
		if err != nil {
			writeError(w, 400, "invalid_connector", err.Error())
			return
		}
		body.Config.EndpointURL = normalized
		binding, err := repo.CreateConnector(ctx, repository.CreateConnectorParams{WorkspaceID: ws, ActorUserID: caller.UserID, Name: body.Name, Config: body.Config})
		if err != nil {
			writeError(w, 409, "connector_unavailable", "connector could not be saved; check the workspace secret and configuration")
			return
		}
		writeJSON(w, 201, binding)
	}))
	router.Post("/workspaces/{workspaceID}/connectors/{snapshotID}/approve", handler(ActionManageInfrastructure, func(w http.ResponseWriter, r *http.Request, caller Caller, ws uuid.UUID) {
		var body struct {
			TargetOrigin   string    `json:"target_origin"`
			SecretID       uuid.UUID `json:"secret_id"`
			SecretRevision int64     `json:"secret_revision"`
		}
		snapshot, err := uuid.Parse(chi.URLParam(r, "snapshotID"))
		if err != nil {
			writeError(w, 400, "invalid_snapshot", "invalid snapshot")
			return
		}
		if err = decodeConnectorBody(w, r, &body); err != nil {
			writeError(w, 400, "invalid_approval", err.Error())
			return
		}
		binding, err := repo.GetConnector(r.Context(), ws, snapshot)
		if err != nil {
			writeError(w, 404, "connector_not_found", "connector not found")
			return
		}
		if binding.TargetOrigin != body.TargetOrigin || binding.SecretID != body.SecretID || binding.SecretRevision != body.SecretRevision {
			writeError(w, 409, "stale_approval", "approval does not match the reviewed snapshot")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if _, err = provider.ValidateBaseURLWithResolver(ctx, binding.Config.EndpointURL, resolver); err != nil {
			writeError(w, 400, "invalid_connector", err.Error())
			return
		}
		binding, err = repo.ApproveConnector(ctx, binding, caller.UserID)
		if err != nil {
			writeError(w, 409, "stale_approval", "workspace credential changed; save a new connector snapshot")
			return
		}
		writeJSON(w, 200, binding)
	}))
	router.Post("/workspaces/{workspaceID}/connectors/{snapshotID}/revoke", handler(ActionManageInfrastructure, func(w http.ResponseWriter, r *http.Request, _ Caller, ws uuid.UUID) {
		snapshot, err := uuid.Parse(chi.URLParam(r, "snapshotID"))
		if err != nil {
			writeError(w, 400, "invalid_snapshot", "invalid snapshot")
			return
		}
		if err = repo.RevokeConnector(r.Context(), ws, snapshot); err != nil {
			writeError(w, 404, "connector_not_found", "connector not found")
			return
		}
		writeJSON(w, 200, map[string]bool{"revoked": true})
	}))
}
func decodeConnectorBody(w http.ResponseWriter, r *http.Request, target any) error {
	if err := requireJSONContentType(r); err != nil {
		return err
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, connectors.MaxBodyBytes))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return errors.New("request must be bounded JSON with known fields")
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("request must contain exactly one JSON object")
	}
	return nil
}

// WithConnectorReadiness composes the actual canonical dependency owner with the
// established API checks; inference configuration and liveness are unchanged.
func WithConnectorReadiness(existing func(context.Context) map[string]string, repo interface {
	ConnectorDependencyChecks(context.Context) map[string]string
}) func(context.Context) map[string]string {
	return func(ctx context.Context) map[string]string {
		out := map[string]string{}
		if existing != nil {
			for key, value := range existing(ctx) {
				out[key] = value
			}
		}
		for key, value := range repo.ConnectorDependencyChecks(ctx) {
			out[key] = value
		}
		return out
	}
}
