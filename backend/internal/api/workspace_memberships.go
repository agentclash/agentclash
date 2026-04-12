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

type WorkspaceMembershipService interface {
	ListWorkspaceMemberships(ctx context.Context, caller Caller, workspaceID uuid.UUID, limit, offset int32) (ListWorkspaceMembershipsResult, error)
	InviteWorkspaceMember(ctx context.Context, caller Caller, workspaceID uuid.UUID, input InviteWorkspaceMemberInput) (WorkspaceMembershipResult, error)
	UpdateWorkspaceMembership(ctx context.Context, caller Caller, membershipID uuid.UUID, input UpdateWorkspaceMembershipInput) (WorkspaceMembershipResult, error)
}

type InviteWorkspaceMemberInput struct {
	Email string
	Role  string
}

type UpdateWorkspaceMembershipInput struct {
	Role   *string
	Status *string
}

type WorkspaceMembershipResult struct {
	ID               uuid.UUID `json:"id"`
	WorkspaceID      uuid.UUID `json:"workspace_id"`
	OrganizationID   uuid.UUID `json:"organization_id"`
	UserID           uuid.UUID `json:"user_id"`
	Email            string    `json:"email"`
	DisplayName      string    `json:"display_name"`
	Role             string    `json:"role"`
	MembershipStatus string    `json:"membership_status"`
	CreatedAt        time.Time `json:"created_at"`
}

type ListWorkspaceMembershipsResult struct {
	Items  []WorkspaceMembershipResult `json:"items"`
	Total  int64                       `json:"total"`
	Limit  int32                       `json:"limit"`
	Offset int32                       `json:"offset"`
}

type WorkspaceMembershipRepository interface {
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	ListWorkspaceMemberships(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) ([]repository.WorkspaceMembershipFullRow, error)
	CountWorkspaceMemberships(ctx context.Context, workspaceID uuid.UUID) (int64, error)
	GetUserByEmail(ctx context.Context, email string) (repository.User, error)
	CreateUser(ctx context.Context, input repository.CreateUserInput) (repository.User, error)
	GetOrgMembershipByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID) (repository.OrgMembershipFullRow, error)
	GetWorkspaceMembershipByWorkspaceAndUser(ctx context.Context, workspaceID, userID uuid.UUID) (repository.WorkspaceMembershipFullRow, error)
	CreateWorkspaceMembership(ctx context.Context, input repository.CreateWorkspaceMembershipInput) (repository.WorkspaceMembershipFullRow, error)
	GetWorkspaceMembershipByID(ctx context.Context, membershipID uuid.UUID) (repository.WorkspaceMembershipFullRow, error)
	UpdateWorkspaceMembership(ctx context.Context, membershipID uuid.UUID, input repository.UpdateWorkspaceMembershipInput) (repository.WorkspaceMembershipFullRow, error)
	CountActiveWorkspaceAdmins(ctx context.Context, workspaceID uuid.UUID) (int64, error)
}

type WorkspaceMembershipManager struct {
	repo WorkspaceMembershipRepository
}

func NewWorkspaceMembershipManager(repo WorkspaceMembershipRepository) *WorkspaceMembershipManager {
	return &WorkspaceMembershipManager{repo: repo}
}

