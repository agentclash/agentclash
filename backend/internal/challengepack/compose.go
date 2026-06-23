package challengepack

import (
	"encoding/json"
	"fmt"

	"github.com/agentclash/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Composition is the visual pack builder's working document for a
// pack-in-progress. It is stored verbatim in challenge_pack_drafts.composition
// and resolved + snapshotted into a runnable Bundle at compile/publish time via
// ComposeBundle.
//
// Pieces are referenced by ID (a challenge_pieces row) or inlined (a
// not-yet-promoted definition). Each piece's definition is exactly the matching
// Bundle sub-struct — a ValidatorDeclaration, an LLMJudgeDeclaration, a
// ChallengeDefinition, or an InputSetDefinition — so composing is a
// resolve-and-append, not a translation. The per-pack scorecard wiring
// (dimensions referencing validator/judge keys) is authored directly on the
// composition because, by design, validators/judges only resolve once wired
// into a scorecard.
type Composition struct {
	SchemaVersion int32                `json:"schema_version,omitempty"`
	Pack          PackMetadata         `json:"pack"`
	Version       CompositionVersion   `json:"version"`
	Challenges    []PieceRef           `json:"challenges,omitempty"`
	InputSets     []PieceRef           `json:"input_sets,omitempty"`
	Validators    []PieceRef           `json:"validators,omitempty"`
	Judges        []PieceRef           `json:"judges,omitempty"`
	Scorecard     CompositionScorecard `json:"scorecard"`
}

// CompositionVersion holds the pack-version-level config the builder edits
// directly (everything except the evaluation spec, which is assembled from
// pieces + scorecard wiring).
type CompositionVersion struct {
	Number        int32          `json:"number,omitempty"`
	ExecutionMode string         `json:"execution_mode,omitempty"`
	ToolPolicy    map[string]any `json:"tool_policy,omitempty"`
	Sandbox       *SandboxConfig `json:"sandbox,omitempty"`
}

// PieceRef references a reusable challenge_pieces row by ID, or inlines a
// not-yet-promoted definition. Exactly one of RefID / Inline should be set;
// RefID wins if both are present.
type PieceRef struct {
	RefID  *uuid.UUID      `json:"ref_id,omitempty"`
	Inline json.RawMessage `json:"inline,omitempty"`
}

// CompositionScorecard is the per-pack scoring wiring the builder authors
// directly. It maps onto scoring.EvaluationSpec's spec-level metadata plus its
// ScorecardDeclaration; dimensions reference validator/judge piece keys.
type CompositionScorecard struct {
	Name          string                         `json:"name,omitempty"`
	VersionNumber int32                          `json:"version_number,omitempty"`
	JudgeMode     scoring.JudgeMode              `json:"judge_mode,omitempty"`
	Strategy      scoring.ScoringStrategy        `json:"strategy,omitempty"`
	PassThreshold *float64                       `json:"pass_threshold,omitempty"`
	Dimensions    []scoring.DimensionDeclaration `json:"dimensions,omitempty"`
}

// ResolvedPieces maps a referenced challenge_pieces id to its definition JSON,
// supplied by the caller (the manager loads referenced pieces from the DB).
type ResolvedPieces map[uuid.UUID]json.RawMessage

// ReferencedPieceIDs returns the set of library piece ids the composition
// references, so the caller can resolve them before composing.
func (c Composition) ReferencedPieceIDs() []uuid.UUID {
	seen := make(map[uuid.UUID]struct{})
	var ids []uuid.UUID
	for _, group := range [][]PieceRef{c.Challenges, c.InputSets, c.Validators, c.Judges} {
		for _, ref := range group {
			if ref.RefID == nil {
				continue
			}
			if _, ok := seen[*ref.RefID]; ok {
				continue
			}
			seen[*ref.RefID] = struct{}{}
			ids = append(ids, *ref.RefID)
		}
	}
	return ids
}

// ComposeBundle assembles a Bundle from a composition and its resolved pieces.
// It is intentionally lenient — an incomplete draft produces an incomplete
// Bundle rather than an error — so callers can render a live spec card and a
// validation list for work-in-progress. Run ValidateBundle (or ManifestJSON)
// for the authoritative pass/fail.
func ComposeBundle(comp Composition, resolved ResolvedPieces) (Bundle, error) {
	bundle := Bundle{
		Pack: comp.Pack,
		Version: VersionMetadata{
			Number:        comp.Version.Number,
			ExecutionMode: comp.Version.ExecutionMode,
			ToolPolicy:    comp.Version.ToolPolicy,
			Sandbox:       comp.Version.Sandbox,
		},
	}
	if bundle.Version.Number == 0 {
		bundle.Version.Number = 1
	}
	if bundle.Version.ExecutionMode == "" {
		bundle.Version.ExecutionMode = ExecutionModeNative
	}

	for i, ref := range comp.Challenges {
		var challenge ChallengeDefinition
		if err := decodePiece(ref, resolved, &challenge); err != nil {
			return Bundle{}, fmt.Errorf("challenge piece %d: %w", i, err)
		}
		bundle.Challenges = append(bundle.Challenges, challenge)
	}

	for i, ref := range comp.InputSets {
		var inputSet InputSetDefinition
		if err := decodePiece(ref, resolved, &inputSet); err != nil {
			return Bundle{}, fmt.Errorf("input set piece %d: %w", i, err)
		}
		bundle.InputSets = append(bundle.InputSets, inputSet)
	}

	spec := scoring.EvaluationSpec{
		Name:          comp.Scorecard.Name,
		VersionNumber: comp.Scorecard.VersionNumber,
		JudgeMode:     comp.Scorecard.JudgeMode,
		Scorecard: scoring.ScorecardDeclaration{
			Dimensions:    comp.Scorecard.Dimensions,
			Strategy:      comp.Scorecard.Strategy,
			PassThreshold: comp.Scorecard.PassThreshold,
		},
	}

	for i, ref := range comp.Validators {
		var validator scoring.ValidatorDeclaration
		if err := decodePiece(ref, resolved, &validator); err != nil {
			return Bundle{}, fmt.Errorf("validator piece %d: %w", i, err)
		}
		spec.Validators = append(spec.Validators, validator)
	}

	for i, ref := range comp.Judges {
		var judge scoring.LLMJudgeDeclaration
		if err := decodePiece(ref, resolved, &judge); err != nil {
			return Bundle{}, fmt.Errorf("judge piece %d: %w", i, err)
		}
		spec.LLMJudges = append(spec.LLMJudges, judge)
	}

	if spec.Name == "" {
		spec.Name = bundle.Pack.Slug
	}
	if spec.VersionNumber == 0 {
		spec.VersionNumber = bundle.Version.Number
	}
	if spec.JudgeMode == "" {
		spec.JudgeMode = inferJudgeMode(spec)
	}

	bundle.Version.EvaluationSpec = spec
	return bundle, nil
}

// decodePiece resolves a piece reference to its definition JSON and decodes it
// into dst.
func decodePiece(ref PieceRef, resolved ResolvedPieces, dst any) error {
	raw, err := resolvePieceJSON(ref, resolved)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decode piece definition: %w", err)
	}
	return nil
}

