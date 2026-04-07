package repository

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/challengepack"
	"github.com/google/uuid"
)

func TestBuildScoringEvidenceInputsReturnsMarshalError(t *testing.T) {
	inputSet := &ChallengeInputSetExecutionContext{
		Cases: []ChallengeCaseExecutionContext{
			{
				ChallengeIdentityID: uuid.New(),
				ChallengeKey:        "ticket-1",
				CaseKey:             "case-1",
				ItemKey:             "case-1",
				Inputs: []challengepack.CaseInput{
					{
						Key:   "broken",
						Kind:  "json",
						Value: map[string]any{"ch": make(chan int)},
					},
				},
			},
		},
	}

	_, err := BuildScoringEvidenceInputs(json.RawMessage(`{"version":{"assets":[]}}`), inputSet)
	if err == nil {
		t.Fatal("BuildScoringEvidenceInputs returned nil error")
	}
	if !strings.Contains(err.Error(), `marshal case input "broken"`) {
		t.Fatalf("error = %q, want marshal case input context", err.Error())
	}
}
