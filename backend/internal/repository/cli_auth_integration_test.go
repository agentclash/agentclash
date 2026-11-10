package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
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

	consumed, err := repo.GetDeviceAuthCodeByDeviceCode(ctx, code.DeviceCode)
	if err != nil {
		t.Fatalf("GetDeviceAuthCodeByDeviceCode() error = %v", err)
	}
	if consumed.Status != string(repository.DeviceAuthCodeStatusConsumed) || consumed.RawToken != nil {
		t.Fatalf("code = status %q raw token %v, want consumed without raw token", consumed.Status, consumed.RawToken)
	}
}

func TestRepositoryExpireStaleDeviceAuthCodesKeepsConsumedToken(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	code, err := repo.CreateDeviceAuthCode(ctx, "dc_received", "RCVD-2345", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateDeviceAuthCode() error = %v", err)
	}
	if _, err := repo.ApproveDeviceAuthCodeWithToken(ctx, code.UserCode, fixture.userID, "hash-received", "CLI Device Login", "clitok_received", nil); err != nil {
		t.Fatalf("ApproveDeviceAuthCodeWithToken() error = %v", err)
	}
	if rawToken, err := repo.ConsumeDeviceRawToken(ctx, code.ID); err != nil || rawToken != "clitok_received" {
		t.Fatalf("ConsumeDeviceRawToken() = %q, %v", rawToken, err)
	}

	// The CLI holds the token but has not used it when the code expires.
	if _, err := db.Exec(ctx, `UPDATE device_auth_codes SET expires_at = now() - interval '1 minute' WHERE id = $1`, code.ID); err != nil {
		t.Fatalf("expire device code: %v", err)
	}
	if err := repo.ExpireStaleDeviceAuthCodes(ctx); err != nil {
		t.Fatalf("ExpireStaleDeviceAuthCodes() error = %v", err)
	}
	if err := repo.ExpireDeviceAuthCode(ctx, code.ID); err != nil {
		t.Fatalf("ExpireDeviceAuthCode() error = %v", err)
	}

	after, err := repo.GetDeviceAuthCodeByDeviceCode(ctx, code.DeviceCode)
	if err != nil {
		t.Fatalf("GetDeviceAuthCodeByDeviceCode() error = %v", err)
	}
	if after.Status != string(repository.DeviceAuthCodeStatusConsumed) {
		t.Fatalf("status = %q, want consumed", after.Status)
	}
	token, err := repo.GetCLITokenByHash(ctx, "hash-received")
	if err != nil {
		t.Fatalf("GetCLITokenByHash() error = %v", err)
	}
	if token.RevokedAt != nil {
		t.Fatal("a token that the CLI received must not be revoked by the sweeper")
	}
}

func TestRepositoryExpireStaleDeviceAuthCodesKeepsLegacyConsumedToken(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	code, err := repo.CreateDeviceAuthCode(ctx, "dc_legacy", "LGCY-2345", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateDeviceAuthCode() error = %v", err)
	}
	if _, err := repo.ApproveDeviceAuthCodeWithToken(ctx, code.UserCode, fixture.userID, "hash-legacy", "CLI Device Login", "clitok_legacy", nil); err != nil {
		t.Fatalf("ApproveDeviceAuthCodeWithToken() error = %v", err)
	}
	// An older API server consumed the token without writing 'consumed'.
	if _, err := db.Exec(ctx, `UPDATE device_auth_codes SET raw_token = NULL, expires_at = now() - interval '1 minute' WHERE id = $1`, code.ID); err != nil {
		t.Fatalf("simulate legacy consume: %v", err)
	}
	if err := repo.ExpireStaleDeviceAuthCodes(ctx); err != nil {
		t.Fatalf("ExpireStaleDeviceAuthCodes() error = %v", err)
	}

	after, err := repo.GetDeviceAuthCodeByDeviceCode(ctx, code.DeviceCode)
	if err != nil {
		t.Fatalf("GetDeviceAuthCodeByDeviceCode() error = %v", err)
	}
	if after.Status != string(repository.DeviceAuthCodeStatusApproved) {
		t.Fatalf("status = %q, want approved", after.Status)
	}
	token, err := repo.GetCLITokenByHash(ctx, "hash-legacy")
	if err != nil {
		t.Fatalf("GetCLITokenByHash() error = %v", err)
	}
	if token.RevokedAt != nil {
		t.Fatal("a token handed out before the consumed status existed must not be revoked")
	}
}

