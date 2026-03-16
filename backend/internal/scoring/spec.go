package scoring

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
	Dimensions []ScorecardDimension `json:"dimensions"`
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

func (d ScorecardDimension) IsValid() bool {
	switch d {
	case ScorecardDimensionCorrectness, ScorecardDimensionReliability, ScorecardDimensionLatency, ScorecardDimensionCost:
		return true
	default:
		return false
	}
}
