package runevents

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// TranscriptTurn is one user↔assistant turn reconstructed from run events.
type TranscriptTurn struct {
	TurnIndex         int
	PhaseID           string
	Actor             string
	UserMessage       string
	AssistantMessage  string
	Mismatch          bool
	Completed         bool
	AwaitingHuman     bool
	AwaitingHumanHint string
	UserSimulated     bool
	StateSnapshotRef  string
	FirstSequence     int64
	LastSequence      int64
}

// TranscriptFromEvents builds ordered transcript turns from persisted run events.
// Events are sorted by sequence_number; turn_index groups user and assistant messages.
func TranscriptFromEvents(events []Envelope) ([]TranscriptTurn, error) {
	if len(events) == 0 {
		return nil, nil
	}

	sorted := append([]Envelope(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left := sorted[i].SequenceNumber
		right := sorted[j].SequenceNumber
		if left == 0 && right == 0 {
			return sorted[i].OccurredAt.Before(sorted[j].OccurredAt)
		}
		if left == 0 {
			return true
		}
		if right == 0 {
			return false
		}
		return left < right
	})

	byIndex := make(map[int]*TranscriptTurn)
	order := make([]int, 0)

	ensureTurn := func(turnIndex int) *TranscriptTurn {
		if turn, ok := byIndex[turnIndex]; ok {
			return turn
		}
		turn := &TranscriptTurn{TurnIndex: turnIndex}
		byIndex[turnIndex] = turn
		order = append(order, turnIndex)
		return turn
	}

	for _, event := range sorted {
		seq := event.SequenceNumber
		switch event.EventType {
		case EventTypeTurnUserMessage:
			if event.Summary.TurnIndex == nil {
				return nil, fmt.Errorf("turn.user.message missing turn_index at sequence %d", seq)
			}
			turn := ensureTurn(*event.Summary.TurnIndex)
			content, err := decodeTurnUserMessageContent(event.Payload)
			if err != nil {
				return nil, fmt.Errorf("sequence %d: %w", seq, err)
			}
			turn.UserMessage = content
			turn.PhaseID = firstNonEmpty(event.Summary.PhaseID, turn.PhaseID)
			turn.Actor = firstNonEmpty(event.Summary.Actor, turn.Actor)
			turn.trackSequence(seq)

		case EventTypeTurnUserSimulated:
			if event.Summary.TurnIndex == nil {
				return nil, fmt.Errorf("turn.user.simulated missing turn_index at sequence %d", seq)
			}
			turn := ensureTurn(*event.Summary.TurnIndex)
			turn.UserSimulated = true
			turn.PhaseID = firstNonEmpty(event.Summary.PhaseID, turn.PhaseID)
			turn.trackSequence(seq)

		case EventTypeTurnAssistantMessage:
			if event.Summary.TurnIndex == nil {
				return nil, fmt.Errorf("turn.assistant.message missing turn_index at sequence %d", seq)
			}
			turn := ensureTurn(*event.Summary.TurnIndex)
			content, err := decodeTurnAssistantMessageContent(event.Payload)
			if err != nil {
				return nil, fmt.Errorf("sequence %d: %w", seq, err)
			}
			turn.AssistantMessage = content
			turn.PhaseID = firstNonEmpty(event.Summary.PhaseID, turn.PhaseID)
			turn.trackSequence(seq)

		case EventTypeTurnCompleted:
			if event.Summary.TurnIndex == nil {
				continue
			}
			turn := ensureTurn(*event.Summary.TurnIndex)
			turn.Completed = true
			turn.PhaseID = firstNonEmpty(event.Summary.PhaseID, turn.PhaseID)
			turn.Actor = firstNonEmpty(event.Summary.Actor, turn.Actor)
			if event.Summary.Mismatch != nil {
				turn.Mismatch = *event.Summary.Mismatch
			}
			turn.trackSequence(seq)

		case EventTypeTurnAwaitingHuman:
			if event.Summary.TurnIndex == nil {
				return nil, fmt.Errorf("turn.awaiting_human missing turn_index at sequence %d", seq)
			}
			turn := ensureTurn(*event.Summary.TurnIndex)
			turn.AwaitingHuman = true
			turn.PhaseID = firstNonEmpty(event.Summary.PhaseID, turn.PhaseID)
			var payload turnAwaitingHumanPayload
			if len(event.Payload) > 0 {
				if err := json.Unmarshal(event.Payload, &payload); err != nil {
					return nil, fmt.Errorf("sequence %d: decode turn.awaiting_human: %w", seq, err)
				}
			}
			turn.AwaitingHumanHint = payload.PromptHint
			turn.trackSequence(seq)

		case EventTypeTurnStateCaptured:
			if event.Summary.TurnIndex == nil {
				continue
			}
			turn := ensureTurn(*event.Summary.TurnIndex)
			var payload turnStateCapturedPayload
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				return nil, fmt.Errorf("sequence %d: decode turn.state.captured: %w", seq, err)
			}
			turn.StateSnapshotRef = payload.SnapshotRef
			turn.trackSequence(seq)

		case EventTypeConversationCompleted:
			// Terminal marker; transcript turns already captured.
		}
	}

	sort.Ints(order)
	out := make([]TranscriptTurn, 0, len(order))
	for _, idx := range order {
		out = append(out, *byIndex[idx])
	}
	return out, nil
}

func (t *TranscriptTurn) trackSequence(seq int64) {
	if seq <= 0 {
		return
	}
	if t.FirstSequence == 0 || seq < t.FirstSequence {
		t.FirstSequence = seq
	}
	if seq > t.LastSequence {
		t.LastSequence = seq
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
