package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	billingpkg "github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/email"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type OrgMembershipService interface {
	ListOrgMemberships(ctx context.Context, caller Caller, orgID uuid.UUID, limit, offset int32) (ListOrgMembershipsResult, error)
	InviteOrgMember(ctx context.Context, caller Caller, orgID uuid.UUID, input InviteOrgMemberInput) (OrgMembershipResult, error)
	UpdateOrgMembership(ctx context.Context, caller Caller, membershipID uuid.UUID, input UpdateOrgMembershipInput) (OrgMembershipResult, error)
	AcceptOrgInvite(ctx context.Context, caller Caller, inviteToken string) (OrgMembershipResult, error)
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
	AcceptURL        string    `json:"accept_url,omitempty"`
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
	GetOrganizationByID(ctx context.Context, orgID uuid.UUID) (repository.OrganizationRow, error)
	GetOrgMembershipByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID) (repository.OrgMembershipFullRow, error)
	CreateOrgMembership(ctx context.Context, input repository.CreateOrgMembershipInput) (repository.OrgMembershipFullRow, error)
	GetOrgMembershipByID(ctx context.Context, membershipID uuid.UUID) (repository.OrgMembershipFullRow, error)
	GetOrgMembershipByInviteToken(ctx context.Context, inviteToken string) (repository.OrgMembershipFullRow, error)
	UpdateOrgMembership(ctx context.Context, membershipID uuid.UUID, input repository.UpdateOrgMembershipInput) (repository.OrgMembershipFullRow, error)
	CountActiveOrgAdmins(ctx context.Context, orgID uuid.UUID) (int64, error)
	CascadeOrgMembershipStatusToWorkspaces(ctx context.Context, orgID, userID uuid.UUID, status string) error
}

const inviteExpiryDays = 7

type OrgMembershipManager struct {
	orgAuthz        OrganizationAuthorizer
	repo            OrgMembershipRepository
	emailSender     email.Sender
	frontendURL     string
	entitlementGate EntitlementGateService
}

