package challengepack

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/scoring"
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
    allowed_tool_kinds: ["file"]
    allow_shell: true
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

func TestParseYAMLDeploymentDefaultsRoundTrip(t *testing.T) {
	bundle, err := ParseYAML([]byte(`
pack:
  slug: support-eval
  name: Support Eval
  family: support
version:
  number: 1
  deployment_defaults:
    aliases:
      candidate: " Candidate Agent "
      baseline: Baseline Agent
    lineups:
      default: ["candidate", "baseline"]
      smoke: ["candidate"]
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
    difficulty: medium
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
`))
	if err != nil {
		t.Fatalf("ParseYAML returned error: %v", err)
	}
	if bundle.Version.DeploymentDefaults == nil {
		t.Fatal("deployment defaults missing")
	}
	if got := bundle.Version.DeploymentDefaults.Aliases["candidate"]; got != "Candidate Agent" {
		t.Fatalf("candidate alias = %q, want Candidate Agent", got)
	}

	manifest, err := ManifestJSON(bundle)
	if err != nil {
		t.Fatalf("ManifestJSON returned error: %v", err)
	}
	var decoded struct {
		Version struct {
			DeploymentDefaults DeploymentDefaults `json:"deployment_defaults"`
		} `json:"version"`
	}
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if got := decoded.Version.DeploymentDefaults.Lineups["default"]; len(got) != 2 || got[0] != "candidate" || got[1] != "baseline" {
		t.Fatalf("default lineup = %#v, want [candidate baseline]", got)
	}
}

func TestValidateBundleRejectsDeploymentDefaultsWithoutDefaultLineup(t *testing.T) {
	bundle := minimalBundle()
	bundle.Version.DeploymentDefaults = &DeploymentDefaults{
		Aliases: map[string]string{"candidate": "Candidate Agent"},
		Lineups: map[string][]string{"smoke": []string{"candidate"}},
	}

	err := ValidateBundle(bundle)
	if err == nil {
		t.Fatal("ValidateBundle returned nil error")
	}
	errs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("error type = %T, want ValidationErrors", err)
	}
	if !containsField(errs, "version.deployment_defaults.lineups.default") {
		t.Fatalf("validation errors = %+v, want default lineup error", errs)
	}
}

func TestParseYAMLAcceptsVoiceModalityBundle(t *testing.T) {
	bundle, err := ParseYAML([]byte(`
modality: voice
interface_spec:
  transports: [text_sim, webrtc]
  channel_profile: deterministic_text
  supports_barge_in: true
scenario:
  persona: billing_customer
  language: en-US
  max_turns: 6
  max_duration_ms: 120000
pack:
  slug: voice-support-eval
  name: Voice Support Eval
  family: support
version:
  number: 1
  evaluation_spec:
    name: voice-support-v1
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: exact
        type: exact_match
        target: final_output
        expected_from: challenge_input
    metrics:
      - key: voice_latency_ms
        type: numeric
        collector: run_total_latency_ms
        unit: ms
    scorecard:
      dimensions: [correctness]
challenges:
  - key: billing-refund
    title: Billing Refund
    category: support
    difficulty: easy
input_sets:
  - key: default
    name: Default Voice Inputs
    cases:
      - challenge_key: billing-refund
        case_key: duplicate-charge
        inputs:
          - key: prompt
            kind: text
            value: Please refund the duplicate charge.
        expectations:
          - key: answer
            kind: text
            source: input:prompt
`))
	if err != nil {
		t.Fatalf("ParseYAML returned error: %v", err)
	}
	if bundle.Modality != ModalityVoice {
		t.Fatalf("modality = %q, want voice", bundle.Modality)
	}
	if bundle.InterfaceSpec == nil || len(bundle.InterfaceSpec.Transports) != 2 || bundle.InterfaceSpec.Transports[0] != "text_sim" {
		t.Fatalf("interface spec = %#v, want normalized voice transports", bundle.InterfaceSpec)
	}
	if bundle.Scenario == nil || bundle.Scenario.Language != "en-US" || bundle.Scenario.MaxTurns != 6 {
		t.Fatalf("scenario = %#v, want normalized voice scenario", bundle.Scenario)
	}

	manifest, err := ManifestJSON(bundle)
	if err != nil {
		t.Fatalf("ManifestJSON returned error: %v", err)
	}
	var decoded struct {
		Modality      string        `json:"modality"`
		InterfaceSpec InterfaceSpec `json:"interface_spec"`
		Scenario      ScenarioSpec  `json:"scenario"`
	}
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if decoded.Modality != ModalityVoice {
		t.Fatalf("manifest modality = %q, want voice", decoded.Modality)
	}
	if decoded.InterfaceSpec.ChannelProfile != "deterministic_text" {
		t.Fatalf("manifest channel profile = %q, want deterministic_text", decoded.InterfaceSpec.ChannelProfile)
	}
	if decoded.Scenario.MaxDurationMS != 120000 {
		t.Fatalf("manifest max_duration_ms = %d, want 120000", decoded.Scenario.MaxDurationMS)
	}
}

