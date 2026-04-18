package repository

import (
	"math"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestNumericPtrHandlesFractionalValues(t *testing.T) {
	value := pgtype.Numeric{}
	if err := value.Scan("0.125"); err != nil {
		t.Fatalf("scan numeric: %v", err)
	}

	got := numericPtr(value)
	if got == nil {
		t.Fatal("numericPtr returned nil for a valid fractional numeric")
	}
	if math.Abs(*got-0.125) > 1e-12 {
		t.Fatalf("numericPtr() = %v, want 0.125", *got)
	}
}
