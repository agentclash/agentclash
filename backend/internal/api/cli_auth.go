package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// --------------------------------------------------------------------------
// Service Interface
// --------------------------------------------------------------------------

// CLIAuthService handles CLI token lifecycle and device code flow.
type CLIAuthService interface {
	CreateDeviceCode(ctx context.Context) (CreateDeviceCodeResult, error)
	PollDeviceToken(ctx context.Context, deviceCode string) (PollDeviceTokenResult, error)
	ApproveDeviceCode(ctx context.Context, caller Caller, userCode string) error
	CreateCLIToken(ctx context.Context, caller Caller, name string) (CreateCLITokenResult, error)
	ListCLITokens(ctx context.Context, caller Caller) ([]CLITokenSummary, error)
	RevokeCLIToken(ctx context.Context, caller Caller, tokenID uuid.UUID) error
}

// CreateDeviceCodeResult follows RFC 8628 Section 3.2 field naming.
type CreateDeviceCodeResult struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type PollDeviceTokenResult struct {
	Token  string `json:"token,omitempty"`
	UserID string `json:"user_id,omitempty"`
	Email  string `json:"email,omitempty"`
}

type CreateCLITokenResult struct {
	ID        uuid.UUID  `json:"id"`
	Token     string     `json:"token"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type CLITokenSummary struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// --------------------------------------------------------------------------
// Manager
// --------------------------------------------------------------------------

// CLIAuthRepository defines the data access methods the manager needs.
type CLIAuthRepository interface {
	CreateCLIToken(ctx context.Context, userID uuid.UUID, tokenHash, name string, expiresAt *time.Time) (repository.CLIToken, error)
	ListCLITokensByUserID(ctx context.Context, userID uuid.UUID) ([]repository.CLIToken, error)
	RevokeCLIToken(ctx context.Context, tokenID, userID uuid.UUID) error
	CreateDeviceAuthCode(ctx context.Context, deviceCode, userCode string, expiresAt time.Time) (repository.DeviceAuthCode, error)
	GetDeviceAuthCodeByDeviceCode(ctx context.Context, deviceCode string) (repository.DeviceAuthCode, error)
	GetDeviceAuthCodeByUserCode(ctx context.Context, userCode string) (repository.DeviceAuthCode, error)
	ApproveDeviceAuthCode(ctx context.Context, id, userID uuid.UUID, cliTokenID uuid.UUID, rawToken string) error
	ConsumeDeviceRawToken(ctx context.Context, id uuid.UUID) (string, error)
	ExpireDeviceAuthCode(ctx context.Context, id uuid.UUID) error
}

// CLIAuthManager implements CLIAuthService.
type CLIAuthManager struct {
	repo   CLIAuthRepository
	logger *slog.Logger
}

func NewCLIAuthManager(repo CLIAuthRepository, logger *slog.Logger) *CLIAuthManager {
	return &CLIAuthManager{repo: repo, logger: logger}
}

func (m *CLIAuthManager) CreateDeviceCode(ctx context.Context) (CreateDeviceCodeResult, error) {
	deviceCode, err := generateSecureToken(32)
	if err != nil {
		return CreateDeviceCodeResult{}, fmt.Errorf("generating device code: %w", err)
	}
	deviceCode = "dc_" + deviceCode

	userCode, err := generateUserCode()
	if err != nil {
		return CreateDeviceCodeResult{}, fmt.Errorf("generating user code: %w", err)
	}

	expiresAt := time.Now().Add(10 * time.Minute)
	if _, err := m.repo.CreateDeviceAuthCode(ctx, deviceCode, userCode, expiresAt); err != nil {
		return CreateDeviceCodeResult{}, fmt.Errorf("storing device code: %w", err)
	}

	return CreateDeviceCodeResult{
		DeviceCode:      deviceCode,
		UserCode:        userCode,
		VerificationURI: "/auth/device",
		ExpiresIn:       600,
		Interval:        5,
	}, nil
}

func (m *CLIAuthManager) PollDeviceToken(ctx context.Context, deviceCode string) (PollDeviceTokenResult, error) {
	code, err := m.repo.GetDeviceAuthCodeByDeviceCode(ctx, deviceCode)
	if err != nil {
		if errors.Is(err, repository.ErrDeviceCodeNotFound) {
			return PollDeviceTokenResult{}, fmt.Errorf("expired_token")
		}
		return PollDeviceTokenResult{}, fmt.Errorf("lookup failed: %w", err)
	}

	if code.ExpiresAt.Before(time.Now()) {
		m.repo.ExpireDeviceAuthCode(ctx, code.ID)
		return PollDeviceTokenResult{}, fmt.Errorf("expired_token")
	}

	switch code.Status {
	case "pending":
		return PollDeviceTokenResult{}, fmt.Errorf("authorization_pending")
	case "denied":
		return PollDeviceTokenResult{}, fmt.Errorf("access_denied")
	case "approved":
		// Atomically consume the raw token (read + NULL in one query).
		rawToken, err := m.repo.ConsumeDeviceRawToken(ctx, code.ID)
		if err != nil {
			return PollDeviceTokenResult{}, fmt.Errorf("consuming token: %w", err)
		}
		if rawToken == "" {
			// Token already consumed by a previous poll.
			return PollDeviceTokenResult{}, fmt.Errorf("expired_token")
		}
		return PollDeviceTokenResult{Token: rawToken}, nil
	default:
		return PollDeviceTokenResult{}, fmt.Errorf("expired_token")
	}
}

func (m *CLIAuthManager) ApproveDeviceCode(ctx context.Context, caller Caller, userCode string) error {
	code, err := m.repo.GetDeviceAuthCodeByUserCode(ctx, userCode)
	if err != nil {
		return fmt.Errorf("device code not found or expired")
	}
	if code.ExpiresAt.Before(time.Now()) {
		m.repo.ExpireDeviceAuthCode(ctx, code.ID)
		return fmt.Errorf("device code expired")
	}

	// Create CLI token for the approving user.
	result, err := m.CreateCLIToken(ctx, caller, "CLI Device Login")
	if err != nil {
		return fmt.Errorf("creating CLI token: %w", err)
	}

	// Atomically approve the device code and store the raw token for one-time retrieval.
	if err := m.repo.ApproveDeviceAuthCode(ctx, code.ID, caller.UserID, result.ID, result.Token); err != nil {
		return fmt.Errorf("approving device code: %w", err)
	}

	return nil
}

func (m *CLIAuthManager) CreateCLIToken(ctx context.Context, caller Caller, name string) (CreateCLITokenResult, error) {
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return CreateCLITokenResult{}, fmt.Errorf("generating token: %w", err)
	}
	rawToken := cliTokenPrefix + base64.RawURLEncoding.EncodeToString(rawBytes)
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	token, err := m.repo.CreateCLIToken(ctx, caller.UserID, tokenHash, name, nil)
	if err != nil {
		return CreateCLITokenResult{}, err
	}

	return CreateCLITokenResult{
		ID:        token.ID,
		Token:     rawToken,
		Name:      token.Name,
		CreatedAt: token.CreatedAt,
		ExpiresAt: token.ExpiresAt,
	}, nil
}

func (m *CLIAuthManager) ListCLITokens(ctx context.Context, caller Caller) ([]CLITokenSummary, error) {
	tokens, err := m.repo.ListCLITokensByUserID(ctx, caller.UserID)
	if err != nil {
		return nil, err
	}

	out := make([]CLITokenSummary, len(tokens))
	for i, t := range tokens {
		out[i] = CLITokenSummary{
			ID:         t.ID,
			Name:       t.Name,
			LastUsedAt: t.LastUsedAt,
			ExpiresAt:  t.ExpiresAt,
			CreatedAt:  t.CreatedAt,
		}
	}
	return out, nil
}

func (m *CLIAuthManager) RevokeCLIToken(ctx context.Context, caller Caller, tokenID uuid.UUID) error {
	return m.repo.RevokeCLIToken(ctx, tokenID, caller.UserID)
}

// --------------------------------------------------------------------------
// Handlers
// --------------------------------------------------------------------------

// registerCLIAuthPublicRoutes adds unauthenticated device code endpoints.
// The router should already be scoped (e.g., under /v1/auth).
func registerCLIAuthPublicRoutes(router chi.Router, logger *slog.Logger, service CLIAuthService) {
	router.Post("/device", createDeviceCodeHandler(logger, service))
	router.Post("/device/token", pollDeviceTokenHandler(logger, service))
}

func createDeviceCodeHandler(logger *slog.Logger, service CLIAuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		result, err := service.CreateDeviceCode(r.Context())
		if err != nil {
			logger.Error("create device code failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to create device code")
			return
		}
		writeJSON(w, http.StatusCreated, result)
	}
}

func pollDeviceTokenHandler(logger *slog.Logger, service CLIAuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		var input struct {
			DeviceCode string `json:"device_code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.DeviceCode == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "device_code is required")
			return
		}

		result, err := service.PollDeviceToken(r.Context(), input.DeviceCode)
		if err != nil {
			errMsg := err.Error()
			switch {
			case errMsg == "authorization_pending":
				writeError(w, http.StatusBadRequest, "authorization_pending", "waiting for user authorization")
			case errMsg == "access_denied":
				writeError(w, http.StatusBadRequest, "access_denied", "user denied authorization")
			case errMsg == "expired_token":
				writeError(w, http.StatusBadRequest, "expired_token", "device code has expired")
			case errMsg == "slow_down":
				writeError(w, http.StatusBadRequest, "slow_down", "polling too frequently, increase interval")
			default:
				logger.Error("poll device token failed", "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "failed to poll token")
			}
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func approveDeviceCodeHandler(logger *slog.Logger, service CLIAuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		var input struct {
			UserCode string `json:"user_code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.UserCode == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "user_code is required")
			return
		}

		if err := service.ApproveDeviceCode(r.Context(), caller, normalizeUserCode(input.UserCode)); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func createCLITokenHandler(logger *slog.Logger, service CLIAuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		var input struct {
			Name string `json:"name"`
		}
		json.NewDecoder(r.Body).Decode(&input)
		if input.Name == "" {
			input.Name = "CLI Token"
		}

		result, err := service.CreateCLIToken(r.Context(), caller, input.Name)
		if err != nil {
			logger.Error("create CLI token failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to create token")
			return
		}

		writeJSON(w, http.StatusCreated, result)
	}
}

func listCLITokensHandler(logger *slog.Logger, service CLIAuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		tokens, err := service.ListCLITokens(r.Context(), caller)
		if err != nil {
			logger.Error("list CLI tokens failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to list tokens")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"items": tokens})
	}
}

func revokeCLITokenHandler(logger *slog.Logger, service CLIAuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		raw := chi.URLParam(r, "id")
		tokenID, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid token ID")
			return
		}

		if err := service.RevokeCLIToken(r.Context(), caller, tokenID); err != nil {
			writeError(w, http.StatusNotFound, "not_found", "token not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func generateSecureToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// generateUserCode creates a code like "RRGQ-BJVS" from a 30-char alphabet
// (A-Z minus I,O and 0-9 minus 0,1 to avoid ambiguity).
func generateUserCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	code := make([]byte, 8)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		code[i] = alphabet[n.Int64()]
	}
	return string(code[:4]) + "-" + string(code[4:]), nil
}

// normalizeUserCode strips whitespace, uppercases, and re-formats as XXXX-YYYY.
func normalizeUserCode(raw string) string {
	clean := strings.ToUpper(strings.TrimSpace(raw))
	clean = strings.ReplaceAll(clean, "-", "")
	clean = strings.ReplaceAll(clean, " ", "")
	if len(clean) >= 8 {
		return clean[:4] + "-" + clean[4:8]
	}
	return clean
}
