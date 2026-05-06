package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/agentclash/agentclash/backend/internal/email"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type WorkspaceMembershipService interface {
	ListWorkspaceMemberships(ctx context.Context, caller Caller, workspaceID uuid.UUID, limit, offset int32) (ListWorkspaceMembershipsResult, error)
	InviteWorkspaceMember(ctx context.Context, caller Caller, workspaceID uuid.UUID, input InviteWorkspaceMemberInput) (WorkspaceMembershipResult, error)
	UpdateWorkspaceMembership(ctx context.Context, caller Caller, membershipID uuid.UUID, input UpdateWorkspaceMembershipInput) (WorkspaceMembershipResult, error)
	AcceptWorkspaceInvite(ctx context.Context, caller Caller, inviteToken string) (WorkspaceMembershipResult, error)
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
	AcceptURL        string    `json:"accept_url,omitempty"`
}

type ListWorkspaceMembershipsResult struct {
	Items  []WorkspaceMembershipResult `json:"items"`
	Total  int64                       `json:"total"`
	Limit  int32                       `json:"limit"`
	Offset int32                       `json:"offset"`
}

type WorkspaceMembershipRepository interface {
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	GetWorkspaceByID(ctx context.Context, workspaceID uuid.UUID) (repository.WorkspaceRow, error)
	ListWorkspaceMemberships(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) ([]repository.WorkspaceMembershipFullRow, error)
	CountWorkspaceMemberships(ctx context.Context, workspaceID uuid.UUID) (int64, error)
	GetUserByEmail(ctx context.Context, email string) (repository.User, error)
	CreateUser(ctx context.Context, input repository.CreateUserInput) (repository.User, error)
	GetOrgMembershipByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID) (repository.OrgMembershipFullRow, error)
	CreateOrgMembership(ctx context.Context, input repository.CreateOrgMembershipInput) (repository.OrgMembershipFullRow, error)
	UpdateOrgMembership(ctx context.Context, membershipID uuid.UUID, input repository.UpdateOrgMembershipInput) (repository.OrgMembershipFullRow, error)
	GetWorkspaceMembershipByWorkspaceAndUser(ctx context.Context, workspaceID, userID uuid.UUID) (repository.WorkspaceMembershipFullRow, error)
	CreateWorkspaceMembership(ctx context.Context, input repository.CreateWorkspaceMembershipInput) (repository.WorkspaceMembershipFullRow, error)
	GetWorkspaceMembershipByID(ctx context.Context, membershipID uuid.UUID) (repository.WorkspaceMembershipFullRow, error)
	GetWorkspaceMembershipByInviteToken(ctx context.Context, inviteToken string) (repository.WorkspaceMembershipFullRow, error)
	UpdateWorkspaceMembership(ctx context.Context, membershipID uuid.UUID, input repository.UpdateWorkspaceMembershipInput) (repository.WorkspaceMembershipFullRow, error)
	CountActiveWorkspaceAdmins(ctx context.Context, workspaceID uuid.UUID) (int64, error)
}

type WorkspaceMembershipManager struct {
	repo            WorkspaceMembershipRepository
	emailSender     email.Sender
	frontendURL     string
	entitlementGate EntitlementGateService
}

