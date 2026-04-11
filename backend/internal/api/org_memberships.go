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

type OrgMembershipService interface {
	ListOrgMemberships(ctx context.Context, caller Caller, orgID uuid.UUID, limit, offset int32) (ListOrgMembershipsResult, error)
	InviteOrgMember(ctx context.Context, caller Caller, orgID uuid.UUID, input InviteOrgMemberInput) (OrgMembershipResult, error)
	UpdateOrgMembership(ctx context.Context, caller Caller, membershipID uuid.UUID, input UpdateOrgMembershipInput) (OrgMembershipResult, error)
}

type InviteOrgMemberInput struct {
	Email string
	Role  string
}

type UpdateOrgMembershipInput struct {
	Role   *string
	Status *string
}

type OrgMembershipResult struct {
	ID               uuid.UUID `json:"id"`
	OrganizationID   uuid.UUID `json:"organization_id"`
	UserID           uuid.UUID `json:"user_id"`
	Email            string    `json:"email"`
	DisplayName      string    `json:"display_name"`
	Role             string    `json:"role"`
	MembershipStatus string    `json:"membership_status"`
	CreatedAt        time.Time `json:"created_at"`
}

type ListOrgMembershipsResult struct {
	Items  []OrgMembershipResult `json:"items"`
	Total  int64                 `json:"total"`
	Limit  int32                 `json:"limit"`
	Offset int32                 `json:"offset"`
}

type OrgMembershipRepository interface {
	ListOrgMemberships(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]repository.OrgMembershipFullRow, error)
	CountOrgMemberships(ctx context.Context, orgID uuid.UUID) (int64, error)
	GetUserByEmail(ctx context.Context, email string) (repository.User, error)
	CreateUser(ctx context.Context, input repository.CreateUserInput) (repository.User, error)
	GetOrgMembershipByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID) (repository.OrgMembershipFullRow, error)
	CreateOrgMembership(ctx context.Context, input repository.CreateOrgMembershipInput) (repository.OrgMembershipFullRow, error)
	GetOrgMembershipByID(ctx context.Context, membershipID uuid.UUID) (repository.OrgMembershipFullRow, error)
	UpdateOrgMembership(ctx context.Context, membershipID uuid.UUID, input repository.UpdateOrgMembershipInput) (repository.OrgMembershipFullRow, error)
	CountActiveOrgAdmins(ctx context.Context, orgID uuid.UUID) (int64, error)
	CascadeOrgMembershipStatusToWorkspaces(ctx context.Context, orgID, userID uuid.UUID, status string) error
}

const inviteExpiryDays = 7

type OrgMembershipManager struct {
	orgAuthz OrganizationAuthorizer
	repo     OrgMembershipRepository
}

func NewOrgMembershipManager(orgAuthz OrganizationAuthorizer, repo OrgMembershipRepository) *OrgMembershipManager {
	return &OrgMembershipManager{orgAuthz: orgAuthz, repo: repo}
}

