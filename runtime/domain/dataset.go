package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidDatasetExampleStatus = errors.New("invalid dataset example status")
	ErrInvalidDatasetExampleSource = errors.New("invalid dataset example source")
	ErrInvalidDatasetInputSchema   = errors.New("invalid dataset input schema")
	ErrDatasetInputSchemaViolation = errors.New("dataset input does not match enforced schema")
)

type DatasetExampleStatus string

const (
	DatasetExampleStatusActive   DatasetExampleStatus = "active"
	DatasetExampleStatusArchived DatasetExampleStatus = "archived"
	DatasetExampleStatusMuted    DatasetExampleStatus = "muted"
)

var datasetExampleStatuses = map[DatasetExampleStatus]struct{}{
	DatasetExampleStatusActive:   {},
	DatasetExampleStatusArchived: {},
	DatasetExampleStatusMuted:    {},
}

func ParseDatasetExampleStatus(raw string) (DatasetExampleStatus, error) {
	status := DatasetExampleStatus(raw)
	if !status.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidDatasetExampleStatus, raw)
	}
	return status, nil
}

func (s DatasetExampleStatus) Valid() bool {
	_, ok := datasetExampleStatuses[s]
	return ok
}

type DatasetExampleSource string

const (
	DatasetExampleSourceManual    DatasetExampleSource = "manual"
	DatasetExampleSourceImport    DatasetExampleSource = "import"
	DatasetExampleSourceTrace     DatasetExampleSource = "trace"
	DatasetExampleSourceSynthetic DatasetExampleSource = "synthetic"
	DatasetExampleSourcePromotion DatasetExampleSource = "promotion"
)

var datasetExampleSources = map[DatasetExampleSource]struct{}{
	DatasetExampleSourceManual:    {},
	DatasetExampleSourceImport:    {},
	DatasetExampleSourceTrace:     {},
	DatasetExampleSourceSynthetic: {},
	DatasetExampleSourcePromotion: {},
}

func ParseDatasetExampleSource(raw string) (DatasetExampleSource, error) {
	source := DatasetExampleSource(raw)
	if !source.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidDatasetExampleSource, raw)
	}
	return source, nil
}

func (s DatasetExampleSource) Valid() bool {
	_, ok := datasetExampleSources[s]
	return ok
}

func ValidateDatasetInputAgainstSchema(schema, input json.RawMessage) error {
	if len(strings.TrimSpace(string(schema))) == 0 || string(schema) == "null" {
		return nil
	}

	var schemaObject map[string]any
	if err := json.Unmarshal(schema, &schemaObject); err != nil {
		return fmt.Errorf("%w: schema must be valid JSON", ErrInvalidDatasetInputSchema)
	}
	var inputValue any
	if err := json.Unmarshal(input, &inputValue); err != nil {
		return fmt.Errorf("%w: input must be valid JSON", ErrDatasetInputSchemaViolation)
	}

	return validateJSONSchemaValue(schemaObject, inputValue, "$")
}

func validateJSONSchemaValue(schema map[string]any, value any, path string) error {
	if rawType, ok := schema["type"].(string); ok {
		if !jsonValueMatchesType(value, rawType) {
			return fmt.Errorf("%w: %s must be %s", ErrDatasetInputSchemaViolation, path, rawType)
		}
	}

	if requiredRaw, ok := schema["required"].([]any); ok {
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: %s must be an object for required fields", ErrDatasetInputSchemaViolation, path)
		}
		for _, item := range requiredRaw {
			name, ok := item.(string)
			if !ok {
				return fmt.Errorf("%w: required entries must be strings", ErrInvalidDatasetInputSchema)
			}
			if _, exists := object[name]; !exists {
				return fmt.Errorf("%w: %s.%s is required", ErrDatasetInputSchemaViolation, path, name)
			}
		}
	}

	properties, hasProperties := schema["properties"].(map[string]any)
	if hasProperties {
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: %s must be an object", ErrDatasetInputSchemaViolation, path)
		}
		for name, propertySchemaRaw := range properties {
			propertyValue, exists := object[name]
			if !exists {
				continue
			}
			propertySchema, ok := propertySchemaRaw.(map[string]any)
			if !ok {
				return fmt.Errorf("%w: property %q must be an object", ErrInvalidDatasetInputSchema, name)
			}
			if err := validateJSONSchemaValue(propertySchema, propertyValue, path+"."+name); err != nil {
				return err
			}
		}
		if additional, ok := schema["additionalProperties"].(bool); ok && !additional {
			for name := range object {
				if _, exists := properties[name]; !exists {
					return fmt.Errorf("%w: %s.%s is not allowed", ErrDatasetInputSchemaViolation, path, name)
				}
			}
		}
	}

	return nil
}

func jsonValueMatchesType(value any, typ string) bool {
	switch typ {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok
	case "integer":
		n, ok := value.(float64)
		return ok && n == float64(int64(n))
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	default:
		return true
	}
}
