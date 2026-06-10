package api

import (
	"net/http/httptest"
	"testing"
)

func TestParseListLimitOffset(t *testing.T) {
	cases := []struct {
		url        string
		wantLimit  int32
		wantOffset int32
	}{
		{"/x", 20, 0},
		{"/x?limit=5&offset=10", 5, 10},
		{"/x?limit=100", 100, 0},
		{"/x?limit=101", 100, 0},                // capped
		{"/x?limit=0", 20, 0},                   // non-positive → default
		{"/x?limit=-3", 20, 0},                  // negative → default
		{"/x?limit=abc", 20, 0},                 // garbage → default
		{"/x?offset=-1", 20, 0},                 // negative → default
		{"/x?offset=banana", 20, 0},             // garbage → default
		{"/x?limit=2147483648", 20, 0},          // int32 wrap → rejected, default
		{"/x?offset=2147483648", 20, 0},         // int32 wrap → default
		{"/x?offset=0", 20, 0},                  // explicit 0 accepted
		{"/x?limit=9223372036854775807", 20, 0}, // max int64 → rejected, default
	}
	for _, tc := range cases {
		r := httptest.NewRequest("GET", tc.url, nil)
		limit, offset := parseListLimitOffset(r)
		if limit != tc.wantLimit || offset != tc.wantOffset {
			t.Errorf("parseListLimitOffset(%q) = (%d, %d), want (%d, %d)", tc.url, limit, offset, tc.wantLimit, tc.wantOffset)
		}
	}
}
