package budget

import (
	"testing"
	"time"
)

func TestComputeWindow_Day(t *testing.T) {
	ref := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)
	start, end := ComputeWindow("day", ref)

	wantStart := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2024, 3, 16, 0, 0, 0, 0, time.UTC)

	if !start.Equal(wantStart) {
		t.Errorf("day start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("day end = %v, want %v", end, wantEnd)
	}
}

func TestComputeWindow_Week(t *testing.T) {
	// 2024-03-13 is a Wednesday.
	ref := time.Date(2024, 3, 13, 10, 0, 0, 0, time.UTC)
	start, end := ComputeWindow("week", ref)

	wantStart := time.Date(2024, 3, 11, 0, 0, 0, 0, time.UTC) // Monday
	wantEnd := time.Date(2024, 3, 18, 0, 0, 0, 0, time.UTC)   // next Monday

	if !start.Equal(wantStart) {
		t.Errorf("week start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("week end = %v, want %v", end, wantEnd)
	}
}

func TestComputeWindow_WeekOnMonday(t *testing.T) {
	// Edge case: reference time is already Monday.
	ref := time.Date(2024, 3, 11, 8, 0, 0, 0, time.UTC)
	start, end := ComputeWindow("week", ref)

	wantStart := time.Date(2024, 3, 11, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2024, 3, 18, 0, 0, 0, 0, time.UTC)

	if !start.Equal(wantStart) {
		t.Errorf("week-on-monday start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("week-on-monday end = %v, want %v", end, wantEnd)
	}
}

func TestComputeWindow_WeekOnSunday(t *testing.T) {
	// Edge case: reference time is Sunday.
	ref := time.Date(2024, 3, 17, 23, 59, 0, 0, time.UTC)
	start, end := ComputeWindow("week", ref)

	wantStart := time.Date(2024, 3, 11, 0, 0, 0, 0, time.UTC) // preceding Monday
	wantEnd := time.Date(2024, 3, 18, 0, 0, 0, 0, time.UTC)

	if !start.Equal(wantStart) {
		t.Errorf("week-on-sunday start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("week-on-sunday end = %v, want %v", end, wantEnd)
	}
}

func TestComputeWindow_Month(t *testing.T) {
	ref := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	start, end := ComputeWindow("month", ref)

	wantStart := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)

	if !start.Equal(wantStart) {
		t.Errorf("month start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("month end = %v, want %v", end, wantEnd)
	}
}

func TestComputeWindow_MonthBoundary(t *testing.T) {
	// January 31 -> start=Jan 1, end=Feb 1.
	ref := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)
	start, end := ComputeWindow("month", ref)

	wantStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	if !start.Equal(wantStart) {
		t.Errorf("month-boundary start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("month-boundary end = %v, want %v", end, wantEnd)
	}
}

func TestComputeWindow_MonthDecemberToJanuary(t *testing.T) {
	// December rolls into January of the next year.
	ref := time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC)
	start, end := ComputeWindow("month", ref)

	wantStart := time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	if !start.Equal(wantStart) {
		t.Errorf("month-dec start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("month-dec end = %v, want %v", end, wantEnd)
	}
}

func TestComputeWindow_Run(t *testing.T) {
	ref := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	start, end := ComputeWindow("run", ref)

	if !start.IsZero() {
		t.Errorf("run start = %v, want zero", start)
	}
	if !end.IsZero() {
		t.Errorf("run end = %v, want zero", end)
	}
}

func TestComputeWindow_DayNonUTC(t *testing.T) {
	// Ensure non-UTC input is normalized to UTC.
	loc := time.FixedZone("UTC+5", 5*60*60)
	// 2024-03-16 02:00 UTC+5 == 2024-03-15 21:00 UTC
	ref := time.Date(2024, 3, 16, 2, 0, 0, 0, loc)
	start, end := ComputeWindow("day", ref)

	wantStart := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2024, 3, 16, 0, 0, 0, 0, time.UTC)

	if !start.Equal(wantStart) {
		t.Errorf("day-nonUTC start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("day-nonUTC end = %v, want %v", end, wantEnd)
	}
}
