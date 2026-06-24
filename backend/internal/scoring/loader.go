package scoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type manifestEnvelope struct {
	EvaluationSpec json.RawMessage `json:"evaluation_spec"`
}

// StrictDecodeEvaluationSpec decodes an evaluation spec with strict
// unknown-field handling but WITHOUT running ValidateEvaluationSpec.
// Callers that need decode-only behaviour (e.g. the challenge-pack
// bundle loader, which runs validation later with more context) go
// through this helper so typos in user-authored JSON still fail loudly.
func StrictDecodeEvaluationSpec(raw json.RawMessage, dst *EvaluationSpec) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ValidationErrors{{Field: "evaluation_spec", Message: "is required"}}
	}
	if err := strictUnmarshal(raw, dst); err != nil {
		return fmt.Errorf("decode evaluation spec: %w", err)
	}
	return nil
}

// strictUnmarshal decodes JSON into dst with DisallowUnknownFields, and
// rejects trailing data after the first value. Every #147 entry point
// that loads a user-authored spec from JSON must go through this helper
// so a typo like "wieght" or "gait" fails loudly at spec-load time
// instead of silently running with default behaviour.
//
// This helper does NOT propagate through types that implement
// json.Unmarshaler (notably DimensionDeclaration) — those types opt out
// of the default decoder's field walk and must enforce strictness in
// their own UnmarshalJSON implementation.
func strictUnmarshal(data []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	// Refuse trailing data: the spec is exactly one JSON value, not a
	// stream. `io.EOF` is the success signal from Decode after the last
	// value is consumed.
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing data after JSON value")
		}
		return err
	}
	return nil
}

func LoadEvaluationSpec(manifest json.RawMessage) (EvaluationSpec, error) {
	if len(bytes.TrimSpace(manifest)) == 0 {
		return EvaluationSpec{}, ValidationErrors{{Field: "manifest", Message: "is required"}}
	}

	// The manifest envelope contains many top-level keys (pack, version,
	// challenges, input_sets, etc.) alongside evaluation_spec. Use standard
	// Unmarshal here so those extra fields are silently ignored. Strict
	// decoding is applied only to the inner evaluation_spec below.
	var envelope manifestEnvelope
	if err := json.Unmarshal(manifest, &envelope); err != nil {
		return EvaluationSpec{}, fmt.Errorf("decode challenge-pack manifest: %w", err)
	}
	if len(bytes.TrimSpace(envelope.EvaluationSpec)) == 0 {
		return EvaluationSpec{}, ValidationErrors{{Field: "evaluation_spec", Message: "is required"}}
	}

	var spec EvaluationSpec
	if err := strictUnmarshal(envelope.EvaluationSpec, &spec); err != nil {
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
