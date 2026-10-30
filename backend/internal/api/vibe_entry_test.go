package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func entryGuestCookie(h *VibeHandler) *http.Cookie {
	token := uuid.NewString()
	mac := hmac.New(sha256.New, []byte(h.CookieSecret))
	mac.Write([]byte(token))
	return &http.Cookie{Name: h.cookieName(), Value: token + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))}
}

func assertEntryConfig(t *testing.T, h *VibeHandler, cookie *http.Cookie, auth string, wantTrial bool, used, pending bool) {
	t.Helper()
	r := httptest.NewRequest("GET", "/config", nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, r)
	var body struct {
		Ceiling int64 `json:"first_build_ceiling_nano_usd"`
		Trial   *struct {
			Used    *bool `json:"initial_build_used"`
			Pending *bool `json:"build_in_progress"`
		} `json:"guest_trial"`
	}
	if w.Code != 200 {
		t.Fatalf("config: %d %s", w.Code, w.Body)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	if body.Ceiling != vibe.FirstBuildSpendCeiling {
		t.Errorf("ceiling=%d, want %d", body.Ceiling, vibe.FirstBuildSpendCeiling)
	}
	if _, present := fields["guest_trial"]; present != wantTrial {
		t.Fatalf("guest_trial presence=%v, want %v: %s", present, wantTrial, w.Body)
	}
	if wantTrial {
		if body.Trial == nil || body.Trial.Used == nil || body.Trial.Pending == nil {
			t.Fatalf("guest_trial must contain both boolean flags: %s", w.Body)
		}
		if *body.Trial.Used != used || *body.Trial.Pending != pending {
			t.Errorf("guest trial used=%v pending=%v, want used=%v pending=%v", *body.Trial.Used, *body.Trial.Pending, used, pending)
		}
	}
	if len(w.Header().Values("Set-Cookie")) != 0 {
		t.Fatal("config issued a cookie")
	}
}

func TestVibeConfigEntryPublic(t *testing.T) {
	h := &VibeHandler{Service: &vibe.Service{}, CookieSecret: strings.Repeat("s", 32)}
	for _, tt := range []struct {
		name, auth string
		cookie     *http.Cookie
		trial      bool
	}{
		{name: "no cookie", trial: true},
		{name: "invalid cookie", cookie: &http.Cookie{Name: h.cookieName(), Value: "invalid"}, trial: true},
		{name: "signed in", auth: "Bearer existing-user", cookie: entryGuestCookie(h)},
	} {
		t.Run(tt.name, func(t *testing.T) { assertEntryConfig(t, h, tt.cookie, tt.auth, tt.trial, false, false) })
	}
}

func TestVibeConfigEntryDatabaseFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, "postgres://unused:unused@127.0.0.1:1/vibe_test?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := &VibeHandler{Service: &vibe.Service{Store: &vibe.Store{DB: db}}, CookieSecret: strings.Repeat("s", 32)}
	assertEntryConfig(t, h, entryGuestCookie(h), "", false, false, false)
}

func TestVibeSessionTitleRejectsBeforeCookie(t *testing.T) {
	h := &VibeHandler{Service: &vibe.Service{}}
	for _, title := range []string{strings.Repeat("a", 81), "bad\nname", "bad\x00name", "bad\u007fname", "\tname", "name\n"} {
		t.Run(title, func(t *testing.T) {
			b, _ := json.Marshal(map[string]any{"id": uuid.New(), "door": "build", "title": title})
			r := httptest.NewRequest("POST", "/sessions", bytes.NewReader(b))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.Routes().ServeHTTP(w, r)
			if w.Code != 400 || !strings.Contains(w.Body.String(), `"code":"invalid_request"`) {
				t.Fatalf("invalid title: %d %s", w.Code, w.Body)
			}
			if len(w.Header().Values("Set-Cookie")) != 0 {
				t.Fatal("invalid title issued cookie")
			}
		})
	}
}
