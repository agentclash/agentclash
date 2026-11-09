package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var tryoutFixtureOrg = uuid.MustParse("b8000000-0000-4000-8000-000000000001")
var tryoutFixtureWorkspace = uuid.MustParse("b8000000-0000-4000-8000-000000000002")
var tryoutFixtureOtherWorkspace = uuid.MustParse("b8000000-0000-4000-8000-000000000003")
var tryoutFixtureNewWorkspace = uuid.MustParse("b8000000-0000-4000-8000-000000000004")

const tryoutFixtureUser = "a8000000-0000-4000-8000-000000000001"
const tryoutFixtureNewUser = "a8000000-0000-4000-8000-000000000006"

type browserTryoutRepository struct{ *fakeAgentTryoutRepository }

func (r *browserTryoutRepository) GetOrganizationIDByWorkspaceID(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	for _, valid := range []uuid.UUID{tryoutFixtureWorkspace, tryoutFixtureOtherWorkspace, tryoutFixtureNewWorkspace} {
		if id == valid {
			return tryoutFixtureOrg, nil
		}
	}
	return uuid.Nil, repository.ErrAgentTryoutNotFound
}
func (r *browserTryoutRepository) CreateAgentTryout(ctx context.Context, p repository.CreateAgentTryoutParams) (repository.AgentTryout, error) {
	p.Status = repository.AgentTryoutStatusCompleted
	p.RedactionStatus = repository.AgentTryoutRedactionPassed
	p.Summary = json.RawMessage(`{"message":"Synthetic browser result. No model execution."}`)
	return r.fakeAgentTryoutRepository.CreateAgentTryout(ctx, p)
}

