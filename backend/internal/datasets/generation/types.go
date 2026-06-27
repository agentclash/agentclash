package generation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const StrategySelfInstruct = "self_instruct"
const StrategyAgenticSelfInstruct = "agentic_self_instruct"

const (
	SolverModeJudgeOnly      = "judge_only"
	SolverModeDirectProvider = "direct_provider"
)

var ErrUnsupportedStrategy = errors.New("unsupported dataset generation strategy")

// ValidationError marks an error as invalid user-supplied generation input. The
// API layer maps it to an HTTP 400 validation_error response. Using a typed
// error (rather than substring matching on the message) keeps wrapped database
// or provider failures from being misreported to callers as bad input.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// NewValidationError builds a ValidationError with a formatted message.
func NewValidationError(format string, args ...any) error {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

type JobConfig struct {
	ProviderAccountID uuid.UUID `json:"provider_account_id"`
	// Model is the provider model id to generate with (e.g. "gpt-4.1-mini"),
	// chosen directly from the provider connection's live model list.
	Model                   string          `json:"model"`
	SeedsTag                string          `json:"seeds_tag,omitempty"`
	CreateVersion           bool            `json:"create_version,omitempty"`
	VersionLabel            string          `json:"version_label,omitempty"`
	JudgeProviderAccountID  *uuid.UUID      `json:"judge_provider_account_id,omitempty"`
	JudgeModel              string          `json:"judge_model,omitempty"`
	MaxRoundsPerExample     int             `json:"max_rounds_per_example,omitempty"`
	AcceptanceMode          string          `json:"acceptance_mode,omitempty"`
	MinGap                  *float64        `json:"min_gap,omitempty"`
	MaxWeakScore            *float64        `json:"max_weak_score,omitempty"`
	MinStrongScore          *float64        `json:"min_strong_score,omitempty"`
	SolverMode              string          `json:"solver_mode,omitempty"`
	WeakProviderAccountID   *uuid.UUID      `json:"weak_provider_account_id,omitempty"`
	WeakModel               string          `json:"weak_model,omitempty"`
	StrongProviderAccountID *uuid.UUID      `json:"strong_provider_account_id,omitempty"`
	StrongModel             string          `json:"strong_model,omitempty"`
	WeakRollouts            int             `json:"weak_rollouts,omitempty"`
	StrongRollouts          int             `json:"strong_rollouts,omitempty"`
	WeakDeploymentID        *uuid.UUID      `json:"weak_deployment_id,omitempty"`
	StrongDeploymentID      *uuid.UUID      `json:"strong_deployment_id,omitempty"`
	ChallengePackVersionID  *uuid.UUID      `json:"challenge_pack_version_id,omitempty"`
	ChallengeKey            string          `json:"challenge_key,omitempty"`
	FieldMapping            json.RawMessage `json:"field_mapping,omitempty"`
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
	ReasonSolverError     = "solver_error"
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
		return JobConfig{}, NewValidationError("generation job config is required")
	}
	var cfg JobConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return JobConfig{}, err
	}
	if cfg.ProviderAccountID == uuid.Nil {
		return JobConfig{}, NewValidationError("provider_account_id is required")
	}
	if cfg.Model == "" {
		return JobConfig{}, NewValidationError("model is required")
	}
	parsedStrategy, err := ParseStrategy(strategy)
	if err != nil {
		return JobConfig{}, err
	}
	if parsedStrategy == StrategyAgenticSelfInstruct {
		if cfg.JudgeProviderAccountID == nil || *cfg.JudgeProviderAccountID == uuid.Nil {
			return JobConfig{}, NewValidationError("judge_provider_account_id is required for agentic_self_instruct")
		}
		if cfg.JudgeModel == "" {
			return JobConfig{}, NewValidationError("judge_model is required for agentic_self_instruct")
		}
		if cfg.MaxRoundsPerExample < 0 || cfg.MaxRoundsPerExample > 15 {
			return JobConfig{}, NewValidationError("max_rounds_per_example must be between 0 and 15")
		}
		if cfg.AcceptanceMode != "" && cfg.AcceptanceMode != AcceptanceModeJudge && cfg.AcceptanceMode != AcceptanceModeThreshold {
			return JobConfig{}, NewValidationError("acceptance_mode must be judge or threshold")
		}
		if cfg.AcceptanceMode == AcceptanceModeThreshold {
			if cfg.MinGap == nil || cfg.MaxWeakScore == nil || cfg.MinStrongScore == nil {
				return JobConfig{}, NewValidationError("min_gap, max_weak_score, and min_strong_score are all required when acceptance_mode is threshold")
			}
		}
		cfg.SolverMode = NormalizeAgenticSolverMode(cfg.SolverMode)
		switch cfg.SolverMode {
		case SolverModeJudgeOnly:
			if err := validateOptionalRolloutCount("weak_rollouts", cfg.WeakRollouts); err != nil {
				return JobConfig{}, err
			}
			if err := validateOptionalRolloutCount("strong_rollouts", cfg.StrongRollouts); err != nil {
				return JobConfig{}, err
			}
		case SolverModeDirectProvider:
			if cfg.WeakProviderAccountID == nil || *cfg.WeakProviderAccountID == uuid.Nil {
				return JobConfig{}, NewValidationError("weak_provider_account_id is required when solver_mode is direct_provider")
			}
			if cfg.WeakModel == "" {
				return JobConfig{}, NewValidationError("weak_model is required when solver_mode is direct_provider")
			}
			if cfg.StrongProviderAccountID == nil || *cfg.StrongProviderAccountID == uuid.Nil {
				return JobConfig{}, NewValidationError("strong_provider_account_id is required when solver_mode is direct_provider")
			}
			if cfg.StrongModel == "" {
				return JobConfig{}, NewValidationError("strong_model is required when solver_mode is direct_provider")
			}
			if cfg.WeakRollouts == 0 {
				cfg.WeakRollouts = 1
			}
			if cfg.StrongRollouts == 0 {
				cfg.StrongRollouts = 1
			}
			if err := validateRequiredRolloutCount("weak_rollouts", cfg.WeakRollouts); err != nil {
				return JobConfig{}, err
			}
			if err := validateRequiredRolloutCount("strong_rollouts", cfg.StrongRollouts); err != nil {
				return JobConfig{}, err
			}
		default:
			return JobConfig{}, NewValidationError("solver_mode must be judge_only or direct_provider")
		}
		if HasAgenticDeploymentContext(cfg) {
			if cfg.WeakDeploymentID == nil || *cfg.WeakDeploymentID == uuid.Nil {
				return JobConfig{}, NewValidationError("weak_deployment_id is required when deployment context is provided")
			}
			if cfg.StrongDeploymentID == nil || *cfg.StrongDeploymentID == uuid.Nil {
				return JobConfig{}, NewValidationError("strong_deployment_id is required when deployment context is provided")
			}
			if cfg.ChallengePackVersionID == nil || *cfg.ChallengePackVersionID == uuid.Nil {
				return JobConfig{}, NewValidationError("challenge_pack_version_id is required when deployment context is provided")
			}
			if strings.TrimSpace(cfg.ChallengeKey) == "" {
				return JobConfig{}, NewValidationError("challenge_key is required when deployment context is provided")
			}
			if len(cfg.FieldMapping) > 0 {
				if !json.Valid(cfg.FieldMapping) {
					return JobConfig{}, NewValidationError("field_mapping must be valid JSON")
				}
				var fieldMapping map[string]any
				if err := json.Unmarshal(cfg.FieldMapping, &fieldMapping); err != nil || fieldMapping == nil {
					return JobConfig{}, NewValidationError("field_mapping must be a JSON object")
				}
			}
		}
	}
	return cfg, nil
}

func NormalizeAgenticSolverMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return SolverModeJudgeOnly
	}
	return mode
}

func HasAgenticDeploymentContext(cfg JobConfig) bool {
	return cfg.WeakDeploymentID != nil ||
		cfg.StrongDeploymentID != nil ||
		cfg.ChallengePackVersionID != nil ||
		strings.TrimSpace(cfg.ChallengeKey) != "" ||
		len(cfg.FieldMapping) > 0
}

func validateOptionalRolloutCount(name string, value int) error {
	if value == 0 {
		return nil
	}
	return validateRequiredRolloutCount(name, value)
}

func validateRequiredRolloutCount(name string, value int) error {
	if value < 1 || value > 5 {
		return NewValidationError("%s must be between 1 and 5", name)
	}
	return nil
}
