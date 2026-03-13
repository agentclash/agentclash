package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"

	"github.com/google/uuid"
)

var (
	ErrUnauthenticated      = errors.New("unauthenticated")
	ErrForbidden            = errors.New("forbidden")
	ErrCallerMissing        = errors.New("caller missing from request context")
	ErrWorkspaceIDRequired  = errors.New("workspace id is required")
	ErrWorkspaceIDMalformed = errors.New("workspace id is malformed")
)

type Caller struct {
	UserID               uuid.UUID
	WorkOSUserID         string
	Email                string
	DisplayName          string
	WorkspaceMemberships map[uuid.UUID]WorkspaceMembership
}

type WorkspaceMembership struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Role        string    `json:"role"`
}

type Authenticator interface {
	Authenticate(r *http.Request) (Caller, error)
}

type WorkspaceAuthorizer interface {
	AuthorizeWorkspace(ctx context.Context, caller Caller, workspaceID uuid.UUID) error
}

type callerContextKey struct{}
type workspaceIDContextKey struct{}

func CallerFromContext(ctx context.Context) (Caller, error) {
	caller, ok := ctx.Value(callerContextKey{}).(Caller)
	if !ok {
		return Caller{}, ErrCallerMissing
	}

	return caller, nil
}

func WorkspaceIDFromContext(ctx context.Context) (uuid.UUID, error) {
	workspaceID, ok := ctx.Value(workspaceIDContextKey{}).(uuid.UUID)
	if !ok {
		return uuid.Nil, ErrWorkspaceIDRequired
	}

	return workspaceID, nil
}

func SortedWorkspaceMemberships(memberships map[uuid.UUID]WorkspaceMembership) []WorkspaceMembership {
	ordered := make([]WorkspaceMembership, 0, len(memberships))
	for _, membership := range memberships {
		ordered = append(ordered, membership)
	}

	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].WorkspaceID.String() < ordered[j].WorkspaceID.String()
	})

	return ordered
}

func authenticateRequest(logger *slog.Logger, authenticator Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			caller, err := authenticator.Authenticate(r)
			if err != nil {
				logger.Warn("request authentication failed",
					"method", r.Method,
					"path", r.URL.Path,
					"error", err,
				)
				writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}

			ctx := context.WithValue(r.Context(), callerContextKey{}, caller)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type WorkspaceIDResolver func(*http.Request) (uuid.UUID, error)

func authorizeWorkspaceAccess(
	logger *slog.Logger,
	authorizer WorkspaceAuthorizer,
	resolveWorkspaceID WorkspaceIDResolver,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			caller, err := CallerFromContext(r.Context())
			if err != nil {
				writeAuthzError(w, err)
				return
			}

			workspaceID, err := resolveWorkspaceID(r)
			if err != nil {
				switch {
				case errors.Is(err, ErrWorkspaceIDRequired), errors.Is(err, ErrWorkspaceIDMalformed):
					writeError(w, http.StatusBadRequest, "invalid_workspace_id", err.Error())
				default:
					logger.Error("workspace authorization resolver failed",
						"method", r.Method,
						"path", r.URL.Path,
						"error", err,
					)
					writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				}
				return
			}

			if err := authorizer.AuthorizeWorkspace(r.Context(), caller, workspaceID); err != nil {
				writeAuthzError(w, err)
				return
			}

			ctx := context.WithValue(r.Context(), workspaceIDContextKey{}, workspaceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeAuthzError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUnauthenticated), errors.Is(err, ErrCallerMissing):
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "workspace access denied")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

type CallerWorkspaceAuthorizer struct{}

func NewCallerWorkspaceAuthorizer() CallerWorkspaceAuthorizer {
	return CallerWorkspaceAuthorizer{}
}

func (CallerWorkspaceAuthorizer) AuthorizeWorkspace(_ context.Context, caller Caller, workspaceID uuid.UUID) error {
	if _, ok := caller.WorkspaceMemberships[workspaceID]; !ok {
		return fmt.Errorf("%w: caller %s does not belong to workspace %s", ErrForbidden, caller.UserID, workspaceID)
	}

	return nil
}
