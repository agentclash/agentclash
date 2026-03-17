package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type AgentDeploymentReadRepository interface {
	ListActiveAgentDeploymentsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.AgentDeploymentSummary, error)
}

type AgentDeploymentReadService interface {
	ListAgentDeployments(ctx context.Context, workspaceID uuid.UUID) (ListAgentDeploymentsResult, error)
}

type ListAgentDeploymentsResult struct {
	Deployments []repository.AgentDeploymentSummary
}

type AgentDeploymentReadManager struct {
	repo AgentDeploymentReadRepository
}

func NewAgentDeploymentReadManager(repo AgentDeploymentReadRepository) *AgentDeploymentReadManager {
	return &AgentDeploymentReadManager{
		repo: repo,
	}
}

func (m *AgentDeploymentReadManager) ListAgentDeployments(ctx context.Context, workspaceID uuid.UUID) (ListAgentDeploymentsResult, error) {
	deployments, err := m.repo.ListActiveAgentDeploymentsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return ListAgentDeploymentsResult{}, err
	}

	return ListAgentDeploymentsResult{
		Deployments: deployments,
	}, nil
}

type agentDeploymentResponse struct {
	ID               uuid.UUID  `json:"id"`
	OrganizationID   uuid.UUID  `json:"organization_id"`
	WorkspaceID      uuid.UUID  `json:"workspace_id"`
	Name             string     `json:"name"`
	Status           string     `json:"status"`
	LatestSnapshotID *uuid.UUID `json:"latest_snapshot_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type listAgentDeploymentsResponse struct {
	Items []agentDeploymentResponse `json:"items"`
}

func listAgentDeploymentsHandler(logger *slog.Logger, service AgentDeploymentReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		result, err := service.ListAgentDeployments(r.Context(), workspaceID)
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
				ID:               deployment.ID,
				OrganizationID:   deployment.OrganizationID,
				WorkspaceID:      deployment.WorkspaceID,
				Name:             deployment.Name,
				Status:           deployment.Status,
				LatestSnapshotID: deployment.LatestSnapshotID,
				CreatedAt:        deployment.CreatedAt,
				UpdatedAt:        deployment.UpdatedAt,
			})
		}

		writeJSON(w, http.StatusOK, listAgentDeploymentsResponse{Items: responseItems})
	}
}
