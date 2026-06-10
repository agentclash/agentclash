package api

import (
	"math"
	"net/http"
	"strconv"
)

// parseListLimitOffset reads the standard limit/offset query parameters for
// list endpoints: limit defaults to 20 and is capped at 100, offset defaults
// to 0. Invalid or negative values fall back to the defaults — the same
// behavior the runs list has always had — so existing callers can never be
// broken by garbage input.
func parseListLimitOffset(r *http.Request) (limit, offset int32) {
	limit = 20
	// Bound before the int32 cast: 2^31 would wrap negative, bypass the cap,
	// and hand PostgreSQL a negative LIMIT (a 500).
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= math.MaxInt32 {
			limit = int32(parsed)
		}
	}
	if limit > 100 {
		limit = 100
	}

	offset = 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 && parsed <= math.MaxInt32 {
			offset = int32(parsed)
		}
	}
	return limit, offset
}
