package api

import (
	"encoding/json"
	"fmt"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/scoring"
	"github.com/google/uuid"
	"reflect"
	"strings"
)

// VibePackCompiler reuses #1246's narrow blueprint and #1245's shared
// BundleToComposition conversion. The interactive builder's permissive fallback
// is deliberately not used: reduced coverage is an error, never a normal run.
type VibePackCompiler struct{}

// Editing a semantic preview must never discard imported validators, judges,
// dimensions or payload fields. Only a lossless generated-shape round trip is
// eligible; other contracts remain available in the advanced builder.
func canEditVibeEvaluation(content json.RawMessage, l vibe.Limits) bool {
	var b generatedPackBlueprint
	if json.Unmarshal(content, &b) != nil || len(b.Judges) != 1 {
		return false
	}
	p := vibe.DraftProposal{Title: b.Name, AgentPrompt: b.Instructions, SuccessCriteria: b.Judges[0].Assertion}
	for _, c := range b.Cases {
		input, ok := c.Payload["question"].(string)
		if !ok {
			return false
		}
		p.Examples = append(p.Examples, input)
	}
	roundtrip, err := (VibePackCompiler{}).Draft(p, l)
	if err != nil {
		return false
	}
	var original, rebuilt any
	return json.Unmarshal(content, &original) == nil && json.Unmarshal(roundtrip, &rebuilt) == nil && reflect.DeepEqual(original, rebuilt)
}

func (VibePackCompiler) Instructions() string {
	return `draft has exactly this shape:
{"title":"Short title","agent_prompt":"Complete runnable text-agent instructions","examples":["A concrete user request","A relevant boundary case","A tricky or adversarial request"],"success_criteria":"A plain-English true/false claim describing correct behavior for the given example; allow equivalent wording."}
Use one to three examples, each a string containing only what the target agent should receive, never the answer key. Put expected behavior in success_criteria. Make conditional criteria explicit so opposite outcomes can be correct in different examples. Include all requested coverage. Use explicit ages or dates in date-sensitive examples. Do not generate blueprint, regex, validator, judge, dimension or model configuration: Go constructs those. For accepted-agent improvements examples may be [] and success_criteria empty, because the existing evaluation is pinned.`
}

// Draft converts semantic content into the merged #1246 blueprint. The model
// cannot invent scoring enums, regex syntax, global phrase checks or references.
// Compile below still rejects invalid coverage and reuses #1245's composition.
func (VibePackCompiler) Draft(a vibe.DraftProposal, l vibe.Limits) (json.RawMessage, error) {
	if strings.TrimSpace(a.Title) == "" || len(a.Title) > vibe.MaxKeyBytes || strings.TrimSpace(a.AgentPrompt) == "" || len(a.AgentPrompt) > l.MessageBytes {
		return nil, fmt.Errorf("a bounded title and agent prompt are required")
	}
	if len(a.Examples) < 1 || len(a.Examples) > min(3, l.Cases) {
		return nil, fmt.Errorf("a conversational preview requires one to three examples; no examples were removed")
	}
	if strings.TrimSpace(a.SuccessCriteria) == "" || len(a.SuccessCriteria) > 4096 {
		return nil, fmt.Errorf("success_criteria must be a nonempty string of at most 4096 bytes")
	}
	p := generatedPackBlueprint{
		Slug: "agent-check", Name: a.Title, Instructions: a.AgentPrompt,
		Description: "Text preview of the proposed agent instructions.", Difficulty: "easy",
		Validators: []scoring.ValidatorDeclaration{{Key: "has_answer", Type: scoring.ValidatorTypeRegexMatch, Target: "final_output", ExpectedFrom: "literal:.+"}},
		Judges:     []generatedPackJudge{{Key: "behavior", Mode: scoring.JudgeMethodAssertion, Assertion: a.SuccessCriteria}},
		Dimensions: []scoring.DimensionDeclaration{
			{Key: "output_present", Source: scoring.DimensionSourceValidators, Validators: []string{"has_answer"}},
			{Key: "behavior_correctness", Source: scoring.DimensionSourceLLMJudge, JudgeKey: "behavior"},
		},
	}
	for i, example := range a.Examples {
		if strings.TrimSpace(example) == "" || len(example) > l.MessageBytes {
			return nil, fmt.Errorf("example %d must be a nonempty bounded string; no examples were removed", i+1)
		}
		p.Cases = append(p.Cases, generatedPackCase{Key: fmt.Sprintf("case-%d", i+1), Payload: map[string]any{"question": example}})
	}
	return json.Marshal(p)
}

