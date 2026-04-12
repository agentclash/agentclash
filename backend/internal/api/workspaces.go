package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type WorkspaceService interface {
	CreateWorkspace(ctx context.Context, caller Caller, orgID uuid.UUID, input CreateWorkspaceInput) (WorkspaceResult, error)
	GetWorkspace(ctx context.Context, caller Caller, workspaceID uuid.UUID) (WorkspaceResult, error)
	ListWorkspaces(ctx context.Context, caller Caller, orgID uuid.UUID, limit, offset int32) (ListWorkspacesResult, error)
	UpdateWorkspace(ctx context.Context, caller Caller, workspaceID uuid.UUID, input UpdateWorkspaceInput) (WorkspaceResult, error)
}

type CreateWorkspaceInput struct {
	Name string
	Slug *string
}

type UpdateWorkspaceInput struct {
	Name   *string
	Status *string
}

type WorkspaceResult struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ListWorkspacesResult struct {
	Items  []WorkspaceResult `json:"items"`
	Total  int64             `json:"total"`
	Limit  int32             `json:"limit"`
	Offset int32             `json:"offset"`
}

type WorkspaceCRUDRepository interface {
	CreateWorkspaceWithAdmin(ctx context.Context, input repository.CreateWorkspaceWithAdminInput) (repository.WorkspaceRow, error)
	GetWorkspaceByID(ctx context.Context, workspaceID uuid.UUID) (repository.WorkspaceRow, error)
	ListWorkspacesByOrgID(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]repository.WorkspaceRow, error)
	CountWorkspacesByOrgID(ctx context.Context, orgID uuid.UUID) (int64, error)
	ListWorkspacesByOrgIDForMember(ctx context.Context, orgID, userID uuid.UUID, limit, offset int32) ([]repository.WorkspaceRow, error)
	CountWorkspacesByOrgIDForMember(ctx context.Context, orgID, userID uuid.UUID) (int64, error)
	UpdateWorkspace(ctx context.Context, workspaceID uuid.UUID, input repository.UpdateWorkspaceInput) (repository.WorkspaceRow, error)
	ArchiveWorkspaceCascade(ctx context.Context, workspaceID uuid.UUID) (repository.WorkspaceRow, error)
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
}

type WorkspaceManager struct {
	orgAuthz OrganizationAuthorizer
	repo     WorkspaceCRUDRepository
}

func NewWorkspaceManager(orgAuthz OrganizationAuthorizer, repo WorkspaceCRUDRepository) *WorkspaceManager {
	return &WorkspaceManager{orgAuthz: orgAuthz, repo: repo}
}

func (m *WorkspaceManager) CreateWorkspace(ctx context.Context, caller Caller, orgID uuid.UUID, input CreateWorkspaceInput) (WorkspaceResult, error) {
	if err := m.orgAuthz.AuthorizeOrganizationAdmin(ctx, caller, orgID); err != nil {
		return WorkspaceResult{}, err
	}

	slug := ""
	if input.Slug != nil {
		if err := validateSlug(*input.Slug); err != nil {
			return WorkspaceResult{}, err
		}
		slug = *input.Slug
	} else {
		slug = generateSlug(input.Name)
	}
	if err := validateSlug(slug); err != nil {
		return WorkspaceResult{}, err
	}

	ws, err := m.repo.CreateWorkspaceWithAdmin(ctx, repository.CreateWorkspaceWithAdminInput{
		OrganizationID: orgID,
		Name:           input.Name,
		Slug:           slug,
		UserID:         caller.UserID,
	})
	if err != nil {
		return WorkspaceResult{}, err
	}

	return wsRowToResult(ws), nil
}

func (m *WorkspaceManager) GetWorkspace(ctx context.Context, caller Caller, workspaceID uuid.UUID) (WorkspaceResult, error) {
	if err := m.authorizeWorkspaceAccess(ctx, caller, workspaceID); err != nil {
		return WorkspaceResult{}, err
	}

	ws, err := m.repo.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return WorkspaceResult{}, err
	}

	return wsRowToResult(ws), nil
}

