package traces

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/google/uuid"
)

func CandidatesFromRunEvents(runID, runAgentID uuid.UUID, events []runevents.Envelope) ([]Candidate, error) {
	if len(events) == 0 {
		return nil, fmt.Errorf("run agent has no events")
	}
	turns, err := runevents.TranscriptFromEvents(events)
	if err != nil {
		return nil, err
	}
	candidates := make([]Candidate, 0, len(turns))
	for _, turn := range turns {
		if strings.TrimSpace(turn.UserMessage) == "" && strings.TrimSpace(turn.AssistantMessage) == "" {
			continue
		}
		input, err := json.Marshal(map[string]string{"message": turn.UserMessage})
		if err != nil {
			return nil, err
		}
		output, err := json.Marshal(map[string]string{"message": turn.AssistantMessage})
		if err != nil {
			return nil, err
		}
		traceID := fmt.Sprintf("%s:turn:%d", runAgentID.String(), turn.TurnIndex)
		metadata := candidateMetadata(map[string]any{
			"turn_index": turn.TurnIndex,
			"phase_id":   turn.PhaseID,
			"actor":      turn.Actor,
			"mismatch":   turn.Mismatch,
		})
		candidates = append(candidates, Candidate{
			SourceTraceID:    traceID,
			SourceRunID:      &runID,
			SourceRunAgentID: &runAgentID,
			ExternalID:       optionalString(traceID),
			Input:            input,
			Output:           output,
			Expected:         output,
			Metadata:         metadata,
		})
	}
	if len(candidates) > 0 {
		return candidates, nil
	}
	return candidatesFromModelCalls(runID, runAgentID, events)
}

func candidatesFromModelCalls(runID, runAgentID uuid.UUID, events []runevents.Envelope) ([]Candidate, error) {
	candidates := make([]Candidate, 0)
	for idx, event := range events {
		if event.EventType != runevents.EventTypeModelCallCompleted {
			continue
		}
		var payload struct {
			InputMessages json.RawMessage `json:"input_messages"`
			OutputText    string          `json:"output_text"`
			ProviderKey   string          `json:"provider_key"`
			ProviderModel string          `json:"provider_model_id"`
			FinishReason  string          `json:"finish_reason"`
			InputTokens   int64           `json:"input_tokens"`
			OutputTokens  int64           `json:"output_tokens"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			continue
		}
		if len(payload.InputMessages) == 0 && strings.TrimSpace(payload.OutputText) == "" {
			continue
		}
		input := payload.InputMessages
		if len(input) == 0 {
			input = json.RawMessage(`[]`)
		}
		output, err := json.Marshal(map[string]string{"text": payload.OutputText})
		if err != nil {
			return nil, err
		}
		traceID := fmt.Sprintf("%s:model:%d", runAgentID.String(), idx+1)
		metadata := candidateMetadata(map[string]any{
			"provider_key":      payload.ProviderKey,
			"provider_model_id": payload.ProviderModel,
			"finish_reason":     payload.FinishReason,
			"input_tokens":      payload.InputTokens,
			"output_tokens":     payload.OutputTokens,
			"sequence_number":   event.SequenceNumber,
		})
		candidates = append(candidates, Candidate{
			SourceTraceID:    traceID,
			SourceRunID:      &runID,
			SourceRunAgentID: &runAgentID,
			ExternalID:       optionalString(traceID),
			Input:            input,
			Output:           output,
			Expected:         output,
			Metadata:         metadata,
		})
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no promotable run events found")
	}
	return candidates, nil
}
