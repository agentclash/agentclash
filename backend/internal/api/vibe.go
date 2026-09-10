package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type VibeHandler struct {
	Billing      *BillingManager
	Service      *vibe.Service
	Auth         Authenticator
	CookieSecret string
	Secure       bool
}

func (h *VibeHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			// Enforce the absolute transport ceiling before loading any session
			// context. The endpoint then applies the stricter trial/type limits.
			if r.Method == http.MethodPost || r.Method == http.MethodPatch {
				limit := vibe.LimitsFor(false).RequestBytes
				if strings.HasSuffix(r.URL.Path, "/import") {
					limit = vibe.LimitsFor(false).FileBytes
				}
				if encoding := r.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
					vibeError(w, &vibe.Fault{Code: "invalid_request", Message: "Compressed requests are not supported."})
					return
				}
				bounded := http.MaxBytesReader(w, r.Body, int64(limit))
				body, err := io.ReadAll(bounded)
				_ = bounded.Close()
				if err != nil {
					vibeError(w, err)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Get("/config", h.config)
	r.Get("/credits", h.creditBalance)
	r.Post("/credit-checkouts", h.creditCheckout)
	r.Post("/sessions", h.create)
	r.Get("/sessions/{sessionID}", h.get)
	r.Post("/sessions/{sessionID}/messages", h.submit)
	r.Post("/sessions/{sessionID}/import", h.importFile)
	r.Patch("/sessions/{sessionID}", h.edit)
	r.Post("/sessions/{sessionID}/claim", h.claim)
	r.Post("/sessions/{sessionID}/save", h.save)
	r.Get("/sessions/{sessionID}/events", h.events)
	r.Post("/operations/{operationID}/approve", h.approve)
	r.Post("/operations/{operationID}/stop", h.stop)
	r.Get("/operations/{operationID}/case", h.caseEvidence)
	return r
}
func (h *VibeHandler) caseEvidence(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "operationID")
	if err != nil {
		vibeError(w, err)
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" || len(key) > vibe.MaxKeyBytes {
		vibeError(w, &vibe.Fault{Code: "invalid_request", Message: "Choose a case key of at most 128 bytes."})
		return
	}
	if err = h.Service.Gate.Check(r.Context(), "evidence:"+actor, vibe.LimitsFor(strings.HasPrefix(actor, "anon:"))); err != nil {
		vibeError(w, err)
		return
	}
	result, err := h.Service.Store.GetCase(r.Context(), actor, id, key)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, result)
}
func vibeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false) // JSON is fetched as data; never embedded in HTML.
	_ = encoder.Encode(v)
}
func vibeError(w http.ResponseWriter, err error) {
	code, message, status := "unavailable", "Vibe could not complete this request. Try again shortly.", 503
	var f *vibe.Fault
	if errors.As(err, &f) {
		code, message = f.Code, f.Message
		status = 400
		switch code {
		case "not_found":
			status = 404
		case "forbidden":
			status = 403
		case "revision_conflict", "idempotency_conflict", "operation_running", "invalid_state", "saved_model_conflict":
			status = 409
		case "rate_limit", "capacity_limit", "trial_limit":
			status = 429
		case "insufficient_credits":
			status = 402
		case "hosted_disabled", "pricing_unavailable", "accounting_unavailable", "trial_capacity_reached":
			status = 503
		}
	}
	var size *http.MaxBytesError
	if errors.As(err, &size) {
		code, message, status = "request_too_large", "Request exceeds its byte limit.", 413
	}
	issue := map[string]any{"code": code, "message": message}
	if f != nil && f.Context != nil {
		issue["context"] = f.Context
	}
	vibeJSON(w, status, map[string]any{"error": issue})
}
func (h *VibeHandler) cookieName() string {
	if h.Secure {
		return "__Host-vibe_trial"
	}
	return "vibe_trial"
}
func (h *VibeHandler) anonymous(r *http.Request) string {
	if len(h.CookieSecret) < 32 {
		return ""
	}
	cookie, err := r.Cookie(h.cookieName())
	if err != nil {
		return ""
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 || len(parts[0]) > 64 {
		return ""
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(h.CookieSecret))
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return ""
	}
	return "anon:" + vibe.Hash([]byte(cookie.Value))
}
func (h *VibeHandler) actor(r *http.Request) (string, error) {
	if r.Header.Get("Authorization") != "" {
		caller, err := h.Auth.Authenticate(r)
		if err != nil {
			return "", &vibe.Fault{Code: "forbidden", Message: "Sign in again to continue."}
		}
		return "user:" + caller.UserID.String(), nil
	}
	if actor := h.anonymous(r); actor != "" {
		return actor, nil
	}
	return "", &vibe.Fault{Code: "forbidden", Message: "Start a new conversation to create a private trial."}
}
func (h *VibeHandler) issue(w http.ResponseWriter, r *http.Request) (string, error) {
	if len(h.CookieSecret) < 32 {
		return "", &vibe.Fault{Code: "hosted_disabled", Message: "Anonymous sessions are not configured."}
	}
	// RemoteAddr is the trusted transport peer. Forwarded headers are never
	// accepted as a rate-limit identity without a configured trusted proxy.
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if err := h.Service.Gate.Check(r.Context(), "issue:"+ip, vibe.LimitsFor(true)); err != nil {
		return "", err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, []byte(h.CookieSecret))
	mac.Write([]byte(token))
	token += "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	http.SetCookie(w, &http.Cookie{Name: h.cookieName(), Value: token, Path: "/", HttpOnly: true, Secure: h.Secure, SameSite: http.SameSiteStrictMode, MaxAge: 60 * 60 * 24 * 365})
	return "anon:" + vibe.Hash([]byte(token)), nil
}
func vibeBody(w http.ResponseWriter, r *http.Request, anonymous bool, dst any) error {
	if r.Header.Get("Content-Encoding") != "" && r.Header.Get("Content-Encoding") != "identity" {
		return &vibe.Fault{Code: "invalid_request", Message: "Compressed requests are not supported."}
	}
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return &vibe.Fault{Code: "invalid_request", Message: "Send an application/json request."}
	}
	l := vibe.LimitsFor(anonymous)
	r.Body = http.MaxBytesReader(w, r.Body, int64(l.RequestBytes))
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if err = vibe.Decode(b, l, dst); err != nil {
		return &vibe.Fault{Code: "invalid_request", Message: err.Error()}
	}
	return nil
}
func vibeID(r *http.Request, key string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		return uuid.Nil, &vibe.Fault{Code: "not_found", Message: "Resource is unavailable."}
	}
	return id, nil
}
func (h *VibeHandler) config(w http.ResponseWriter, r *http.Request) {
	models := []vibe.ModelProfile{}
	for id := range h.Service.Config.Profiles {
		if p, err := h.Service.Config.Profile(id); err == nil {
			models = append(models, p)
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	vibeJSON(w, 200, map[string]any{"capabilities": vibe.Capabilities(), "enabled": h.Service.Config.Enabled, "free_only": h.Service.Config.FreeOnly, "models": models, "defaults": h.Service.Config.DefaultModels(), "anonymous_limits": vibe.LimitsFor(true), "signed_in_limits": vibe.LimitsFor(false), "trial_budget_nano_usd": vibe.TrialBudget})
}
func (h *VibeHandler) create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID          uuid.UUID  `json:"id"`
		WorkspaceID *uuid.UUID `json:"workspace_id,omitempty"`
	}
	if err := vibeBody(w, r, true, &input); err != nil {
		vibeError(w, err)
		return
	}
	if input.ID == uuid.Nil {
		vibeError(w, &vibe.Fault{Code: "invalid_request", Message: "A client-generated session ID is required."})
		return
	}
	actor, err := h.actor(r)
	if err != nil && r.Header.Get("Authorization") == "" {
		actor, err = h.issue(w, r)
	}
	if err != nil {
		vibeError(w, err)
		return
	}
	if err = h.Service.Gate.Check(r.Context(), actor, vibe.LimitsFor(strings.HasPrefix(actor, "anon:"))); err != nil {
		vibeError(w, err)
		return
	}
	v, err := h.Service.Store.CreateSession(r.Context(), actor, input.WorkspaceID, input.ID, h.Service.Config.DefaultModels())
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 201, v)
}
func (h *VibeHandler) session(r *http.Request) (vibe.Session, error) {
	actor, err := h.actor(r)
	if err != nil {
		return vibe.Session{}, err
	}
	id, err := vibeID(r, "sessionID")
	if err != nil {
		return vibe.Session{}, err
	}
	return h.Service.Store.GetSession(r.Context(), actor, id)
}
func (h *VibeHandler) get(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, v)
}
func (h *VibeHandler) submit(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	var sub vibe.Submission
	if err = vibeBody(w, r, v.Anonymous, &sub); err != nil {
		vibeError(w, err)
		return
	}
	o, err := h.Service.Prepare(r.Context(), v.Actor, v.ID, sub)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 202, o)
}
func (h *VibeHandler) importFile(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	l := vibe.LimitsFor(v.Anonymous)
	if r.Header.Get("Content-Encoding") != "" {
		vibeError(w, &vibe.Fault{Code: "invalid_request", Message: "Compressed imports are disabled."})
		return
	}
	media := strings.Split(r.Header.Get("Content-Type"), ";")[0]
	if media != "application/json" && media != "application/yaml" && media != "text/yaml" {
		vibeError(w, &vibe.Fault{Code: "invalid_import", Message: "Upload a single JSON or YAML evaluation file."})
		return
	}
	revision, err := strconv.ParseInt(r.Header.Get("If-Match"), 10, 64)
	if err != nil {
		vibeError(w, &vibe.Fault{Code: "revision_conflict", Message: "The current conversation revision is required."})
		return
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(l.FileBytes)))
	if err == nil {
		err = h.Service.Import(r.Context(), v.Actor, v.ID, revision, b)
	}
	if err != nil {
		vibeError(w, err)
		return
	}
	h.get(w, r)
}
func (h *VibeHandler) edit(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	var input struct {
		PreviewConsent *bool `json:"preview_consent,omitempty"`
		Evaluation     *struct {
			Examples        []string `json:"examples"`
			SuccessCriteria string   `json:"success_criteria"`
		} `json:"evaluation,omitempty"`
		Revision      int64      `json:"revision"`
		ArtifactID    *uuid.UUID `json:"artifact_id,omitempty"`
		AgentPrompt   *string    `json:"agent_prompt,omitempty"`
		RequirementID *uuid.UUID `json:"requirement_id,omitempty"`
		Status        string     `json:"status,omitempty"`
		Statement     *string    `json:"statement,omitempty"`
	}
	if err = vibeBody(w, r, v.Anonymous, &input); err != nil {
		vibeError(w, err)
		return
	}
	err = h.Service.Store.Edit(r.Context(), v.Actor, v.ID, input.Revision, func(s *vibe.Session) error {
		if input.PreviewConsent != nil {
			s.Document.Journey.PreviewConsent = *input.PreviewConsent
			return nil
		}
		if input.ArtifactID != nil {
			for i := range s.Document.Artifacts {
				a := &s.Document.Artifacts[i]
				if a.ID == *input.ArtifactID {
					if a.IsTestPlan() {
						return &vibe.Fault{Code: "artifact_required", Message: "This is a test plan. Export it or explicitly choose a prompt preview; it is not a runnable agent."}
					}
					if input.Evaluation != nil {
						if e := vibe.ValidatePreviewCriteria(input.Evaluation.SuccessCriteria); e != nil {
							return &vibe.Fault{Code: "invalid_request", Message: e.Error()}
						}
						if !canEditVibeEvaluation(a.Blueprint, vibe.LimitsFor(s.Anonymous)) {
							return &vibe.Fault{Code: "invalid_request", Message: "This imported evaluation has additional coverage. Edit it in the advanced builder; no tests were removed."}
						}
						proposal := vibe.DraftProposal{Title: a.Title, AgentPrompt: a.AgentPrompt, Examples: input.Evaluation.Examples, SuccessCriteria: vibe.PreviewCriteria(input.Evaluation.SuccessCriteria)}
						blueprint, e := h.Service.Compiler.Draft(proposal, vibe.LimitsFor(s.Anonymous))
						if e != nil {
							return &vibe.Fault{Code: "invalid_request", Message: e.Error()}
						}
						copy := *a
						copy.ID = uuid.New()
						copy.ParentID = &a.ID
						copy.Accepted = false
						copy.CreatedAt = time.Now().UTC()
						copy.Blueprint = blueprint
						copy.Proposal = &proposal
						copy.CriteriaRequirementIDs = nil // Edited criteria need a fresh provenance review.
						if _, e = h.Service.Compiler.Compile(blueprint, s.Document.Models.Evaluator, copy.ID, vibe.LimitsFor(s.Anonymous)); e != nil {
							return &vibe.Fault{Code: "invalid_request", Message: e.Error()}
						}
						s.Document.Artifacts = append(s.Document.Artifacts, copy)
						return nil
					}
					if input.AgentPrompt != nil {
						if strings.TrimSpace(*input.AgentPrompt) == "" || len(vibe.PreviewPrompt(*input.AgentPrompt)) > vibe.LimitsFor(s.Anonymous).MessageBytes {
							return &vibe.Fault{Code: "invalid_request", Message: "Agent instructions exceed their size limit."}
						}
						copy := *a
						copy.ID = uuid.New()
						copy.ParentID = &a.ID
						copy.AgentPrompt = vibe.PreviewPrompt(*input.AgentPrompt)
						copy.Accepted = false
						copy.CreatedAt = time.Now().UTC()
						s.Document.Artifacts = append(s.Document.Artifacts, copy)
						return nil
					}
					a.Accepted = true
					s.Document.ActiveArtifactID = &a.ID
					return nil
				}
			}
		}
		if input.RequirementID != nil {
			return vibe.DecideRequirement(s, *input.RequirementID, input.Status, input.Statement)
		}
		return &vibe.Fault{Code: "invalid_request", Message: "Choose an existing proposal or artifact."}
	})
	if err != nil {
		vibeError(w, err)
		return
	}
	h.get(w, r)
}
func (h *VibeHandler) claim(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "sessionID")
	if err != nil {
		vibeError(w, err)
		return
	}
	var body struct{}
	if err = vibeBody(w, r, true, &body); err != nil {
		vibeError(w, err)
		return
	}
	if !strings.HasPrefix(actor, "user:") {
		vibeError(w, &vibe.Fault{Code: "forbidden", Message: "Sign in to keep this conversation."})
		return
	}
	err = h.Service.Store.Claim(r.Context(), h.anonymous(r), actor, id)
	if err != nil {
		vibeError(w, err)
		return
	}
	h.get(w, r)
}
func (h *VibeHandler) save(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	var input struct {
		Revision    int64        `json:"revision"`
		ArtifactID  uuid.UUID    `json:"artifact_id"`
		WorkspaceID uuid.UUID    `json:"workspace_id"`
		Models      *vibe.Models `json:"models,omitempty"`
	}
	if err = vibeBody(w, r, v.Anonymous, &input); err != nil {
		vibeError(w, err)
		return
	}
	id, err := h.Service.Save(r.Context(), v.Actor, v.ID, input.Revision, input.ArtifactID, input.WorkspaceID, input.Models)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, map[string]any{"draft_id": id, "workspace_id": input.WorkspaceID})
}
func (h *VibeHandler) approve(w http.ResponseWriter, r *http.Request) { h.operationAction(w, r, true) }
func (h *VibeHandler) stop(w http.ResponseWriter, r *http.Request)    { h.operationAction(w, r, false) }
func (h *VibeHandler) operationAction(w http.ResponseWriter, r *http.Request, approve bool) {
	actor, err := h.actor(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "operationID")
	if err != nil {
		vibeError(w, err)
		return
	}
	var input struct{}
	if err = vibeBody(w, r, true, &input); err != nil {
		vibeError(w, err)
		return
	}
	if approve {
		if err = h.Service.Gate.Check(r.Context(), actor, vibe.LimitsFor(false)); err == nil {
			err = h.Service.Store.Approve(r.Context(), actor, id, h.Service.Config)
		}
	} else {
		err = h.Service.Store.Stop(r.Context(), actor, id)
	}
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, map[string]bool{"ok": true})
}

