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
}

// NewWorkOSAuthenticator creates an authenticator that validates WorkOS JWTs.
// clientID is the WorkOS client ID (e.g. "client_01..."), used to construct
// the JWKS endpoint at https://api.workos.com/sso/jwks/{clientID}.
func NewWorkOSAuthenticator(clientID string, repo UserRepository) (*WorkOSAuthenticator, error) {
	jwksURL := "https://api.workos.com/sso/jwks/" + clientID
	return newWorkOSAuthenticator(jwksURL, repo)
}

// newWorkOSAuthenticator is the internal constructor that accepts a full JWKS URL.
// Exported constructor builds the URL from the client ID; tests can use this
// directly to point at a local JWKS server.
func newWorkOSAuthenticator(jwksURL string, repo UserRepository) (*WorkOSAuthenticator, error) {
	ctx := context.Background()
	cache := jwk.NewCache(ctx)
	if err := cache.Register(jwksURL, jwk.WithMinRefreshInterval(15*time.Minute)); err != nil {
		return nil, fmt.Errorf("register JWKS URL: %w", err)
	}

	// Verify we can reach the JWKS endpoint at startup.
	if _, err := cache.Refresh(ctx, jwksURL); err != nil {
		return nil, fmt.Errorf("initial JWKS fetch from %s: %w", jwksURL, err)
	}

	return &WorkOSAuthenticator{
		cachedSet: jwk.NewCachedSet(cache, jwksURL),
		repo:      repo,
	}, nil
}

func (a *WorkOSAuthenticator) Authenticate(r *http.Request) (Caller, error) {
	tokenStr, ok := bearerToken(r)
	if !ok {
		return Caller{}, fmt.Errorf("%w: missing or malformed Authorization header", ErrUnauthenticated)
	}

	// Parse and validate the JWT: verifies signature via JWKS and checks expiry.
	// WorkOS's own AuthKit libraries (authkit-nextjs, authkit-remix) skip issuer
	// and audience validation — they only verify signature + expiry — so we do
	// the same here. The JWKS endpoint is client-scoped, so a valid signature
	// already proves the token was issued for this client.
	tok, err := jwt.Parse([]byte(tokenStr),
		jwt.WithKeySet(a.cachedSet),
		jwt.WithValidate(true),
		jwt.WithAcceptableSkew(30*time.Second),
	)
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
func bearerToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return "", false
	}
	token := strings.TrimSpace(auth[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}
