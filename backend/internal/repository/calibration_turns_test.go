package repository

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestHasCompletedCalibrationTurn(t *testing.T) {
	const (
		missingKey = "run-events/missing.json"
		invalidKey = "run-events/invalid.json"
		matchKey   = "run-events/match.json"
		otherKey   = "run-events/other.json"
		inline     = `{"turn_index":0}`
	)
	offloaded := func(key string) string {
		t.Helper()
		ref, err := runevents.MarshalPayloadRef(key, 10, "turn.completed")
		if err != nil {
			t.Fatal(err)
		}
		return string(ref)
	}
	missing, invalid := offloaded(missingKey), offloaded(invalidKey)
	match, other := offloaded(matchKey), offloaded(otherKey)
	errMissing := errors.New("object unavailable")
	tests := []struct {
		name       string
		payloads   []string
		wantOpened []string
		wantMatch  bool
		wantErr    error
	}{
		{"inline match before hydration", []string{missing, inline}, nil, true, nil},
		{"offloaded match after errors", []string{missing, invalid, match}, []string{missingKey, invalidKey, matchKey}, true, nil},
		{"hydration error without match", []string{missing, other}, []string{missingKey, otherKey}, false, errMissing},
		{"decode error without match", []string{invalid, other}, []string{invalidKey, otherKey}, false, &json.UnmarshalTypeError{}},
		{"no match", []string{other}, []string{otherKey}, false, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var opened []string
			resolver := runevents.NewResolver(runevents.OpenFunc(func(_ context.Context, key string) (io.ReadCloser, error) {
				opened = append(opened, key)
				switch key {
				case missingKey:
					return nil, errMissing
				case invalidKey:
					return io.NopCloser(strings.NewReader(`{"turn_index":"invalid"}`)), nil
				case matchKey:
					return io.NopCloser(strings.NewReader(inline)), nil
				case otherKey:
					return io.NopCloser(strings.NewReader(`{"turn_index":1}`)), nil
				default:
					t.Fatalf("unexpected object key %q", key)
					return nil, nil
				}
			}), 1)
			repo := (&Repository{
				queries: repositorysqlc.New(calibrationTurnDB{payloads: tc.payloads}),
			}).WithPayloadResolver(resolver)

			match, err := repo.HasCompletedCalibrationTurn(t.Context(), uuid.New(), 0)
			if match != tc.wantMatch {
				t.Errorf("match = %v, want %v (error = %v)", match, tc.wantMatch, err)
			}
			if _, ok := tc.wantErr.(*json.UnmarshalTypeError); ok {
				var decodeErr *json.UnmarshalTypeError
				if !errors.As(err, &decodeErr) {
					t.Errorf("error = %v, want a JSON type error", err)
				}
			} else if !errors.Is(err, tc.wantErr) {
				t.Errorf("error = %v, want %v", err, tc.wantErr)
			}
			if !slices.Equal(opened, tc.wantOpened) {
				t.Errorf("opened %v, want %v", opened, tc.wantOpened)
			}
		})
	}
}

type calibrationTurnDB struct {
	repositorysqlc.DBTX
	payloads []string
}

func (db calibrationTurnDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &calibrationTurnRows{payloads: db.payloads}, nil
}

type calibrationTurnRows struct {
	pgx.Rows
	payloads []string
}

func (r *calibrationTurnRows) Next() bool { return len(r.payloads) > 0 }
func (r *calibrationTurnRows) Close()     {}
func (r *calibrationTurnRows) Err() error { return nil }

func (r *calibrationTurnRows) Scan(dest ...any) error {
	*dest[0].(*[]byte) = []byte(r.payloads[0])
	r.payloads = r.payloads[1:]
	return nil
}
