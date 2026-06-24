package challengepack

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/agentclash/agentclash/backend/internal/scoring"
)

// BundleToComposition is the inverse of ComposeBundle: it turns a runnable
// Bundle back into a builder Composition with every piece inlined, so a
// published pack can be reopened in the visual builder and re-published with no
// data loss. Fields the builder does not edit are preserved verbatim in
// Composition.Advanced.
//
// Round-trip invariant: ManifestJSON(ComposeBundle(BundleToComposition(b), nil))
// equals ManifestJSON(b) for any valid bundle (modulo ComposeBundle's documented
// defaulting of empty name/version/judge_mode, which a real bundle already has).
func BundleToComposition(bundle Bundle) (Composition, error) {
	comp := Composition{
		SchemaVersion: 1,
		Pack:          bundle.Pack,
		Version: CompositionVersion{
			Number:        bundle.Version.Number,
			ExecutionMode: bundle.Version.ExecutionMode,
			ToolPolicy:    bundle.Version.ToolPolicy,
			Sandbox:       bundle.Version.Sandbox,
		},
	}

	for i := range bundle.Challenges {
		ref, err := inlinePieceRef(bundle.Challenges[i])
		if err != nil {
			return Composition{}, fmt.Errorf("challenge %d: %w", i, err)
		}
		comp.Challenges = append(comp.Challenges, ref)
	}
	for i := range bundle.InputSets {
		ref, err := inlinePieceRef(bundle.InputSets[i])
		if err != nil {
			return Composition{}, fmt.Errorf("input set %d: %w", i, err)
		}
		comp.InputSets = append(comp.InputSets, ref)
	}

	spec := bundle.Version.EvaluationSpec
	for i := range spec.Validators {
		ref, err := inlinePieceRef(spec.Validators[i])
		if err != nil {
			return Composition{}, fmt.Errorf("validator %d: %w", i, err)
		}
		comp.Validators = append(comp.Validators, ref)
	}
	for i := range spec.LLMJudges {
		ref, err := inlinePieceRef(spec.LLMJudges[i])
		if err != nil {
			return Composition{}, fmt.Errorf("judge %d: %w", i, err)
		}
		comp.Judges = append(comp.Judges, ref)
	}

	comp.Scorecard = CompositionScorecard{
		Name:          spec.Name,
		VersionNumber: spec.VersionNumber,
		JudgeMode:     spec.JudgeMode,
		Strategy:      spec.Scorecard.Strategy,
		PassThreshold: spec.Scorecard.PassThreshold,
		Dimensions:    spec.Scorecard.Dimensions,
	}

	adv := CompositionAdvanced{
		Modality:            bundle.Modality,
		InterfaceSpec:       bundle.InterfaceSpec,
		Scenario:            bundle.Scenario,
		Tools:               bundle.Tools,
		Security:            bundle.Security,
		Filesystem:          bundle.Version.Filesystem,
		DeploymentDefaults:  bundle.Version.DeploymentDefaults,
		Assets:              bundle.Version.Assets,
		Metrics:             spec.Metrics,
		Behavioral:          spec.Behavioral,
		PostExecutionChecks: spec.PostExecutionChecks,
		RuntimeLimits:       spec.RuntimeLimits,
		Pricing:             spec.Pricing,
		Normalization:       spec.Scorecard.Normalization,
		JudgeLimits:         spec.Scorecard.JudgeLimits,
	}
	if !reflect.DeepEqual(adv, CompositionAdvanced{}) {
		comp.Advanced = &adv
	}

	return comp, nil
}

func inlinePieceRef(v any) (PieceRef, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return PieceRef{}, fmt.Errorf("marshal inline piece: %w", err)
	}
	return PieceRef{Inline: raw}, nil
}

// ManifestToBundle reconstructs a Bundle from a stored challenge_pack_versions
// manifest (the flattened JSON ManifestJSON produces). It decodes every key
// ManifestJSON writes, so it is loss-free, and is the source for hydrating a
// builder draft from an already-published pack without depending on the
// (optional) bundle artifact store.
func ManifestToBundle(manifest json.RawMessage) (Bundle, error) {
	var raw struct {
		Pack    PackMetadata `json:"pack"`
		Version struct {
			Number             int32               `json:"number"`
			ExecutionMode      string              `json:"execution_mode"`
			DeploymentDefaults *DeploymentDefaults `json:"deployment_defaults"`
			Assets             []AssetReference    `json:"assets"`
		} `json:"version"`
		ToolPolicy     map[string]any         `json:"tool_policy"`
		Filesystem     map[string]any         `json:"filesystem"`
		EvaluationSpec scoring.EvaluationSpec `json:"evaluation_spec"`
		Challenges     []ChallengeDefinition  `json:"challenges"`
		InputSets      []InputSetDefinition   `json:"input_sets"`
		Modality       string                 `json:"modality"`
		InterfaceSpec  *InterfaceSpec         `json:"interface_spec"`
		Scenario       *ScenarioSpec          `json:"scenario"`
		Tools          map[string]any         `json:"tools"`
		Sandbox        *SandboxConfig         `json:"sandbox"`
		Security       *SecurityPolicy        `json:"security"`
	}
	if err := json.Unmarshal(manifest, &raw); err != nil {
		return Bundle{}, fmt.Errorf("decode challenge-pack manifest: %w", err)
	}

	bundle := Bundle{
		Modality:      raw.Modality,
		InterfaceSpec: raw.InterfaceSpec,
		Scenario:      raw.Scenario,
		Pack:          raw.Pack,
		Version: VersionMetadata{
			Number:             raw.Version.Number,
			ExecutionMode:      raw.Version.ExecutionMode,
			ToolPolicy:         raw.ToolPolicy,
			Filesystem:         raw.Filesystem,
			Sandbox:            raw.Sandbox,
			DeploymentDefaults: raw.Version.DeploymentDefaults,
			EvaluationSpec:     raw.EvaluationSpec,
			Assets:             raw.Version.Assets,
		},
		Tools:      raw.Tools,
		Challenges: raw.Challenges,
		InputSets:  raw.InputSets,
		Security:   raw.Security,
	}
	normalizeBundle(&bundle)
	return bundle, nil
}
