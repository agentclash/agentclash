package scoring

import (
	"bytes"
	"encoding/json"
)

type JudgeMode string

const (
	JudgeModeDeterministic JudgeMode = "deterministic"
	JudgeModeLLMJudge      JudgeMode = "llm_judge"
	JudgeModeHybrid        JudgeMode = "hybrid"
)

type ValidatorType string

const (
	ValidatorTypeExactMatch    ValidatorType = "exact_match"
	ValidatorTypeContains      ValidatorType = "contains"
	ValidatorTypeRegexMatch    ValidatorType = "regex_match"
	ValidatorTypeJSONSchema    ValidatorType = "json_schema"
	ValidatorTypeJSONPathMatch ValidatorType = "json_path_match"
	ValidatorTypeBooleanAssert ValidatorType = "boolean_assert"

	ValidatorTypeFuzzyMatch      ValidatorType = "fuzzy_match"
	ValidatorTypeNumericMatch    ValidatorType = "numeric_match"
	ValidatorTypeNormalizedMatch ValidatorType = "normalized_match"
	ValidatorTypeMathEquivalence ValidatorType = "math_equivalence"

	ValidatorTypeFileContentMatch   ValidatorType = "file_content_match"
	ValidatorTypeFileExists         ValidatorType = "file_exists"
	ValidatorTypeFileJSONSchema     ValidatorType = "file_json_schema"
	ValidatorTypeDirectoryStructure ValidatorType = "directory_structure"
	ValidatorTypeCodeExecution      ValidatorType = "code_execution"
)

type MetricType string

const (
	MetricTypeNumeric MetricType = "numeric"
	MetricTypeText    MetricType = "text"
	MetricTypeBoolean MetricType = "boolean"
)

// ScorecardDimension is a string alias kept for backward compatibility with
// code that references the four built-in dimension key constants.
type ScorecardDimension = string

const (
	ScorecardDimensionCorrectness ScorecardDimension = "correctness"
	ScorecardDimensionReliability ScorecardDimension = "reliability"
	ScorecardDimensionLatency     ScorecardDimension = "latency"
	ScorecardDimensionCost        ScorecardDimension = "cost"
	ScorecardDimensionBehavioral  ScorecardDimension = "behavioral"
)

// DimensionSource names the evidence pipeline that produces a dimension's
// score. One name is intentionally absent from this list:
//
//   - "composite": considered and rejected. A dimension that reads other
//     dimensions would require topological ordering and make scoring
//     non-deterministic under partial failures. If you need an aggregate,
//     compute it in computeOverallScore via strategy/weights instead.
//
// DimensionSourceLLMJudge routes to aggregated LLM-as-judge results produced
// by the workflow scoring path. The legacy EvaluateRunAgent entrypoint remains
// deterministic-only for older callers; the workflow now uses
// EvaluateRunAgentWithLLMJudgeResults after executing the declared judges.
type DimensionSource string

const (
	DimensionSourceValidators  DimensionSource = "validators"
	DimensionSourceMetric      DimensionSource = "metric"
	DimensionSourceReliability DimensionSource = "reliability"
	DimensionSourceLatency     DimensionSource = "latency"
	DimensionSourceCost        DimensionSource = "cost"
	DimensionSourceBehavioral  DimensionSource = "behavioral"
	DimensionSourceLLMJudge    DimensionSource = "llm_judge"
)

type BehavioralSignalKey string

const (
	BehavioralSignalRecoveryBehavior      BehavioralSignalKey = "recovery_behavior"
	BehavioralSignalExplorationEfficiency BehavioralSignalKey = "exploration_efficiency"
	BehavioralSignalErrorCascade          BehavioralSignalKey = "error_cascade"
	BehavioralSignalScopeAdherence        BehavioralSignalKey = "scope_adherence"
	BehavioralSignalConfidenceCalibration BehavioralSignalKey = "confidence_calibration"
)

