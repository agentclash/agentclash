package api

import (
	"context"
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
	GetActiveWorkspaceMembershipsByUserID(ctx context.Context, userID uuid.UUID) ([]repository.WorkspaceMembershipRow, error)
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

	user, err := a.repo.GetUserByWorkOSID(r.Context(), workosUserID)
	if err != nil {
		return Caller{}, fmt.Errorf("%w: %v", ErrUnauthenticated, err)
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
		UserID:               user.ID,
		WorkOSUserID:         user.WorkOSUserID,
		Email:                user.Email,
		DisplayName:          user.DisplayName,
		WorkspaceMemberships: membershipMap,
	}, nil
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
