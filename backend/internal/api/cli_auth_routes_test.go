package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

type stubCLIAuthService struct{}

func (stubCLIAuthService) CreateDeviceCode(_ context.Context) (CreateDeviceCodeResult, error) {
	return CreateDeviceCodeResult{
		DeviceCode:              "dc_test",
		UserCode:                "ABCD-EFGH",
		VerificationURI:         "/auth/device",
		VerificationURIComplete: "http://localhost:3000/auth/device?user_code=ABCD-EFGH",
		ExpiresIn:               600,
		Interval:                5,
	}, nil
}

func (stubCLIAuthService) PollDeviceToken(_ context.Context, _ string) (PollDeviceTokenResult, error) {
	return PollDeviceTokenResult{}, errors.New("authorization_pending")
}

func (stubCLIAuthService) ApproveDeviceCode(_ context.Context, _ Caller, _ string) error {
	return nil
}

func (stubCLIAuthService) CreateCLIToken(_ context.Context, _ Caller, _ string) (CreateCLITokenResult, error) {
	return CreateCLITokenResult{
		ID:        uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Token:     "clitok_test",
		Name:      "CLI Token",
		CreatedAt: time.Unix(0, 0),
	}, nil
}

func (stubCLIAuthService) ListCLITokens(_ context.Context, _ Caller) ([]CLITokenSummary, error) {
	return []CLITokenSummary{{
		ID:        uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Name:      "CLI Token",
		CreatedAt: time.Unix(0, 0),
	}}, nil
}

func (stubCLIAuthService) RevokeCLIToken(_ context.Context, _ Caller, _ uuid.UUID) error {
	return nil
}

func TestCLIAuthRoutesDoNotShadowProtectedAuthRoutes(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.Header.Set(headerUserID, "11111111-1111-1111-1111-111111111111")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		stubChallengePackAuthoringService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		stubCLIAuthService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func TestCLIAuthPublicDeviceRouteIsReachable(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/cli-auth/device", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		stubChallengePackAuthoringService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		stubCLIAuthService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}

	var body CreateDeviceCodeResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.VerificationURIComplete == "" {
		t.Fatal("verification_uri_complete should not be empty")
	}
}

func TestCLIAuthProtectedTokensRouteIsReachable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/cli-auth/tokens", nil)
	req.Header.Set(headerUserID, "11111111-1111-1111-1111-111111111111")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		stubChallengePackAuthoringService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		stubCLIAuthService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body struct {
		Items []CLITokenSummary `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(body.Items))
	}
}

func TestCLIAuthProtectedApproveRouteIsReachable(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/cli-auth/device/approve", bytes.NewBufferString(`{"user_code":"ABCD-EFGH"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, "11111111-1111-1111-1111-111111111111")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		stubChallengePackAuthoringService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		stubCLIAuthService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}