func NewWorkspaceMembershipManager(repo WorkspaceMembershipRepository, emailSender email.Sender, frontendURL string, entitlementGate ...EntitlementGateService) *WorkspaceMembershipManager {
	var gate EntitlementGateService
	if len(entitlementGate) > 0 {
		gate = entitlementGate[0]
	}
	if emailSender == nil {
		emailSender = email.NoopSender{}
	}
	return &WorkspaceMembershipManager{repo: repo, emailSender: emailSender, frontendURL: frontendURL, entitlementGate: gate}
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
	includeInviteLinks := m.callerCanCopyWorkspaceInviteLinks(ctx, caller, workspaceID)
	for _, row := range memberships {
		items = append(items, m.wsMembershipRowToResult(row, includeInviteLinks))
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

	if _, err := m.ensureWorkspaceInviteOrgMembership(ctx, orgID, user.ID); err != nil {
		return WorkspaceMembershipResult{}, err
	}

	// Check existing workspace membership.
	existing, err := m.repo.GetWorkspaceMembershipByWorkspaceAndUser(ctx, workspaceID, user.ID)
	if err == nil {
		if existing.MembershipStatus == "active" || existing.MembershipStatus == "invited" {
			return WorkspaceMembershipResult{}, repository.ErrAlreadyMember
		}
		inviteToken, inviteTokenExpiresAt, err := newMembershipInviteToken()
		if err != nil {
			return WorkspaceMembershipResult{}, err
		}
		// Previously archived — re-invite.
		result, err := m.repo.UpdateWorkspaceMembership(ctx, existing.ID, repository.UpdateWorkspaceMembershipInput{
			Role:                 &input.Role,
			Status:               strPtr("invited"),
			InviteToken:          &inviteToken,
			InviteTokenExpiresAt: &inviteTokenExpiresAt,
		})
		if err != nil {
			return WorkspaceMembershipResult{}, err
		}
		acceptURL := workspaceInviteAcceptURL(m.frontendURL, result.InviteToken)
		m.sendInviteEmail(ctx, caller, workspaceID, input.Email, input.Role, acceptURL)
		return m.wsMembershipRowToResult(result, true), nil
	}
	if !errors.Is(err, repository.ErrMembershipNotFound) {
		return WorkspaceMembershipResult{}, err
	}

	inviteToken, inviteTokenExpiresAt, err := newMembershipInviteToken()
	if err != nil {
		return WorkspaceMembershipResult{}, err
	}
	result, err := m.repo.CreateWorkspaceMembership(ctx, repository.CreateWorkspaceMembershipInput{
		OrganizationID:       orgID,
		WorkspaceID:          workspaceID,
		UserID:               user.ID,
		Role:                 input.Role,
		InviteToken:          inviteToken,
		InviteTokenExpiresAt: inviteTokenExpiresAt,
	})
	if err != nil {
		return WorkspaceMembershipResult{}, err
	}

	acceptURL := workspaceInviteAcceptURL(m.frontendURL, result.InviteToken)
	m.sendInviteEmail(ctx, caller, workspaceID, input.Email, input.Role, acceptURL)
	return m.wsMembershipRowToResult(result, true), nil
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
	isLegacyInviteLink := membership.InviteToken == ""
	isInvitedUser := isLegacyInviteLink && membership.UserID == caller.UserID && membership.MembershipStatus == "invited"
	isInviteAcceptance := input.Role == nil && input.Status != nil && *input.Status == "active" && membership.MembershipStatus == "invited"
	isInviteLinkHolder := isLegacyInviteLink && !isAdmin && !isInvitedUser && isInviteAcceptance && inviteEmailMatchesCaller(caller, membership.Email)

	if !isAdmin && !isInvitedUser && !isInviteLinkHolder {
		return WorkspaceMembershipResult{}, ErrForbidden
	}

	if (isInvitedUser || isInviteLinkHolder) && !isAdmin {
		if !isInviteAcceptance {
			return WorkspaceMembershipResult{}, ErrForbidden
		}
		if wsInviteExpired(membership) {
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

	updateInput := repository.UpdateWorkspaceMembershipInput{
		Role:   input.Role,
		Status: input.Status,
	}
	if isInviteLinkHolder {
		updateInput.UserID = &caller.UserID
	}
	if input.Status != nil && (*input.Status == "active" || *input.Status == "archived") && membership.MembershipStatus == "invited" {
		updateInput.ClearInviteToken = true
	}

	result, err := m.repo.UpdateWorkspaceMembership(ctx, membershipID, updateInput)
	if errors.Is(err, repository.ErrOrgMembershipRequired) && updateInput.UserID != nil && input.Status != nil && *input.Status == "active" {
		if _, ensureErr := m.ensureActiveOrgMembership(ctx, membership.OrganizationID, caller.UserID); ensureErr != nil {
			return WorkspaceMembershipResult{}, ensureErr
		}
		result, err = m.repo.UpdateWorkspaceMembership(ctx, membershipID, updateInput)
	}
	if err != nil {
		return WorkspaceMembershipResult{}, err
	}

	return m.wsMembershipRowToResult(result, isAdmin), nil
}

func (m *WorkspaceMembershipManager) AcceptWorkspaceInvite(ctx context.Context, caller Caller, inviteToken string) (WorkspaceMembershipResult, error) {
	if !validInviteToken(inviteToken) {
		return WorkspaceMembershipResult{}, repository.ErrMembershipNotFound
	}

	membership, err := m.repo.GetWorkspaceMembershipByInviteToken(ctx, inviteToken)
	if err != nil {
		return WorkspaceMembershipResult{}, err
	}
	if !inviteEmailMatchesCaller(caller, membership.Email) {
		return WorkspaceMembershipResult{}, ErrForbidden
	}
	if wsInviteExpired(membership) {
		return WorkspaceMembershipResult{}, repository.ErrInviteExpired
	}

	if _, err := m.ensureActiveOrgMembership(ctx, membership.OrganizationID, caller.UserID); err != nil {
		return WorkspaceMembershipResult{}, err
	}

	status := "active"
	updateInput := repository.UpdateWorkspaceMembershipInput{
		Status:           &status,
		ClearInviteToken: true,
	}
	if membership.UserID != caller.UserID {
		updateInput.UserID = &caller.UserID
	}

	result, err := m.repo.UpdateWorkspaceMembership(ctx, membership.ID, updateInput)
	if err != nil {
		return WorkspaceMembershipResult{}, err
	}
	return m.wsMembershipRowToResult(result, false), nil
}

func (m *WorkspaceMembershipManager) ensureWorkspaceInviteOrgMembership(ctx context.Context, orgID, userID uuid.UUID) (repository.OrgMembershipFullRow, error) {
	orgMembership, err := m.repo.GetOrgMembershipByOrgAndUser(ctx, orgID, userID)
	if err == nil {
		switch orgMembership.MembershipStatus {
		case "active", "invited":
			return orgMembership, nil
		case "archived":
			return m.reinviteOrgMembershipForWorkspace(ctx, orgMembership)
		default:
			return repository.OrgMembershipFullRow{}, repository.ErrOrgMembershipRequired
		}
	}
	if !errors.Is(err, repository.ErrMembershipNotFound) {
		return repository.OrgMembershipFullRow{}, err
	}

	inviteToken, inviteTokenExpiresAt, err := newMembershipInviteToken()
	if err != nil {
		return repository.OrgMembershipFullRow{}, err
	}
	var entitlementGate *repository.OrganizationEntitlementGate
	if m.entitlementGate != nil {
		entitlementGate, err = m.entitlementGate.BuildSeatGate(ctx, orgID, false)
		if err != nil {
			return repository.OrgMembershipFullRow{}, err
		}
	}
	return m.repo.CreateOrgMembership(ctx, repository.CreateOrgMembershipInput{
		OrganizationID:       orgID,
		UserID:               userID,
		Role:                 "org_member",
		InviteToken:          inviteToken,
		InviteTokenExpiresAt: inviteTokenExpiresAt,
		EntitlementGate:      entitlementGate,
	})
}

func (m *WorkspaceMembershipManager) reinviteOrgMembershipForWorkspace(ctx context.Context, orgMembership repository.OrgMembershipFullRow) (repository.OrgMembershipFullRow, error) {
	inviteToken, inviteTokenExpiresAt, err := newMembershipInviteToken()
	if err != nil {
		return repository.OrgMembershipFullRow{}, err
	}
	var entitlementGate *repository.OrganizationEntitlementGate
	if m.entitlementGate != nil {
		entitlementGate, err = m.entitlementGate.BuildSeatGate(ctx, orgMembership.OrganizationID, false)
		if err != nil {
			return repository.OrgMembershipFullRow{}, err
		}
	}
	role := "org_member"
	status := "invited"
	return m.repo.UpdateOrgMembership(ctx, orgMembership.ID, repository.UpdateOrgMembershipInput{
		Role:                 &role,
		Status:               &status,
		InviteToken:          &inviteToken,
		InviteTokenExpiresAt: &inviteTokenExpiresAt,
		EntitlementGate:      entitlementGate,
	})
}

func (m *WorkspaceMembershipManager) ensureActiveOrgMembership(ctx context.Context, orgID, userID uuid.UUID) (repository.OrgMembershipFullRow, error) {
	orgMembership, err := m.repo.GetOrgMembershipByOrgAndUser(ctx, orgID, userID)
	if errors.Is(err, repository.ErrMembershipNotFound) {
		inviteToken, inviteTokenExpiresAt, tokenErr := newMembershipInviteToken()
		if tokenErr != nil {
			return repository.OrgMembershipFullRow{}, tokenErr
		}
		var entitlementGate *repository.OrganizationEntitlementGate
		if m.entitlementGate != nil {
			entitlementGate, err = m.entitlementGate.BuildSeatGate(ctx, orgID, false)
			if err != nil {
				return repository.OrgMembershipFullRow{}, err
			}
		}
		orgMembership, err = m.repo.CreateOrgMembership(ctx, repository.CreateOrgMembershipInput{
			OrganizationID:       orgID,
			UserID:               userID,
			Role:                 "org_member",
			InviteToken:          inviteToken,
			InviteTokenExpiresAt: inviteTokenExpiresAt,
			EntitlementGate:      entitlementGate,
		})
	}
	if err != nil {
		return repository.OrgMembershipFullRow{}, err
	}
	if orgMembership.MembershipStatus == "active" {
		return orgMembership, nil
	}
	if orgMembership.MembershipStatus != "invited" {
		return repository.OrgMembershipFullRow{}, repository.ErrOrgMembershipRequired
	}

	var entitlementGate *repository.OrganizationEntitlementGate
	if m.entitlementGate != nil {
		entitlementGate, err = m.entitlementGate.BuildSeatGate(ctx, orgID, false)
		if err != nil {
			return repository.OrgMembershipFullRow{}, err
		}
	}
	status := "active"
	return m.repo.UpdateOrgMembership(ctx, orgMembership.ID, repository.UpdateOrgMembershipInput{
		Status:           &status,
		ClearInviteToken: true,
		EntitlementGate:  entitlementGate,
	})
}

func wsInviteExpired(membership repository.WorkspaceMembershipFullRow) bool {
	if membership.InviteTokenExpiresAt != nil {
		return time.Now().After(*membership.InviteTokenExpiresAt)
	}
	invitedAt := membership.UpdatedAt
	if invitedAt.IsZero() {
		invitedAt = membership.CreatedAt
	}
	if invitedAt.IsZero() {
		return false
	}
	return time.Since(invitedAt) > inviteExpiryDays*24*time.Hour
}

func (m *WorkspaceMembershipManager) sendInviteEmail(ctx context.Context, caller Caller, workspaceID uuid.UUID, inviteeEmail, role, acceptURL string) {
	workspace, err := m.repo.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		slog.Default().Warn("failed to load workspace for invite email", "workspace_id", workspaceID, "error", err)
		return
	}

	inviterEmail := caller.Email

	if err := m.emailSender.SendInvite(ctx, email.InviteEmail{
		To:           inviteeEmail,
		ResourceName: workspace.Name,
		ResourceKind: "workspace",
		InviterName:  caller.DisplayName,
		InviterEmail: inviterEmail,
		Role:         role,
		AcceptURL:    acceptURL,
	}); err != nil {
		slog.Default().Warn("failed to send invite email", "to", inviteeEmail, "workspace_id", workspaceID, "error", err)
	}
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

func (m *WorkspaceMembershipManager) callerCanCopyWorkspaceInviteLinks(ctx context.Context, caller Caller, workspaceID uuid.UUID) bool {
	if wm, ok := caller.WorkspaceMemberships[workspaceID]; ok && wm.Role == "workspace_admin" {
		return true
	}
	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		slog.Default().Warn("failed to load organization for invite link visibility", "workspace_id", workspaceID, "error", err)
		return false
	}
	if om, ok := caller.OrganizationMemberships[orgID]; ok && om.Role == "org_admin" {
		return true
	}
	return false
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

func (m *WorkspaceMembershipManager) wsMembershipRowToResult(row repository.WorkspaceMembershipFullRow, includeInviteLink bool) WorkspaceMembershipResult {
	result := WorkspaceMembershipResult{
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
	if includeInviteLink && row.MembershipStatus == "invited" && !wsInviteExpired(row) {
		inviteToken := row.InviteToken
		if inviteToken == "" {
			inviteToken = row.ID.String()
		}
		result.AcceptURL = workspaceInviteAcceptURL(m.frontendURL, inviteToken)
	}
	return result
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

func acceptWorkspaceInviteHandler(logger *slog.Logger, service WorkspaceMembershipService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		inviteToken := chi.URLParam(r, "inviteToken")
		if !validInviteToken(inviteToken) {
			writeError(w, http.StatusBadRequest, "invalid_invite_token", "invite token is malformed")
			return
		}

		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		var req struct {
			Status *string `json:"status,omitempty"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
			return
		}
		if req.Status == nil || *req.Status != "active" {
			writeError(w, http.StatusBadRequest, "validation_error", "status must be active")
			return
		}

		result, err := service.AcceptWorkspaceInvite(r.Context(), caller, inviteToken)
		if err != nil {
			handleMembershipError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}