func (m *WorkspaceMembershipManager) ListWorkspaceMemberships(ctx context.Context, caller Caller, workspaceID uuid.UUID, limit, offset int32) (ListWorkspaceMembershipsResult, error) {
	if err := m.authorizeAccess(ctx, caller, workspaceID); err != nil {
		return ListWorkspaceMembershipsResult{}, err
	}

	memberships, err := m.repo.ListWorkspaceMemberships(ctx, workspaceID, limit, offset)
	if err != nil {
		return ListWorkspaceMembershipsResult{}, err
	}

	total, err := m.repo.CountWorkspaceMemberships(ctx, workspaceID)
	if err != nil {
		return ListWorkspaceMembershipsResult{}, err
	}

	items := make([]WorkspaceMembershipResult, 0, len(memberships))
	for _, row := range memberships {
		items = append(items, wsMembershipRowToResult(row))
	}

	return ListWorkspaceMembershipsResult{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (m *WorkspaceMembershipManager) InviteWorkspaceMember(ctx context.Context, caller Caller, workspaceID uuid.UUID, input InviteWorkspaceMemberInput) (WorkspaceMembershipResult, error) {
	if err := m.authorizeAdmin(ctx, caller, workspaceID); err != nil {
		return WorkspaceMembershipResult{}, err
	}

	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return WorkspaceMembershipResult{}, err
	}

	// Look up or create user.
	user, err := m.repo.GetUserByEmail(ctx, input.Email)
	if errors.Is(err, repository.ErrUserNotFound) {
		user, err = m.repo.CreateUser(ctx, repository.CreateUserInput{
			WorkOSUserID: "pending:" + uuid.New().String(),
			Email:        input.Email,
		})
		if err != nil && !errors.Is(err, repository.ErrUserAlreadyExists) {
			return WorkspaceMembershipResult{}, err
		}
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			user, err = m.repo.GetUserByEmail(ctx, input.Email)
			if err != nil {
				return WorkspaceMembershipResult{}, err
			}
		}
	} else if err != nil {
		return WorkspaceMembershipResult{}, err
	}

	// Pre-check: user must have active org membership.
	orgMembership, err := m.repo.GetOrgMembershipByOrgAndUser(ctx, orgID, user.ID)
	if errors.Is(err, repository.ErrMembershipNotFound) || (err == nil && orgMembership.MembershipStatus != "active") {
		return WorkspaceMembershipResult{}, repository.ErrOrgMembershipRequired
	}
	if err != nil {
		return WorkspaceMembershipResult{}, err
	}

	// Check existing workspace membership.
	existing, err := m.repo.GetWorkspaceMembershipByWorkspaceAndUser(ctx, workspaceID, user.ID)
	if err == nil {
		if existing.MembershipStatus == "active" || existing.MembershipStatus == "invited" {
			return WorkspaceMembershipResult{}, repository.ErrAlreadyMember
		}
		// Previously archived — re-invite.
		result, err := m.repo.UpdateWorkspaceMembership(ctx, existing.ID, repository.UpdateWorkspaceMembershipInput{
			Role:   &input.Role,
			Status: strPtr("invited"),
		})
		if err != nil {
			return WorkspaceMembershipResult{}, err
		}
		return wsMembershipRowToResult(result), nil
	}
	if !errors.Is(err, repository.ErrMembershipNotFound) {
		return WorkspaceMembershipResult{}, err
	}

	result, err := m.repo.CreateWorkspaceMembership(ctx, repository.CreateWorkspaceMembershipInput{
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		UserID:         user.ID,
		Role:           input.Role,
	})
	if err != nil {
		return WorkspaceMembershipResult{}, err
	}

	return wsMembershipRowToResult(result), nil
}

func (m *WorkspaceMembershipManager) UpdateWorkspaceMembership(ctx context.Context, caller Caller, membershipID uuid.UUID, input UpdateWorkspaceMembershipInput) (WorkspaceMembershipResult, error) {
	membership, err := m.repo.GetWorkspaceMembershipByID(ctx, membershipID)
	if err != nil {
		return WorkspaceMembershipResult{}, err
	}

	// Authorization: workspace_admin, org_admin of parent org, or invited user accepting.
	isAdmin := false
	if wm, ok := caller.WorkspaceMemberships[membership.WorkspaceID]; ok && wm.Role == "workspace_admin" {
		isAdmin = true
	}
	if !isAdmin {
		orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, membership.WorkspaceID)
		if err == nil {
			if om, ok := caller.OrganizationMemberships[orgID]; ok && om.Role == "org_admin" {
				isAdmin = true
			}
		}
	}
	isInvitedUser := membership.UserID == caller.UserID && membership.MembershipStatus == "invited"

	if !isAdmin && !isInvitedUser {
		return WorkspaceMembershipResult{}, ErrForbidden
	}

	if isInvitedUser && !isAdmin {
		if input.Status == nil || *input.Status != "active" {
			return WorkspaceMembershipResult{}, ErrForbidden
		}
		invitedAt := membership.UpdatedAt
		if invitedAt.IsZero() {
			invitedAt = membership.CreatedAt
		}
		if time.Since(invitedAt) > inviteExpiryDays*24*time.Hour {
			return WorkspaceMembershipResult{}, repository.ErrInviteExpired
		}
	}

	if input.Status != nil {
		if err := validateWsMembershipTransition(membership.MembershipStatus, *input.Status); err != nil {
			return WorkspaceMembershipResult{}, err
		}
	}

	if input.Role != nil && membership.UserID == caller.UserID {
		return WorkspaceMembershipResult{}, ErrForbidden
	}

	// Last admin protection.
	if isRemovingWsAdmin(membership, input) {
		count, err := m.repo.CountActiveWorkspaceAdmins(ctx, membership.WorkspaceID)
		if err != nil {
			return WorkspaceMembershipResult{}, err
		}
		if count <= 1 {
			return WorkspaceMembershipResult{}, repository.ErrLastWorkspaceAdmin
		}
	}

	result, err := m.repo.UpdateWorkspaceMembership(ctx, membershipID, repository.UpdateWorkspaceMembershipInput{
		Role:   input.Role,
		Status: input.Status,
	})
	if err != nil {
		return WorkspaceMembershipResult{}, err
	}

	return wsMembershipRowToResult(result), nil
}

