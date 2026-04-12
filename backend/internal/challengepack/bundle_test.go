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
    cases:
      - challenge_key: ticket-1
        case_key: sample-1
        inputs:
          - key: prompt
            kind: text
            value: hello
        expectations:
          - key: answer
            kind: text
            source: input:prompt
tools:
  allowed: ["read_file"]
  denied: ["exec"]
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
	if len(bundle.InputSets[0].Cases) != 1 {
		t.Fatalf("case count = %d, want 1", len(bundle.InputSets[0].Cases))
	}
	if bundle.InputSets[0].Cases[0].CaseKey != "sample-1" {
		t.Fatalf("case key = %q, want sample-1", bundle.InputSets[0].Cases[0].CaseKey)
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
	if _, ok := decoded["tools"]; !ok {
		t.Fatalf("manifest tools block missing")
	}
}

func TestManifestJSONPreservesGeneralizedContract(t *testing.T) {
	bundle := Bundle{
		Pack: PackMetadata{
			Slug:   "support-eval",
			Name:   "Support Eval",
			Family: "support",
		},
		Version: VersionMetadata{
			Number:         1,
			EvaluationSpec: minimalSpec(),
			Assets: []AssetReference{
				{Key: "workspace", Kind: "workspace", Path: "assets/workspace.zip"},
			},
		},
		Challenges: []ChallengeDefinition{
			{
				Key:          "ticket-1",
				Title:        "Ticket One",
				Category:     "support",
				Difficulty:   "easy",
				ArtifactRefs: []ArtifactRef{{Key: "workspace"}},
			},
		},
		InputSets: []InputSetDefinition{
			{
				Key:  "default",
				Name: "Default",
				Cases: []CaseDefinition{
					{
						ChallengeKey: "ticket-1",
						CaseKey:      "case-1",
						Inputs: []CaseInput{
							{Key: "prompt", Kind: "text", Value: "hello"},
							{Key: "fixture", Kind: "workspace", ArtifactKey: "workspace"},
						},
						Expectations: []CaseExpectation{
							{Key: "answer", Kind: "text", Source: "input:prompt"},
						},
						Artifacts: []ArtifactRef{{Key: "workspace"}},
					},
				},
			},
		},
	}

	manifest, err := ManifestJSON(bundle)
	if err != nil {
		t.Fatalf("ManifestJSON returned error: %v", err)
	}

	var decoded struct {
		InputSets []struct {
			Cases []struct {
				CaseKey      string            `json:"case_key"`
				Inputs       []CaseInput       `json:"inputs"`
				Expectations []CaseExpectation `json:"expectations"`
				Artifacts    []ArtifactRef     `json:"artifacts"`
			} `json:"cases"`
		} `json:"input_sets"`
	}
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		t.Fatalf("unmarshal generalized manifest: %v", err)
	}
	if len(decoded.InputSets) != 1 || len(decoded.InputSets[0].Cases) != 1 {
		t.Fatalf("manifest input_sets/cases = %#v, want one generalized case", decoded.InputSets)
	}
	if decoded.InputSets[0].Cases[0].CaseKey != "case-1" {
		t.Fatalf("case_key = %q, want case-1", decoded.InputSets[0].Cases[0].CaseKey)
	}
	if len(decoded.InputSets[0].Cases[0].Inputs) != 2 {
		t.Fatalf("inputs count = %d, want 2", len(decoded.InputSets[0].Cases[0].Inputs))
	}
	if len(decoded.InputSets[0].Cases[0].Expectations) != 1 {
		t.Fatalf("expectations count = %d, want 1", len(decoded.InputSets[0].Cases[0].Expectations))
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
    cases:
      - challenge_key: ticket-2
        case_key: sample-1
        expectations:
          - key: answer
            kind: text
            source: input:missing
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
		"input_sets[0].cases[0].challenge_key",
		"input_sets[0].cases[0].expectations[0].source",
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
		InputSets: []InputSetDefinition{
			{
				Key:  "default",
				Name: "Default",
				Cases: []CaseDefinition{
					{ChallengeKey: "ticket-1", CaseKey: "case-1"},
				},
			},
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

func TestValidateBundleRejectsCaseShapeViolations(t *testing.T) {
	err := ValidateBundle(Bundle{
		Pack: PackMetadata{
			Slug:   "support-eval",
			Name:   "Support Eval",
			Family: "support",
		},
		Version: VersionMetadata{
			Number:         1,
			EvaluationSpec: minimalSpec(),
		},
		Challenges: []ChallengeDefinition{
			{Key: "ticket-1", Title: "Ticket One", Category: "support", Difficulty: "easy"},
		},
		InputSets: []InputSetDefinition{
			{
				Key:  "default",
				Name: "Default",
				Cases: []CaseDefinition{
					{
						ChallengeKey: "ticket-1",
						Inputs: []CaseInput{
							{Kind: "text", Value: "hello"},
						},
						Expectations: []CaseExpectation{
							{Key: "answer", Source: "bad-prefix"},
						},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateBundle returned nil error")
	}

	validationErrs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("error type = %T, want ValidationErrors", err)
	}
	expectedFields := []string{
		"input_sets[0].cases[0].case_key",
		"input_sets[0].cases[0].inputs[0].key",
		"input_sets[0].cases[0].expectations[0].kind",
		"input_sets[0].cases[0].expectations[0].source",
	}
	for _, field := range expectedFields {
		if !containsField(validationErrs, field) {
			t.Fatalf("expected field %q in validation errors: %v", field, validationErrs)
		}
	}
}

func TestParseYAMLLegacyItemsNormalizeIntoCases(t *testing.T) {
	bundle, err := ParseYAML([]byte(`
pack:
  slug: support-eval
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
    difficulty: easy
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

	if len(bundle.InputSets[0].Cases) != 1 {
		t.Fatalf("case count = %d, want 1", len(bundle.InputSets[0].Cases))
	}
	if bundle.InputSets[0].Cases[0].CaseKey != "sample-1" {
		t.Fatalf("case key = %q, want sample-1", bundle.InputSets[0].Cases[0].CaseKey)
	}
	if bundle.InputSets[0].Cases[0].Payload["content"] != "hello" {
		t.Fatalf("payload.content = %#v, want hello", bundle.InputSets[0].Cases[0].Payload["content"])
	}
}

func TestValidateBundleRejectsUnknownCaseArtifactRef(t *testing.T) {
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
				{Key: "workspace", Kind: "workspace", Path: "assets/workspace.zip"},
			},
		},
		Challenges: []ChallengeDefinition{
			{Key: "ticket-1", Title: "Ticket One", Category: "support", Difficulty: "easy"},
		},
		InputSets: []InputSetDefinition{
			{
				Key:  "default",
				Name: "Default",
				Cases: []CaseDefinition{
					{
						ChallengeKey: "ticket-1",
						CaseKey:      "case-1",
						Artifacts:    []ArtifactRef{{Key: "missing"}},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateBundle returned nil error")
	}

	validationErrs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("error type = %T, want ValidationErrors", err)
	}
	if !containsField(validationErrs, "input_sets[0].cases[0].artifacts[0].key") {
		t.Fatalf("expected unknown artifact ref error, got %v", validationErrs)
	}
}

func TestValidateBundleRejectsExpectationReferenceMisses(t *testing.T) {
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
				{Key: "answer-file", Kind: "file", Path: "assets/answer.json"},
			},
		},
		Challenges: []ChallengeDefinition{
			{Key: "ticket-1", Title: "Ticket One", Category: "support", Difficulty: "easy"},
		},
		InputSets: []InputSetDefinition{
			{
				Key:  "default",
				Name: "Default",
				Cases: []CaseDefinition{
					{
						ChallengeKey: "ticket-1",
						CaseKey:      "case-1",
						Inputs: []CaseInput{
							{Key: "prompt", Kind: "text", Value: "hello"},
						},
						Expectations: []CaseExpectation{
							{Key: "answer", Kind: "text", Source: "input:missing"},
							{Key: "json", Kind: "json", Source: "artifact:missing-file"},
						},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateBundle returned nil error")
	}

	validationErrs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("error type = %T, want ValidationErrors", err)
	}
	if !containsField(validationErrs, "input_sets[0].cases[0].expectations[0].source") {
		t.Fatalf("expected unknown input source error, got %v", validationErrs)
	}
	if !containsField(validationErrs, "input_sets[0].cases[0].expectations[1].source") {
		t.Fatalf("expected unknown artifact source error, got %v", validationErrs)
	}
}

func TestParseYAMLPromptEvalBundle(t *testing.T) {
	bundle, err := ParseYAML([]byte(`
pack:
  slug: translation-eval
  name: Translation Eval
  family: nlp
version:
  number: 1
  execution_mode: prompt_eval
  evaluation_spec:
    name: translation-v1
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: check-output
        type: contains
        target: final_output
        expected_from: challenge_input
    scorecard:
      dimensions: [correctness]
challenges:
  - key: translate-greeting
    title: Translate a greeting
    category: translation
    difficulty: easy
    instructions: "Translate {{text}} to {{language}}"
input_sets:
  - key: default
    name: Default
    cases:
      - challenge_key: translate-greeting
        case_key: french-hello
        inputs:
          - key: text
            kind: text
            value: hello world
          - key: language
            kind: text
            value: French
        expectations:
          - key: answer
            kind: text
            source: input:text
`))
	if err != nil {
		t.Fatalf("ParseYAML returned error: %v", err)
	}
	if bundle.Version.ExecutionMode != ExecutionModePromptEval {
		t.Fatalf("execution mode = %q, want prompt_eval", bundle.Version.ExecutionMode)
	}

	manifest, err := ManifestJSON(bundle)
	if err != nil {
		t.Fatalf("ManifestJSON returned error: %v", err)
	}
	var decoded struct {
		Version struct {
			ExecutionMode string `json:"execution_mode"`
		} `json:"version"`
	}
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if decoded.Version.ExecutionMode != ExecutionModePromptEval {
		t.Fatalf("manifest version.execution_mode = %q, want prompt_eval", decoded.Version.ExecutionMode)
	}
}

func TestValidateBundleRejectsPromptEvalWithTools(t *testing.T) {
	err := ValidateBundle(Bundle{
		Pack: PackMetadata{Slug: "t", Name: "T", Family: "f"},
		Version: VersionMetadata{
			Number:         1,
			ExecutionMode:  ExecutionModePromptEval,
			EvaluationSpec: minimalSpec(),
			Sandbox:        &SandboxConfig{NetworkAccess: true},
		},
		Tools: map[string]any{"custom": []any{}},
		Challenges: []ChallengeDefinition{
			{Key: "c1", Title: "C1", Category: "cat", Difficulty: "easy"},
		},
		InputSets: []InputSetDefinition{
			{Key: "default", Name: "Default", Cases: []CaseDefinition{{ChallengeKey: "c1", CaseKey: "k"}}},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error for prompt_eval pack with tools/sandbox")
	}
	errs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	if !containsField(errs, "tools") {
		t.Fatalf("expected tools validation error; got %v", errs)
	}
	if !containsField(errs, "version.sandbox") {
		t.Fatalf("expected version.sandbox validation error; got %v", errs)
	}
}

func TestValidateBundleRejectsUnknownExecutionMode(t *testing.T) {
	err := ValidateBundle(Bundle{
		Pack: PackMetadata{Slug: "t", Name: "T", Family: "f"},
		Version: VersionMetadata{
			Number:         1,
			ExecutionMode:  "exotic",
			EvaluationSpec: minimalSpec(),
		},
		Challenges: []ChallengeDefinition{
			{Key: "c1", Title: "C1", Category: "cat", Difficulty: "easy"},
		},
		InputSets: []InputSetDefinition{
			{Key: "default", Name: "Default", Cases: []CaseDefinition{{ChallengeKey: "c1", CaseKey: "k"}}},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error for unknown execution_mode")
	}
	errs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	if !containsField(errs, "version.execution_mode") {
		t.Fatalf("expected version.execution_mode error; got %v", errs)
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
