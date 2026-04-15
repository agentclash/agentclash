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

type CLITokenRepository interface {
	GetCLITokenByHash(ctx context.Context, tokenHash string) (repository.CLIToken, error)
	TouchCLITokenLastUsed(ctx context.Context, tokenID uuid.UUID) error
	GetUserByID(ctx context.Context, userID uuid.UUID) (repository.User, error)
	GetActiveOrganizationMembershipsByUserID(ctx context.Context, userID uuid.UUID) ([]repository.OrgMembershipRow, error)
	GetActiveWorkspaceMembershipsByUserID(ctx context.Context, userID uuid.UUID) ([]repository.WorkspaceMembershipRow, error)
}

type CLITokenAuthenticator struct {
	repo   CLITokenRepository
	logger *slog.Logger
}

func NewCLITokenAuthenticator(repo CLITokenRepository, logger *slog.Logger) *CLITokenAuthenticator {
	return &CLITokenAuthenticator{repo: repo, logger: logger}
}

func (a *CLITokenAuthenticator) Authenticate(r *http.Request) (Caller, error) {
	tokenStr, ok := bearerToken(r)
	if !ok {
		return Caller{}, fmt.Errorf("%w: missing or malformed Authorization header", ErrUnauthenticated)
	}
	if !IsCLIToken(tokenStr) {
		return Caller{}, fmt.Errorf("%w: not a CLI token", ErrUnauthenticated)
	}

	token, err := a.repo.GetCLITokenByHash(r.Context(), hashToken(tokenStr))
	if err != nil {
		return Caller{}, fmt.Errorf("%w: invalid CLI token", ErrUnauthenticated)
	}
	if token.RevokedAt != nil {
		return Caller{}, fmt.Errorf("%w: CLI token has been revoked", ErrUnauthenticated)
	}
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		return Caller{}, fmt.Errorf("%w: CLI token has expired", ErrUnauthenticated)
	}

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

	if token.LastUsedAt == nil || time.Since(*token.LastUsedAt) > time.Hour {
		if err := a.repo.TouchCLITokenLastUsed(r.Context(), token.ID); err != nil {
			a.logger.Warn("failed to update CLI token last_used_at", "token_id", token.ID, "error", err)
		}
	}

	return Caller{
		UserID:                  user.ID,
		WorkOSUserID:            user.WorkOSUserID,
		Email:                   user.Email,
		DisplayName:             user.DisplayName,
		OrganizationMemberships: orgMap,
		WorkspaceMemberships:    wsMap,
	}, nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func IsCLIToken(token string) bool {
	return strings.HasPrefix(token, cliTokenPrefix)
}
