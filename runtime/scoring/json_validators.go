package scoring

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

const (
	jsonSchemaDraft07          = "http://json-schema.org/draft-07/schema#"
	jsonSchemaDraft07HTTPS     = "https://json-schema.org/draft-07/schema#"
	jsonSchemaDraft202012      = "https://json-schema.org/draft/2020-12/schema"
	jsonPathComparatorEquals   = "equals"
	jsonPathComparatorContains = "contains"
	jsonPathComparatorGreater  = "greater_than"
	jsonPathComparatorLess     = "less_than"
	jsonPathComparatorExists   = "exists"
)

type validatorOutcome struct {
	verdict         string
	normalizedScore *float64
	reason          string
	evidence        map[string]any
}

type jsonPathExpectation struct {
	Path       string `json:"path"`
	Comparator string `json:"comparator"`
	Value      any    `json:"value"`
}

func validateJSONSchema(actual string, expected string) validatorOutcome {
	// google/jsonschema-go type-checks values by reflecting on the Go kind.
	// json.Number (from decoder.UseNumber) is a string alias, so the library
	// rejects integer values as type "string". Decode without UseNumber for
	// schema validation so integers land as float64 and match `type: integer`.
	actualDocument, err := parseJSONDocument(actual)
	if err != nil {
		return validatorError("parse actual JSON document", err, nil)
	}

	schemaValue, schemaDraft, err := parseJSONSchema(expected)
	if err != nil {
		return validatorError("parse JSON schema", err, nil)
	}

	resolved, err := schemaValue.Resolve(nil)
	if err != nil {
		return validatorError("resolve JSON schema", err, map[string]any{
			"schema_draft": schemaDraft,
		})
	}

	if err := resolved.Validate(actualDocument); err != nil {
		return validatorOutcome{
			verdict:         "fail",
			normalizedScore: floatPtr(0),
			reason:          "json schema validation failed",
			evidence: map[string]any{
				"schema_draft":      schemaDraft,
				"validation_error":  err.Error(),
				"validation_target": "actual_value",
			},
		}
	}

	return validatorOutcome{
		verdict:         "pass",
		normalizedScore: floatPtr(1),
		evidence: map[string]any{
			"schema_draft": schemaDraft,
		},
	}
}

func validateJSONPathMatch(actual string, expected string) validatorOutcome {
	actualDocument, err := parseJSONValue(actual)
	if err != nil {
		return validatorError("parse actual JSON document", err, nil)
	}

	expectation, err := parseJSONPathExpectation(expected)
	if err != nil {
		return validatorError("parse json_path_match expectation", err, nil)
	}

	actualValue, exists, err := extractJSONPathValue(actualDocument, expectation.Path)
	if err != nil {
		return validatorError("evaluate JSONPath expression", err, map[string]any{
			"path": expectation.Path,
		})
	}

	pass, reason, err := compareJSONPathValue(actualValue, exists, expectation)
	if err != nil {
		return validatorError("compare JSONPath value", err, map[string]any{
			"path":       expectation.Path,
			"comparator": expectation.Comparator,
			"actual":     actualValue,
			"exists":     exists,
		})
	}

	outcome := validatorOutcome{
		verdict:         "fail",
		normalizedScore: floatPtr(0),
		reason:          reason,
		evidence: map[string]any{
			"path":       expectation.Path,
			"comparator": expectation.Comparator,
			"expected":   expectation.Value,
			"actual":     actualValue,
			"exists":     exists,
		},
	}
	if pass {
		outcome.verdict = "pass"
		outcome.normalizedScore = floatPtr(1)
		outcome.reason = ""
	}
	return outcome
}

func validatorError(context string, err error, evidence map[string]any) validatorOutcome {
	if evidence == nil {
		evidence = map[string]any{}
	}
	evidence["error"] = err.Error()
	return validatorOutcome{
		verdict:  "error",
		reason:   fmt.Sprintf("%s: %v", context, err),
		evidence: evidence,
	}
}

func parseJSONValue(raw string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, fmt.Errorf("multiple JSON values are not allowed")
	}
	return value, nil
}

// parseJSONDocument decodes a JSON value with the default number handling
// (float64), suitable for feeding into google/jsonschema-go which type-checks
// via Go kinds. parseJSONValue above uses decoder.UseNumber() for lossless
// numeric comparisons in the JSONPath validator, but that path is wrong for
// schema validation because json.Number is a string alias.
func parseJSONDocument(raw string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, fmt.Errorf("multiple JSON values are not allowed")
	}
	return value, nil
}