func (m *WorkspaceMembershipManager) authorizeAccess(ctx context.Context, caller Caller, workspaceID uuid.UUID) error {
	if _, ok := caller.WorkspaceMemberships[workspaceID]; ok {
		return nil
	}
	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if om, ok := caller.OrganizationMemberships[orgID]; ok && om.Role == "org_admin" {
		return nil
	}
	return ErrForbidden
}

func (m *WorkspaceMembershipManager) authorizeAdmin(ctx context.Context, caller Caller, workspaceID uuid.UUID) error {
	if wm, ok := caller.WorkspaceMemberships[workspaceID]; ok && wm.Role == "workspace_admin" {
		return nil
	}
	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if om, ok := caller.OrganizationMemberships[orgID]; ok && om.Role == "org_admin" {
		return nil
	}
	return ErrForbidden
}

func isRemovingWsAdmin(membership repository.WorkspaceMembershipFullRow, input UpdateWorkspaceMembershipInput) bool {
	if membership.Role != "workspace_admin" || membership.MembershipStatus != "active" {
		return false
	}
	if input.Role != nil && *input.Role != "workspace_admin" {
		return true
	}
	if input.Status != nil && (*input.Status == "archived" || *input.Status == "suspended") {
		return true
	}
	return false
}

func validateWsMembershipTransition(from, to string) error {
	valid := map[string][]string{
		"invited":   {"active", "archived"},
		"active":    {"suspended", "archived"},
		"suspended": {"active", "archived"},
	}
	for _, allowed := range valid[from] {
		if to == allowed {
			return nil
		}
	}
	return ErrInvalidTransition
}

func wsMembershipRowToResult(row repository.WorkspaceMembershipFullRow) WorkspaceMembershipResult {
	return WorkspaceMembershipResult{
		ID:               row.ID,
		WorkspaceID:      row.WorkspaceID,
		OrganizationID:   row.OrganizationID,
		UserID:           row.UserID,
		Email:            row.Email,
		DisplayName:      row.DisplayName,
		Role:             row.Role,
		MembershipStatus: row.MembershipStatus,
		CreatedAt:        row.CreatedAt,
	}
}

// --- Handlers ---

func listWorkspaceMembershipsHandler(logger *slog.Logger, service WorkspaceMembershipService) http.HandlerFunc {
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

		result, err := service.ListWorkspaceMemberships(r.Context(), caller, workspaceID, limit, offset)
		if err != nil {
			handleMembershipError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func inviteWorkspaceMemberHandler(logger *slog.Logger, service WorkspaceMembershipService) http.HandlerFunc {
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
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
			return
		}
		if req.Email == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "email is required")
			return
		}
		if req.Role != "workspace_admin" && req.Role != "workspace_member" && req.Role != "workspace_viewer" {
			writeError(w, http.StatusBadRequest, "validation_error", "role must be workspace_admin, workspace_member, or workspace_viewer")
			return
		}

		result, err := service.InviteWorkspaceMember(r.Context(), caller, workspaceID, InviteWorkspaceMemberInput{
			Email: req.Email,
			Role:  req.Role,
		})
		if err != nil {
			handleMembershipError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusCreated, result)
	}
}

func updateWorkspaceMembershipHandler(logger *slog.Logger, service WorkspaceMembershipService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		membershipID, err := uuid.Parse(chi.URLParam(r, "membershipID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_membership_id", "membership ID is malformed")
			return
		}

		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		var req struct {
			Role   *string `json:"role,omitempty"`
			Status *string `json:"status,omitempty"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
			return
		}
		if req.Role == nil && req.Status == nil {
			writeError(w, http.StatusBadRequest, "validation_error", "at least one of role or status is required")
			return
		}
		if req.Role != nil && *req.Role != "workspace_admin" && *req.Role != "workspace_member" && *req.Role != "workspace_viewer" {
			writeError(w, http.StatusBadRequest, "validation_error", "role must be workspace_admin, workspace_member, or workspace_viewer")
			return
		}

		result, err := service.UpdateWorkspaceMembership(r.Context(), caller, membershipID, UpdateWorkspaceMembershipInput{
			Role:   req.Role,
			Status: req.Status,
		})
		if err != nil {
			handleMembershipError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}