func TestValidateBundleVoiceModalityCases(t *testing.T) {
	cases := []struct {
		name       string
		mutate     func(*Bundle)
		wantFields []string
	}{
		{
			name: "valid existing pack",
		},
		{
			name: "valid minimal voice pack",
			mutate: func(bundle *Bundle) {
				*bundle = minimalVoiceBundle()
			},
		},
		{
			name: "invalid modality",
			mutate: func(bundle *Bundle) {
				bundle.Modality = "video"
			},
			wantFields: []string{"modality"},
		},
		{
			name: "invalid interface_spec transports",
			mutate: func(bundle *Bundle) {
				*bundle = minimalVoiceBundle()
				bundle.InterfaceSpec.Transports = []string{"text_sim", "satellite"}
			},
			wantFields: []string{"interface_spec.transports[1]"},
		},
		{
			name: "missing scenario max_turns",
			mutate: func(bundle *Bundle) {
				*bundle = minimalVoiceBundle()
				bundle.Scenario.MaxTurns = 0
			},
			wantFields: []string{"scenario.max_turns"},
		},
		{
			name: "missing scenario language",
			mutate: func(bundle *Bundle) {
				*bundle = minimalVoiceBundle()
				bundle.Scenario.Language = ""
			},
			wantFields: []string{"scenario.language"},
		},
		{
			name: "missing scenario persona",
			mutate: func(bundle *Bundle) {
				*bundle = minimalVoiceBundle()
				bundle.Scenario.Persona = ""
			},
			wantFields: []string{"scenario.persona"},
		},
		{
			name: "missing scenario max_duration_ms",
			mutate: func(bundle *Bundle) {
				*bundle = minimalVoiceBundle()
				bundle.Scenario.MaxDurationMS = 0
			},
			wantFields: []string{"scenario.max_duration_ms"},
		},
		{
			name: "duplicate metric keys",
			mutate: func(bundle *Bundle) {
				bundle.Version.EvaluationSpec.Metrics = []scoring.MetricDeclaration{
					{Key: "latency", Type: scoring.MetricTypeNumeric, Collector: "run_total_latency_ms"},
					{Key: "latency", Type: scoring.MetricTypeNumeric, Collector: "run_ttft_ms"},
				}
			},
			wantFields: []string{"version.evaluation_spec.metrics[1].key"},
		},
		{
			name: "duplicate validator keys",
			mutate: func(bundle *Bundle) {
				bundle.Version.EvaluationSpec.Validators = append(bundle.Version.EvaluationSpec.Validators, scoring.ValidatorDeclaration{
					Key:          "exact",
					Type:         scoring.ValidatorTypeContains,
					Target:       "final_output",
					ExpectedFrom: "challenge_input",
				})
			},
			wantFields: []string{"version.evaluation_spec.validators[1].key"},
		},
		{
			name: "voice blocks without modality",
			mutate: func(bundle *Bundle) {
				bundle.InterfaceSpec = &InterfaceSpec{Transports: []string{"text_sim"}, ChannelProfile: "deterministic_text"}
				bundle.Scenario = &ScenarioSpec{Persona: "billing_customer", Language: "en-US", MaxTurns: 6, MaxDurationMS: 120000}
			},
			wantFields: []string{"modality"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle := minimalBundle()
			if tc.mutate != nil {
				tc.mutate(&bundle)
			}

			err := ValidateBundle(bundle)
			if len(tc.wantFields) == 0 {
				if err != nil {
					t.Fatalf("ValidateBundle returned error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidateBundle returned nil error")
			}
			errs, ok := err.(ValidationErrors)
			if !ok {
				t.Fatalf("error type = %T, want ValidationErrors", err)
			}
			for _, field := range tc.wantFields {
				if !containsField(errs, field) {
					t.Fatalf("validation errors = %+v, want field %q", errs, field)
				}
			}
		})
	}
}

func TestNormalizeInterfaceSpecOmitsEmptyVoiceTransports(t *testing.T) {
	normalized := normalizeInterfaceSpec(&InterfaceSpec{ChannelProfile: "deterministic_text"})
	encoded, err := json.Marshal(normalized)
	if err != nil {
		t.Fatalf("marshal normalized interface spec: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal normalized interface spec: %v", err)
	}
	if _, ok := decoded["transports"]; ok {
		t.Fatalf("normalized transports present, want omitted")
	}
}

func TestValidateBundleRejectsEmptyDeploymentDefaultSelectors(t *testing.T) {
	bundle := minimalBundle()
	bundle.Version.DeploymentDefaults = &DeploymentDefaults{
		Aliases: map[string]string{"candidate": ""},
		Lineups: map[string][]string{"default": []string{""}},
	}

	err := ValidateBundle(bundle)
	if err == nil {
		t.Fatal("ValidateBundle returned nil error")
	}
	errs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("error type = %T, want ValidationErrors", err)
	}
	if !containsField(errs, "version.deployment_defaults.aliases[\"candidate\"]") {
		t.Fatalf("validation errors = %+v, want empty alias error", errs)
	}
	if !containsField(errs, "version.deployment_defaults.lineups[\"default\"][0]") {
		t.Fatalf("validation errors = %+v, want empty lineup selector error", errs)
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

func TestParseYAMLAcceptsToolCallAssertionValidator(t *testing.T) {
	bundle, err := ParseYAML([]byte(`
pack:
  slug: tool-eval
  name: Tool Eval
  family: support
version:
  number: 1
  evaluation_spec:
    name: tool-v1
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: submitted
        type: tool_call_assertion
        target: tool_calls
        config:
          tool_name: submit
          arguments_contain:
            answer: "42"
    scorecard:
      dimensions: [correctness]
challenges:
  - key: ticket-1
    title: Ticket One
    category: support
    difficulty: easy
input_sets:
  - key: default
    name: Default
    cases:
      - challenge_key: ticket-1
        case_key: sample-1
`))
	if err != nil {
		t.Fatalf("ParseYAML returned error: %v", err)
	}
	validator := bundle.Version.EvaluationSpec.Validators[0]
	if validator.Type != scoring.ValidatorTypeToolCallAssertion {
		t.Fatalf("validator type = %q, want %q", validator.Type, scoring.ValidatorTypeToolCallAssertion)
	}
	if validator.ExpectedFrom != "" {
		t.Fatalf("expected_from = %q, want empty", validator.ExpectedFrom)
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

func TestValidateBundleRejectsInputSetCasesAcrossMultipleChallenges(t *testing.T) {
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
			{Key: "ticket-2", Title: "Ticket Two", Category: "support", Difficulty: "easy"},
		},
		InputSets: []InputSetDefinition{
			{
				Key:  "default",
				Name: "Default",
				Cases: []CaseDefinition{
					{ChallengeKey: "ticket-1", CaseKey: "case-1"},
					{ChallengeKey: "ticket-2", CaseKey: "case-2"},
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
	if !containsField(validationErrs, "input_sets[0].cases[1].challenge_key") {
		t.Fatalf("expected mixed challenge_key error, got %v", validationErrs)
	}
	if got := validationErrs.Error(); !strings.Contains(got, "same challenge") {
		t.Fatalf("error = %q, want same challenge guidance", got)
	}
}

func TestValidateBundleRejectsInputSetMixedChallengesAfterMissingKey(t *testing.T) {
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
			{Key: "ticket-2", Title: "Ticket Two", Category: "support", Difficulty: "easy"},
		},
		InputSets: []InputSetDefinition{
			{
				Key:  "default",
				Name: "Default",
				Cases: []CaseDefinition{
					{CaseKey: "missing-challenge"},
					{ChallengeKey: "ticket-1", CaseKey: "case-1"},
					{ChallengeKey: "ticket-2", CaseKey: "case-2"},
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
	for _, field := range []string{
		"input_sets[0].cases[0].challenge_key",
		"input_sets[0].cases[2].challenge_key",
	} {
		if !containsField(validationErrs, field) {
			t.Fatalf("expected field %q in validation errors: %v", field, validationErrs)
		}
	}
}

func TestValidateBundleAllowsInputSetCasesForOneChallenge(t *testing.T) {
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
					{ChallengeKey: "ticket-1", CaseKey: "case-1"},
					{ChallengeKey: "ticket-1", CaseKey: "case-2"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ValidateBundle returned error: %v", err)
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

func TestValidateBundleAllowsResponsesWithSandboxAndToolPolicy(t *testing.T) {
	err := ValidateBundle(Bundle{
		Pack: PackMetadata{Slug: "research", Name: "Research", Family: "eval"},
		Version: VersionMetadata{
			Number:         1,
			ExecutionMode:  ExecutionModeResponses,
			EvaluationSpec: minimalSpec(),
			Sandbox:        &SandboxConfig{NetworkAccess: true},
			ToolPolicy:     map[string]any{"allow_shell": true, "allowed_tool_kinds": []any{"file", "network"}},
		},
		Challenges: []ChallengeDefinition{
			{Key: "c1", Title: "C1", Category: "cat", Difficulty: "easy"},
		},
		InputSets: []InputSetDefinition{
			{Key: "default", Name: "Default", Cases: []CaseDefinition{{ChallengeKey: "c1", CaseKey: "k"}}},
		},
	})
	if err != nil {
		t.Fatalf("ValidateBundle returned error: %v", err)
	}
}

func TestValidateBundleRejectsResponsesWithPackTools(t *testing.T) {
	err := ValidateBundle(Bundle{
		Pack: PackMetadata{Slug: "research", Name: "Research", Family: "eval"},
		Version: VersionMetadata{
			Number:         1,
			ExecutionMode:  ExecutionModeResponses,
			EvaluationSpec: minimalSpec(),
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
		t.Fatal("expected validation error for responses pack with tools")
	}
	errs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	if !containsField(errs, "tools") {
		t.Fatalf("expected tools validation error; got %v", errs)
	}
}

func TestValidateBundleAllowsBrowserToolKind(t *testing.T) {
	err := ValidateBundle(Bundle{
		Pack: PackMetadata{Slug: "browser", Name: "Browser", Family: "browser"},
		Version: VersionMetadata{
			Number:         1,
			ExecutionMode:  ExecutionModeNative,
			ToolPolicy:     map[string]any{"allowed_tool_kinds": []any{"browser"}},
			EvaluationSpec: minimalSpec(),
		},
		Challenges: []ChallengeDefinition{
			{Key: "c1", Title: "C1", Category: "browser", Difficulty: "easy"},
		},
		InputSets: []InputSetDefinition{
			{Key: "default", Name: "Default", Cases: []CaseDefinition{{ChallengeKey: "c1", CaseKey: "k"}}},
		},
	})
	if err != nil {
		t.Fatalf("ValidateBundle returned error: %v", err)
	}
}

func TestValidateBundleRejectsUnknownToolKind(t *testing.T) {
	err := ValidateBundle(Bundle{
		Pack: PackMetadata{Slug: "browser", Name: "Browser", Family: "browser"},
		Version: VersionMetadata{
			Number:         1,
			ExecutionMode:  ExecutionModeNative,
			ToolPolicy:     map[string]any{"allowed_tool_kinds": []any{"browser", "telepathy"}},
			EvaluationSpec: minimalSpec(),
		},
		Challenges: []ChallengeDefinition{
			{Key: "c1", Title: "C1", Category: "browser", Difficulty: "easy"},
		},
		InputSets: []InputSetDefinition{
			{Key: "default", Name: "Default", Cases: []CaseDefinition{{ChallengeKey: "c1", CaseKey: "k"}}},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error for unknown tool kind")
	}
	errs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	if !containsField(errs, "version.tool_policy.allowed_tool_kinds[1]") {
		t.Fatalf("expected allowed_tool_kinds validation error; got %v", errs)
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
			Dimensions: []scoring.DimensionDeclaration{{Key: "correctness"}},
		},
	}
}

func minimalBundle() Bundle {
	return Bundle{
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
			{
				Key:        "ticket-1",
				Title:      "Ticket One",
				Category:   "support",
				Difficulty: "easy",
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
						},
						Expectations: []CaseExpectation{
							{Key: "answer", Kind: "text", Source: "input:prompt"},
						},
					},
				},
			},
		},
	}
}

func minimalVoiceBundle() Bundle {
	bundle := minimalBundle()
	bundle.Modality = ModalityVoice
	bundle.InterfaceSpec = &InterfaceSpec{
		Transports:      []string{"text_sim"},
		ChannelProfile:  "deterministic_text",
		SupportsBargeIn: false,
	}
	bundle.Scenario = &ScenarioSpec{
		Persona:       "billing_customer",
		Language:      "en-US",
		MaxTurns:      6,
		MaxDurationMS: 120000,
	}
	return bundle
}

func containsField(errs ValidationErrors, field string) bool {
	for _, err := range errs {
		if err.Field == field {
			return true
		}
	}
	return false
}
