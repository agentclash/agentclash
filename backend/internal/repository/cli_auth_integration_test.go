package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
)

func TestRepositoryDeviceAuthApproveConsumesRawTokenOnce(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	code, err := repo.CreateDeviceAuthCode(ctx, "dc_consume", "ABCD-EFGH", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateDeviceAuthCode() error = %v", err)
	}
	if _, err := repo.ApproveDeviceAuthCodeWithToken(ctx, "ABCD-EFGH", fixture.userID, "hash-consume", "CLI Device Login", "clitok_raw", nil); err != nil {
		t.Fatalf("ApproveDeviceAuthCodeWithToken() error = %v", err)
	}

	rawToken, err := repo.ConsumeDeviceRawToken(ctx, code.ID)
	if err != nil {
		t.Fatalf("ConsumeDeviceRawToken() first error = %v", err)
	}
	if rawToken != "clitok_raw" {
		t.Fatalf("first raw token = %q, want clitok_raw", rawToken)
	}

	rawToken, err = repo.ConsumeDeviceRawToken(ctx, code.ID)
	if err != nil {
		t.Fatalf("ConsumeDeviceRawToken() second error = %v", err)
	}
	if rawToken != "" {
		t.Fatalf("second raw token = %q, want empty", rawToken)
	}
}

func TestRepositoryDenyDeviceAuthCode(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	seedFixture(t, ctx, db)
	repo := repository.New(db)

	code, err := repo.CreateDeviceAuthCode(ctx, "dc_deny", "WXYZ-2345", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateDeviceAuthCode() error = %v", err)
	}
	if err := repo.DenyDeviceAuthCode(ctx, "WXYZ-2345"); err != nil {
		t.Fatalf("DenyDeviceAuthCode() error = %v", err)
	}

	denied, err := repo.GetDeviceAuthCodeByDeviceCode(ctx, code.DeviceCode)
	if err != nil {
		t.Fatalf("GetDeviceAuthCodeByDeviceCode() error = %v", err)
	}
	if denied.Status != "denied" {
		t.Fatalf("status = %q, want denied", denied.Status)
	}
}

func TestRepositoryDenyDeviceAuthCodeRejectsExpiredCode(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	seedFixture(t, ctx, db)
	repo := repository.New(db)

	if _, err := repo.CreateDeviceAuthCode(ctx, "dc_expired_deny", "JKLM-6789", time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("CreateDeviceAuthCode() error = %v", err)
	}
	err := repo.DenyDeviceAuthCode(ctx, "JKLM-6789")
	if !errors.Is(err, repository.ErrDeviceCodeExpired) {
		t.Fatalf("DenyDeviceAuthCode() error = %v, want ErrDeviceCodeExpired", err)
	}
}

func TestRepositoryExpireStaleDeviceAuthCodesRevokesOnlyUnusedApprovedTokens(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	unusedCode, err := repo.CreateDeviceAuthCode(ctx, "dc_unused", "QRST-2345", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateDeviceAuthCode() unused error = %v", err)
	}
	if _, err := repo.ApproveDeviceAuthCodeWithToken(ctx, unusedCode.UserCode, fixture.userID, "hash-unused", "CLI Device Login", "clitok_unused", nil); err != nil {
		t.Fatalf("ApproveDeviceAuthCodeWithToken() unused error = %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE device_auth_codes SET expires_at = now() - interval '1 minute' WHERE id = $1`, unusedCode.ID); err != nil {
		t.Fatalf("expire unused device code: %v", err)
	}

	usedCode, err := repo.CreateDeviceAuthCode(ctx, "dc_used", "UVWX-6789", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateDeviceAuthCode() used error = %v", err)
	}
	usedToken, err := repo.ApproveDeviceAuthCodeWithToken(ctx, usedCode.UserCode, fixture.userID, "hash-used", "CLI Device Login", "clitok_used", nil)
	if err != nil {
		t.Fatalf("ApproveDeviceAuthCodeWithToken() used error = %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE cli_tokens SET last_used_at = now() WHERE id = $1`, usedToken.ID); err != nil {
		t.Fatalf("mark used token: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE device_auth_codes SET expires_at = now() - interval '1 minute' WHERE id = $1`, usedCode.ID); err != nil {
		t.Fatalf("expire used device code: %v", err)
	}

	if err := repo.ExpireStaleDeviceAuthCodes(ctx); err != nil {
		t.Fatalf("ExpireStaleDeviceAuthCodes() error = %v", err)
	}

	unusedToken, err := repo.GetCLITokenByHash(ctx, "hash-unused")
	if err != nil {
		t.Fatalf("GetCLITokenByHash() unused error = %v", err)
	}
	if unusedToken.RevokedAt == nil {
		t.Fatal("unused stale token should be revoked")
	}

	usedToken, err = repo.GetCLITokenByHash(ctx, "hash-used")
	if err != nil {
		t.Fatalf("GetCLITokenByHash() used error = %v", err)
	}
	if usedToken.RevokedAt != nil {
		t.Fatal("used stale token should not be revoked")
	}
}
