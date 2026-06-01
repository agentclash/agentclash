package generation

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

const StrategySelfInstruct = "self_instruct"

var ErrUnsupportedStrategy = errors.New("unsupported dataset generation strategy")

type JobConfig struct {
	ProviderAccountID uuid.UUID `json:"provider_account_id"`
	ModelAliasID      uuid.UUID `json:"model_alias_id"`
	SeedsTag          string    `json:"seeds_tag,omitempty"`
	CreateVersion     bool      `json:"create_version,omitempty"`
	VersionLabel      string    `json:"version_label,omitempty"`
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
)

func ParseStrategy(raw string) (string, error) {
	switch raw {
	case StrategySelfInstruct, "self-instruct":
		return StrategySelfInstruct, nil
	default:
		return "", ErrUnsupportedStrategy
	}
}

func DecodeJobConfig(raw json.RawMessage) (JobConfig, error) {
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
	if cfg.ModelAliasID == uuid.Nil {
		return JobConfig{}, errors.New("model_alias_id is required")
	}
	return cfg, nil
}