// ScoringStrategy controls how per-dimension scores combine into a single
// overall score and pass/fail verdict.
//
//   - weighted: weighted average of available dimension scores; passed is true
//     iff every gated dimension (if any) clears its pass_threshold.
//   - binary:   every dimension is an implicit gate; overall score is 1.0 iff
//     all dims clear their pass_threshold, else 0.0.
//   - hybrid:   gates must pass AND the weighted average of NON-GATE
//     dimensions must clear the scorecard-level pass_threshold. A gate
//     failure short-circuits to overall=0 and passed=false; gates are
//     intentionally excluded from the weighted mean so a barely-passing
//     gate can't drag the score down.
//
// DEVIATION from issue #147: the issue's weighted example shows no gate
// fields, implying gates are a hybrid-only feature. This implementation
// permits `gate: true` on weighted-strategy dims as well — a gated
// dimension is still checked, and a gate failure forces passed=false
// while the weighted average still computes over ALL dims (gates
// included). This is more permissive than the issue text but has been
// in production since Phase 2 and removing it now would break existing
// specs. For clean gate semantics, prefer hybrid.
type ScoringStrategy string

const (
	ScoringStrategyWeighted ScoringStrategy = "weighted"
	ScoringStrategyBinary   ScoringStrategy = "binary"
	ScoringStrategyHybrid   ScoringStrategy = "hybrid"
)

func (s ScoringStrategy) IsValid() bool {
	switch s {
	case ScoringStrategyWeighted, ScoringStrategyBinary, ScoringStrategyHybrid:
		return true
	default:
		return false
	}
}

// DimensionDeclaration describes a single scoring dimension. It supports both
// the old string format ("correctness") and the new object format with explicit
// source routing. When unmarshalled from a plain string, only Key is populated;
// normalizeEvaluationSpec expands the rest for known built-in keys.
type DimensionDeclaration struct {
	Key             string                  `json:"key"`
	Source          DimensionSource         `json:"source"`
	Validators      []string                `json:"validators,omitempty"`
	Metric          string                  `json:"metric,omitempty"`
	BetterDirection string                  `json:"better_direction,omitempty"`
	Normalization   *DimensionNormalization `json:"normalization,omitempty"`
	Weight          *float64                `json:"weight,omitempty"`
	// JudgeKey references a single LLMJudgeDeclaration whose aggregated score
	// feeds this dimension. Required when Source is llm_judge; must be empty
	// otherwise. The 1:1 mapping is deliberate — packs that want multiple
	// judge-backed dims declare one judge per dim instead of sharing judges
	// across dims, which keeps normalization and multi-sample variance
	// attached to a single scoring axis.
	JudgeKey string `json:"judge_key,omitempty"`
	// Gate marks a dimension as a hard pass/fail requirement. In the hybrid
	// strategy a gate failure forces overall=0 and passed=false. In the binary
	// strategy every dimension is implicitly gated regardless of this flag.
	Gate bool `json:"gate,omitempty"`
	// PassThreshold is the score (0..1) a dimension must meet to pass its gate.
	// Required when Gate is true or when strategy is binary.
	PassThreshold *float64 `json:"pass_threshold,omitempty"`
}

// UnmarshalJSON handles both the legacy string format ("correctness") and
// the new object format ({ "key": "correctness", "source": "validators", ... }).
//
// The object path uses DisallowUnknownFields because a custom Unmarshaler
// opts out of the outer decoder's strict walk — without this, a spec like
// `{"key":"correctness","wieght":0.5}` would silently discard the typo and
// run with weight=nil. strictUnmarshal surfaces the misspelling at
// spec-load time instead.
func (d *DimensionDeclaration) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		d.Key = s
		return nil
	}

	type Alias DimensionDeclaration
	var alias Alias
	if err := strictUnmarshal(data, &alias); err != nil {
		return err
	}
	*d = DimensionDeclaration(alias)
	return nil
}

// DimensionNormalization configures linear normalization for a dimension.
// Target is the ideal value (score=1.0), Max is the worst-case boundary (score=0.0).
type DimensionNormalization struct {
	Target *float64 `json:"target,omitempty"`
	Max    *float64 `json:"max,omitempty"`
}

