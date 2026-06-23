package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func catalogTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func withChiURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestChallengePackCatalogListHandlerReturnsItems(t *testing.T) {
	handler := challengePackCatalogListHandler(catalogTestLogger())
	req := httptest.NewRequest(http.MethodGet, "/v1/challenge-pack-catalog", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Items []struct {
			Slug     string `json:"slug"`
			Category string `json:"category"`
			YAML     string `json:"yaml"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(response.Items) == 0 {
		t.Fatal("expected catalog items")
	}
	found := false
	for _, item := range response.Items {
		if item.Slug == "text-to-sql" {
			found = true
		}
		if item.YAML != "" {
			t.Errorf("list response must omit yaml; %s included it", item.Slug)
		}
		if item.Category == "" {
			t.Errorf("%s has empty category", item.Slug)
		}
	}
	if !found {
		t.Error("expected text-to-sql in the catalog list")
	}
}

func TestChallengePackCatalogListHandlerFiltersByCategory(t *testing.T) {
	handler := challengePackCatalogListHandler(catalogTestLogger())
	req := httptest.NewRequest(http.MethodGet, "/v1/challenge-pack-catalog?category=enterprise", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response struct {
		Items []struct {
			Category string `json:"category"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(response.Items) == 0 {
		t.Fatal("expected at least one enterprise pack")
	}
	for _, item := range response.Items {
		if item.Category != challengepackCategoryEnterprise {
			t.Errorf("category = %q, want enterprise", item.Category)
		}
	}
}

const challengepackCategoryEnterprise = "enterprise"

func TestChallengePackCatalogDetailHandlerReturnsYAML(t *testing.T) {
	handler := challengePackCatalogDetailHandler(catalogTestLogger())
	req := httptest.NewRequest(http.MethodGet, "/v1/challenge-pack-catalog/json-output-conformance", nil)
	req = withChiURLParam(req, "slug", "json-output-conformance")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Slug string `json:"slug"`
		YAML string `json:"yaml"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Slug != "json-output-conformance" {
		t.Fatalf("slug = %q", response.Slug)
	}
	if response.YAML == "" {
		t.Fatal("detail response must include yaml")
	}
}

func TestChallengePackCatalogDetailHandlerNotFound(t *testing.T) {
	handler := challengePackCatalogDetailHandler(catalogTestLogger())
	req := httptest.NewRequest(http.MethodGet, "/v1/challenge-pack-catalog/nope", nil)
	req = withChiURLParam(req, "slug", "nope")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestInstantiateCatalogPackManagerCreatesPack(t *testing.T) {
	repo := &fakeChallengePackAuthoringRepository{}
	manager := NewChallengePackAuthoringManager(repo, nil)

	result, err := manager.InstantiateCatalogPack(context.Background(), uuid.New(), "json-output-conformance")
	if err != nil {
		t.Fatalf("InstantiateCatalogPack: %v", err)
	}
	if result.AlreadyExisted {
		t.Error("AlreadyExisted = true, want false for a fresh clone")
	}
	if !result.Runnable {
		t.Error("Runnable = false, want true")
	}
	if result.ChallengePackID == uuid.Nil || result.ChallengePackVersionID == uuid.Nil {
		t.Fatal("expected non-nil pack and version ids")
	}
	if result.Slug != "json-output-conformance" {
		t.Fatalf("slug = %q", result.Slug)
	}
}

func TestInstantiateCatalogPackManagerIsIdempotent(t *testing.T) {
	existingPackID := uuid.New()
	existingVersionID := uuid.New()
	repo := &fakeChallengePackAuthoringRepository{
		publishErr:   repository.ErrChallengePackVersionExists,
		bySlugPackID: existingPackID,
		bySlugVerID:  existingVersionID,
		bySlugFound:  true,
	}
	manager := NewChallengePackAuthoringManager(repo, nil)

	result, err := manager.InstantiateCatalogPack(context.Background(), uuid.New(), "text-to-sql")
	if err != nil {
		t.Fatalf("InstantiateCatalogPack: %v", err)
	}
	if !result.AlreadyExisted {
		t.Error("AlreadyExisted = false, want true when the pack already exists")
	}
	if result.ChallengePackID != existingPackID || result.ChallengePackVersionID != existingVersionID {
		t.Fatalf("ids = %s/%s, want %s/%s", result.ChallengePackID, result.ChallengePackVersionID, existingPackID, existingVersionID)
	}
}

func TestInstantiateCatalogPackManagerUnknownSlug(t *testing.T) {
	manager := NewChallengePackAuthoringManager(&fakeChallengePackAuthoringRepository{}, nil)
	_, err := manager.InstantiateCatalogPack(context.Background(), uuid.New(), "does-not-exist")
	if err != errCatalogPackNotFound {
		t.Fatalf("error = %v, want errCatalogPackNotFound", err)
	}
}

func TestInstantiateChallengePackCatalogHandlerReturnsCreated(t *testing.T) {
	logger := catalogTestLogger()
	service := NewChallengePackAuthoringManager(&fakeChallengePackAuthoringRepository{}, nil)
	workspaceID := uuid.New()
	userID := uuid.New()

	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/challenge-pack-catalog/text-to-sql/instantiate", nil)
	req = withChiURLParam(req, "slug", "text-to-sql")
	ctx := context.WithValue(req.Context(), callerContextKey{}, Caller{
		UserID: userID,
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceAdmin},
		},
	})
	ctx = context.WithValue(ctx, workspaceIDContextKey{}, workspaceID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	instantiateChallengePackCatalogHandler(logger, service, NewCallerWorkspaceAuthorizer()).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var response InstantiateCatalogPackResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.ChallengePackID == uuid.Nil || response.ChallengePackVersionID == uuid.Nil {
		t.Fatal("expected non-nil ids")
	}
	if !response.Runnable {
		t.Error("Runnable = false, want true")
	}
}

func TestInstantiateChallengePackCatalogHandlerUnknownSlug(t *testing.T) {
	logger := catalogTestLogger()
	service := NewChallengePackAuthoringManager(&fakeChallengePackAuthoringRepository{}, nil)
	workspaceID := uuid.New()
	userID := uuid.New()

	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/challenge-pack-catalog/nope/instantiate", nil)
	req = withChiURLParam(req, "slug", "nope")
	ctx := context.WithValue(req.Context(), callerContextKey{}, Caller{
		UserID: userID,
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceAdmin},
		},
	})
	ctx = context.WithValue(ctx, workspaceIDContextKey{}, workspaceID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	instantiateChallengePackCatalogHandler(logger, service, NewCallerWorkspaceAuthorizer()).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}
