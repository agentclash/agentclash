package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

const (
	initialJWKSFetchTimeout = 10 * time.Second

	// defaultWorkOSIssuer is the default JWT issuer for WorkOS tokens.
	// When a custom auth domain is configured in WorkOS, set WORKOS_ISSUER
	// to override this value.
	defaultWorkOSIssuer = "https://api.workos.com"
)

// UserRepository is the subset of repository.Repository needed for auth.
type UserRepository interface {
	GetUserByWorkOSID(ctx context.Context, workosUserID string) (repository.User, error)
	GetUserByEmail(ctx context.Context, email string) (repository.User, error)
	GetActiveWorkspaceMembershipsByUserID(ctx context.Context, userID uuid.UUID) ([]repository.WorkspaceMembershipRow, error)
	GetActiveOrganizationMembershipsByUserID(ctx context.Context, userID uuid.UUID) ([]repository.OrgMembershipRow, error)
	CreateUser(ctx context.Context, input repository.CreateUserInput) (repository.User, error)
	LinkWorkOSUser(ctx context.Context, userID uuid.UUID, workosUserID string) (repository.User, error)
}

// WorkOSAuthenticator validates WorkOS AuthKit JWTs using the public JWKS
// endpoint and resolves the token subject to an internal user.
type WorkOSAuthenticator struct {
	cachedSet jwk.Set
	repo      UserRepository
	issuer    string
	clientID  string
}

// WorkOSAuthenticatorConfig holds the settings for constructing a WorkOSAuthenticator.
type WorkOSAuthenticatorConfig struct {
	// ClientID is the WorkOS client ID (e.g. "client_01...").
	ClientID string
	// Issuer is the expected JWT issuer. Defaults to "https://api.workos.com".
	// Set to your custom auth domain if configured in WorkOS.
	Issuer string
}

// NewWorkOSAuthenticator creates an authenticator that validates WorkOS JWTs.
func NewWorkOSAuthenticator(cfg WorkOSAuthenticatorConfig, repo UserRepository) (*WorkOSAuthenticator, error) {
	jwksURL := "https://api.workos.com/sso/jwks/" + cfg.ClientID
	issuer := cfg.Issuer
	if issuer == "" {
		issuer = defaultWorkOSIssuer
	}
	return newWorkOSAuthenticator(jwksURL, cfg.ClientID, issuer, repo)
}

// newWorkOSAuthenticator is the internal constructor that accepts a full JWKS URL.
// Tests use this directly to point at a local JWKS server.
func newWorkOSAuthenticator(jwksURL, clientID, issuer string, repo UserRepository) (*WorkOSAuthenticator, error) {
	cacheCtx := context.Background()
	cache := jwk.NewCache(cacheCtx)
	if err := cache.Register(jwksURL, jwk.WithMinRefreshInterval(15*time.Minute)); err != nil {
		return nil, fmt.Errorf("register JWKS URL: %w", err)
	}

	// Verify we can reach the JWKS endpoint at startup with a bounded timeout
	// so the process fails fast instead of hanging on a network blackhole.
	refreshCtx, cancel := context.WithTimeout(context.Background(), initialJWKSFetchTimeout)
	defer cancel()
	if _, err := cache.Refresh(refreshCtx, jwksURL); err != nil {
		return nil, fmt.Errorf("initial JWKS fetch from %s: %w", jwksURL, err)
	}

	return &WorkOSAuthenticator{
		cachedSet: jwk.NewCachedSet(cache, jwksURL),
		repo:      repo,
		issuer:    issuer,
		clientID:  clientID,
	}, nil
}

