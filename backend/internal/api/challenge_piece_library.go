package api

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/challengepack"
)

// challengePieceLibraryHandler serves the in-code starter piece catalog (the
// challenge-pack analogue of GET /tool-library). It is a global, read-only
// catalog — not workspace-scoped — so it needs no workspace authorization
// beyond being an authenticated request.
func challengePieceLibraryHandler(_ *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := strings.TrimSpace(r.URL.Query().Get("kind"))
		all := challengepack.StarterPieceLibrary()
		items := make([]challengepack.StarterPiece, 0, len(all))
		for _, piece := range all {
			if kind == "" || piece.Kind == kind {
				items = append(items, piece)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}
