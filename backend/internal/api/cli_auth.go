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
	mrand "math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Sentinel errors for device-code polling responses.
var (
	errAuthorizationPending = errors.New("authorization_pending")
	errAccessDenied         = errors.New("access_denied")
	errExpiredToken         = errors.New("expired_token")
)

type CLIAuthService interface {
	CreateDeviceCode(ctx context.Context) (CreateDeviceCodeResult, error)
	PollDeviceToken(ctx context.Context, deviceCode string) (PollDeviceTokenResult, error)
	ApproveDeviceCode(ctx context.Context, caller Caller, userCode string) error
	CreateCLIToken(ctx context.Context, caller Caller, name string) (CreateCLITokenResult, error)
	ListCLITokens(ctx context.Context, caller Caller) ([]CLITokenSummary, error)
	RevokeCLIToken(ctx context.Context, caller Caller, tokenID uuid.UUID) error
}

type CreateDeviceCodeResult struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type PollDeviceTokenResult struct {
	Token string `json:"token,omitempty"`
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

type CLIAuthRepository interface {
	CreateCLIToken(ctx context.Context, userID uuid.UUID, tokenHash, name string, expiresAt *time.Time) (repository.CLIToken, error)
	ListCLITokensByUserID(ctx context.Context, userID uuid.UUID) ([]repository.CLIToken, error)
	RevokeCLIToken(ctx context.Context, tokenID, userID uuid.UUID) error
	CreateDeviceAuthCode(ctx context.Context, deviceCode, userCode string, expiresAt time.Time) (repository.DeviceAuthCode, error)
	GetDeviceAuthCodeByDeviceCode(ctx context.Context, deviceCode string) (repository.DeviceAuthCode, error)
	ApproveDeviceAuthCodeWithToken(ctx context.Context, userCode string, userID uuid.UUID, tokenHash, tokenName, rawToken string, expiresAt *time.Time) (repository.CLIToken, error)
	ConsumeDeviceRawToken(ctx context.Context, id uuid.UUID) (string, error)
	ExpireDeviceAuthCode(ctx context.Context, id uuid.UUID) error
	ExpireStaleDeviceAuthCodes(ctx context.Context) error
}

type CLIAuthManager struct {
	repo        CLIAuthRepository
	logger      *slog.Logger
	frontendURL string
}

func NewCLIAuthManager(repo CLIAuthRepository, logger *slog.Logger, frontendURL string) *CLIAuthManager {
	return &CLIAuthManager{
		repo:        repo,
		logger:      logger,
		frontendURL: strings.TrimRight(frontendURL, "/"),
	}
}

func (m *CLIAuthManager) CreateDeviceCode(ctx context.Context) (CreateDeviceCodeResult, error) {
	m.cleanupExpiredCodes(ctx)

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

	verificationURI := "/auth/device"
	verificationURIComplete := m.frontendURL + verificationURI + "?user_code=" + url.QueryEscape(userCode)

	return CreateDeviceCodeResult{
		DeviceCode:              deviceCode,
		UserCode:                userCode,
		VerificationURI:         verificationURI,
		VerificationURIComplete: verificationURIComplete,
		ExpiresIn:               600,
		Interval:                5,
	}, nil
}

func (m *CLIAuthManager) PollDeviceToken(ctx context.Context, deviceCode string) (PollDeviceTokenResult, error) {
	m.cleanupExpiredCodes(ctx)

	code, err := m.repo.GetDeviceAuthCodeByDeviceCode(ctx, deviceCode)
	if err != nil {
		if errors.Is(err, repository.ErrDeviceCodeNotFound) {
			return PollDeviceTokenResult{}, errExpiredToken
		}
		return PollDeviceTokenResult{}, fmt.Errorf("lookup failed: %w", err)
	}

	if code.ExpiresAt.Before(time.Now()) {
		_ = m.repo.ExpireDeviceAuthCode(ctx, code.ID)
		return PollDeviceTokenResult{}, errExpiredToken
	}

	switch code.Status {
	case "pending":
		return PollDeviceTokenResult{}, errAuthorizationPending
	case "denied":
		return PollDeviceTokenResult{}, errAccessDenied
	case "approved":
		rawToken, err := m.repo.ConsumeDeviceRawToken(ctx, code.ID)
		if err != nil {
			return PollDeviceTokenResult{}, fmt.Errorf("consuming token: %w", err)
		}
		if rawToken == "" {
			return PollDeviceTokenResult{}, errExpiredToken
		}
		return PollDeviceTokenResult{Token: rawToken}, nil
	default:
		return PollDeviceTokenResult{}, errExpiredToken
	}
}

func (m *CLIAuthManager) ApproveDeviceCode(ctx context.Context, caller Caller, userCode string) error {
	m.cleanupExpiredCodes(ctx)

	rawToken, tokenHash, err := newRawCLIToken()
	if err != nil {
		return fmt.Errorf("generating token: %w", err)
	}

	if _, err := m.repo.ApproveDeviceAuthCodeWithToken(
		ctx,
		normalizeUserCode(userCode),
		caller.UserID,
		tokenHash,
		"CLI Device Login",
		rawToken,
		nil,
	); err != nil {
		switch {
		case errors.Is(err, repository.ErrDeviceCodeExpired):
			return fmt.Errorf("device code expired")
		case errors.Is(err, repository.ErrDeviceCodeNotFound):
			return fmt.Errorf("device code not found or expired")
		default:
			return fmt.Errorf("approving device code: %w", err)
		}
	}

	return nil
}

func (m *CLIAuthManager) CreateCLIToken(ctx context.Context, caller Caller, name string) (CreateCLITokenResult, error) {
	rawToken, tokenHash, err := newRawCLIToken()
	if err != nil {
		return CreateCLITokenResult{}, fmt.Errorf("generating token: %w", err)
	}

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

func (m *CLIAuthManager) cleanupExpiredCodes(ctx context.Context) {
	if mrand.IntN(20) != 0 {
		return
	}
	if err := m.repo.ExpireStaleDeviceAuthCodes(ctx); err != nil {
		m.logger.Warn("failed to cleanup stale CLI auth device codes", "error", err)
	}
}

func newRawCLIToken() (rawToken string, tokenHash string, err error) {
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", "", err
	}

	rawToken = cliTokenPrefix + base64.RawURLEncoding.EncodeToString(rawBytes)
	hash := sha256.Sum256([]byte(rawToken))
	return rawToken, hex.EncodeToString(hash[:]), nil
}

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
			switch {
			case errors.Is(err, errAuthorizationPending):
				writeError(w, http.StatusBadRequest, "authorization_pending", "waiting for user authorization")
			case errors.Is(err, errAccessDenied):
				writeError(w, http.StatusBadRequest, "access_denied", "user denied authorization")
			case errors.Is(err, errExpiredToken):
				writeError(w, http.StatusBadRequest, "expired_token", "device code has expired")
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

		if err := service.ApproveDeviceCode(r.Context(), caller, input.UserCode); err != nil {
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
		_ = json.NewDecoder(r.Body).Decode(&input)
		if strings.TrimSpace(input.Name) == "" {
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

		tokenID, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid token ID")
			return
		}

		if err := service.RevokeCLIToken(r.Context(), caller, tokenID); err != nil {
			switch {
			case errors.Is(err, repository.ErrCLITokenNotFound):
				writeError(w, http.StatusNotFound, "not_found", "token not found")
			default:
				logger.Error("revoke CLI token failed", "token_id", tokenID, "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "failed to revoke token")
			}
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func generateSecureToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

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

func normalizeUserCode(raw string) string {
	clean := strings.ToUpper(strings.TrimSpace(raw))
	clean = strings.ReplaceAll(clean, "-", "")
	clean = strings.ReplaceAll(clean, " ", "")
	if len(clean) >= 8 {
		return clean[:4] + "-" + clean[4:8]
	}
	return clean
}
