package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AgentDeploymentRepository interface {
	ListActiveAgentDeploymentsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) ([]repository.AgentDeploymentSummary, error)
	CountActiveAgentDeploymentsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (int64, error)
	AgentDeploymentIsConnector(ctx context.Context, workspaceID, deploymentID uuid.UUID) (bool, error)
	ArchiveAgentDeployment(ctx context.Context, workspaceID, deploymentID uuid.UUID) error
}

type AgentDeploymentService interface {
	ListAgentDeployments(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) (ListAgentDeploymentsResult, error)
	ArchiveAgentDeployment(ctx context.Context, caller Caller, workspaceID, deploymentID uuid.UUID) error
}

type ListAgentDeploymentsResult struct {
	Deployments []repository.AgentDeploymentSummary
	Total       int64
	Limit       int32
	Offset      int32
}

type AgentDeploymentManager struct {
	authorizer WorkspaceAuthorizer
	repo       AgentDeploymentRepository
}

func NewAgentDeploymentManager(authorizer WorkspaceAuthorizer, repo AgentDeploymentRepository) *AgentDeploymentManager {
	return &AgentDeploymentManager{
		authorizer: authorizer,
		repo:       repo,
	}
}

func (m *AgentDeploymentManager) ArchiveAgentDeployment(ctx context.Context, caller Caller, workspaceID, deploymentID uuid.UUID) error {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionCreateAgentDeployment); err != nil {
		return err
	}
	// A deployment's connector type is fixed at creation, so checking before the update is enough.
	isConnector, err := m.repo.AgentDeploymentIsConnector(ctx, workspaceID, deploymentID)
	if err != nil {
		return err
	}
	if isConnector {
		if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionManageInfrastructure); err != nil {
			return err
		}
	}
	return m.repo.ArchiveAgentDeployment(ctx, workspaceID, deploymentID)
}

func (m *AgentDeploymentManager) ListAgentDeployments(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) (ListAgentDeploymentsResult, error) {
	deployments, err := m.repo.ListActiveAgentDeploymentsByWorkspaceID(ctx, workspaceID, limit, offset)
	if err != nil {
		return ListAgentDeploymentsResult{}, err
	}
	total, err := m.repo.CountActiveAgentDeploymentsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return ListAgentDeploymentsResult{}, err
	}

	return ListAgentDeploymentsResult{
		Deployments: deployments,
		Total:       total,
		Limit:       limit,
		Offset:      offset,
	}, nil
}

type agentDeploymentResponse struct {
	Adapter               string     `json:"adapter"`
	ID                    uuid.UUID  `json:"id"`
	OrganizationID        uuid.UUID  `json:"organization_id"`
	WorkspaceID           uuid.UUID  `json:"workspace_id"`
	CurrentBuildVersionID uuid.UUID  `json:"current_build_version_id"`
	Name                  string     `json:"name"`
	Status                string     `json:"status"`
	LatestSnapshotID      *uuid.UUID `json:"latest_snapshot_id,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type listAgentDeploymentsResponse struct {
	Items  []agentDeploymentResponse `json:"items"`
	Total  int64                     `json:"total"`
	Limit  int32                     `json:"limit"`
	Offset int32                     `json:"offset"`
}

func listAgentDeploymentsHandler(logger *slog.Logger, service AgentDeploymentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		limit, offset := parseListLimitOffset(r)
		result, err := service.ListAgentDeployments(r.Context(), workspaceID, limit, offset)
		if err != nil {
			logger.Error("list agent deployments request failed",
				"method", r.Method,
				"path", r.URL.Path,
				"workspace_id", workspaceID,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		responseItems := make([]agentDeploymentResponse, 0, len(result.Deployments))
		for _, deployment := range result.Deployments {
			responseItems = append(responseItems, agentDeploymentResponse{
				ID:                    deployment.ID,
				Adapter:               deploymentAdapter(deployment.Adapter),
				OrganizationID:        deployment.OrganizationID,
				WorkspaceID:           deployment.WorkspaceID,
				CurrentBuildVersionID: deployment.CurrentBuildVersionID,
				Name:                  deployment.Name,
				Status:                deployment.Status,
				LatestSnapshotID:      deployment.LatestSnapshotID,
				CreatedAt:             deployment.CreatedAt,
				UpdatedAt:             deployment.UpdatedAt,
			})
		}

		writeJSON(w, http.StatusOK, listAgentDeploymentsResponse{
			Items:  responseItems,
			Total:  result.Total,
			Limit:  result.Limit,
			Offset: result.Offset,
		})
	}
}

func archiveAgentDeploymentHandler(logger *slog.Logger, service AgentDeploymentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		deploymentID, err := uuid.Parse(chi.URLParam(r, "deploymentID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_deployment_id", "deployment ID is malformed")
			return
		}
		if err := service.ArchiveAgentDeployment(r.Context(), caller, workspaceID, deploymentID); err != nil {
			if errors.Is(err, repository.ErrAgentDeploymentNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "deployment not found")
				return
			}
			handleServiceError(w, logger, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeAgentDeploymentSlugConflict(w http.ResponseWriter) {
	writeError(w, http.StatusConflict, "agent_deployment_slug_taken", "A deployment with this slug already exists in the workspace. It may be archived.")
}

func deploymentAdapter(value string) string {
	if value == "" {
		return "native"
	}
	return value
}
