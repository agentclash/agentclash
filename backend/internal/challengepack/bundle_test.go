package challengepack

import (
	"encoding/json"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
)

func TestParseYAMLValidBundle(t *testing.T) {
	bundle, err := ParseYAML([]byte(`
pack:
  slug: support-eval
  name: Support Eval
  family: support
version:
  number: 1
  tool_policy:
    allowed_tool_kinds: ["file", "shell"]
  evaluation_spec:
    name: support-v1
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: exact
        type: exact_match
        target: final_output
        expected_from: challenge_input
    metrics:
      - key: total_latency_ms
        type: numeric
        collector: run_total_latency_ms
        unit: ms
    runtime_limits:
      max_duration_ms: 60000
    scorecard:
      dimensions: [correctness, latency]
      normalization:
        latency:
          target_ms: 1000
challenges:
  - key: ticket-1
    title: Ticket One
    category: support
    difficulty: medium
    instructions: Solve the ticket
input_sets:
  - key: default
    name: Default Inputs
    items:
      - challenge_key: ticket-1
        item_key: sample-1
        payload:
          content: hello
`))
	if err != nil {
		t.Fatalf("ParseYAML returned error: %v", err)
	}

	if bundle.Pack.Slug != "support-eval" {
		t.Fatalf("slug = %q, want support-eval", bundle.Pack.Slug)
	}
	if bundle.Challenges[0].Definition["instructions"] != "Solve the ticket" {
		t.Fatalf("definition.instructions = %#v, want Solve the ticket", bundle.Challenges[0].Definition["instructions"])
	}

	manifest, err := ManifestJSON(bundle)
	if err != nil {
		t.Fatalf("ManifestJSON returned error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if decoded["schema_version"] != float64(1) {
		t.Fatalf("schema_version = %#v, want 1", decoded["schema_version"])
	}
}

func TestParseYAMLRejectsInvalidBundle(t *testing.T) {
	_, err := ParseYAML([]byte(`
pack:
  slug: ""
  name: Support Eval
  family: support
version:
  number: 1
  evaluation_spec:
    name: support-v1
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: exact
        type: exact_match
        target: final_output
        expected_from: challenge_input
    scorecard:
      dimensions: [correctness]
challenges:
  - key: ticket-1
    title: Ticket One
    category: support
    difficulty: impossible
input_sets:
  - key: default
    name: Default Inputs
    items:
      - challenge_key: ticket-2
        item_key: sample-1
`))
	if err == nil {
		t.Fatal("ParseYAML returned nil error")
	}

	validationErrs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("error type = %T, want ValidationErrors", err)
	}

	expectedFields := []string{
		"pack.slug",
		"challenges[0].difficulty",
		"input_sets[0].items[0].challenge_key",
	}
	for _, field := range expectedFields {
		if !containsField(validationErrs, field) {
			t.Fatalf("expected field %q in validation errors: %v", field, validationErrs)
		}
	}
}

func TestValidateBundleRejectsDuplicateAssetKeys(t *testing.T) {
	err := ValidateBundle(Bundle{
		Pack: PackMetadata{
			Slug:   "support-eval",
			Name:   "Support Eval",
			Family: "support",
		},
		Version: VersionMetadata{
			Number:         1,
			EvaluationSpec: minimalSpec(),
			Assets: []AssetReference{
				{Key: "workspace", Path: "assets/workspace.zip"},
				{Key: "workspace", Path: "assets/workspace-v2.zip"},
			},
		},
		Challenges: []ChallengeDefinition{
			{Key: "ticket-1", Title: "Ticket One", Category: "support", Difficulty: "easy"},
		},
	})
	if err == nil {
		t.Fatal("ValidateBundle returned nil error")
	}

	validationErrs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("error type = %T, want ValidationErrors", err)
	}
	if !containsField(validationErrs, "version.assets[1].key") {
		t.Fatalf("expected duplicate version asset key error, got %v", validationErrs)
	}
}

func minimalSpec() scoring.EvaluationSpec {
	return scoring.EvaluationSpec{
		Name:          "support-v1",
		VersionNumber: 1,
		JudgeMode:     scoring.JudgeModeDeterministic,
		Validators: []scoring.ValidatorDeclaration{
			{Key: "exact", Type: scoring.ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
		},
		Scorecard: scoring.ScorecardDeclaration{
			Dimensions: []scoring.ScorecardDimension{scoring.ScorecardDimensionCorrectness},
		},
	}
}

func containsField(errs ValidationErrors, field string) bool {
	for _, err := range errs {
		if err.Field == field {
			return true
		}
	}
	return false
}