func (VibePackCompiler) Compile(content json.RawMessage, evaluator string, id uuid.UUID, l vibe.Limits) (vibe.Compiled, error) {
	var out vibe.Compiled
	if err := vibe.ValidateJSON(content, l); err != nil {
		return out, err
	}
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(content, &shape); err != nil {
		return out, err
	}
	var b challengepack.Bundle
	if _, ok := shape["pack"]; ok {
		if err := vibe.Decode(content, l, &b); err != nil {
			return out, err
		}
		if b.Version.ExecutionMode != "" && b.Version.ExecutionMode != challengepack.ExecutionModePromptEval && b.Version.ExecutionMode != challengepack.ExecutionModeNative {
			return out, fmt.Errorf("this pack needs an advanced execution mode; open it in the pack builder")
		}
	} else {
		var p generatedPackBlueprint
		if err := vibe.Decode(content, l, &p); err != nil {
			return out, err
		}
		if err := checkGeneratedPackChoices(p); err != nil {
			return out, err
		}
		if len(p.Cases) == 0 {
			return out, fmt.Errorf("the generated evaluation contains no cases")
		}
		seen := map[string]bool{}
		for _, c := range p.Cases {
			if c.Key == "" || seen[c.Key] {
				return out, fmt.Errorf("case keys must be nonempty and unique; no cases were removed")
			}
			seen[c.Key] = true
		}
		for _, j := range p.Judges {
			if strings.TrimSpace(j.Key) == "" {
				return out, fmt.Errorf("an evaluator has an empty key; no evaluators were removed")
			}
		}
		b = generatedPackBundle(p, evaluator, id, true)
		if len(b.Version.EvaluationSpec.LLMJudges) != len(p.Judges) || len(b.Version.EvaluationSpec.Scorecard.Dimensions) != len(p.Dimensions) {
			return out, fmt.Errorf("the requested evaluation has invalid evaluator references; no coverage was removed")
		}
	}
	if len(b.Tools) > 0 || len(b.Version.ToolPolicy) > 0 || len(b.Version.Filesystem) > 0 || b.Version.Sandbox != nil || len(b.Version.Assets) > 0 || b.Modality != "" || b.Security != nil {
		return out, fmt.Errorf("this pack requires capabilities unavailable in a text preview; open the original pack in the builder")
	}
	if len(b.Challenges) != 1 {
		return out, fmt.Errorf("a text preview supports one task; split multi-task packs explicitly")
	}
	if len(b.Challenges[0].Assets) > 0 || len(b.Challenges[0].ArtifactRefs) > 0 {
		return out, fmt.Errorf("file-backed challenges need the advanced runner")
	}
	spec := &b.Version.EvaluationSpec
	if len(spec.Metrics) > 0 || spec.Behavioral != nil || len(spec.PostExecutionChecks) > 0 {
		return out, fmt.Errorf("metrics, behavioral signals and post-execution checks require the advanced runner; no coverage was removed")
	}
	if len(spec.Validators) == 0 || len(spec.Validators) > l.Checks || len(spec.LLMJudges) > l.Evaluators {
		return out, fmt.Errorf("evaluation exceeds the validator or evaluator limit")
	}
	for _, v := range spec.Validators {
		if len(v.Key) > vibe.MaxKeyBytes {
			return out, fmt.Errorf("validator key exceeds 128 bytes")
		}
		if err := checkGeneratedPackChoice("validator", v.Key, v.Type, generatedPackValidatorTypes); err != nil {
			return out, err
		}
		if v.Target != "final_output" {
			return out, fmt.Errorf("preview validators must evaluate the agent output")
		}
		if v.Type == scoring.ValidatorTypeRegexMatch && len(v.ExpectedFrom) > 1024 {
			return out, fmt.Errorf("regex exceeds the 1 KiB preview limit")
		}
		if len(v.ExpectedFrom) > 2048 {
			return out, fmt.Errorf("validator reference exceeds its bounded text allowance")
		}
	}
	for i := range spec.LLMJudges {
		j := &spec.LLMJudges[i]
		if len(j.Key) > vibe.MaxKeyBytes {
			return out, fmt.Errorf("evaluator key exceeds 128 bytes")
		}
		if len(j.Models) > 0 || j.Samples > 1 {
			return out, fmt.Errorf("multi-model judges and repeated sampling require an advanced run")
		}
		if j.Mode != scoring.JudgeMethodAssertion && j.Mode != scoring.JudgeMethodRubric {
			return out, fmt.Errorf("this evaluator needs evidence unavailable to the text preview")
		}
		if len(j.ContextFrom) > 0 || len(j.OutputSchema) > 0 || j.Consensus != nil || j.ReferenceFrom != "" {
			return out, fmt.Errorf("this evaluator has context or schema requirements unavailable in a text preview; no evaluators were removed")
		}
		j.Model = evaluator // explicit role policy, never the authoring/target model
	}
	for _, set := range b.InputSets {
		for _, c := range set.Cases {
			if len(c.CaseKey) > vibe.MaxKeyBytes {
				return out, fmt.Errorf("case key exceeds 128 bytes")
			}
			if len(c.Artifacts) > 0 || len(c.Assets) > 0 || c.UserSimulator != nil {
				return out, fmt.Errorf("artifact-backed cases need the advanced runner")
			}
			for _, input := range c.Inputs {
				if input.ArtifactKey != "" || input.Path != "" {
					return out, fmt.Errorf("file inputs require an advanced runner")
				}
			}
			for _, expectation := range c.Expectations {
				if expectation.ArtifactKey != "" {
					return out, fmt.Errorf("file expectations require an advanced runner")
				}
			}
			out.Cases = append(out.Cases, c)
		}
	}
	if len(out.Cases) == 0 || len(out.Cases) > l.ImportCases {
		return out, fmt.Errorf("evaluation must contain between 1 and %d cases", l.ImportCases)
	}
	b.Version.ExecutionMode = challengepack.ExecutionModePromptEval
	composition, err := challengepack.BundleToComposition(b)
	if err != nil {
		return out, err
	}
	composed, err := challengepack.ComposeBundle(composition, challengepack.ResolvedPieces{})
	if err != nil {
		return out, err
	}
	if err = challengepack.ValidateBundle(composed); err != nil {
		return out, fmt.Errorf("evaluation did not compile without reducing coverage: %w", err)
	}
	// ValidateBundle normalizes only a local scoring copy. Use the shared
	// decoder's normalization for all executable evidence preflight as well
	// as coverage checks. Keep the original bundle/composition for identity.
	definition, err := json.Marshal(composed.Version.EvaluationSpec)
	if err != nil {
		return out, err
	}
	normalized, err := scoring.DecodeDefinition(definition)
	if err != nil {
		return out, err
	}
	for _, c := range out.Cases {
		if len(vibe.TargetInput(c, normalized)) == 0 {
			return out, fmt.Errorf("case %s has no safe target inputs after withholding expected answers; payload paths must resolve without removing the whole input", c.CaseKey)
		}
		// Resolve the same declarations the shared scorer will execute, not
		// raw references that can select a whitespace-suffixed decoy key.
		for _, validator := range normalized.Validators {
			if !validator.Type.RequiresExpectedFrom() {
				continue
			}
			value, _, err := scoring.ResolveEvidenceValueForJudge(validator.ExpectedFrom, scoring.EvaluationInput{ChallengeInputs: []scoring.EvidenceInput{vibe.CaseEvidence(c)}})
			if err != nil || value == nil {
				return out, fmt.Errorf("validator %s has unavailable expected evidence in case %s", validator.Key, c.CaseKey)
			}
			if validator.Type == scoring.ValidatorTypeRegexMatch && len(*value) > 1024 {
				return out, fmt.Errorf("resolved regex exceeds the 1 KiB preview limit")
			}
			if validator.Type == scoring.ValidatorTypeFuzzyMatch && len(*value) > 2048 {
				return out, fmt.Errorf("resolved fuzzy expected value exceeds the 2048-byte preview limit")
			}
		}
	}
	// Vibe persists validator/judge verdicts, not advanced dimension results.
	for _, dimension := range normalized.Scorecard.Dimensions {
		if dimension.Source != scoring.DimensionSourceValidators && dimension.Source != scoring.DimensionSourceLLMJudge {
			return out, fmt.Errorf("dimension %s uses unsupported preview source %s; open it in the advanced runner; no coverage was removed", dimension.Key, dimension.Source)
		}
	}
	// Execute the compiled bundle, including inferred judge mode and defaults.
	// The authoring bundle intentionally leaves these fields for the builder.
	out.Bundle = composed
	out.Composition, err = json.Marshal(composition)
	if err != nil {
		return out, err
	}
	return out, nil
}
