package generation

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

const StrategySelfInstruct = "self_instruct"
const StrategyAgenticSelfInstruct = "agentic_self_instruct"

var ErrUnsupportedStrategy = errors.New("unsupported dataset generation strategy")

type JobConfig struct {
	ProviderAccountID uuid.UUID `json:"provider_account_id"`
	// Model is the provider model id to generate with (e.g. "gpt-4.1-mini"),
	// chosen directly from the provider connection's live model list.
	Model                  string     `json:"model"`
	SeedsTag               string     `json:"seeds_tag,omitempty"`
	CreateVersion          bool       `json:"create_version,omitempty"`
	VersionLabel           string     `json:"version_label,omitempty"`
	JudgeProviderAccountID *uuid.UUID `json:"judge_provider_account_id,omitempty"`
	JudgeModel             string     `json:"judge_model,omitempty"`
	MaxRoundsPerExample    int        `json:"max_rounds_per_example,omitempty"`
	AcceptanceMode         string     `json:"acceptance_mode,omitempty"`
	MinGap                 *float64   `json:"min_gap,omitempty"`
	MaxWeakScore           *float64   `json:"max_weak_score,omitempty"`
	MinStrongScore         *float64   `json:"min_strong_score,omitempty"`
}

type SeedExample struct {
	Input    json.RawMessage `json:"input"`
	Expected json.RawMessage `json:"expected,omitempty"`
}

type Candidate struct {
	Input    json.RawMessage `json:"input"`
	Expected json.RawMessage `json:"expected,omitempty"`
}

const (
	ReasonParseError      = "parse_error"
	ReasonSchemaViolation = "schema_violation"
	ReasonDuplicateInput  = "duplicate_input"
	ReasonProviderError   = "provider_error"
	ReasonJudgeParseError = "judge_parse_error"
	ReasonQualityRejected = "quality_rejected"
)

func ParseStrategy(raw string) (string, error) {
	switch raw {
	case StrategySelfInstruct, "self-instruct":
		return StrategySelfInstruct, nil
	case StrategyAgenticSelfInstruct, "agentic-self-instruct":
		return StrategyAgenticSelfInstruct, nil
	default:
		return "", ErrUnsupportedStrategy
	}
}

func DecodeJobConfig(raw json.RawMessage) (JobConfig, error) {
	return DecodeJobConfigForStrategy(raw, StrategySelfInstruct)
}

func DecodeJobConfigForStrategy(raw json.RawMessage, strategy string) (JobConfig, error) {
	if len(raw) == 0 {
		return JobConfig{}, errors.New("generation job config is required")
	}
	var cfg JobConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return JobConfig{}, err
	}
	if cfg.ProviderAccountID == uuid.Nil {
		return JobConfig{}, errors.New("provider_account_id is required")
	}
	if cfg.Model == "" {
		return JobConfig{}, errors.New("model is required")
	}
	parsedStrategy, err := ParseStrategy(strategy)
	if err != nil {
		return JobConfig{}, err
	}
	if parsedStrategy == StrategyAgenticSelfInstruct {
		if cfg.JudgeProviderAccountID == nil || *cfg.JudgeProviderAccountID == uuid.Nil {
			return JobConfig{}, errors.New("judge_provider_account_id is required for agentic_self_instruct")
		}
		if cfg.JudgeModel == "" {
			return JobConfig{}, errors.New("judge_model is required for agentic_self_instruct")
		}
		if cfg.MaxRoundsPerExample < 0 || cfg.MaxRoundsPerExample > 15 {
			return JobConfig{}, errors.New("max_rounds_per_example must be between 0 and 15")
		}
		if cfg.AcceptanceMode != "" && cfg.AcceptanceMode != AcceptanceModeJudge && cfg.AcceptanceMode != AcceptanceModeThreshold {
			return JobConfig{}, errors.New("acceptance_mode must be judge or threshold")
		}
		if cfg.AcceptanceMode == AcceptanceModeThreshold {
			if cfg.MinGap == nil || cfg.MaxWeakScore == nil || cfg.MinStrongScore == nil {
				return JobConfig{}, errors.New("min_gap, max_weak_score, and min_strong_score are all required when acceptance_mode is threshold")
			}
		}
	}
	return cfg, nil
}
