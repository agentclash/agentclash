package ratelimit

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// Config holds the rate limiting configuration.
type Config struct {
	DefaultRPS           float64
	DefaultBurst         int
	RunCreationRPM       float64
	RunCreationBurst     int
	RankingInsightsRPM   float64
	RankingInsightsBurst int
}

// limiterKey uniquely identifies a rate limiter by workspace and group.
type limiterKey struct {
	WorkspaceID uuid.UUID
	Group       string
}

// Limiter provides per-workspace, per-group rate limiting backed by
// in-memory token bucket limiters from golang.org/x/time/rate.
type Limiter struct {
	cfg      Config
	limiters sync.Map // map[limiterKey]*rate.Limiter
}

// NewLimiter creates a new Limiter with the given configuration.
func NewLimiter(cfg Config) *Limiter {
	return &Limiter{cfg: cfg}
}

// Allow checks whether a request from the given workspace in the given group
// is allowed. It returns true if the request is permitted, or false with the
// duration the caller should wait before retrying.
func (l *Limiter) Allow(workspaceID uuid.UUID, group string) (allowed bool, retryAfter time.Duration) {
	key := limiterKey{WorkspaceID: workspaceID, Group: group}

	val, loaded := l.limiters.Load(key)
	if !loaded {
		val, _ = l.limiters.LoadOrStore(key, limiterForGroup(l.cfg, group))
	}
	lim := val.(*rate.Limiter)

	r := lim.Reserve()
	if !r.OK() {
		return false, 0
	}

	delay := r.Delay()
	if delay > 0 {
		// The reservation requires waiting, which means the rate is exceeded.
		// Cancel the reservation so the token is returned.
		r.Cancel()
		return false, delay
	}

	return true, 0
}

// Middleware returns chi-compatible HTTP middleware that rate limits requests
// by workspace ID for the given group. The extractWorkspaceID function is
// called to obtain the workspace ID from each request; if it returns false
// (e.g. unauthenticated requests), the request passes through without
// rate limiting.
func (l *Limiter) Middleware(group string, extractWorkspaceID func(*http.Request) (uuid.UUID, bool)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wsID, ok := extractWorkspaceID(r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			allowed, retryAfter := l.Allow(wsID, group)

			// Look up the limiter to write headers.
			key := limiterKey{WorkspaceID: wsID, Group: group}
			val, _ := l.limiters.Load(key)
			lim := val.(*rate.Limiter)

			// Write rate limit headers on every response.
			burstLimit := lim.Burst()
			tokensNow := lim.Tokens()
			remaining := int(math.Max(0, math.Floor(tokensNow)))

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(burstLimit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Second).Unix(), 10))

			if !allowed {
				retryAfterSec := int(math.Ceil(retryAfter.Seconds()))
				if retryAfterSec < 1 {
					retryAfterSec = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retryAfterSec))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprintf(w, `{"error":{"code":"rate_limited","message":"too many requests, retry after %d seconds"}}`, retryAfterSec)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// limiterForGroup creates a *rate.Limiter configured for the given group.
func limiterForGroup(cfg Config, group string) *rate.Limiter {
	if group == "run_creation" {
		rps := cfg.RunCreationRPM / 60.0
		return rate.NewLimiter(rate.Limit(rps), cfg.RunCreationBurst)
	}
	if group == "run_ranking_insights" || strings.HasPrefix(group, "run_ranking_insights:") {
		rps := cfg.RankingInsightsRPM / 60.0
		return rate.NewLimiter(rate.Limit(rps), cfg.RankingInsightsBurst)
	}
	return rate.NewLimiter(rate.Limit(cfg.DefaultRPS), cfg.DefaultBurst)
}