func (m *OrgMembershipManager) ListOrgMemberships(ctx context.Context, caller Caller, orgID uuid.UUID, limit, offset int32) (ListOrgMembershipsResult, error) {
	if err := m.orgAuthz.AuthorizeOrganization(ctx, caller, orgID); err != nil {
		return ListOrgMembershipsResult{}, err
	}

	memberships, err := m.repo.ListOrgMemberships(ctx, orgID, limit, offset)
	if err != nil {
		return ListOrgMembershipsResult{}, err
	}

	total, err := m.repo.CountOrgMemberships(ctx, orgID)
	if err != nil {
		return ListOrgMembershipsResult{}, err
	}

	items := make([]OrgMembershipResult, 0, len(memberships))
	for _, row := range memberships {
		items = append(items, orgMembershipRowToResult(row))
	}

	return ListOrgMembershipsResult{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (m *OrgMembershipManager) InviteOrgMember(ctx context.Context, caller Caller, orgID uuid.UUID, input InviteOrgMemberInput) (OrgMembershipResult, error) {
	if err := m.orgAuthz.AuthorizeOrganizationAdmin(ctx, caller, orgID); err != nil {
		return OrgMembershipResult{}, err
	}

	// Look up user by email. Create stub if not found.
	user, err := m.repo.GetUserByEmail(ctx, input.Email)
	if errors.Is(err, repository.ErrUserNotFound) {
		user, err = m.repo.CreateUser(ctx, repository.CreateUserInput{
			WorkOSUserID: "pending:" + uuid.New().String(), // placeholder — linked on first WorkOS login
			Email:        input.Email,
		})
		if err != nil && !errors.Is(err, repository.ErrUserAlreadyExists) {
			return OrgMembershipResult{}, err
		}
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			// Race: another request created the user. Fetch it.
			user, err = m.repo.GetUserByEmail(ctx, input.Email)
			if err != nil {
				return OrgMembershipResult{}, err
			}
		}
	} else if err != nil {
		return OrgMembershipResult{}, err
	}

	// Check for existing membership.
	existing, err := m.repo.GetOrgMembershipByOrgAndUser(ctx, orgID, user.ID)
	if err == nil {
		if existing.MembershipStatus == "active" || existing.MembershipStatus == "invited" {
			return OrgMembershipResult{}, repository.ErrAlreadyMember
		}
		// Previously archived — allow re-invite by updating status.
		result, err := m.repo.UpdateOrgMembership(ctx, existing.ID, repository.UpdateOrgMembershipInput{
			Role:   &input.Role,
			Status: strPtr("invited"),
		})
		if err != nil {
			return OrgMembershipResult{}, err
		}
		return orgMembershipRowToResult(result), nil
	}
	if !errors.Is(err, repository.ErrMembershipNotFound) {
		return OrgMembershipResult{}, err
	}

	result, err := m.repo.CreateOrgMembership(ctx, repository.CreateOrgMembershipInput{
		OrganizationID: orgID,
		UserID:         user.ID,
		Role:           input.Role,
	})
	if err != nil {
		return OrgMembershipResult{}, err
	}

	// TODO: Send invitation email notification (stubbed for now).

	return orgMembershipRowToResult(result), nil
}

func (m *OrgMembershipManager) UpdateOrgMembership(ctx context.Context, caller Caller, membershipID uuid.UUID, input UpdateOrgMembershipInput) (OrgMembershipResult, error) {
	membership, err := m.repo.GetOrgMembershipByID(ctx, membershipID)
	if err != nil {
		return OrgMembershipResult{}, err
	}

	// Authorization: org_admin of the org, or the invited user accepting their invite.
	isAdmin := false
	if m, ok := caller.OrganizationMemberships[membership.OrganizationID]; ok && m.Role == "org_admin" {
		isAdmin = true
	}
	isInvitedUser := membership.UserID == caller.UserID && membership.MembershipStatus == "invited"

	if !isAdmin && !isInvitedUser {
		return OrgMembershipResult{}, ErrForbidden
	}

	// Invited user can only accept (invited -> active).
	if isInvitedUser && !isAdmin {
		if input.Status == nil || *input.Status != "active" {
			return OrgMembershipResult{}, ErrForbidden
		}
		// Check invite expiry. Use UpdatedAt because re-invites refresh it
		// via the DB trigger, while CreatedAt is the original row creation.
		invitedAt := membership.UpdatedAt
		if invitedAt.IsZero() {
			invitedAt = membership.CreatedAt
		}
		if time.Since(invitedAt) > inviteExpiryDays*24*time.Hour {
			return OrgMembershipResult{}, repository.ErrInviteExpired
		}
	}

	// Validate status transitions.
	if input.Status != nil {
		if err := validateOrgMembershipTransition(membership.MembershipStatus, *input.Status); err != nil {
			return OrgMembershipResult{}, err
		}
	}

	// Cannot change own role.
	if input.Role != nil && membership.UserID == caller.UserID {
		return OrgMembershipResult{}, ErrForbidden
	}

	// Last admin protection.
	if isRemovingAdmin(membership, input) {
		count, err := m.repo.CountActiveOrgAdmins(ctx, membership.OrganizationID)
		if err != nil {
			return OrgMembershipResult{}, err
		}
		if count <= 1 {
			return OrgMembershipResult{}, repository.ErrLastOrgAdmin
		}
	}

	result, err := m.repo.UpdateOrgMembership(ctx, membershipID, repository.UpdateOrgMembershipInput{
		Role:   input.Role,
		Status: input.Status,
	})
	if err != nil {
		return OrgMembershipResult{}, err
	}

	// When archiving or suspending an org member, also revoke their workspace
	// access in this org so they can't reach workspace-scoped endpoints.
	if input.Status != nil && (*input.Status == "archived" || *input.Status == "suspended") {
		if err := m.repo.CascadeOrgMembershipStatusToWorkspaces(ctx, membership.OrganizationID, membership.UserID, *input.Status); err != nil {
			return OrgMembershipResult{}, err
		}
	}

	return orgMembershipRowToResult(result), nil
}

func isRemovingAdmin(membership repository.OrgMembershipFullRow, input UpdateOrgMembershipInput) bool {
	if membership.Role != "org_admin" || membership.MembershipStatus != "active" {
		return false
	}
	// Demoting from org_admin to org_member.
	if input.Role != nil && *input.Role != "org_admin" {
		return true
	}
	// Archiving or suspending an active admin.
	if input.Status != nil && (*input.Status == "archived" || *input.Status == "suspended") {
		return true
	}
	return false
}

func validateOrgMembershipTransition(from, to string) error {
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

var ErrInvalidTransition = errors.New("invalid status transition")

func orgMembershipRowToResult(row repository.OrgMembershipFullRow) OrgMembershipResult {
	return OrgMembershipResult{
		ID:               row.ID,
		OrganizationID:   row.OrganizationID,
		UserID:           row.UserID,
		Email:            row.Email,
		DisplayName:      row.DisplayName,
		Role:             row.Role,
		MembershipStatus: row.MembershipStatus,
		CreatedAt:        row.CreatedAt,
	}
}

func strPtr(s string) *string { return &s }

// --- Handlers ---

func listOrgMembershipsHandler(logger *slog.Logger, service OrgMembershipService) http.HandlerFunc {
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

		result, err := service.ListOrgMemberships(r.Context(), caller, orgID, limit, offset)
		if err != nil {
			handleMembershipError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func inviteOrgMemberHandler(logger *slog.Logger, service OrgMembershipService) http.HandlerFunc {
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
		if req.Role != "org_admin" && req.Role != "org_member" {
			writeError(w, http.StatusBadRequest, "validation_error", "role must be org_admin or org_member")
			return
		}

		result, err := service.InviteOrgMember(r.Context(), caller, orgID, InviteOrgMemberInput{
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

func updateOrgMembershipHandler(logger *slog.Logger, service OrgMembershipService) http.HandlerFunc {
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
		if req.Role != nil && *req.Role != "org_admin" && *req.Role != "org_member" {
			writeError(w, http.StatusBadRequest, "validation_error", "role must be org_admin or org_member")
			return
		}

		result, err := service.UpdateOrgMembership(r.Context(), caller, membershipID, UpdateOrgMembershipInput{
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

func handleMembershipError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
	case errors.Is(err, repository.ErrMembershipNotFound):
		writeError(w, http.StatusNotFound, "not_found", "membership not found")
	case errors.Is(err, repository.ErrAlreadyMember):
		writeError(w, http.StatusConflict, "already_a_member", "user is already a member")
	case errors.Is(err, repository.ErrLastOrgAdmin):
		writeError(w, http.StatusConflict, "last_org_admin", "cannot remove or demote the last organization admin")
	case errors.Is(err, repository.ErrLastWorkspaceAdmin):
		writeError(w, http.StatusConflict, "last_workspace_admin", "cannot remove or demote the last workspace admin")
	case errors.Is(err, repository.ErrOrgMembershipRequired):
		writeError(w, http.StatusBadRequest, "org_membership_required", "user must be a member of the organization first")
	case errors.Is(err, repository.ErrInviteExpired):
		writeError(w, http.StatusGone, "invite_expired", "invitation has expired")
	case errors.Is(err, ErrInvalidTransition):
		writeError(w, http.StatusBadRequest, "invalid_transition", "invalid membership status transition")
	default:
		logger.Error("membership operation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
