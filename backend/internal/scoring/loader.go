package scoring

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type manifestEnvelope struct {
	EvaluationSpec json.RawMessage `json:"evaluation_spec"`
}

func LoadEvaluationSpec(manifest json.RawMessage) (EvaluationSpec, error) {
	if len(bytes.TrimSpace(manifest)) == 0 {
		return EvaluationSpec{}, ValidationErrors{{Field: "manifest", Message: "is required"}}
	}

	var envelope manifestEnvelope
	if err := json.Unmarshal(manifest, &envelope); err != nil {
		return EvaluationSpec{}, fmt.Errorf("decode challenge-pack manifest: %w", err)
	}
	if len(bytes.TrimSpace(envelope.EvaluationSpec)) == 0 {
		return EvaluationSpec{}, ValidationErrors{{Field: "evaluation_spec", Message: "is required"}}
	}

	var spec EvaluationSpec
	if err := json.Unmarshal(envelope.EvaluationSpec, &spec); err != nil {
		return EvaluationSpec{}, fmt.Errorf("decode evaluation_spec: %w", err)
	}

	normalizeEvaluationSpec(&spec)
	if err := ValidateEvaluationSpec(spec); err != nil {
		return EvaluationSpec{}, err
	}

	return spec, nil
}

func MarshalDefinition(spec EvaluationSpec) (json.RawMessage, error) {
	normalized := spec
	normalizeEvaluationSpec(&normalized)
	if err := ValidateEvaluationSpec(normalized); err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal evaluation spec: %w", err)
	}
	return encoded, nil
}
