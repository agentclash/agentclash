package challengepack

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/scoring"
)

func TestValidateUserSimulator_AcceptsHybridScriptedPhase(t *testing.T) {
	errs := validateUserSimulatorSpec("user_simulator", validUserSimulatorSpec(), CaseDefinition{
		Payload: map[string]any{"order_id": "123"},
	}, nil)
	if len(errs) > 0 {
		t.Fatalf("validateUserSimulatorSpec returned errors: %v", errs)
	}
}

func TestValidateUserSimulator_RejectsUnknownActor(t *testing.T) {
	spec := validUserSimulatorSpec()
	spec.Phases[0].Actor = "bot"
	errs := validateUserSimulatorSpec("user_simulator", spec, CaseDefinition{}, nil)
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
	if !strings.Contains(errs.Error(), "actor") {
		t.Fatalf("error = %v, want actor validation", errs)
	}
}

func TestValidateUserSimulator_RejectsUnknownTrigger(t *testing.T) {
	spec := validUserSimulatorSpec()
	spec.Phases = append(spec.Phases, UserSimulatorPhase{
		ID:      "pushback",
		Actor:   UserSimulatorActorScripted,
		Trigger: "on_magic_event",
		Turns:   []UserSimulatorTurn{{Message: "Try again"}},
	})
	errs := validateUserSimulatorSpec("user_simulator", spec, CaseDefinition{}, nil)
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
	if !strings.Contains(errs.Error(), "trigger") {
		t.Fatalf("error = %v, want trigger validation", errs)
	}
}

func TestValidateUserSimulator_ScriptedPhaseRequiresTurnMessages(t *testing.T) {
	spec := validUserSimulatorSpec()
	spec.Phases[0].Turns = []UserSimulatorTurn{{Message: "  "}}
	errs := validateUserSimulatorSpec("user_simulator", spec, CaseDefinition{}, nil)
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
	if !strings.Contains(errs.Error(), "message") {
		t.Fatalf("error = %v, want message validation", errs)
	}
}

func TestValidateUserSimulator_LLMPhaseRequiresPersona(t *testing.T) {
	spec := validUserSimulatorSpec()
	spec.Phases = append(spec.Phases, UserSimulatorPhase{
		ID:      "dynamic",
		Actor:   UserSimulatorActorLLM,
		Trigger: UserSimulatorTriggerOnAssistantMismatch,
	})
	errs := validateUserSimulatorSpec("user_simulator", spec, CaseDefinition{}, nil)
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
	if !strings.Contains(errs.Error(), "persona") {
		t.Fatalf("error = %v, want persona validation", errs)
	}
}

func TestValidateBundle_MultiTurnRequiresUserSimulatorOnCases(t *testing.T) {
	bundle := multiTurnTestBundle(nil)
	err := ValidateBundle(bundle)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "user_simulator") {
		t.Fatalf("error = %v, want user_simulator required", err)
	}
}

func TestValidateBundle_NativeRejectsUserSimulator(t *testing.T) {
	simulator := validUserSimulatorSpec()
	bundle := multiTurnTestBundle(&simulator)
	bundle.Version.ExecutionMode = ExecutionModeNative
	err := ValidateBundle(bundle)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "only allowed when version.execution_mode is multi_turn") {
		t.Fatalf("error = %v, want multi_turn-only user_simulator error", err)
	}
}

func TestValidateBundle_MultiTurnAcceptsValidUserSimulator(t *testing.T) {
	simulator := validUserSimulatorSpec()
	bundle := multiTurnTestBundle(&simulator)
	bundle.Version.EvaluationSpec.PostExecutionChecks = []scoring.PostExecutionCheck{
		{Key: "generated_code", Type: scoring.PostExecutionCheckTypeFileCapture, Path: "/workspace/app.py"},
	}
	if err := ValidateBundle(bundle); err != nil {
		t.Fatalf("ValidateBundle returned error: %v", err)
	}
}

func TestValidateBundleUserSimulatorTemplates_RejectsUnresolvedPlaceholder(t *testing.T) {
	spec := validUserSimulatorSpec()
	spec.Phases[0].Turns[0].Message = "Refund order {{missing_key}}"
	errs := validateUserSimulatorTemplates("user_simulator", spec, CaseDefinition{
		Payload: map[string]any{"order_id": "123"},
	})
	if len(errs) == 0 {
		t.Fatal("expected template validation errors")
	}
	if !strings.Contains(errs.Error(), "missing_key") {
		t.Fatalf("error = %v, want missing_key placeholder error", errs)
	}
}

func validUserSimulatorSpec() UserSimulatorSpec {
	return UserSimulatorSpec{
		SchemaVersion: UserSimulatorSchemaVersionV1,
		Kind:          UserSimulatorKindHybrid,
		MaxTurns:      20,
		Phases: []UserSimulatorPhase{
			{
				ID:    "open",
				Actor: UserSimulatorActorScripted,
				Turns: []UserSimulatorTurn{{Message: "Refund order {{order_id}}"}},
			},
		},
	}
}

func multiTurnTestBundle(simulator *UserSimulatorSpec) Bundle {
	caseDef := CaseDefinition{
		ChallengeKey: "c1",
		CaseKey:      "case-1",
		Payload:      map[string]any{"order_id": "123"},
	}
	if simulator != nil {
		caseDef.UserSimulator = simulator
	}
	return Bundle{
		Pack: PackMetadata{Slug: "demo", Name: "Demo", Family: "demo"},
		Version: VersionMetadata{
			Number:        1,
			ExecutionMode: ExecutionModeMultiTurn,
			EvaluationSpec: scoring.EvaluationSpec{
				Name:          "demo",
				VersionNumber: 1,
				JudgeMode:     scoring.JudgeModeDeterministic,
				Validators: []scoring.ValidatorDeclaration{
					{
						Key:    "tests",
						Type:   scoring.ValidatorTypeCodeExecution,
						Target: "file:generated_code",
						Config: json.RawMessage(`{"test_command":"pytest -q","scoring":"all_or_nothing"}`),
					},
				},
				Scorecard: scoring.ScorecardDeclaration{
					Dimensions: []scoring.DimensionDeclaration{{Key: scoring.ScorecardDimensionCorrectness}},
				},
			},
		},
		Challenges: []ChallengeDefinition{
			{Key: "c1", Title: "C1", Category: "demo", Difficulty: "easy"},
		},
		InputSets: []InputSetDefinition{
			{Key: "default", Name: "Default", Cases: []CaseDefinition{caseDef}},
		},
	}
}