type EvaluationSpec struct {
	Name          string                 `json:"name"`
	VersionNumber int32                  `json:"version_number"`
	JudgeMode     JudgeMode              `json:"judge_mode"`
	Validators    []ValidatorDeclaration `json:"validators"`
	Metrics       []MetricDeclaration    `json:"metrics"`
	Behavioral    *BehavioralConfig      `json:"behavioral,omitempty"`
	// LLMJudges declares LLM-as-judge graders that run after deterministic
	// validator/metric evaluation and feed llm_judge-backed scorecard dims.
	LLMJudges           []LLMJudgeDeclaration `json:"llm_judges,omitempty"`
	PostExecutionChecks []PostExecutionCheck  `json:"post_execution_checks,omitempty"`
	RuntimeLimits       RuntimeLimits         `json:"runtime_limits,omitempty"`
	Pricing             PricingConfig         `json:"pricing,omitempty"`
	Scorecard           ScorecardDeclaration  `json:"scorecard"`
}

type ValidatorDeclaration struct {
	Key          string          `json:"key"`
	Type         ValidatorType   `json:"type"`
	Target       string          `json:"target"`
	ExpectedFrom string          `json:"expected_from,omitempty"`
	Config       json.RawMessage `json:"config,omitempty"`
}

type MetricDeclaration struct {
	Key       string     `json:"key"`
	Type      MetricType `json:"type"`
	Collector string     `json:"collector"`
	Unit      string     `json:"unit,omitempty"`
}

type BehavioralConfig struct {
	Signals []BehavioralSignalDeclaration `json:"signals"`
}

type BehavioralSignalDeclaration struct {
	Key           BehavioralSignalKey `json:"key"`
	Weight        float64             `json:"weight"`
	Gate          bool                `json:"gate,omitempty"`
	PassThreshold *float64            `json:"pass_threshold,omitempty"`
}

type ScorecardDeclaration struct {
	Dimensions    []DimensionDeclaration `json:"dimensions"`
	Normalization ScorecardNormalization `json:"normalization,omitempty"`
	Strategy      ScoringStrategy        `json:"strategy,omitempty"`
	// PassThreshold is the minimum overall score (0..1) an agent must clear
	// for the scorecard-level pass verdict. It stacks with per-dimension gates:
	// gates still have to pass, and the overall score has to clear this bar.
	//
	//   - weighted: optional. When set, passed is true iff the weighted average
	//     clears the threshold. When unset, passed defaults to "no gate failed".
	//   - hybrid:   optional. When set, passed requires gates-pass AND overall
	//     >= threshold.
	//   - binary:   MUST be nil. Binary derives pass/fail purely from per-dim
	//     gates; a scorecard-level threshold is ambiguous there and is rejected
	//     during validation to prevent silent footguns.
	//
	// Comparisons are inclusive — an overall score exactly equal to the
	// threshold passes, matching the release-gate convention.
	PassThreshold *float64 `json:"pass_threshold,omitempty"`
	// JudgeLimits bounds per-run cost and sample count for LLM-as-judge
	// evaluation. Ignored when the spec declares no LLMJudges. See
	// JudgeLimits doc comment for enforcement semantics.
	JudgeLimits *JudgeLimits `json:"judge_limits,omitempty"`
}

// JudgeLimits caps the blast radius of LLM-as-judge evaluation for a single
// run. The evaluator enforces these cumulatively across every judge × sample
// × model in the spec; when a cap trips, remaining samples are marked
// unable_to_judge and the dimension falls through to OutputStateUnavailable.
//
// These are user-facing spec knobs. Hard Go-code ceilings (see
// JudgeMaxSamplesCeiling) still apply on top — packs cannot raise the
// ceilings by rewriting their spec.
type JudgeLimits struct {
	// MaxSamplesPerJudge overrides the per-judge Samples cap. Clamped to
	// [0, JudgeMaxSamplesCeiling] at validation time; 0 means "use the
	// per-judge default".
	MaxSamplesPerJudge int `json:"max_samples_per_judge,omitempty"`
	// MaxCallsUSD is the cumulative per-run budget for judge LLM calls.
	// 0 means unbounded (subject to RuntimeLimits.MaxCostUSD, which covers
	// agent spend; judge cost is accounted separately on purpose — see
	// Q7 in the #148 analysis).
	MaxCallsUSD float64 `json:"max_calls_usd,omitempty"`
	// MaxTokens is the cumulative per-run token budget for judge LLM
	// calls. 0 means unbounded.
	MaxTokens int64 `json:"max_tokens,omitempty"`
}