func parseJSONSchema(raw string) (*jsonschema.Schema, string, error) {
	var schemaValue jsonschema.Schema
	if err := json.Unmarshal([]byte(raw), &schemaValue); err != nil {
		return nil, "", err
	}

	draft := schemaValue.Schema
	switch draft {
	case "", jsonSchemaDraft202012:
		if draft == "" {
			draft = jsonSchemaDraft202012
		}
	case jsonSchemaDraft07, jsonSchemaDraft07HTTPS:
		// jsonschema-go validates against 2020-12 only, so draft-07 schemas are
		// accepted only for the overlapping keyword subset used by current specs.
		schemaValue.Schema = ""
	default:
		return nil, draft, fmt.Errorf("unsupported JSON schema draft %q", draft)
	}

	return &schemaValue, draft, nil
}

func parseJSONPathExpectation(raw string) (jsonPathExpectation, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return jsonPathExpectation{}, fmt.Errorf("expectation is empty")
	}

	if strings.HasPrefix(trimmed, "$") {
		return jsonPathExpectation{
			Path:       trimmed,
			Comparator: jsonPathComparatorExists,
		}, nil
	}

	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.UseNumber()

	var expectation jsonPathExpectation
	if err := decoder.Decode(&expectation); err != nil {
		return jsonPathExpectation{}, err
	}
	if decoder.More() {
		return jsonPathExpectation{}, fmt.Errorf("multiple JSON values are not allowed")
	}
	if strings.TrimSpace(expectation.Path) == "" {
		return jsonPathExpectation{}, fmt.Errorf("path is required")
	}
	expectation.Path = strings.TrimSpace(expectation.Path)
	expectation.Comparator = strings.TrimSpace(expectation.Comparator)
	if expectation.Comparator == "" {
		if expectation.Value == nil {
			expectation.Comparator = jsonPathComparatorExists
		} else {
			expectation.Comparator = jsonPathComparatorEquals
		}
	}

	switch expectation.Comparator {
	case jsonPathComparatorEquals, jsonPathComparatorContains, jsonPathComparatorGreater, jsonPathComparatorLess, jsonPathComparatorExists:
		return expectation, nil
	default:
		return jsonPathExpectation{}, fmt.Errorf("unsupported comparator %q", expectation.Comparator)
	}
}

func compareJSONPathValue(actual any, exists bool, expectation jsonPathExpectation) (bool, string, error) {
	if expectation.Comparator == jsonPathComparatorExists {
		if exists {
			return true, "", nil
		}
		return false, "json path did not resolve to a value", nil
	}
	if !exists {
		return false, "json path did not resolve to a value", nil
	}

	switch expectation.Comparator {
	case jsonPathComparatorEquals:
		if jsonValuesEqual(actual, expectation.Value) {
			return true, "", nil
		}
		return false, "json path value did not equal expected value", nil
	case jsonPathComparatorContains:
		return jsonValueContains(actual, expectation.Value)
	case jsonPathComparatorGreater:
		actualNumber, ok := anyNumber(actual)
		if !ok {
			return false, "", fmt.Errorf("actual value is not numeric")
		}
		expectedNumber, ok := anyNumber(expectation.Value)
		if !ok {
			return false, "", fmt.Errorf("expected value is not numeric")
		}
		if actualNumber > expectedNumber {
			return true, "", nil
		}
		return false, "json path value was not greater than expected value", nil
	case jsonPathComparatorLess:
		actualNumber, ok := anyNumber(actual)
		if !ok {
			return false, "", fmt.Errorf("actual value is not numeric")
		}
		expectedNumber, ok := anyNumber(expectation.Value)
		if !ok {
			return false, "", fmt.Errorf("expected value is not numeric")
		}
		if actualNumber < expectedNumber {
			return true, "", nil
		}
		return false, "json path value was not less than expected value", nil
	default:
		return false, "", fmt.Errorf("unsupported comparator %q", expectation.Comparator)
	}
}

func jsonValueContains(actual any, expected any) (bool, string, error) {
	switch typed := actual.(type) {
	case string:
		expectedText, ok := expected.(string)
		if !ok {
			return false, "", fmt.Errorf("expected value must be a string for contains comparator")
		}
		if strings.Contains(typed, expectedText) {
			return true, "", nil
		}
		return false, "json path value did not contain expected substring", nil
	case []any:
		for _, item := range typed {
			if jsonValuesEqual(item, expected) {
				return true, "", nil
			}
		}
		return false, "json path array did not contain expected value", nil
	default:
		actualText := stringifyJSONValue(actual)
		expectedText := stringifyJSONValue(expected)
		if strings.Contains(actualText, expectedText) {
			return true, "", nil
		}
		return false, "json path value did not contain expected value", nil
	}
}