func TestRepositoryConsumeDeviceRawTokenRefusesExpiredCode(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	code, err := repo.CreateDeviceAuthCode(ctx, "dc_late", "LATE-2345", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateDeviceAuthCode() error = %v", err)
	}
	if _, err := repo.ApproveDeviceAuthCodeWithToken(ctx, code.UserCode, fixture.userID, "hash-late", "CLI Device Login", "clitok_late", nil); err != nil {
		t.Fatalf("ApproveDeviceAuthCodeWithToken() error = %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE device_auth_codes SET expires_at = now() - interval '1 minute' WHERE id = $1`, code.ID); err != nil {
		t.Fatalf("expire device code: %v", err)
	}

	rawToken, err := repo.ConsumeDeviceRawToken(ctx, code.ID)
	if err != nil {
		t.Fatalf("ConsumeDeviceRawToken() error = %v", err)
	}
	if rawToken != "" {
		t.Fatalf("raw token = %q, want empty for an expired code", rawToken)
	}

	if err := repo.ExpireStaleDeviceAuthCodes(ctx); err != nil {
		t.Fatalf("ExpireStaleDeviceAuthCodes() error = %v", err)
	}
	after, err := repo.GetDeviceAuthCodeByDeviceCode(ctx, code.DeviceCode)
	if err != nil {
		t.Fatalf("GetDeviceAuthCodeByDeviceCode() error = %v", err)
	}
	if after.Status != string(repository.DeviceAuthCodeStatusExpired) {
		t.Fatalf("status = %q, want expired", after.Status)
	}
	token, err := repo.GetCLITokenByHash(ctx, "hash-late")
	if err != nil {
		t.Fatalf("GetCLITokenByHash() error = %v", err)
	}
	if token.RevokedAt == nil {
		t.Fatal("a token that was never handed out must be revoked when its code expires")
	}

	// Expired is terminal.
	if rawToken, err := repo.ConsumeDeviceRawToken(ctx, code.ID); err != nil || rawToken != "" {
		t.Fatalf("ConsumeDeviceRawToken() after expiry = %q, %v", rawToken, err)
	}
	if err := repo.DenyDeviceAuthCode(ctx, code.UserCode); !errors.Is(err, repository.ErrDeviceCodeNotFound) {
		t.Fatalf("DenyDeviceAuthCode() after expiry error = %v, want ErrDeviceCodeNotFound", err)
	}
}

func TestRepositoryConsumedDeviceCodeIsTerminal(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	code, err := repo.CreateDeviceAuthCode(ctx, "dc_final", "FINL-2345", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateDeviceAuthCode() error = %v", err)
	}
	if _, err := repo.ApproveDeviceAuthCodeWithToken(ctx, code.UserCode, fixture.userID, "hash-final", "CLI Device Login", "clitok_final", nil); err != nil {
		t.Fatalf("ApproveDeviceAuthCodeWithToken() error = %v", err)
	}
	if _, err := repo.ConsumeDeviceRawToken(ctx, code.ID); err != nil {
		t.Fatalf("ConsumeDeviceRawToken() error = %v", err)
	}

	if _, err := repo.ApproveDeviceAuthCodeWithToken(ctx, code.UserCode, fixture.userID, "hash-final-2", "CLI Device Login", "clitok_final_2", nil); !errors.Is(err, repository.ErrDeviceCodeNotFound) {
		t.Fatalf("approve consumed code error = %v, want ErrDeviceCodeNotFound", err)
	}
	if err := repo.DenyDeviceAuthCode(ctx, code.UserCode); !errors.Is(err, repository.ErrDeviceCodeNotFound) {
		t.Fatalf("deny consumed code error = %v, want ErrDeviceCodeNotFound", err)
	}
	if err := repo.ExpireDeviceAuthCode(ctx, code.ID); err != nil {
		t.Fatalf("ExpireDeviceAuthCode() error = %v", err)
	}
	after, err := repo.GetDeviceAuthCodeByDeviceCode(ctx, code.DeviceCode)
	if err != nil {
		t.Fatalf("GetDeviceAuthCodeByDeviceCode() error = %v", err)
	}
	if after.Status != string(repository.DeviceAuthCodeStatusConsumed) {
		t.Fatalf("status = %q, want consumed", after.Status)
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
