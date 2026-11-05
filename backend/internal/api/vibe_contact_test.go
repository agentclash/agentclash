package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/enquiries"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func contactHandler(t *testing.T, withGate ...bool) *VibeHandler {
	t.Helper()
	var rc *redis.Client
	if len(withGate) == 0 || withGate[0] {
		rc = redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
		t.Cleanup(func() { _ = rc.Close() })
	}
	return &VibeHandler{Service: &vibe.Service{Gate: vibe.Gate{Redis: rc}}, CookieSecret: strings.Repeat("s", 32), Enquiries: &enquiries.Store{Recipient: "team@example.test", Sender: &testEnquirySender{}}}
}
func contactBody(website string) string {
	b, _ := json.Marshal(enquiries.ContactRequest{ClientID: uuid.New(), Email: "visitor@example.test", Work: "Automate returns", Website: website})
	return string(b)
}
func TestVibeContactHandler(t *testing.T) {
	for _, test := range []struct {
		name, body string
		disabled   bool
		status     int
		code       string
	}{
		{name: "honeypot receipt", body: contactBody("bot"), status: 201},
		{name: "invalid", body: `{"client_id":"` + uuid.NewString() + `","email":"invalid","work":"task"}`, status: 400, code: "invalid_request"},
		{name: "disabled", body: contactBody(""), disabled: true, status: 503, code: "hosted_disabled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := contactHandler(t)
			if test.disabled {
				h.Enquiries = nil
			}
			r := httptest.NewRequest("POST", "/enquiries", strings.NewReader(test.body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.Routes().ServeHTTP(w, r)
			if w.Code != test.status || (test.code != "" && !strings.Contains(w.Body.String(), test.code)) {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
			if test.status == 201 {
				var receipt enquiries.Receipt
				if err := json.Unmarshal(w.Body.Bytes(), &receipt); err != nil || receipt.ID == uuid.Nil || receipt.Status != "received" {
					t.Fatalf("bad receipt: %s", w.Body)
				}
				if len(w.Result().Cookies()) == 0 {
					t.Fatal("missing anonymous cookie")
				}
			}
		})
	}
}
func TestVibeContactBadBearer(t *testing.T) {
	h := contactHandler(t, false)
	h.Auth = &stubAuthenticator{err: errors.New("bad token")}
	r := httptest.NewRequest("POST", "/enquiries", strings.NewReader(contactBody("bot")))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer bad")
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, r)
	if w.Code != 401 || !strings.Contains(w.Body.String(), "unauthenticated") || len(w.Result().Cookies()) != 0 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}
func TestVibeContactIPLimitIgnoresForwardedHeaders(t *testing.T) {
	h := contactHandler(t)
	for i := 0; i < 6; i++ {
		r := httptest.NewRequest("POST", "/enquiries", strings.NewReader(contactBody("bot")))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = "192.0.2.1:1234"
		r.Header.Set("X-Forwarded-For", uuid.NewString())
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, r)
		want := 201
		if i == 5 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("request %d: %d %s", i, w.Code, w.Body)
		}
	}
}
func TestVibeConfigContactDirect(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		h := contactHandler(t, false)
		if !enabled {
			h.Enquiries = nil
		}
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, httptest.NewRequest("GET", "/config", nil))
		var body struct {
			Contact struct {
				Direct    *bool  `json:"direct"`
				Available bool   `json:"available"`
				Email     string `json:"email"`
			} `json:"contact"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Contact.Direct == nil || *body.Contact.Direct != enabled || body.Contact.Available != enabled {
			t.Fatalf("bad config: %s", w.Body)
		}
	}
}

func TestVibeContactCreateQuotaAndConflict(t *testing.T) {
	if os.Getenv("VIBE_TEST_DATABASE_URL") == "" {
		t.Skip("isolated migrated database required")
	}
	// Use a fresh migrated database: delivery tests in the enquiries package
	// consume the whole outbox and must not claim this handler test's rows.
	db := browserFixtureDatabase(t, context.Background())
	h := contactHandler(t)
	h.Enquiries.DB = db
	var cookie *http.Cookie
	i := 0
	post := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		i++
		r := httptest.NewRequest("POST", "/enquiries", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = fmt.Sprintf("192.0.2.%d:1234", i)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, r)
		if cookie == nil && len(w.Result().Cookies()) > 0 {
			cookie = w.Result().Cookies()[0]
		}
		return w
	}
	body := contactBody("")
	first := post(body)
	if first.Code != 201 {
		t.Fatalf("%d %s", first.Code, first.Body)
	}
	request := httptest.NewRequest("GET", "/", nil)
	request.AddCookie(cookie)
	actor := h.anonymous(request)
	defer db.Exec(context.Background(), `DELETE FROM vibe_enquiries WHERE actor=$1`, actor)
	again := post(body)
	if again.Code != 201 || again.Body.String() != first.Body.String() {
		t.Fatalf("idempotent retry: %d %s", again.Code, again.Body)
	}
	conflict := post(strings.Replace(body, "Automate returns", "Different work", 1))
	if conflict.Code != 409 || !strings.Contains(conflict.Body.String(), "idempotency_conflict") {
		t.Fatalf("conflict: %d %s", conflict.Code, conflict.Body)
	}
	for j := 0; j < 2; j++ {
		w := post(contactBody(""))
		if w.Code != 201 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	}
	quota := post(contactBody(""))
	if quota.Code != 429 || !strings.Contains(quota.Body.String(), "rate_limit") || strings.Contains(quota.Body.String(), "project") {
		t.Fatalf("quota: %d %s", quota.Code, quota.Body)
	}
}
