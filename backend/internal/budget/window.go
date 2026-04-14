package budget

import "time"

// ComputeWindow returns the start and end of the budget window containing
// referenceTime for the given windowKind. All times are in UTC.
//
// Supported window kinds:
//   - "day":   midnight-to-midnight UTC
//   - "week":  Monday midnight UTC to the following Monday
//   - "month": first of the month to first of the next month (UTC)
//   - "run":   per-run window; returns zero times (no accumulation)
func ComputeWindow(windowKind string, referenceTime time.Time) (start time.Time, end time.Time) {
	ref := referenceTime.UTC()

	switch windowKind {
	case "day":
		start = time.Date(ref.Year(), ref.Month(), ref.Day(), 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 0, 1)

	case "week":
		// Weekday: Sunday=0 ... Saturday=6. We want Monday=0-based offset.
		weekday := ref.Weekday()
		daysSinceMonday := int(weekday+6) % 7 // Monday=0, Tuesday=1, ..., Sunday=6
		start = time.Date(ref.Year(), ref.Month(), ref.Day()-daysSinceMonday, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 0, 7)

	case "month":
		start = time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, 0)

	case "run":
		// Per-run windows have no accumulation period.
		start = time.Time{}
		end = time.Time{}
	}

	return start, end
}
