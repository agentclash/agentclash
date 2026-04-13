package api

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

var authTestLogger = slog.Default()

// --- test helpers ---

// testJWKS creates an RSA key pair and serves the public key as a JWKS
// endpoint via httptest. Returns the private key (for signing) and the server.
func testJWKS(t *testing.T) (*rsa.PrivateKey, *httptest.Server) {
	t.Helper()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	pubJWK, err := jwk.FromRaw(privKey.PublicKey)
	if err != nil {
		t.Fatalf("create public JWK: %v", err)
	}
	if err := pubJWK.Set(jwk.KeyIDKey, "test-kid-1"); err != nil {
		t.Fatalf("set kid: %v", err)
	}
	if err := pubJWK.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		t.Fatalf("set alg: %v", err)
	}

	keySet := jwk.NewSet()
	if err := keySet.AddKey(pubJWK); err != nil {
		t.Fatalf("add key to set: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(keySet)
	}))
	t.Cleanup(server.Close)

	return privKey, server
}

func signTestJWT(t *testing.T, privKey *rsa.PrivateKey, claims map[string]interface{}) string {
	t.Helper()

	builder := jwt.New()
	for k, v := range claims {
		if err := builder.Set(k, v); err != nil {
			t.Fatalf("set claim %q: %v", k, err)
		}
	}

	privJWK, err := jwk.FromRaw(privKey)
	if err != nil {
		t.Fatalf("create private JWK: %v", err)
	}
	if err := privJWK.Set(jwk.KeyIDKey, "test-kid-1"); err != nil {
		t.Fatalf("set kid on private key: %v", err)
	}

	signed, err := jwt.Sign(builder, jwt.WithKey(jwa.RS256, privJWK))
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}
	return string(signed)
}

// --- stub UserRepository for tests ---

type stubUserRepo struct {
	user              repository.User
	memberships       []repository.WorkspaceMembershipRow
	orgMemberships    []repository.OrgMembershipRow
	createdUser       repository.User
	err               error
	membershipErr     error
	orgMembershipErr  error
	createUserErr     error
}

func (s stubUserRepo) GetUserByWorkOSID(_ context.Context, _ string) (repository.User, error) {
	if s.err != nil {
		return repository.User{}, s.err
	}
	return s.user, nil
}

func (s stubUserRepo) GetActiveWorkspaceMembershipsByUserID(_ context.Context, _ uuid.UUID) ([]repository.WorkspaceMembershipRow, error) {
	if s.membershipErr != nil {
		return nil, s.membershipErr
	}
	return s.memberships, nil
}

func (s stubUserRepo) GetActiveOrganizationMembershipsByUserID(_ context.Context, _ uuid.UUID) ([]repository.OrgMembershipRow, error) {
	if s.orgMembershipErr != nil {
		return nil, s.orgMembershipErr
	}
	return s.orgMemberships, nil
}

func (s stubUserRepo) CreateUser(_ context.Context, input repository.CreateUserInput) (repository.User, error) {
	if s.createUserErr != nil {
		return repository.User{}, s.createUserErr
	}
	if s.createdUser.ID != uuid.Nil {
		return s.createdUser, nil
	}
	return repository.User{
		ID:           uuid.New(),
		WorkOSUserID: input.WorkOSUserID,
		Email:        input.Email,
		DisplayName:  input.DisplayName,
	}, nil
}

func (s stubUserRepo) GetUserByEmail(_ context.Context, _ string) (repository.User, error) {
	return repository.User{}, repository.ErrUserNotFound
}

func (s stubUserRepo) LinkWorkOSUser(_ context.Context, _ uuid.UUID, _ string) (repository.User, error) {
	return repository.User{}, errors.New("not implemented in stub")
}

// --- tests ---

