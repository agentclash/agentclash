package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type AgentDeploymentReadRepository interface {
	ListActiveAgentDeploymentsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) ([]repository.AgentDeploymentSummary, error)
	CountActiveAgentDeploymentsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (int64, error)
}

type AgentDeploymentReadService interface {
	ListAgentDeployments(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) (ListAgentDeploymentsResult, error)
}

type ListAgentDeploymentsResult struct {
	Deployments []repository.AgentDeploymentSummary
	Total       int64
	Limit       int32
	Offset      int32
}

type AgentDeploymentReadManager struct {
	repo AgentDeploymentReadRepository
}

func NewAgentDeploymentReadManager(repo AgentDeploymentReadRepository) *AgentDeploymentReadManager {
	return &AgentDeploymentReadManager{
		repo: repo,
	}
}

func (m *AgentDeploymentReadManager) ListAgentDeployments(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) (ListAgentDeploymentsResult, error) {
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

func listAgentDeploymentsHandler(logger *slog.Logger, service AgentDeploymentReadService) http.HandlerFunc {
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
