package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			recorder := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(recorder, r)

			logger.Info("http request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.Status(),
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
		})
	}
}

func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("panic recovered from http handler",
						"method", r.Method,
						"path", r.URL.Path,
						"panic", fmt.Sprint(recovered),
						"stack", string(debug.Stack()),
					)
					writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