func TestWorkOSAuthenticator_ValidToken(t *testing.T) {
	privKey, jwksServer := testJWKS(t)

	workspaceID := uuid.New()
	userID := uuid.New()

	repo := stubUserRepo{
		user: repository.User{
			ID:           userID,
			WorkOSUserID: "user_01ABC",
			Email:        "test@example.com",
			DisplayName:  "Test User",
		},
		memberships: []repository.WorkspaceMembershipRow{
			{WorkspaceID: workspaceID, Role: "workspace_admin"},
		},
	}

	auth, err := newWorkOSAuthenticator(jwksServer.URL, "test-client", "https://api.workos.com", repo, authTestLogger)
	if err != nil {
		t.Fatalf("create authenticator: %v", err)
	}

	token := signTestJWT(t, privKey, map[string]interface{}{
		"sub": "user_01ABC",
		"iss": "https://api.workos.com",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"jti": uuid.New().String(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	caller, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	if caller.UserID != userID {
		t.Errorf("UserID = %v, want %v", caller.UserID, userID)
	}
	if caller.WorkOSUserID != "user_01ABC" {
		t.Errorf("WorkOSUserID = %q, want %q", caller.WorkOSUserID, "user_01ABC")
	}
	if caller.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", caller.Email, "test@example.com")
	}
	if caller.DisplayName != "Test User" {
		t.Errorf("DisplayName = %q, want %q", caller.DisplayName, "Test User")
	}
	if len(caller.WorkspaceMemberships) != 1 {
		t.Fatalf("WorkspaceMemberships len = %d, want 1", len(caller.WorkspaceMemberships))
	}
	if m, ok := caller.WorkspaceMemberships[workspaceID]; !ok || m.Role != "workspace_admin" {
		t.Errorf("membership for workspace %v = %+v, want role workspace_admin", workspaceID, m)
	}
}

func TestWorkOSAuthenticator_MissingAuthorizationHeader(t *testing.T) {
	privKey, jwksServer := testJWKS(t)
	_ = privKey

	auth, err := newWorkOSAuthenticator(jwksServer.URL, "test-client", "https://api.workos.com", stubUserRepo{}, authTestLogger)
	if err != nil {
		t.Fatalf("create authenticator: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	_, err = auth.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for missing Authorization header")
	}
}

func TestWorkOSAuthenticator_MalformedBearerToken(t *testing.T) {
	_, jwksServer := testJWKS(t)

	auth, err := newWorkOSAuthenticator(jwksServer.URL, "test-client", "https://api.workos.com", stubUserRepo{}, authTestLogger)
	if err != nil {
		t.Fatalf("create authenticator: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	_, err = auth.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for non-Bearer Authorization")
	}
}

func TestWorkOSAuthenticator_ExpiredToken(t *testing.T) {
	privKey, jwksServer := testJWKS(t)

	auth, err := newWorkOSAuthenticator(jwksServer.URL, "test-client", "https://api.workos.com", stubUserRepo{}, authTestLogger)
	if err != nil {
		t.Fatalf("create authenticator: %v", err)
	}

	token := signTestJWT(t, privKey, map[string]interface{}{
		"sub": "user_01ABC",
		"iss": "https://api.workos.com",
		"iat": time.Now().Add(-10 * time.Minute).Unix(),
		"exp": time.Now().Add(-5 * time.Minute).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err = auth.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestWorkOSAuthenticator_InvalidSignature(t *testing.T) {
	_, jwksServer := testJWKS(t)

	// Generate a different key — tokens signed with this won't verify against the JWKS
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate other key: %v", err)
	}

	auth, err := newWorkOSAuthenticator(jwksServer.URL, "test-client", "https://api.workos.com", stubUserRepo{}, authTestLogger)
	if err != nil {
		t.Fatalf("create authenticator: %v", err)
	}

	token := signTestJWT(t, otherKey, map[string]interface{}{
		"sub": "user_01ABC",
		"iss": "https://api.workos.com",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err = auth.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for token signed with wrong key")
	}
}

func TestWorkOSAuthenticator_FirstLoginCreatesUser(t *testing.T) {
	privKey, jwksServer := testJWKS(t)

	createdUserID := uuid.New()
	repo := stubUserRepo{
		err: repository.ErrUserNotFound,
		createdUser: repository.User{
			ID:           createdUserID,
			WorkOSUserID: "user_01NEW",
			Email:        "new@example.com",
		},
	}

	auth, err := newWorkOSAuthenticator(jwksServer.URL, "test-client", "https://api.workos.com", repo, authTestLogger)
	if err != nil {
		t.Fatalf("create authenticator: %v", err)
	}

	token := signTestJWT(t, privKey, map[string]interface{}{
		"sub":   "user_01NEW",
		"iss":   "https://api.workos.com",
		"email": "new@example.com",
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(5 * time.Minute).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	caller, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("expected auto-create to succeed, got: %v", err)
	}
	if caller.UserID != createdUserID {
		t.Errorf("UserID = %v, want %v", caller.UserID, createdUserID)
	}
	if len(caller.OrganizationMemberships) != 0 {
		t.Errorf("OrganizationMemberships len = %d, want 0", len(caller.OrganizationMemberships))
	}
	if len(caller.WorkspaceMemberships) != 0 {
		t.Errorf("WorkspaceMemberships len = %d, want 0", len(caller.WorkspaceMemberships))
	}
}

func TestWorkOSAuthenticator_FirstLoginCreateUserFails(t *testing.T) {
	privKey, jwksServer := testJWKS(t)

	repo := stubUserRepo{
		err:           repository.ErrUserNotFound,
		createUserErr: errors.New("db connection lost"),
	}

	auth, err := newWorkOSAuthenticator(jwksServer.URL, "test-client", "https://api.workos.com", repo, authTestLogger)
	if err != nil {
		t.Fatalf("create authenticator: %v", err)
	}

	token := signTestJWT(t, privKey, map[string]interface{}{
		"sub": "user_01NEW",
		"iss": "https://api.workos.com",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err = auth.Authenticate(req)
	if err == nil {
		t.Fatal("expected error when CreateUser fails")
	}
}

func TestWorkOSAuthenticator_EmptySubClaim(t *testing.T) {
	privKey, jwksServer := testJWKS(t)

	auth, err := newWorkOSAuthenticator(jwksServer.URL, "test-client", "https://api.workos.com", stubUserRepo{}, authTestLogger)
	if err != nil {
		t.Fatalf("create authenticator: %v", err)
	}

	token := signTestJWT(t, privKey, map[string]interface{}{
		"sub": "",
		"iss": "https://api.workos.com",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err = auth.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for empty sub claim")
	}
}

func TestWorkOSAuthenticator_GarbageToken(t *testing.T) {
	_, jwksServer := testJWKS(t)

	auth, err := newWorkOSAuthenticator(jwksServer.URL, "test-client", "https://api.workos.com", stubUserRepo{}, authTestLogger)
	if err != nil {
		t.Fatalf("create authenticator: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-jwt")

	_, err = auth.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for garbage token")
	}
}

func TestWorkOSAuthenticator_MembershipLoadError(t *testing.T) {
	privKey, jwksServer := testJWKS(t)

	repo := stubUserRepo{
		user: repository.User{
			ID:           uuid.New(),
			WorkOSUserID: "user_01ABC",
			Email:        "test@example.com",
		},
		membershipErr: errors.New("db connection lost"),
	}

	auth, err := newWorkOSAuthenticator(jwksServer.URL, "test-client", "https://api.workos.com", repo, authTestLogger)
	if err != nil {
		t.Fatalf("create authenticator: %v", err)
	}

	token := signTestJWT(t, privKey, map[string]interface{}{
		"sub": "user_01ABC",
		"iss": "https://api.workos.com",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err = auth.Authenticate(req)
	if err == nil {
		t.Fatal("expected error when membership loading fails")
	}
}

func TestWorkOSAuthenticator_WrongIssuer(t *testing.T) {
	privKey, jwksServer := testJWKS(t)

	// Authenticator expects "https://api.workos.com" but token has a different issuer
	auth, err := newWorkOSAuthenticator(jwksServer.URL, "test-client", "https://api.workos.com", stubUserRepo{}, authTestLogger)
	if err != nil {
		t.Fatalf("create authenticator: %v", err)
	}

	token := signTestJWT(t, privKey, map[string]interface{}{
		"sub": "user_01ABC",
		"iss": "https://evil.example.com",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err = auth.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for token with wrong issuer")
	}
}

func TestWorkOSAuthenticator_CustomIssuer(t *testing.T) {
	privKey, jwksServer := testJWKS(t)

	// Authenticator configured with a custom auth domain as issuer
	repo := stubUserRepo{
		user: repository.User{
			ID:           uuid.New(),
			WorkOSUserID: "user_01ABC",
			Email:        "test@example.com",
		},
		memberships: []repository.WorkspaceMembershipRow{},
	}

	auth, err := newWorkOSAuthenticator(jwksServer.URL, "test-client", "https://auth.mycompany.com", repo, authTestLogger)
	if err != nil {
		t.Fatalf("create authenticator: %v", err)
	}

	// Token with the matching custom issuer should succeed
	token := signTestJWT(t, privKey, map[string]interface{}{
		"sub": "user_01ABC",
		"iss": "https://auth.mycompany.com",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	caller, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("expected success for matching custom issuer, got: %v", err)
	}
	if caller.WorkOSUserID != "user_01ABC" {
		t.Errorf("WorkOSUserID = %q, want %q", caller.WorkOSUserID, "user_01ABC")
	}

	// Token with the default issuer should be rejected when custom is expected
	wrongToken := signTestJWT(t, privKey, map[string]interface{}{
		"sub": "user_01ABC",
		"iss": "https://api.workos.com",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	req2 := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req2.Header.Set("Authorization", "Bearer "+wrongToken)

	_, err = auth.Authenticate(req2)
	if err == nil {
		t.Fatal("expected error for token with default issuer when custom issuer is configured")
	}
}