type RuntimeLimits struct {
	MaxTotalTokens *int64   `json:"max_total_tokens,omitempty"`
	MaxCostUSD     *float64 `json:"max_cost_usd,omitempty"`
	MaxDurationMS  *int64   `json:"max_duration_ms,omitempty"`
}

type PricingConfig struct {
	Models []ModelPricing `json:"models,omitempty"`
}

type ModelPricing struct {
	ProviderKey                string  `json:"provider_key"`
	ProviderModelID            string  `json:"provider_model_id"`
	InputCostPerMillionTokens  float64 `json:"input_cost_per_million_tokens"`
	OutputCostPerMillionTokens float64 `json:"output_cost_per_million_tokens"`
}

// ScorecardNormalization is the legacy normalization block. Kept for backward
// compatibility with specs that declare normalization at the scorecard level.
// normalizeEvaluationSpec copies these into per-dimension normalization.
type ScorecardNormalization struct {
	Latency *LatencyNormalization `json:"latency,omitempty"`
	Cost    *CostNormalization    `json:"cost,omitempty"`
}

type LatencyNormalization struct {
	TargetMS *float64 `json:"target_ms,omitempty"`
	MaxMS    *float64 `json:"max_ms,omitempty"`
}

type CostNormalization struct {
	TargetUSD *float64 `json:"target_usd,omitempty"`
	MaxUSD    *float64 `json:"max_usd,omitempty"`
}

func (m JudgeMode) IsValid() bool {
	switch m {
	case JudgeModeDeterministic, JudgeModeLLMJudge, JudgeModeHybrid:
		return true
	default:
		return false
	}
}

func (t ValidatorType) IsValid() bool {
	switch t {
	case ValidatorTypeExactMatch, ValidatorTypeContains, ValidatorTypeRegexMatch,
		ValidatorTypeJSONSchema, ValidatorTypeJSONPathMatch, ValidatorTypeBooleanAssert,
		ValidatorTypeFuzzyMatch, ValidatorTypeNumericMatch, ValidatorTypeNormalizedMatch,
		ValidatorTypeMathEquivalence,
		ValidatorTypeFileContentMatch, ValidatorTypeFileExists,
		ValidatorTypeFileJSONSchema, ValidatorTypeDirectoryStructure,
		ValidatorTypeCodeExecution:
		return true
	default:
		return false
	}
}

// IsFileValidator returns true for validator types that rely on sandbox file
// targets rather than only the agent's final output.
func (t ValidatorType) IsFileValidator() bool {
	switch t {
	case ValidatorTypeFileContentMatch, ValidatorTypeFileExists,
		ValidatorTypeFileJSONSchema, ValidatorTypeDirectoryStructure,
		ValidatorTypeCodeExecution:
		return true
	default:
		return false
	}
}

// RequiresExpectedFrom returns true for validator types that need a non-empty
// expected_from field. File validators that use config-only expectations
// (file_exists, file_json_schema, directory_structure) return false.
func (t ValidatorType) RequiresExpectedFrom() bool {
	switch t {
	case ValidatorTypeFileExists, ValidatorTypeFileJSONSchema, ValidatorTypeDirectoryStructure, ValidatorTypeCodeExecution:
		return false
	default:
		return true
	}
}

func (t MetricType) IsValid() bool {
	switch t {
	case MetricTypeNumeric, MetricTypeText, MetricTypeBoolean:
		return true
	default:
		return false
	}
}

func (s DimensionSource) IsValid() bool {
	switch s {
	case DimensionSourceValidators, DimensionSourceMetric, DimensionSourceReliability, DimensionSourceLatency, DimensionSourceCost, DimensionSourceBehavioral, DimensionSourceLLMJudge:
		return true
	default:
		return false
	}
}

func (k BehavioralSignalKey) IsValid() bool {
	switch k {
	case BehavioralSignalRecoveryBehavior,
		BehavioralSignalExplorationEfficiency,
		BehavioralSignalErrorCascade,
		BehavioralSignalScopeAdherence,
		BehavioralSignalConfidenceCalibration:
		return true
	default:
		return false
	}
}

// isBuiltinDimensionKey returns true for the four legacy dimension names that
// have built-in scoring logic. Used during auto-expansion of old-format specs.
func isBuiltinDimensionKey(key string) bool {
	switch key {
	case ScorecardDimensionCorrectness, ScorecardDimensionReliability, ScorecardDimensionLatency, ScorecardDimensionCost, ScorecardDimensionBehavioral:
		return true
	default:
		return false
	}
}

