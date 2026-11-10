package api

import (
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTryoutCatalogAnnouncesCreatorProtocol(t *testing.T) {
	rec := httptest.NewRecorder()
	listAgentTryoutTemplatesHandler(slog.Default(), &fakeAgentTryoutService{}).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/agent-tryout-templates", nil))
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), `"creator_access_protocol":"cookie-v1"`) {
		t.Fatalf("catalog status=%d cache=%q body=%s", rec.Code, rec.Header().Get("Cache-Control"), rec.Body.String())
	}
}

func TestAnonymousTryoutRejectsStaleProtocolBeforeService(t *testing.T) {
	for _, protocol := range []string{"", `,"creator_access_protocol":"cookie-v2"`, `,"creator_access_protocol":null`, `,"creator_access_protocol":""`} {
		t.Run(protocol, func(t *testing.T) {
			service := &fakeAgentTryoutService{}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/agent-tryouts", strings.NewReader(`{"template_slug":"meeting-minutes","input":{"notes":"fixture"}`+protocol+`}`))
			createAnonymousAgentTryoutHandler(slog.Default(), service).ServeHTTP(rec, req)
			if rec.Code != 409 || !strings.Contains(rec.Body.String(), `"code":"creator_access_protocol_required"`) || !strings.Contains(rec.Body.String(), "Reload") {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if service.createAnonymousInput.TemplateSlug != "" || len(rec.Result().Cookies()) != 0 {
				t.Fatal("stale protocol invoked creation or issued a cookie")
			}
		})
	}
}
