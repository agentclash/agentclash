package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

const cliTokenPrefix = "clitok_"

// CLITokenRepository is the subset of repository needed for CLI token auth.
type CLITokenRepository interface {
	GetCLITokenByHash(ctx context.Context, tokenHash string) (repository.CLIToken, error)
	TouchCLITokenLastUsed(ctx context.Context, tokenID uuid.UUID) error
	GetUserByID(ctx context.Context, userID uuid.UUID) (repository.User, error)
	GetActiveOrganizationMembershipsByUserID(ctx context.Context, userID uuid.UUID) ([]repository.OrgMembershipRow, error)
	GetActiveWorkspaceMembershipsByUserID(ctx context.Context, userID uuid.UUID) ([]repository.WorkspaceMembershipRow, error)
}

// CLITokenAuthenticator validates CLI tokens (prefixed with "clitok_") by
// looking up their SHA-256 hash in the database.
type CLITokenAuthenticator struct {
	repo   CLITokenRepository
	logger *slog.Logger
}

func NewCLITokenAuthenticator(repo CLITokenRepository, logger *slog.Logger) *CLITokenAuthenticator {
	return &CLITokenAuthenticator{repo: repo, logger: logger}
}

func (a *CLITokenAuthenticator) Authenticate(r *http.Request) (Caller, error) {
	tokenStr := extractBearerToken(r)
	if tokenStr == "" {
		return Caller{}, fmt.Errorf("%w: missing authorization header", ErrUnauthenticated)
	}

	if !strings.HasPrefix(tokenStr, cliTokenPrefix) {
		return Caller{}, fmt.Errorf("%w: not a CLI token", ErrUnauthenticated)
	}

	hash := hashToken(tokenStr)
	token, err := a.repo.GetCLITokenByHash(r.Context(), hash)
	if err != nil {
		return Caller{}, fmt.Errorf("%w: invalid CLI token", ErrUnauthenticated)
	}

	if token.RevokedAt != nil {
		return Caller{}, fmt.Errorf("%w: CLI token has been revoked", ErrUnauthenticated)
	}
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		return Caller{}, fmt.Errorf("%w: CLI token has expired", ErrUnauthenticated)
	}

	// GetUserByID only returns active (non-archived) users.
	user, err := a.repo.GetUserByID(r.Context(), token.UserID)
	if err != nil {
		return Caller{}, fmt.Errorf("%w: user not found or deactivated", ErrUnauthenticated)
	}

	orgMemberships, err := a.repo.GetActiveOrganizationMembershipsByUserID(r.Context(), user.ID)
	if err != nil {
		return Caller{}, fmt.Errorf("%w: failed to load organization memberships", ErrUnauthenticated)
	}
	orgMap := make(map[uuid.UUID]OrganizationMembership, len(orgMemberships))
	for _, m := range orgMemberships {
		orgMap[m.OrganizationID] = OrganizationMembership{
			OrganizationID: m.OrganizationID,
			Role:           m.Role,
		}
	}

	wsMemberships, err := a.repo.GetActiveWorkspaceMembershipsByUserID(r.Context(), user.ID)
	if err != nil {
		return Caller{}, fmt.Errorf("%w: failed to load workspace memberships", ErrUnauthenticated)
	}
	wsMap := make(map[uuid.UUID]WorkspaceMembership, len(wsMemberships))
	for _, m := range wsMemberships {
		wsMap[m.WorkspaceID] = WorkspaceMembership{
			WorkspaceID: m.WorkspaceID,
			Role:        m.Role,
		}
	}

	// Best-effort update of last_used_at using the request context.
	// If the server is shutting down this will fail gracefully.
	go func() {
		if err := a.repo.TouchCLITokenLastUsed(r.Context(), token.ID); err != nil {
			a.logger.Warn("failed to update CLI token last_used_at", "token_id", token.ID, "error", err)
		}
	}()

	return Caller{
		UserID:                  user.ID,
		WorkOSUserID:            user.WorkOSUserID,
		Email:                   user.Email,
		DisplayName:             user.DisplayName,
		OrganizationMemberships: orgMap,
		WorkspaceMemberships:    wsMap,
	}, nil
}

// hashToken computes the SHA-256 hex digest of a raw token.
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// extractBearerToken gets the Bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

// IsCLIToken returns true if the token string has the CLI token prefix.
func IsCLIToken(token string) bool {
	return strings.HasPrefix(token, cliTokenPrefix)
}
