package scoring

import "encoding/json"

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
)

type DimensionSource string

const (
	DimensionSourceValidators  DimensionSource = "validators"
	DimensionSourceMetric      DimensionSource = "metric"
	DimensionSourceReliability DimensionSource = "reliability"
	DimensionSourceLatency     DimensionSource = "latency"
	DimensionSourceCost        DimensionSource = "cost"
)

// ScoringStrategy controls how per-dimension scores combine into a single
// overall score and pass/fail verdict.
//
//   - weighted: weighted average of available dimension scores; passed is true
//     iff every gated dimension (if any) clears its pass_threshold.
//   - binary:   every dimension is an implicit gate; overall score is 1.0 iff
//     all dims clear their pass_threshold, else 0.0.
//   - hybrid:   weighted average like weighted, but any gate failure forces
//     overall score to 0 and passed to false.
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
func (d *DimensionDeclaration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		d.Key = s
		return nil
	}

	type Alias DimensionDeclaration
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
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
	RuntimeLimits RuntimeLimits          `json:"runtime_limits,omitempty"`
	Pricing       PricingConfig          `json:"pricing,omitempty"`
	Scorecard     ScorecardDeclaration   `json:"scorecard"`
}

type ValidatorDeclaration struct {
	Key          string        `json:"key"`
	Type         ValidatorType `json:"type"`
	Target       string        `json:"target"`
	ExpectedFrom string        `json:"expected_from"`
}

type MetricDeclaration struct {
	Key       string     `json:"key"`
	Type      MetricType `json:"type"`
	Collector string     `json:"collector"`
	Unit      string     `json:"unit,omitempty"`
}

type ScorecardDeclaration struct {
	Dimensions    []DimensionDeclaration `json:"dimensions"`
	Normalization ScorecardNormalization `json:"normalization,omitempty"`
	Strategy      ScoringStrategy        `json:"strategy,omitempty"`
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
	case ValidatorTypeExactMatch, ValidatorTypeContains, ValidatorTypeRegexMatch, ValidatorTypeJSONSchema, ValidatorTypeJSONPathMatch, ValidatorTypeBooleanAssert:
		return true
	default:
		return false
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
	case DimensionSourceValidators, DimensionSourceMetric, DimensionSourceReliability, DimensionSourceLatency, DimensionSourceCost:
		return true
	default:
		return false
	}
}

// isBuiltinDimensionKey returns true for the four legacy dimension names that
// have built-in scoring logic. Used during auto-expansion of old-format specs.
func isBuiltinDimensionKey(key string) bool {
	switch key {
	case ScorecardDimensionCorrectness, ScorecardDimensionReliability, ScorecardDimensionLatency, ScorecardDimensionCost:
		return true
	default:
		return false
	}
}
