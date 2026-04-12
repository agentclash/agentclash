package api

import (
	"context"
	"log/slog"
	"net/http"
	"sort"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type UserService interface {
	GetMe(ctx context.Context, caller Caller) (GetUserMeResult, error)
}

type GetUserMeResult struct {
	UserID        uuid.UUID            `json:"user_id"`
	WorkOSUserID  string               `json:"workos_user_id,omitempty"`
	Email         string               `json:"email,omitempty"`
	DisplayName   string               `json:"display_name,omitempty"`
	Organizations []UserMeOrganization `json:"organizations"`
}

type UserMeOrganization struct {
	ID         uuid.UUID         `json:"id"`
	Name       string            `json:"name"`
	Slug       string            `json:"slug"`
	Role       string            `json:"role"`
	Workspaces []UserMeWorkspace `json:"workspaces"`
}

type UserMeWorkspace struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
	Role string    `json:"role"`
}

type UserMeRepository interface {
	GetOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]repository.UserMeOrgRow, error)
	GetWorkspacesForUser(ctx context.Context, userID uuid.UUID) ([]repository.UserMeWorkspaceRow, error)
	GetAllWorkspacesForOrgs(ctx context.Context, orgIDs []uuid.UUID) ([]repository.UserMeWorkspaceRow, error)
}

type UserManager struct {
	repo UserMeRepository
}

func NewUserManager(repo UserMeRepository) *UserManager {
	return &UserManager{repo: repo}
}

func (m *UserManager) GetMe(ctx context.Context, caller Caller) (GetUserMeResult, error) {
	orgs, err := m.repo.GetOrganizationsForUser(ctx, caller.UserID)
	if err != nil {
		return GetUserMeResult{}, err
	}

	// Determine which org IDs the caller is org_admin for (implicit workspace access).
	var adminOrgIDs []uuid.UUID
	for _, org := range orgs {
		if org.Role == "org_admin" {
			adminOrgIDs = append(adminOrgIDs, org.ID)
		}
	}

	// Get explicit workspace memberships.
	explicitWorkspaces, err := m.repo.GetWorkspacesForUser(ctx, caller.UserID)
	if err != nil {
		return GetUserMeResult{}, err
	}

	// For org_admin orgs, get ALL workspaces (implicit access).
	var implicitWorkspaces []repository.UserMeWorkspaceRow
	if len(adminOrgIDs) > 0 {
		implicitWorkspaces, err = m.repo.GetAllWorkspacesForOrgs(ctx, adminOrgIDs)
		if err != nil {
			return GetUserMeResult{}, err
		}
	}

	// Build workspace map per org, deduplicating (explicit wins over implicit).
	type wsEntry struct {
		ws   repository.UserMeWorkspaceRow
		role string
	}
	workspacesByOrg := make(map[uuid.UUID]map[uuid.UUID]wsEntry)

	// Add implicit workspaces first (org_admin access).
	for _, ws := range implicitWorkspaces {
		if workspacesByOrg[ws.OrganizationID] == nil {
			workspacesByOrg[ws.OrganizationID] = make(map[uuid.UUID]wsEntry)
		}
		workspacesByOrg[ws.OrganizationID][ws.ID] = wsEntry{ws: ws, role: "org_admin"}
	}

	// Add explicit workspaces (overwrite implicit if present — explicit role wins).
	for _, ws := range explicitWorkspaces {
		if workspacesByOrg[ws.OrganizationID] == nil {
			workspacesByOrg[ws.OrganizationID] = make(map[uuid.UUID]wsEntry)
		}
		workspacesByOrg[ws.OrganizationID][ws.ID] = wsEntry{ws: ws, role: ws.Role}
	}

	// Assemble response.
	organizations := make([]UserMeOrganization, 0, len(orgs))
	for _, org := range orgs {
		wsMap := workspacesByOrg[org.ID]
		workspaces := make([]UserMeWorkspace, 0, len(wsMap))
		for _, entry := range wsMap {
			workspaces = append(workspaces, UserMeWorkspace{
				ID:   entry.ws.ID,
				Name: entry.ws.Name,
				Slug: entry.ws.Slug,
				Role: entry.role,
			})
		}
		sort.Slice(workspaces, func(i, j int) bool {
			return workspaces[i].Name < workspaces[j].Name
		})

		organizations = append(organizations, UserMeOrganization{
			ID:         org.ID,
			Name:       org.Name,
			Slug:       org.Slug,
			Role:       org.Role,
			Workspaces: workspaces,
		})
	}

	return GetUserMeResult{
		UserID:        caller.UserID,
		WorkOSUserID:  caller.WorkOSUserID,
		Email:         caller.Email,
		DisplayName:   caller.DisplayName,
		Organizations: organizations,
	}, nil
}

func getUserMeHandler(logger *slog.Logger, service UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		result, err := service.GetMe(r.Context(), caller)
		if err != nil {
			logger.Error("get user me failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}
