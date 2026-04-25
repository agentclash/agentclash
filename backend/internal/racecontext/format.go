package racecontext

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// FormatInput is the pure-function input for the race-context newswire
// formatter. Now is passed explicitly to keep Format deterministic under
// test (no wall-clock reads).
type FormatInput struct {
	Snapshot       map[uuid.UUID]StandingsEntry
	SelfRunAgentID uuid.UUID
	SelfStepIndex  int
	Now            time.Time
}

// Format renders the race-context newswire message. It returns the text
// that will be injected as a role=user message and an estimated token
// count for billable-token accounting. The format is deliberately
// neutral: third-person, factual, no imperatives, no second-person
// pressure language.
//
// Ordering: submitters pinned to top in submission-time order; remaining
// peers ranked by step count descending, ties broken on run_agent_id.
func Format(in FormatInput) (string, int) {
	entries := orderStandings(in.Snapshot)

	running, submitted := 0, 0
	for _, e := range entries {
		switch e.State {
		case StandingsStateSubmitted:
			submitted++
		case StandingsStateRunning, StandingsStateNotStarted, "":
			running++
		}
		// FAILED and TIMED OUT are shown but excluded from both totals.
	}

	var b strings.Builder
	fmt.Fprintf(&b, "[RACE UPDATE · your step %d]\n", in.SelfStepIndex)
	fmt.Fprintf(&b, "%d agents running, %d submitted.\n", running, submitted)

	for _, entry := range entries {
		b.WriteString("- ")
		b.WriteString(formatEntryLine(entry, in.SelfRunAgentID))
		b.WriteString("\n")
	}

	text := strings.TrimRight(b.String(), "\n")
	return text, EstimateTokens(text)
}

func orderStandings(snapshot map[uuid.UUID]StandingsEntry) []StandingsEntry {
	entries := make([]StandingsEntry, 0, len(snapshot))
	for _, e := range snapshot {
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		iSubmitted := entries[i].State == StandingsStateSubmitted
		jSubmitted := entries[j].State == StandingsStateSubmitted
		if iSubmitted != jSubmitted {
			return iSubmitted
		}
		if iSubmitted && jSubmitted {
			switch {
			case entries[i].SubmittedAt == nil && entries[j].SubmittedAt == nil:
				// fall through
			case entries[i].SubmittedAt == nil:
				return false
			case entries[j].SubmittedAt == nil:
				return true
			default:
				if !entries[i].SubmittedAt.Equal(*entries[j].SubmittedAt) {
					return entries[i].SubmittedAt.Before(*entries[j].SubmittedAt)
				}
			}
		}
		if entries[i].Step != entries[j].Step {
			return entries[i].Step > entries[j].Step
		}
		return entries[i].RunAgentID.String() < entries[j].RunAgentID.String()
	})
	return entries
}

func formatEntryLine(entry StandingsEntry, selfRunAgentID uuid.UUID) string {
	modelLabel := entry.Model
	if modelLabel == "" {
		modelLabel = "agent-" + entry.RunAgentID.String()[:8]
	}
	prefix := modelLabel
	if entry.RunAgentID == selfRunAgentID {
		prefix = "you (" + modelLabel + ")"
	}

	switch entry.State {
	case StandingsStateSubmitted:
		elapsed := formatElapsed(entry.StartedAt, entry.SubmittedAt)
		return fmt.Sprintf("%s — submitted at step %d (%s elapsed) · verifying", prefix, entry.Step, elapsed)
	case StandingsStateFailed:
		return fmt.Sprintf("%s — FAILED at step %d", prefix, entry.Step)
	case StandingsStateTimedOut:
		return fmt.Sprintf("%s — TIMED OUT at step %d", prefix, entry.Step)
	case StandingsStateNotStarted, "":
		return fmt.Sprintf("%s — not started", prefix)
	default:
		return fmt.Sprintf("%s — %d steps, %d tool calls, %d tokens", prefix, entry.Step, entry.ToolCalls, entry.TokensUsed)
	}
}

func formatElapsed(start, end *time.Time) string {
	if start == nil || end == nil {
		return "—"
	}
	d := end.Sub(*start)
	if d < 0 {
		d = 0
	}
	minutes := int(d / time.Minute)
	seconds := int((d % time.Minute) / time.Second)
	return fmt.Sprintf("%dm%02ds", minutes, seconds)
}

// EstimateTokens approximates prompt-side token count at ~4 chars/token.
// Not provider-accurate; used only for the token-accounting split so
// race-context bytes don't get counted as billable model spend.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}