// --- LLM-as-judge types ---
//
// The type surface here deliberately covers all 5 grader methods from the
// issue under a single LLMJudgeDeclaration struct. Mode selects the
// dispatch; Model/Models and Samples are orthogonal fan-out controls that
// compose with every mode. See backend/.claude/analysis/issue-148-deep-
// analysis.md Part 3 for the architectural rationale.
//
// These declarations validate, round-trip, and drive the workflow-side judge
// evaluator that persists aggregated llm_judge_results for scorecard use.

// JudgeMaxSamplesCeiling is the hard upper bound on LLMJudgeDeclaration.Samples.
// Enforced in Go code, not config — a malicious or broken pack cannot request
// more than this many samples regardless of what it writes in its spec. Every
// judge × every sample × every consensus model is one LLM call, so the
// ceiling is a load-bearing cost-attack guard.
const JudgeMaxSamplesCeiling = 10

// JudgeDefaultSamples is the Samples value used when a judge omits it.
// Matches the default in Anthropic's eval strategy ("run each judge ~3 times
// and take the median") and gives enough signal for variance tracking
// without burning budget on low-value calls.
const JudgeDefaultSamples = 3

// JudgeMethodMode selects which grader method a judge implements.
//
//   - rubric     — numeric score against a structured rubric (Anthropic's
//     "rubric-based scoring")
//   - assertion  — yes/no answer to a natural-language claim about the
//     output (Anthropic's "natural language assertions")
//   - n_wise     — rank all N agents in a run simultaneously with optional
//     position debiasing (generalized pairwise comparison)
//   - reference  — rubric scored relative to a gold-standard reference
//     answer resolved from the challenge input
type JudgeMethodMode string

const (
	JudgeMethodRubric    JudgeMethodMode = "rubric"
	JudgeMethodAssertion JudgeMethodMode = "assertion"
	JudgeMethodNWise     JudgeMethodMode = "n_wise"
	JudgeMethodReference JudgeMethodMode = "reference"
)

// IsValid reports whether mode is a recognised grader method.
func (m JudgeMethodMode) IsValid() bool {
	switch m {
	case JudgeMethodRubric, JudgeMethodAssertion, JudgeMethodNWise, JudgeMethodReference:
		return true
	default:
		return false
	}
}

// IsNumeric reports whether mode produces a per-agent numeric score that
// can feed a DimensionSourceLLMJudge-sourced dimension. All four modes
// currently qualify — assertion normalizes its pass/fail majority to
// 0.0 or 1.0; n_wise normalizes its Borda count to [0, 1].
func (m JudgeMethodMode) IsNumeric() bool {
	switch m {
	case JudgeMethodRubric, JudgeMethodReference, JudgeMethodNWise, JudgeMethodAssertion:
		return true
	default:
		return false
	}
}

// IsBooleanScope reports whether mode naturally produces a boolean-per-sample
// verdict. Only assertion qualifies today. Consensus aggregations
// majority_vote and unanimous are restricted to boolean-scope modes at
// validation time so packs can't accidentally mean-average a yes/no.
func (m JudgeMethodMode) IsBooleanScope() bool {
	return m == JudgeMethodAssertion
}

// ConsensusAggregation controls how per-model scores combine when a judge
// declares multiple Models. Orthogonal to the per-model multi-sample
// aggregation, which is always median for numeric modes and majority for
// assertion mode.
type ConsensusAggregation string

const (
	ConsensusAggMedian       ConsensusAggregation = "median"
	ConsensusAggMean         ConsensusAggregation = "mean"
	ConsensusAggMajorityVote ConsensusAggregation = "majority_vote"
	ConsensusAggUnanimous    ConsensusAggregation = "unanimous"
)

// IsValid reports whether c is a recognised aggregation strategy.
func (c ConsensusAggregation) IsValid() bool {
	switch c {
	case ConsensusAggMedian, ConsensusAggMean, ConsensusAggMajorityVote, ConsensusAggUnanimous:
		return true
	default:
		return false
	}
}