func NewOrgMembershipManager(orgAuthz OrganizationAuthorizer, repo OrgMembershipRepository, emailSender email.Sender, frontendURL string, entitlementGate ...EntitlementGateService) *OrgMembershipManager {
	var gate EntitlementGateService
	if len(entitlementGate) > 0 {
		gate = entitlementGate[0]
	}
	if emailSender == nil {
		emailSender = email.NoopSender{}
	}
	return &OrgMembershipManager{orgAuthz: orgAuthz, repo: repo, emailSender: emailSender, frontendURL: frontendURL, entitlementGate: gate}
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
	includeInviteLinks := false
	if membership, ok := caller.OrganizationMemberships[orgID]; ok && membership.Role == "org_admin" {
		includeInviteLinks = true
	}
	for _, row := range memberships {
		items = append(items, m.orgMembershipRowToResult(row, includeInviteLinks))
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
		inviteToken, inviteTokenExpiresAt, err := newMembershipInviteToken()
		if err != nil {
			return OrgMembershipResult{}, err
		}
		var entitlementGate *repository.OrganizationEntitlementGate
		if m.entitlementGate != nil {
			entitlementGate, err = m.entitlementGate.BuildSeatGate(ctx, orgID, false)
			if err != nil {
				return OrgMembershipResult{}, err
			}
		}
		// Previously archived — allow re-invite by updating status.
		result, err := m.repo.UpdateOrgMembership(ctx, existing.ID, repository.UpdateOrgMembershipInput{
			Role:                 &input.Role,
			Status:               strPtr("invited"),
			InviteToken:          &inviteToken,
			InviteTokenExpiresAt: &inviteTokenExpiresAt,
			EntitlementGate:      entitlementGate,
		})
		if err != nil {
			return OrgMembershipResult{}, err
		}
		acceptURL := organizationInviteAcceptURL(m.frontendURL, result.InviteToken)
		m.sendInviteEmail(ctx, caller, orgID, input.Email, input.Role, acceptURL)
		return m.orgMembershipRowToResult(result, true), nil
	}
	if !errors.Is(err, repository.ErrMembershipNotFound) {
		return OrgMembershipResult{}, err
	}
	var entitlementGate *repository.OrganizationEntitlementGate
	if m.entitlementGate != nil {
		entitlementGate, err = m.entitlementGate.BuildSeatGate(ctx, orgID, false)
		if err != nil {
			return OrgMembershipResult{}, err
		}
	}

	inviteToken, inviteTokenExpiresAt, err := newMembershipInviteToken()
	if err != nil {
		return OrgMembershipResult{}, err
	}
	result, err := m.repo.CreateOrgMembership(ctx, repository.CreateOrgMembershipInput{
		OrganizationID:       orgID,
		UserID:               user.ID,
		Role:                 input.Role,
		InviteToken:          inviteToken,
		InviteTokenExpiresAt: inviteTokenExpiresAt,
		EntitlementGate:      entitlementGate,
	})
	if err != nil {
		return OrgMembershipResult{}, err
	}

	acceptURL := organizationInviteAcceptURL(m.frontendURL, result.InviteToken)
	m.sendInviteEmail(ctx, caller, orgID, input.Email, input.Role, acceptURL)
	return m.orgMembershipRowToResult(result, true), nil
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
	isLegacyInviteLink := membership.InviteToken == ""
	isInvitedUser := isLegacyInviteLink && membership.UserID == caller.UserID && membership.MembershipStatus == "invited"
	isInviteAcceptance := input.Role == nil && input.Status != nil && *input.Status == "active" && membership.MembershipStatus == "invited"
	isInviteLinkHolder := isLegacyInviteLink && !isAdmin && !isInvitedUser && isInviteAcceptance && inviteEmailMatchesCaller(caller, membership.Email)

	if !isAdmin && !isInvitedUser && !isInviteLinkHolder {
		return OrgMembershipResult{}, ErrForbidden
	}

	// Non-admins can only accept a pending invite link.
	if (isInvitedUser || isInviteLinkHolder) && !isAdmin {
		if !isInviteAcceptance {
			return OrgMembershipResult{}, ErrForbidden
		}
		if orgInviteExpired(membership) {
			return OrgMembershipResult{}, repository.ErrInviteExpired
		}
	}

	// Validate status transitions.
	var entitlementGate *repository.OrganizationEntitlementGate
	if input.Status != nil {
		if err := validateOrgMembershipTransition(membership.MembershipStatus, *input.Status); err != nil {
			return OrgMembershipResult{}, err
		}
		if *input.Status == "active" && membership.MembershipStatus != "active" && m.entitlementGate != nil {
			entitlementGate, err = m.entitlementGate.BuildSeatGate(ctx, membership.OrganizationID, false)
			if err != nil {
				return OrgMembershipResult{}, err
			}
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

	updateInput := repository.UpdateOrgMembershipInput{
		Role:            input.Role,
		Status:          input.Status,
		EntitlementGate: entitlementGate,
	}
	if isInviteLinkHolder {
		updateInput.UserID = &caller.UserID
	}
	if input.Status != nil && (*input.Status == "active" || *input.Status == "archived") && membership.MembershipStatus == "invited" {
		updateInput.ClearInviteToken = true
	}

	result, err := m.repo.UpdateOrgMembership(ctx, membershipID, updateInput)
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

	return m.orgMembershipRowToResult(result, isAdmin), nil
}

func (m *OrgMembershipManager) AcceptOrgInvite(ctx context.Context, caller Caller, inviteToken string) (OrgMembershipResult, error) {
	if !validInviteToken(inviteToken) {
		return OrgMembershipResult{}, repository.ErrMembershipNotFound
	}

	membership, err := m.repo.GetOrgMembershipByInviteToken(ctx, inviteToken)
	if err != nil {
		return OrgMembershipResult{}, err
	}
	if err := inviteTokenAcceptError(caller, membership.Email); err != nil {
		return OrgMembershipResult{}, withInviteAcceptDenialContext(err, "organization", caller.UserID, membership.ID, membership.OrganizationID, uuid.Nil)
	}
	if orgInviteExpired(membership) {
		return OrgMembershipResult{}, repository.ErrInviteExpired
	}

	var entitlementGate *repository.OrganizationEntitlementGate
	if m.entitlementGate != nil {
		entitlementGate, err = m.entitlementGate.BuildSeatGate(ctx, membership.OrganizationID, false)
		if err != nil {
			return OrgMembershipResult{}, err
		}
	}

	status := "active"
	updateInput := repository.UpdateOrgMembershipInput{
		Status:           &status,
		ClearInviteToken: true,
		EntitlementGate:  entitlementGate,
	}
	if membership.UserID != caller.UserID {
		updateInput.UserID = &caller.UserID
	}

	result, err := m.repo.UpdateOrgMembership(ctx, membership.ID, updateInput)
	if err != nil {
		return OrgMembershipResult{}, err
	}
	return m.orgMembershipRowToResult(result, false), nil
}

func inviteEmailMatchesCaller(caller Caller, inviteEmail string) bool {
	return strings.EqualFold(strings.TrimSpace(caller.Email), strings.TrimSpace(inviteEmail)) && strings.TrimSpace(caller.Email) != ""
}

type inviteTokenAcceptDeniedError struct {
	Kind               string
	Reason             string
	CallerUserID       uuid.UUID
	MembershipID       uuid.UUID
	OrganizationID     uuid.UUID
	WorkspaceID        uuid.UUID
	CallerEmailPresent bool
	InviteEmailPresent bool
	EmailMatch         bool
}

func (e *inviteTokenAcceptDeniedError) Error() string {
	return ErrForbidden.Error() + ": invite token accept denied: " + e.Reason
}

func (e *inviteTokenAcceptDeniedError) Unwrap() error {
	return ErrForbidden
}

func inviteTokenAcceptError(caller Caller, inviteEmail string) error {
	inviteEmail = strings.TrimSpace(inviteEmail)
	callerEmail := strings.TrimSpace(caller.Email)
	if inviteEmail == "" {
		return &inviteTokenAcceptDeniedError{
			Reason:             "invite_email_empty",
			CallerEmailPresent: callerEmail != "",
			InviteEmailPresent: false,
			EmailMatch:         false,
		}
	}
	if callerEmail == "" {
		// The invite token is a high-entropy bearer secret. Some WorkOS session
		// tokens do not expose email claims, so token possession is the fallback.
		return nil
	}
	if !strings.EqualFold(callerEmail, inviteEmail) {
		return &inviteTokenAcceptDeniedError{
			Reason:             "caller_email_mismatch",
			CallerEmailPresent: true,
			InviteEmailPresent: true,
			EmailMatch:         false,
		}
	}
	return nil
}

func withInviteAcceptDenialContext(err error, kind string, callerUserID, membershipID, organizationID, workspaceID uuid.UUID) error {
	var denial *inviteTokenAcceptDeniedError
	if !errors.As(err, &denial) {
		return err
	}
	withContext := *denial
	withContext.Kind = kind
	withContext.CallerUserID = callerUserID
	withContext.MembershipID = membershipID
	withContext.OrganizationID = organizationID
	withContext.WorkspaceID = workspaceID
	return &withContext
}

func orgInviteExpired(membership repository.OrgMembershipFullRow) bool {
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

func (m *OrgMembershipManager) sendInviteEmail(ctx context.Context, caller Caller, orgID uuid.UUID, inviteeEmail, role, acceptURL string) {
	org, err := m.repo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		slog.Default().Warn("failed to load organization for invite email", "organization_id", orgID, "error", err)
		return
	}

	inviterEmail := caller.Email

	if err := m.emailSender.SendInvite(ctx, email.InviteEmail{
		To:           inviteeEmail,
		ResourceName: org.Name,
		ResourceKind: "organization",
		InviterName:  caller.DisplayName,
		InviterEmail: inviterEmail,
		Role:         role,
		AcceptURL:    acceptURL,
	}); err != nil {
		slog.Default().Warn("failed to send invite email", "to", inviteeEmail, "organization_id", orgID, "error", err)
	}
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

func (m *OrgMembershipManager) orgMembershipRowToResult(row repository.OrgMembershipFullRow, includeInviteLink bool) OrgMembershipResult {
	result := OrgMembershipResult{
		ID:               row.ID,
		OrganizationID:   row.OrganizationID,
		UserID:           row.UserID,
		Email:            row.Email,
		DisplayName:      row.DisplayName,
		Role:             row.Role,
		MembershipStatus: row.MembershipStatus,
		CreatedAt:        row.CreatedAt,
	}
	if includeInviteLink && row.MembershipStatus == "invited" && !orgInviteExpired(row) {
		inviteToken := row.InviteToken
		if inviteToken == "" {
			inviteToken = row.ID.String()
		}
		result.AcceptURL = organizationInviteAcceptURL(m.frontendURL, inviteToken)
	}
	return result
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

func acceptOrgInviteHandler(logger *slog.Logger, service OrgMembershipService) http.HandlerFunc {
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

		result, err := service.AcceptOrgInvite(r.Context(), caller, inviteToken)
		if err != nil {
			handleMembershipError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func handleMembershipError(w http.ResponseWriter, logger *slog.Logger, err error) {
	var gateErr billingpkg.GateError
	switch {
	case errors.Is(err, ErrForbidden):
		var inviteDenial *inviteTokenAcceptDeniedError
		if errors.As(err, &inviteDenial) {
			attrs := []any{
				"kind", inviteDenial.Kind,
				"reason", inviteDenial.Reason,
				"caller_user_id", inviteDenial.CallerUserID,
				"membership_id", inviteDenial.MembershipID,
				"organization_id", inviteDenial.OrganizationID,
				"caller_email_present", inviteDenial.CallerEmailPresent,
				"invite_email_present", inviteDenial.InviteEmailPresent,
				"email_match", inviteDenial.EmailMatch,
			}
			if inviteDenial.WorkspaceID != uuid.Nil {
				attrs = append(attrs, "workspace_id", inviteDenial.WorkspaceID)
			}
			logger.Warn("invite accept forbidden", attrs...)
		}
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
	case errors.As(err, &gateErr):
		writeBillingGateError(w, gateErr.Decision)
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