// Fetch-stream SSE uses cookie/header auth. A cursor only selects a snapshot;
// neither reconnect nor a stale cursor ever creates or dispatches operations.
func (h *VibeHandler) events(w http.ResponseWriter, r *http.Request) {
	v, err := h.session(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	redis := h.Service.Gate.Redis
	if redis == nil {
		vibeError(w, &vibe.Fault{Code: "accounting_unavailable", Message: "Event streaming is unavailable."})
		return
	}
	key := "vibe:sse:" + vibe.Hash([]byte(v.Actor))
	lease := uuid.NewString()
	now := time.Now().UnixMilli()
	n, err := redis.Eval(r.Context(), `redis.call('ZREMRANGEBYSCORE',KEYS[1],'-inf',ARGV[1]); if redis.call('ZCARD',KEYS[1])>=2 then return 0 end; redis.call('ZADD',KEYS[1],ARGV[2],ARGV[3]); redis.call('PEXPIRE',KEYS[1],70000); return 1`, []string{key}, now, now+60000, lease).Int()
	if err != nil || n != 1 {
		vibeError(w, &vibe.Fault{Code: "capacity_limit", Message: "Too many event connections. Close another tab and reconnect."})
		return
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
		defer cancel()
		redis.ZRem(ctx, key, lease)
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	cursor := int64(-1)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	deadline := time.NewTimer(45 * time.Second)
	defer deadline.Stop()
	for {
		// Re-authorize against the database every refresh, including role changes.
		if err = h.Service.Store.Authorize(r.Context(), v, false); err != nil {
			return
		}
		latest, err := h.Service.Store.Cursor(r.Context(), v.ID)
		if err != nil {
			return
		}
		var e error
		if latest != cursor {
			v, err = h.Service.Store.GetSession(r.Context(), v.Actor, v.ID)
			if err != nil {
				return
			}
			latest = v.EventCursor
			var encoded bytes.Buffer
			encoder := json.NewEncoder(&encoded)
			encoder.SetEscapeHTML(false)
			if err = encoder.Encode(v); err != nil {
				return
			}
			b := bytes.TrimSpace(encoded.Bytes())
			_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, e = fmt.Fprintf(w, "id: %d\nevent: snapshot\ndata: %s\n\n", latest, b); e != nil {
				return
			}
			if e = controller.Flush(); e != nil {
				return
			}
			cursor = latest
		}
		select {
		case <-r.Context().Done():
			return
		case <-deadline.C:
			return
		case <-ticker.C:
		}
	}
}