func resolvePieceJSON(ref PieceRef, resolved ResolvedPieces) (json.RawMessage, error) {
	if ref.RefID != nil {
		raw, ok := resolved[*ref.RefID]
		if !ok || len(raw) == 0 {
			return nil, fmt.Errorf("referenced piece %s not found", ref.RefID)
		}
		return raw, nil
	}
	if len(ref.Inline) > 0 {
		return ref.Inline, nil
	}
	return nil, fmt.Errorf("piece reference has neither ref_id nor inline definition")
}

// inferJudgeMode picks a judge mode from what the spec actually declares, so
// builder authors don't have to set it by hand.
func inferJudgeMode(spec scoring.EvaluationSpec) scoring.JudgeMode {
	hasJudges := len(spec.LLMJudges) > 0
	hasDeterministic := len(spec.Validators) > 0 || len(spec.Metrics) > 0
	switch {
	case hasJudges && hasDeterministic:
		return scoring.JudgeModeHybrid
	case hasJudges:
		return scoring.JudgeModeLLMJudge
	default:
		return scoring.JudgeModeDeterministic
	}
}

// BundleYAML renders a composed Bundle as YAML for the advanced "edit YAML"
// escape hatch. It marshals via JSON first so the json tags used by
// scoring.EvaluationSpec drive the keys, then re-emits as YAML; the resulting
// document round-trips back through ParseYAML.
func BundleYAML(bundle Bundle) ([]byte, error) {
	encoded, err := json.Marshal(bundle)
	if err != nil {
		return nil, fmt.Errorf("marshal bundle: %w", err)
	}
	var generic any
	if err := json.Unmarshal(encoded, &generic); err != nil {
		return nil, fmt.Errorf("re-decode bundle: %w", err)
	}
	out, err := yaml.Marshal(generic)
	if err != nil {
		return nil, fmt.Errorf("encode bundle yaml: %w", err)
	}
	return out, nil
}