func jsonValuesEqual(left any, right any) bool {
	if leftNumber, ok := anyNumber(left); ok {
		rightNumber, rightOK := anyNumber(right)
		return rightOK && leftNumber == rightNumber
	}
	return reflect.DeepEqual(normalizeJSONValue(left), normalizeJSONValue(right))
}

func normalizeJSONValue(value any) any {
	switch typed := value.(type) {
	case json.Number:
		if strings.ContainsAny(string(typed), ".eE") {
			if parsed, err := typed.Float64(); err == nil {
				return parsed
			}
		}
		if parsed, err := typed.Int64(); err == nil {
			return parsed
		}
		if parsed, err := typed.Float64(); err == nil {
			return parsed
		}
		return string(typed)
	case []any:
		normalized := make([]any, 0, len(typed))
		for _, item := range typed {
			normalized = append(normalized, normalizeJSONValue(item))
		}
		return normalized
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, item := range typed {
			normalized[key] = normalizeJSONValue(item)
		}
		return normalized
	default:
		return typed
	}
}

func stringifyJSONValue(value any) string {
	if value == nil {
		return "null"
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(payload)
}

func extractJSONPathValue(document any, path string) (any, bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false, fmt.Errorf("path is empty")
	}
	if path[0] != '$' {
		return nil, false, fmt.Errorf("path must start with '$'")
	}

	current := document
	index := 1
	for index < len(path) {
		switch path[index] {
		case '.':
			index++
			start := index
			for index < len(path) && isJSONPathIdentifierChar(path[index]) {
				index++
			}
			if start == index {
				return nil, false, fmt.Errorf("missing property name at position %d", start)
			}
			name := path[start:index]
			object, ok := current.(map[string]any)
			if !ok {
				return nil, false, nil
			}
			next, ok := object[name]
			if !ok {
				return nil, false, nil
			}
			current = next
		case '[':
			closeIndex, token, err := parseJSONPathBracket(path, index)
			if err != nil {
				return nil, false, err
			}
			index = closeIndex + 1

			if token.index != nil {
				array, ok := current.([]any)
				if !ok {
					return nil, false, nil
				}
				// Negative indices are intentionally unsupported in this subset.
				if *token.index < 0 || *token.index >= len(array) {
					return nil, false, nil
				}
				current = array[*token.index]
				continue
			}

			object, ok := current.(map[string]any)
			if !ok {
				return nil, false, nil
			}
			next, ok := object[token.property]
			if !ok {
				return nil, false, nil
			}
			current = next
		default:
			return nil, false, fmt.Errorf("unexpected token %q at position %d", path[index], index)
		}
	}

	return current, true, nil
}

type jsonPathBracketToken struct {
	property string
	index    *int
}

func parseJSONPathBracket(path string, start int) (int, jsonPathBracketToken, error) {
	end := start + 1
	for end < len(path) && path[end] != ']' {
		end++
	}
	if end >= len(path) {
		return 0, jsonPathBracketToken{}, fmt.Errorf("unterminated bracket expression at position %d", start)
	}

	content := strings.TrimSpace(path[start+1 : end])
	if content == "" {
		return 0, jsonPathBracketToken{}, fmt.Errorf("empty bracket expression at position %d", start)
	}

	if content[0] == '\'' || content[0] == '"' {
		unquoted, err := strconv.Unquote(content)
		if err != nil {
			return 0, jsonPathBracketToken{}, fmt.Errorf("invalid quoted property %q: %w", content, err)
		}
		return end, jsonPathBracketToken{property: unquoted}, nil
	}

	parsedIndex, err := strconv.Atoi(content)
	if err == nil {
		return end, jsonPathBracketToken{index: &parsedIndex}, nil
	}

	return end, jsonPathBracketToken{property: content}, nil
}

func isJSONPathIdentifierChar(ch byte) bool {
	return ch == '_' || ch == '-' ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9')
}

func mergeEvidence(base map[string]any, extra map[string]any) map[string]any {
	if len(extra) == 0 {
		return base
	}
	merged := make(map[string]any, len(base)+len(extra))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		if _, exists := merged[key]; exists {
			merged["evidence_"+key] = value
			continue
		}
		merged[key] = value
	}
	return merged
}
