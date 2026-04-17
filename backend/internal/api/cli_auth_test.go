package api

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type fakeCLIAuthRepository struct {
	createdDeviceCode repository.DeviceAuthCode
	deviceByCode      repository.DeviceAuthCode
	deniedUserCode    string
	denyErr           error
}

func (f *fakeCLIAuthRepository) CreateCLIToken(context.Context, uuid.UUID, string, string, *time.Time) (repository.CLIToken, error) {
	return repository.CLIToken{}, nil
}

func (f *fakeCLIAuthRepository) ListCLITokensByUserID(context.Context, uuid.UUID) ([]repository.CLIToken, error) {
	return nil, nil
}

func (f *fakeCLIAuthRepository) RevokeCLIToken(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (f *fakeCLIAuthRepository) CreateDeviceAuthCode(_ context.Context, deviceCode, userCode string, expiresAt time.Time) (repository.DeviceAuthCode, error) {
	f.createdDeviceCode = repository.DeviceAuthCode{
		ID:         uuid.New(),
		DeviceCode: deviceCode,
		UserCode:   userCode,
		Status:     "pending",
		ExpiresAt:  expiresAt,
		CreatedAt:  time.Now(),
	}
	return f.createdDeviceCode, nil
}

func (f *fakeCLIAuthRepository) GetDeviceAuthCodeByDeviceCode(context.Context, string) (repository.DeviceAuthCode, error) {
	if f.deviceByCode.DeviceCode != "" {
		return f.deviceByCode, nil
	}
	return repository.DeviceAuthCode{}, repository.ErrDeviceCodeNotFound
}

func (f *fakeCLIAuthRepository) ApproveDeviceAuthCodeWithToken(context.Context, string, uuid.UUID, string, string, string, *time.Time) (repository.CLIToken, error) {
	return repository.CLIToken{}, nil
}

func (f *fakeCLIAuthRepository) DenyDeviceAuthCode(_ context.Context, userCode string) error {
	f.deniedUserCode = userCode
	return f.denyErr
}

func (f *fakeCLIAuthRepository) ConsumeDeviceRawToken(context.Context, uuid.UUID) (string, error) {
	return "", nil
}

func (f *fakeCLIAuthRepository) ExpireDeviceAuthCode(context.Context, uuid.UUID) error {
	return nil
}

func (f *fakeCLIAuthRepository) ExpireStaleDeviceAuthCodes(context.Context) error {
	return nil
}

func TestCLIAuthManagerCreateDeviceCodeReturnsAbsoluteVerificationURLs(t *testing.T) {
	repo := &fakeCLIAuthRepository{}
	manager := NewCLIAuthManager(repo, slog.Default(), "https://app.agentclash.dev")

	result, err := manager.CreateDeviceCode(context.Background())
	if err != nil {
		t.Fatalf("CreateDeviceCode() error = %v", err)
	}
	if result.VerificationURI != "https://app.agentclash.dev/auth/device" {
		t.Fatalf("verification_uri = %q", result.VerificationURI)
	}
	if !strings.HasPrefix(result.VerificationURIComplete, "https://app.agentclash.dev/auth/device?user_code=") {
		t.Fatalf("verification_uri_complete = %q", result.VerificationURIComplete)
	}
	if repo.createdDeviceCode.UserCode != result.UserCode {
		t.Fatalf("stored user code = %q, want %q", repo.createdDeviceCode.UserCode, result.UserCode)
	}
}

func TestCLIAuthManagerDenyDeviceCodeNormalizesUserCode(t *testing.T) {
	repo := &fakeCLIAuthRepository{}
	manager := NewCLIAuthManager(repo, slog.Default(), "https://app.agentclash.dev")

	err := manager.DenyDeviceCode(context.Background(), Caller{UserID: uuid.New()}, "ab cd-efgh")
	if err != nil {
		t.Fatalf("DenyDeviceCode() error = %v", err)
	}
	if repo.deniedUserCode != "ABCD-EFGH" {
		t.Fatalf("denied user code = %q, want ABCD-EFGH", repo.deniedUserCode)
	}
}

func TestCLIAuthManagerPollDeviceTokenReturnsAccessDeniedForDeniedCode(t *testing.T) {
	repo := &fakeCLIAuthRepository{
		deviceByCode: repository.DeviceAuthCode{
			ID:         uuid.New(),
			DeviceCode: "dc_denied",
			UserCode:   "ABCD-EFGH",
			Status:     "denied",
			ExpiresAt:  time.Now().Add(time.Hour),
		},
	}
	manager := NewCLIAuthManager(repo, slog.Default(), "https://app.agentclash.dev")

	_, err := manager.PollDeviceToken(context.Background(), "dc_denied")
	if !errors.Is(err, errAccessDenied) {
		t.Fatalf("PollDeviceToken() error = %v, want errAccessDenied", err)
	}
}
