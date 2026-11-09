package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestTryoutCreatorCookieAndClaimBoundary(t *testing.T) {
	workspace := uuid.New()
	repo := newFakeAgentTryoutRepository(uuid.New(), workspace)
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := chi.NewRouter()
	registerPublicAgentTryoutRoutes(router, logger, manager)
	caller := callerWithWorkspace(workspace)
	router.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), callerContextKey{}, caller)))
			})
		})
		registerProtectedAgentTryoutRoutes(r, logger, manager)
	})
	request := func(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	create := request("POST", "/agent-tryouts", `{"template_slug":"meeting-minutes","input":{"notes":"synthetic fixture"}}`, nil)
	if create.Code != 201 {
		t.Fatalf("create status=%d", create.Code)
	}
	var result struct {
		ID           uuid.UUID `json:"id"`
		SaveEligible bool      `json:"save_eligible"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	cookies := create.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("creator cookie count=%d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.Domain != "" || cookie.Path != "/v1/agent-tryouts/"+result.ID.String() || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge <= 0 {
		t.Fatal("creator cookie attributes are unsafe")
	}
	if strings.Contains(create.Body.String(), cookie.Value) || strings.Contains(create.Body.String(), "proof") {
		t.Fatal("proof leaked in response")
	}
	if result.SaveEligible {
		t.Fatal("queued result must not advertise save eligibility")
	}
	path := "/agent-tryouts/" + result.ID.String()
	if got := request("GET", path, "", nil).Code; got != 404 {
		t.Fatalf("missing proof read=%d", got)
	}
	wrong := *cookie
	wrong.Value = strings.Repeat("x", 43)
	if got := request("GET", path, "", &wrong).Code; got != 404 {
		t.Fatalf("wrong proof read=%d", got)
	}
	if got := request("GET", path, "", cookie).Code; got != 200 {
		t.Fatalf("creator read=%d", got)
	}
	if got := request("GET", path+"/events", "", nil).Code; got != 404 {
		t.Fatalf("missing proof events=%d", got)
	}
	if got := request("POST", path+"/turns", `{"message":"hello"}`, nil).Code; got != 404 {
		t.Fatalf("missing proof turn=%d", got)
	}
	stored := repo.tryouts[result.ID]
	if stored.AnonymousOwnerProofHash == nil || len(*stored.AnonymousOwnerProofHash) != 64 || *stored.AnonymousOwnerProofHash == cookie.Value {
		t.Fatal("proof digest not persisted")
	}
	originalExpiry := stored.ExpiresAt
	expired := time.Now().Add(-time.Minute)
	stored.ExpiresAt = &expired
	repo.tryouts[result.ID] = stored
	if got := request("GET", path, "", cookie).Code; got != 410 {
		t.Fatalf("expired owner read=%d", got)
	}
	if got := request("GET", path, "", nil).Code; got != 404 {
		t.Fatalf("expiry leaked to nonowner=%d", got)
	}
	if got := request("POST", path+"/claim", `{"workspace_id":"`+workspace.String()+`"}`, cookie).Code; got != 410 {
		t.Fatalf("expired claim=%d", got)
	}
	stored.ExpiresAt = originalExpiry
	repo.tryouts[result.ID] = stored
	claim := `{"workspace_id":"` + workspace.String() + `"}`
	if got := request("POST", path+"/claim", claim, cookie).Code; got != 409 {
		t.Fatalf("queued claim status=%d", got)
	}
	if got := request("POST", path+"/turns", `{"message":"keep working"}`, cookie).Code; got != 202 {
		t.Fatalf("creator reply status=%d", got)
	}
	if got := request("POST", path+"/turns", `{"end":true}`, cookie).Code; got != 202 {
		t.Fatalf("creator end status=%d", got)
	}
	stored.Status = repository.AgentTryoutStatusCompleted
	repo.tryouts[result.ID] = stored
	read := request("GET", path, "", cookie)
	if err := json.Unmarshal(read.Body.Bytes(), &result); err != nil || !result.SaveEligible {
		t.Fatal("finished creator result must advertise eligibility")
	}
	if got := request("POST", path+"/claim", claim, nil).Code; got != 404 {
		t.Fatalf("missing proof claim=%d", got)
	}
	if got := request("POST", path+"/claim", `{"workspace_id":"`+uuid.NewString()+`"}`, cookie).Code; got != 403 {
		t.Fatalf("unauthorized workspace=%d", got)
	}
	if got := request("POST", path+"/claim", claim, cookie).Code; got != 200 {
		t.Fatalf("claim=%d", got)
	}
	before := repo.tryouts[result.ID]
	if got := request("POST", path+"/claim", claim, cookie).Code; got != 200 {
		t.Fatalf("same claim retry=%d", got)
	}
	after := repo.tryouts[result.ID]
	if before.ClaimedAt == nil || after.ClaimedAt == nil || !before.ClaimedAt.Equal(*after.ClaimedAt) {
		t.Fatal("retry changed claim transition")
	}
	if got := request("GET", path, "", cookie).Code; got != 404 {
		t.Fatalf("private result anonymous read=%d", got)
	}
	if got := request("GET", path+"/saved", "", nil).Code; got != 200 {
		t.Fatalf("authenticated saved read=%d", got)
	}
}

func TestTryoutLegacyCannotBeClaimedOrMutated(t *testing.T) {
	workspace := uuid.New()
	repo := newFakeAgentTryoutRepository(uuid.New(), workspace)
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)
	id := uuid.New()
	expires := time.Now().Add(time.Hour)
	repo.tryouts[id] = repository.AgentTryout{ID: id, Status: repository.AgentTryoutStatusRunning, ExpiresAt: &expires}
	if _, err := manager.GetPublicTryout(context.Background(), id); err != nil {
		t.Fatal("legacy public read unexpectedly unavailable", err)
	}
	if err := manager.SubmitAnonymousTryoutTurn(context.Background(), id, SubmitAgentTryoutTurnInput{Message: "mutate"}); err == nil {
		t.Fatal("legacy mutation accepted")
	}
	if _, err := manager.ClaimTryout(context.Background(), callerWithWorkspace(workspace), ClaimAgentTryoutInput{ID: id, WorkspaceID: workspace}); err == nil {
		t.Fatal("legacy ID-only claim accepted")
	}
}

func TestTryoutCreationRequiresCreatorDigest(t *testing.T) {
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)
	for _, proof := range []string{"", "invalid", strings.Repeat("z", 64)} {
		_, err := manager.CreateAnonymousTryout(context.Background(), CreateAnonymousAgentTryoutInput{
			TemplateSlug: "meeting-minutes", Input: json.RawMessage(`{"notes":"fixture"}`), OwnerProofHash: proof,
		})
		if err == nil {
			t.Fatal("creation without a valid creator digest accepted")
		}
	}
	if len(repo.tryouts) != 0 {
		t.Fatal("invalid ownership created a proofless legacy-shaped record")
	}
}