func (a *WorkOSAuthenticator) Authenticate(r *http.Request) (Caller, error) {
	tokenStr, ok := bearerToken(r)
	if !ok {
		return Caller{}, fmt.Errorf("%w: missing or malformed Authorization header", ErrUnauthenticated)
	}

	// Parse and validate the JWT: verify signature via JWKS, check expiry,
	// and enforce issuer. WorkOS docs say iss becomes the custom auth domain
	// when configured, so we validate against the configured issuer.
	parseOpts := []jwt.ParseOption{
		jwt.WithKeySet(a.cachedSet),
		jwt.WithValidate(true),
		jwt.WithAcceptableSkew(30 * time.Second),
		jwt.WithIssuer(a.issuer),
	}
	tok, err := jwt.Parse([]byte(tokenStr), parseOpts...)
	if err != nil {
		return Caller{}, fmt.Errorf("%w: invalid token: %v", ErrUnauthenticated, err)
	}

	workosUserID := tok.Subject()
	if workosUserID == "" {
		return Caller{}, fmt.Errorf("%w: token missing sub claim", ErrUnauthenticated)
	}

	// Extract email from JWT claims if available (WorkOS includes email in
	// the token for AuthKit). Fall back to empty string if absent.
	email, _ := tok.Get("email")
	emailStr, _ := email.(string)

	user, err := a.resolveUser(r.Context(), workosUserID, emailStr)
	if err != nil {
		return Caller{}, err
	}

	orgMemberships, err := a.repo.GetActiveOrganizationMembershipsByUserID(r.Context(), user.ID)
	if err != nil {
		return Caller{}, fmt.Errorf("%w: failed to load organization memberships: %v", ErrUnauthenticated, err)
	}

	orgMembershipMap := make(map[uuid.UUID]OrganizationMembership, len(orgMemberships))
	for _, m := range orgMemberships {
		orgMembershipMap[m.OrganizationID] = OrganizationMembership{
			OrganizationID: m.OrganizationID,
			Role:           m.Role,
		}
	}

	memberships, err := a.repo.GetActiveWorkspaceMembershipsByUserID(r.Context(), user.ID)
	if err != nil {
		return Caller{}, fmt.Errorf("%w: failed to load workspace memberships: %v", ErrUnauthenticated, err)
	}

	membershipMap := make(map[uuid.UUID]WorkspaceMembership, len(memberships))
	for _, m := range memberships {
		membershipMap[m.WorkspaceID] = WorkspaceMembership{
			WorkspaceID: m.WorkspaceID,
			Role:        m.Role,
		}
	}

	return Caller{
		UserID:                  user.ID,
		WorkOSUserID:            user.WorkOSUserID,
		Email:                   user.Email,
		DisplayName:             user.DisplayName,
		OrganizationMemberships: orgMembershipMap,
		WorkspaceMemberships:    membershipMap,
	}, nil
}

// resolveUser finds or creates the internal user for a WorkOS login.
// It handles three cases:
//  1. User already exists with this WorkOS ID → return it.
//  2. A stub user exists from an invite (matched by email, has a placeholder
//     workos_user_id) → link the real WorkOS ID and return it.
//  3. Completely new user → create and return.
func (a *WorkOSAuthenticator) resolveUser(ctx context.Context, workosUserID, email string) (repository.User, error) {
	user, err := a.repo.GetUserByWorkOSID(ctx, workosUserID)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, repository.ErrUserNotFound) {
		return repository.User{}, fmt.Errorf("%w: %v", ErrUnauthenticated, err)
	}

	// No user with this WorkOS ID. Check if a stub user was created via invite.
	// Only link to stub users (workos_user_id starts with "pending:") — never
	// overwrite a real user's identity, which would be an account takeover.
	if email != "" {
		stubUser, emailErr := a.repo.GetUserByEmail(ctx, email)
		if emailErr == nil && strings.HasPrefix(stubUser.WorkOSUserID, "pending:") {
			linked, linkErr := a.repo.LinkWorkOSUser(ctx, stubUser.ID, workosUserID)
			if linkErr != nil {
				return repository.User{}, fmt.Errorf("link workos user to invited stub: %w", linkErr)
			}
			return linked, nil
		}
	}

	// Truly new user — auto-create.
	user, err = a.repo.CreateUser(ctx, repository.CreateUserInput{
		WorkOSUserID: workosUserID,
		Email:        email,
	})
	if err != nil {
		return repository.User{}, fmt.Errorf("auto-create user: %w", err)
	}
	return user, nil
}

// bearerToken extracts a Bearer token from the Authorization header.
// The scheme comparison is case-insensitive per RFC 6750.
func bearerToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", false
	}
	parts := strings.Fields(auth)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return parts[1], true
}
