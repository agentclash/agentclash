package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type tryoutProofContextKey struct{}
type tryoutDevelopmentContextKey struct{}

func tryoutCookieName(ctx context.Context, id uuid.UUID) string {
	prefix := "__Secure-agentclash_tryout_"
	if development, _ := ctx.Value(tryoutDevelopmentContextKey{}).(bool); development {
		prefix = "agentclash_tryout_"
	}
	return prefix + id.String()
}

func newTryoutProof() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:]), nil
}

func hashTryoutProof(raw string) string {
	if len(raw) != 43 {
		return ""
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) != 32 {
		return ""
	}
	digest := sha256.Sum256(decoded)
	return hex.EncodeToString(digest[:])
}

func tryoutProofFromContext(ctx context.Context) string {
	proof, _ := ctx.Value(tryoutProofContextKey{}).(string)
	return proof
}

func tryoutRequestContext(r *http.Request, id uuid.UUID) context.Context {
	proof := ""
	if cookie, err := r.Cookie(tryoutCookieName(r.Context(), id)); err == nil {
		proof = hashTryoutProof(cookie.Value)
	}
	return context.WithValue(r.Context(), tryoutProofContextKey{}, proof)
}

func setTryoutCookie(w http.ResponseWriter, r *http.Request, tryout repository.AgentTryout, proof string) {
	if tryout.ExpiresAt == nil {
		return
	}
	development, _ := r.Context().Value(tryoutDevelopmentContextKey{}).(bool)
	http.SetCookie(w, &http.Cookie{Name: tryoutCookieName(r.Context(), tryout.ID), Value: proof,
		Path: "/v1/agent-tryouts/" + tryout.ID.String(), HttpOnly: true, Secure: !development, SameSite: http.SameSiteLaxMode,
		Expires: *tryout.ExpiresAt, MaxAge: max(1, int(time.Until(*tryout.ExpiresAt).Seconds()))})
}

func ownsAnonymousTryout(ctx context.Context, tryout repository.AgentTryout) bool {
	proof := tryoutProofFromContext(ctx)
	return len(proof) == 64 && tryout.AnonymousOwnerProofHash != nil && subtle.ConstantTimeCompare([]byte(proof), []byte(*tryout.AnonymousOwnerProofHash)) == 1
}

func (m *AgentTryoutManager) authorizeAnonymousTryout(ctx context.Context, tryout repository.AgentTryout, write bool) error {
	if tryout.WorkspaceID != nil || tryout.OrganizationID != nil || tryout.ClaimedByUserID != nil {
		return repository.ErrAgentTryoutNotFound
	}
	if (write || tryout.AnonymousOwnerProofHash != nil) && !ownsAnonymousTryout(ctx, tryout) {
		return repository.ErrAgentTryoutNotFound
	}
	if tryout.ExpiresAt != nil && !tryout.ExpiresAt.After(m.now()) {
		return repository.ErrAgentTryoutExpired
	}
	if tryout.AnonymousOwnerProofHash != nil && tryout.ExpiresAt == nil {
		return repository.ErrAgentTryoutNotFound
	}
	return nil
}

func getSavedAgentTryoutHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "tryoutID"))
		if err != nil {
			writeError(w, 400, "invalid_tryout_id", "tryout_id must be a UUID")
			return
		}
		tryout, err := service.GetWorkspaceTryout(r.Context(), caller, id)
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, 200, mapAgentTryoutResponse(tryout))
	}
}