func (m *WorkspaceManager) ListWorkspaces(ctx context.Context, caller Caller, orgID uuid.UUID, limit, offset int32) (ListWorkspacesResult, error) {
	if err := m.orgAuthz.AuthorizeOrganization(ctx, caller, orgID); err != nil {
		return ListWorkspacesResult{}, err
	}

	// org_admin sees all workspaces; org_member sees only their own.
	membership := caller.OrganizationMemberships[orgID]
	var workspaces []repository.WorkspaceRow
	var total int64
	var err error

	if membership.Role == "org_admin" {
		workspaces, err = m.repo.ListWorkspacesByOrgID(ctx, orgID, limit, offset)
		if err != nil {
			return ListWorkspacesResult{}, err
		}
		total, err = m.repo.CountWorkspacesByOrgID(ctx, orgID)
	} else {
		workspaces, err = m.repo.ListWorkspacesByOrgIDForMember(ctx, orgID, caller.UserID, limit, offset)
		if err != nil {
			return ListWorkspacesResult{}, err
		}
		total, err = m.repo.CountWorkspacesByOrgIDForMember(ctx, orgID, caller.UserID)
	}
	if err != nil {
		return ListWorkspacesResult{}, err
	}

	items := make([]WorkspaceResult, 0, len(workspaces))
	for _, ws := range workspaces {
		items = append(items, wsRowToResult(ws))
	}

	return ListWorkspacesResult{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (m *WorkspaceManager) UpdateWorkspace(ctx context.Context, caller Caller, workspaceID uuid.UUID, input UpdateWorkspaceInput) (WorkspaceResult, error) {
	if err := m.authorizeWorkspaceAdmin(ctx, caller, workspaceID); err != nil {
		return WorkspaceResult{}, err
	}

	if input.Status != nil && *input.Status == "archived" {
		ws, err := m.repo.ArchiveWorkspaceCascade(ctx, workspaceID)
		if err != nil {
			return WorkspaceResult{}, err
		}
		return wsRowToResult(ws), nil
	}

	ws, err := m.repo.UpdateWorkspace(ctx, workspaceID, repository.UpdateWorkspaceInput{
		Name:   input.Name,
		Status: input.Status,
	})
	if err != nil {
		return WorkspaceResult{}, err
	}

	return wsRowToResult(ws), nil
}

// authorizeWorkspaceAccess checks if the caller can access the workspace — either through
// explicit workspace membership or org_admin of the parent org.
func (m *WorkspaceManager) authorizeWorkspaceAccess(ctx context.Context, caller Caller, workspaceID uuid.UUID) error {
	if _, ok := caller.WorkspaceMemberships[workspaceID]; ok {
		return nil
	}
	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if membership, ok := caller.OrganizationMemberships[orgID]; ok && membership.Role == "org_admin" {
		return nil
	}
	return ErrForbidden
}

// authorizeWorkspaceAdmin checks if the caller is a workspace_admin or org_admin of the parent org.
func (m *WorkspaceManager) authorizeWorkspaceAdmin(ctx context.Context, caller Caller, workspaceID uuid.UUID) error {
	if membership, ok := caller.WorkspaceMemberships[workspaceID]; ok && membership.Role == "workspace_admin" {
		return nil
	}
	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if membership, ok := caller.OrganizationMemberships[orgID]; ok && membership.Role == "org_admin" {
		return nil
	}
	return ErrForbidden
}

func wsRowToResult(ws repository.WorkspaceRow) WorkspaceResult {
	return WorkspaceResult{
		ID:             ws.ID,
		OrganizationID: ws.OrganizationID,
		Name:           ws.Name,
		Slug:           ws.Slug,
		Status:         ws.Status,
		CreatedAt:      ws.CreatedAt,
		UpdatedAt:      ws.UpdatedAt,
	}
}

// --- Handlers ---

func createWorkspaceHandler(logger *slog.Logger, service WorkspaceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		orgID, err := uuid.Parse(chi.URLParam(r, "organizationID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_organization_id", "organization ID is malformed")
			return
		}

		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		var req struct {
			Name string  `json:"name"`
			Slug *string `json:"slug,omitempty"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "name is required")
			return
		}

		result, err := service.CreateWorkspace(r.Context(), caller, orgID, CreateWorkspaceInput{
			Name: req.Name,
			Slug: req.Slug,
		})
		if err != nil {
			handleWorkspaceError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusCreated, result)
	}
}

func getWorkspaceHandler(logger *slog.Logger, service WorkspaceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace ID is malformed")
			return
		}

		result, err := service.GetWorkspace(r.Context(), caller, workspaceID)
		if err != nil {
			handleWorkspaceError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func listWorkspacesHandler(logger *slog.Logger, service WorkspaceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		orgID, err := uuid.Parse(chi.URLParam(r, "organizationID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_organization_id", "organization ID is malformed")
			return
		}

		limit := int32(50)
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 {
				limit = int32(parsed)
				if limit > 100 {
					limit = 100
				}
			}
		}
		offset := int32(0)
		if raw := r.URL.Query().Get("offset"); raw != "" {
			if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed >= 0 {
				offset = int32(parsed)
			}
		}

		result, err := service.ListWorkspaces(r.Context(), caller, orgID, limit, offset)
		if err != nil {
			handleWorkspaceError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func updateWorkspaceHandler(logger *slog.Logger, service WorkspaceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace ID is malformed")
			return
		}

		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		var req struct {
			Name   *string `json:"name,omitempty"`
			Status *string `json:"status,omitempty"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
			return
		}
		if req.Name == nil && req.Status == nil {
			writeError(w, http.StatusBadRequest, "validation_error", "at least one of name or status is required")
			return
		}
		if req.Status != nil && *req.Status != "active" && *req.Status != "archived" {
			writeError(w, http.StatusBadRequest, "validation_error", "status must be active or archived")
			return
		}

		result, err := service.UpdateWorkspace(r.Context(), caller, workspaceID, UpdateWorkspaceInput{
			Name:   req.Name,
			Status: req.Status,
		})
		if err != nil {
			handleWorkspaceError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func handleWorkspaceError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
	case errors.Is(err, repository.ErrWorkspaceNotFound):
		writeError(w, http.StatusNotFound, "not_found", "workspace not found")
	case errors.Is(err, repository.ErrSlugTaken):
		writeError(w, http.StatusConflict, "slug_taken", "a workspace with this slug already exists in this organization")
	case errors.Is(err, ErrInvalidSlug):
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
	default:
		logger.Error("workspace operation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
