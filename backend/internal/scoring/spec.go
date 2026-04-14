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

	ValidatorTypeFuzzyMatch      ValidatorType = "fuzzy_match"
	ValidatorTypeNumericMatch    ValidatorType = "numeric_match"
	ValidatorTypeNormalizedMatch ValidatorType = "normalized_match"
)

type MetricType string

const (
	MetricTypeNumeric MetricType = "numeric"
	MetricTypeText    MetricType = "text"
	MetricTypeBoolean MetricType = "boolean"
)

type ScorecardDimension string

const (
	ScorecardDimensionCorrectness ScorecardDimension = "correctness"
	ScorecardDimensionReliability ScorecardDimension = "reliability"
	ScorecardDimensionLatency     ScorecardDimension = "latency"
	ScorecardDimensionCost        ScorecardDimension = "cost"
)

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
	Key          string          `json:"key"`
	Type         ValidatorType   `json:"type"`
	Target       string          `json:"target"`
	ExpectedFrom string          `json:"expected_from"`
	Config       json.RawMessage `json:"config,omitempty"`
}

type MetricDeclaration struct {
	Key       string     `json:"key"`
	Type      MetricType `json:"type"`
	Collector string     `json:"collector"`
	Unit      string     `json:"unit,omitempty"`
}

type ScorecardDeclaration struct {
	Dimensions    []ScorecardDimension   `json:"dimensions"`
	Normalization ScorecardNormalization `json:"normalization,omitempty"`
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
	case ValidatorTypeExactMatch, ValidatorTypeContains, ValidatorTypeRegexMatch, ValidatorTypeJSONSchema, ValidatorTypeJSONPathMatch, ValidatorTypeBooleanAssert,
		ValidatorTypeFuzzyMatch, ValidatorTypeNumericMatch, ValidatorTypeNormalizedMatch:
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

func (d ScorecardDimension) IsValid() bool {
	switch d {
	case ScorecardDimensionCorrectness, ScorecardDimensionReliability, ScorecardDimensionLatency, ScorecardDimensionCost:
		return true
	default:
		return false
	}
}