// An opt-in loopback HTTP fixture using production handlers/managers/CORS. The
// in-memory repository is deliberately distinct from the real SQL race test.
func TestTryoutBrowserFixture(t *testing.T) {
	if os.Getenv("TRYOUT_BROWSER_FIXTURE") != "1" {
		t.Skip("browser fixture only")
	}
	address := os.Getenv("TRYOUT_BROWSER_API_ADDRESS")
	origin := os.Getenv("TRYOUT_BROWSER_WEB_ORIGIN")
	if !strings.HasPrefix(address, "127.0.0.1:") || !strings.HasPrefix(origin, "http://127.0.0.1:") {
		t.Fatal("fixture must bind loopback")
	}
	repo := &browserTryoutRepository{newFakeAgentTryoutRepository(tryoutFixtureOrg, tryoutFixtureWorkspace)}
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := chi.NewRouter()
	var lock sync.Mutex
	memberships := map[string][]uuid.UUID{tryoutFixtureUser: {tryoutFixtureWorkspace, tryoutFixtureOtherWorkspace}, tryoutFixtureNewUser: {}}
	missingCapability := map[string]bool{}
	names := map[uuid.UUID]string{tryoutFixtureWorkspace: "Existing workspace", tryoutFixtureOtherWorkspace: "Selected workspace", tryoutFixtureNewWorkspace: "Created workspace"}
	// Serialize the fake store only; SQL concurrency is covered by real Postgres.
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lock.Lock()
			defer lock.Unlock()
			r = r.WithContext(context.WithValue(r.Context(), tryoutDevelopmentContextKey{}, true))
			id := strings.TrimPrefix(r.URL.Path, "/v1/agent-tryouts/")
			if r.Method == http.MethodGet && missingCapability[id] {
				rec := httptest.NewRecorder()
				next.ServeHTTP(rec, r)
				var body map[string]any
				if rec.Code == 200 && json.Unmarshal(rec.Body.Bytes(), &body) == nil {
					delete(body, "save_eligible")
					for key, values := range rec.Header() {
						w.Header()[key] = values
					}
					writeJSON(w, 200, body)
					return
				}
				for key, values := range rec.Header() {
					w.Header()[key] = values
				}
				w.WriteHeader(rec.Code)
				_, _ = w.Write(rec.Body.Bytes())
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	router.Use(newCORSMiddleware("workos", map[string]struct{}{origin: {}}))
	router.Get("/__fixture/ready", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]bool{"ready": true}) })
	router.Get("/__fixture/result/{id}", func(w http.ResponseWriter, r *http.Request) {
		row, ok := repo.tryouts[uuid.MustParse(chi.URLParam(r, "id"))]
		if !ok {
			w.WriteHeader(404)
			return
		}
		writeJSON(w, 200, map[string]any{"id": row.ID, "workspace_id": row.WorkspaceID, "claimed_by_user_id": row.ClaimedByUserID, "claimed_at": row.ClaimedAt})
	})
	router.Post("/__fixture/result/{id}/{mode}", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			w.WriteHeader(400)
			return
		}
		row, ok := repo.tryouts[id]
		if !ok {
			w.WriteHeader(404)
			return
		}
		switch chi.URLParam(r, "mode") {
		case "expired":
			past := time.Now().Add(-time.Hour)
			row.ExpiresAt = &past
		case "old-capability":
			missingCapability[id.String()] = true
		case "legacy":
			row.AnonymousOwnerProofHash = nil
		case "private":
			row.WorkspaceID = &tryoutFixtureOtherWorkspace
			row.OrganizationID = &tryoutFixtureOrg
		default:
			w.WriteHeader(400)
			return
		}
		repo.tryouts[id] = row
		w.WriteHeader(204)
	})
	router.Route("/v1", func(r chi.Router) {
		r.Get("/tool-library", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]any{"items": []any{}}) })
		registerPublicAgentTryoutRoutes(r, logger, manager)
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Tokens come from the local fake identity provider. No production auth is replaced.
					parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), ".")
					var payload struct {
						Sub string `json:"sub"`
					}
					if len(parts) != 3 {
						writeError(w, 401, "unauthenticated", "Fixture sign in required")
						return
					}
					bytes, err := base64.RawURLEncoding.DecodeString(parts[1])
					if err != nil || json.Unmarshal(bytes, &payload) != nil {
						w.WriteHeader(401)
						return
					}
					spaces, ok := memberships[payload.Sub]
					if !ok {
						w.WriteHeader(401)
						return
					}
					caller := Caller{UserID: uuid.MustParse(payload.Sub), OrganizationMemberships: map[uuid.UUID]OrganizationMembership{}, WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{}}
					for _, id := range spaces {
						caller.WorkspaceMemberships[id] = WorkspaceMembership{WorkspaceID: id, Role: RoleWorkspaceAdmin}
					}
					if len(spaces) > 0 {
						caller.OrganizationMemberships[tryoutFixtureOrg] = OrganizationMembership{OrganizationID: tryoutFixtureOrg, Role: RoleOrgAdmin}
					}
					next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), callerContextKey{}, caller)))
				})
			})
			r.Get("/auth/session", sessionHandler)
			r.Get("/users/me", func(w http.ResponseWriter, r *http.Request) {
				caller, _ := CallerFromContext(r.Context())
				spaces := []map[string]any{}
				for _, id := range memberships[caller.UserID.String()] {
					spaces = append(spaces, map[string]any{"id": id, "name": names[id], "slug": "fixture", "role": "workspace_admin"})
				}
				orgs := []map[string]any{}
				if len(spaces) > 0 {
					orgs = append(orgs, map[string]any{"id": tryoutFixtureOrg, "name": "Fixture org", "slug": "fixture", "role": "org_admin", "workspaces": spaces})
				}
				writeJSON(w, 200, map[string]any{"user_id": caller.UserID, "organizations": orgs})
			})
			r.Post("/onboarding", func(w http.ResponseWriter, r *http.Request) {
				caller, _ := CallerFromContext(r.Context())
				var input struct {
					OrganizationName string `json:"organization_name"`
					WorkspaceName    string `json:"workspace_name"`
				}
				if json.NewDecoder(r.Body).Decode(&input) != nil || strings.TrimSpace(input.WorkspaceName) == "" {
					w.WriteHeader(400)
					return
				}
				if len(memberships[caller.UserID.String()]) > 0 {
					writeError(w, 409, "already_onboarded", "Already onboarded")
					return
				}
				memberships[caller.UserID.String()] = []uuid.UUID{tryoutFixtureNewWorkspace}
				names[tryoutFixtureNewWorkspace] = input.WorkspaceName
				writeJSON(w, 201, map[string]any{"organization": map[string]any{"id": tryoutFixtureOrg, "name": input.OrganizationName}, "workspace": map[string]any{"id": tryoutFixtureNewWorkspace, "name": input.WorkspaceName}})
			})
			registerProtectedAgentTryoutRoutes(r, logger, manager)
		})
	})
	server := &http.Server{Addr: address, Handler: router, ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		t.Fatal(err)
	}
}
