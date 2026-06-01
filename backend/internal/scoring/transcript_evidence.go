package scoring

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type transcriptTurnEvidence struct {
	TurnIndex        int
	PhaseID          string
	Actor            string
	UserMessage      string
	AssistantMessage string
	Mismatch         bool
}

func buildTranscriptFromEvents(events []Event) []transcriptTurnEvidence {
	if len(events) == 0 {
		return nil
	}
	sorted := append([]Event(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].SequenceNumber > 0 && sorted[j].SequenceNumber > 0 {
			return sorted[i].SequenceNumber < sorted[j].SequenceNumber
		}
		return sorted[i].OccurredAt.Before(sorted[j].OccurredAt)
	})

	byIndex := map[int]*transcriptTurnEvidence{}
	order := []int{}
	ensure := func(turnIndex int) *transcriptTurnEvidence {
		if turn, ok := byIndex[turnIndex]; ok {
			return turn
		}
		turn := &transcriptTurnEvidence{TurnIndex: turnIndex}
		byIndex[turnIndex] = turn
		order = append(order, turnIndex)
		return turn
	}

	for _, event := range sorted {
		payload := decodePayload(event.Payload)
		turnIndex, ok := intValue(payload, "turn_index")
		if !ok {
			continue
		}
		turn := ensure(turnIndex)
		if phaseID, ok := stringValue(payload, "phase_id"); ok {
			turn.PhaseID = phaseID
		}
		switch event.Type {
		case "turn.user.message":
			if content, ok := stringValue(payload, "content"); ok {
				turn.UserMessage = content
			}
			if actor, ok := stringValue(payload, "actor"); ok {
				turn.Actor = actor
			}
		case "turn.assistant.message":
			if content, ok := stringValue(payload, "content"); ok {
				turn.AssistantMessage = content
			}
		case "turn.completed":
			if mismatch, ok := payload["mismatch"].(bool); ok {
				turn.Mismatch = mismatch
			}
			if actor, ok := stringValue(payload, "actor"); ok && turn.Actor == "" {
				turn.Actor = actor
			}
		}
	}

	sort.Ints(order)
	out := make([]transcriptTurnEvidence, 0, len(order))
	for _, idx := range order {
		out = append(out, *byIndex[idx])
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func resolveTranscriptEvidence(source string, turns []transcriptTurnEvidence) (*string, string, error) {
	switch source {
	case "transcript.full":
		return stringifyTranscript(turns)
	case "turn.expectations":
		return stringifyTurnExpectations(turns)
	default:
		if strings.HasPrefix(source, "transcript.last_n:") {
			nRaw := strings.TrimPrefix(source, "transcript.last_n:")
			n, err := strconv.Atoi(strings.TrimSpace(nRaw))
			if err != nil || n <= 0 {
				return nil, "", fmt.Errorf("unsupported evidence source %q", source)
			}
			if len(turns) == 0 {
				return nil, "transcript evidence is unavailable", nil
			}
			start := len(turns) - n
			if start < 0 {
				start = 0
			}
			return stringifyTranscript(turns[start:])
		}
		if source == "transcript.from_mismatch" {
			firstMismatch := -1
			for i, turn := range turns {
				if turn.Mismatch {
					firstMismatch = i
					break
				}
			}
			if firstMismatch < 0 {
				return nil, "transcript mismatch evidence is unavailable", nil
			}
			return stringifyTranscript(turns[firstMismatch:])
		}
		return nil, "", fmt.Errorf("unsupported evidence source %q", source)
	}
}

func stringifyTranscript(turns []transcriptTurnEvidence) (*string, string, error) {
	if len(turns) == 0 {
		return nil, "transcript evidence is unavailable", nil
	}
	lines := make([]string, 0, len(turns)*2)
	for _, turn := range turns {
		if strings.TrimSpace(turn.UserMessage) != "" {
			lines = append(lines, fmt.Sprintf("user[%d]: %s", turn.TurnIndex, turn.UserMessage))
		}
		if strings.TrimSpace(turn.AssistantMessage) != "" {
			lines = append(lines, fmt.Sprintf("assistant[%d]: %s", turn.TurnIndex, turn.AssistantMessage))
		}
	}
	value := strings.Join(lines, "\n")
	return &value, "", nil
}

func stringifyTurnExpectations(turns []transcriptTurnEvidence) (*string, string, error) {
	if len(turns) == 0 {
		return nil, "turn expectations evidence is unavailable", nil
	}
	type row struct {
		TurnIndex int    `json:"turn_index"`
		PhaseID   string `json:"phase_id,omitempty"`
		Mismatch  bool   `json:"mismatch"`
	}
	rows := make([]row, 0, len(turns))
	for _, turn := range turns {
		rows = append(rows, row{TurnIndex: turn.TurnIndex, PhaseID: turn.PhaseID, Mismatch: turn.Mismatch})
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		return nil, "", err
	}
	value := string(encoded)
	return &value, "", nil
}

func multiTurnRecoveryScore(turns []transcriptTurnEvidence, validators []ValidatorResult) (*float64, string, OutputState) {
	hadMismatch := false
	for _, turn := range turns {
		if turn.Mismatch {
			hadMismatch = true
			break
		}
	}
	if !hadMismatch {
		return floatPtr(1), "", OutputStateAvailable
	}
	outcome, reason, ok := validatorBinaryOutcome(validators)
	if !ok {
		return nil, firstNonEmpty(reason, "validator outcome is unavailable"), OutputStateUnavailable
	}
	if outcome >= 1 {
		return floatPtr(1), "", OutputStateAvailable
	}
	return floatPtr(0), "conversation mismatch without final validator recovery", OutputStateAvailable
}
