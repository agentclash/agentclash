package vibe

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/vibe/interaction"
	"github.com/google/uuid"
)

const guidedAuthoringVersion = 14

func (p Plan) guided() bool { return p.AuthoringVersion == guidedAuthoringVersion || p.interpreted() }

// The router may supply one illustration, never UI code, evidence or actions.
// Keeping it separate from message text prevents it entering source selection.
type GuidanceExample struct {
	Input    string `json:"input"`
	Expected string `json:"expected"`
}

const guidedRoutePrompt = `
Guidance v14: use example=null by default. When the person asks what a test means or a vague goal needs a concrete illustration, provide one small illustrative input/expected pair (at most 600 characters each). The UI labels it Example only; it does not execute anything or adopt its rules. Explain a test as an example task plus what a good result should look like. For "convert PDFs properly", show a tiny PDF-to-Markdown example and ask one useful question about what must be preserved. Do not claim to read files or run a converter.
Precise requests need no lesson: prepare their requested tests immediately. If the user asks for both an explanation and tests, example can accompany prepare_tests; keep memory.guidance_kind empty for mutations and code records the displayed illustration. Never make an illustration into policy, suggested rules or a test unless explicitly requested. Respect dismissed help and brevity; don't repeat an example already shown unless asked.
A business owner without an agent needs one concrete job, not technical setup questions. Offer useful tests to keep or hand to their team. For an existing agent explain the supported choices only when relevant: test pasted instructions with a model here, or assess supplied replies. Tools/data and live apps are not connected. Existing pack owners keep their original tests and grading; skip introductory explanations. Never infer expertise or change the job from ownership alone.`

func validateGuidanceExample(p Plan, example *GuidanceExample) error {
	if example == nil {
		return nil
	}
	if !p.guided() {
		return fmt.Errorf("illustrations require the current conversation contract")
	}
	for _, text := range []string{example.Input, example.Expected} {
		if strings.TrimSpace(text) == "" || len([]rune(text)) > 600 {
			return fmt.Errorf("keep each illustration field between 1 and 600 characters")
		}
	}
	return nil
}

func exampleCard(example GuidanceExample, scope string, messageID uuid.UUID) json.RawMessage {
	return raw(map[string]any{"id": deterministicID(messageID, "example").String(), "scope_id": scope,
		"origin_message_id": messageID.String(), "kind": "example", "input": example.Input,
		"expected": example.Expected, "illustrative": true})
}

// Only code-made illustrations may be attached by an authoring completion.
// Execution, evidence and choices use their own stored operation/state data.
func validateMessageCards(p Plan, cards []json.RawMessage, state *ConversationState, messageID uuid.UUID) error {
	if len(cards) == 0 {
		return nil
	}
	if !p.guided() || state == nil || len(cards) > 1 {
		return fault("invalid_completion", "The response cards do not match this conversation.")
	}
	for _, card := range cards {
		envelope, err := interaction.Decode(raw(map[string]any{"version": 1, "kind": "card", "payload": card}))
		if err != nil || envelope.Kind != "card" {
			return fault("invalid_completion", "The response card is invalid.")
		}
		var fields struct {
			ID              string `json:"id"`
			ScopeID         string `json:"scope_id"`
			OriginMessageID string `json:"origin_message_id"`
			Kind            string `json:"kind"`
		}
		if json.Unmarshal(card, &fields) != nil || fields.Kind != "example" || fields.ScopeID != state.Brief.ScopeID || fields.OriginMessageID != messageID.String() || fields.ID != deterministicID(messageID, "example").String() {
			return fault("invalid_completion", "The response card references another message or scope.")
		}
	}
	return nil
}