// ScoreScale describes the numeric range a rubric or reference judge is
// instructed to score on. The evaluator normalizes the raw score into [0, 1]
// before it feeds the dimension:
//
//	normalized := (raw - Min) / (Max - Min)
//
// Min must be strictly less than Max. Defaults to 1..5 at normalization
// time when omitted, matching Anthropic's example rubrics.
type ScoreScale struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// ConsensusConfig parameterises cross-model aggregation when an
// LLMJudgeDeclaration supplies multiple Models. Required whenever
// len(Models) > 1.
type ConsensusConfig struct {
	Aggregation ConsensusAggregation `json:"aggregation"`
	// MinAgreementThreshold is the max allowed spread between per-model
	// scores before the judge result is flagged low-confidence. Only
	// meaningful for numeric aggregations. Range [0, 1].
	MinAgreementThreshold float64 `json:"min_agreement_threshold,omitempty"`
	// FlagOnDisagreement emits a warning on the judge result whenever the
	// observed spread exceeds MinAgreementThreshold, even when the
	// aggregation itself succeeds.
	FlagOnDisagreement bool `json:"flag_on_disagreement,omitempty"`
}

// LLMJudgeDeclaration is the unified shape for every grader method. A
// single struct with a Mode discriminator matches the issue's wire-format
// intent (one JSON array of judge entries) and keeps spec authoring
// symmetric with Validators and Metrics.
//
// Mode-specific fields (Rubric, Assertion, Prompt, ReferenceFrom) are all
// optional at the type level and validated for presence by
// validateLLMJudges per mode.
type LLMJudgeDeclaration struct {
	Mode JudgeMethodMode `json:"mode"`
	Key  string          `json:"key"`

	// Fan-out (orthogonal to mode)
	//
	// Exactly one of Model or Models must be set (enforced in validation).
	// Samples controls how many times each model judges; 0 normalizes to
	// JudgeDefaultSamples.
	Model   string   `json:"model,omitempty"`
	Models  []string `json:"models,omitempty"`
	Samples int      `json:"samples,omitempty"`

	// ContextFrom lists evidence references that the evaluator substitutes
	// into the prompt envelope. Every entry must pass
	// isSupportedEvidenceReference.
	ContextFrom []string `json:"context_from,omitempty"`

	// OutputSchema is an optional JSON Schema (draft-07 or 2020-12) used
	// to validate parsed judge responses. When nil, the evaluator uses a
	// default schema per mode.
	OutputSchema json.RawMessage `json:"output_schema,omitempty"`

	// ScoreScale, when provided on rubric/reference modes, overrides the
	// default 1..5 normalization range.
	ScoreScale *ScoreScale `json:"score_scale,omitempty"`

	// Rubric text for rubric and reference modes. Required for both.
	Rubric string `json:"rubric,omitempty"`

	// Assertion text for assertion mode. Required when Mode=assertion.
	Assertion string `json:"assertion,omitempty"`
	// Expect flips the "pass" polarity of an assertion. Nil means
	// "expect true". Only meaningful when Mode=assertion.
	Expect *bool `json:"expect,omitempty"`

	// Prompt is the cross-agent ranking prompt for n_wise mode.
	Prompt string `json:"prompt,omitempty"`
	// PositionDebiasing enables cyclic-shift ordering across samples so no
	// agent is consistently shown in the same position.
	PositionDebiasing bool `json:"position_debiasing,omitempty"`

	// ReferenceFrom names an evidence reference that resolves to the
	// gold-standard answer for reference mode. Must pass
	// isSupportedEvidenceReference.
	ReferenceFrom string `json:"reference_from,omitempty"`

	// Consensus parameterises multi-model aggregation. Required when
	// len(Models) > 1.
	Consensus *ConsensusConfig `json:"consensus,omitempty"`

	// AntiGamingClauses is additional pack-authored safety language
	// appended to the prompt envelope. The evaluator ALWAYS injects its
	// own default anti-gaming clauses regardless of what packs declare
	// here — this field is additive, not replacement.
	AntiGamingClauses []string `json:"anti_gaming_clauses,omitempty"`

	// TimeoutMS is the per-judge kill switch. 0 means "use evaluator
	// default." Never exceeds the enclosing activity timeout.
	TimeoutMS *int64 `json:"timeout_ms,omitempty"`
}
