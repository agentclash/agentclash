package traces

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ImportFromPayload(platform SourcePlatform, data []byte) (ImportResult, error) {
	switch platform {
	case SourceOTel:
		return parseOTLPJSON(data)
	case SourceBraintrust, SourceLangSmith, SourcePhoenix:
		return parseVendorSpanExport(platform, data)
	default:
		return ImportResult{}, errUnsupportedPlatform(string(platform))
	}
}

func parseVendorSpanExport(platform SourcePlatform, data []byte) (ImportResult, error) {
	adapterResult, err := importVendorSpans(platform, data)
	if err != nil {
		return ImportResult{}, err
	}
	result := ImportResult{Platform: platform, Errors: make([]ParseError, 0, len(adapterResult.Errors))}
	for _, rowErr := range adapterResult.Errors {
		result.Errors = append(result.Errors, ParseError{Row: rowErr.Row, Field: rowErr.Field, Message: rowErr.Message})
	}
	for _, example := range adapterResult.Examples {
		traceID := ""
		if example.ExternalID != nil {
			traceID = strings.TrimSpace(*example.ExternalID)
		}
		result.Candidates = append(result.Candidates, Candidate{
			SourceTraceID: traceID,
			ExternalID:    example.ExternalID,
			Input:         example.Input,
			Output:        example.Expected,
			Expected:      example.Expected,
			Metadata:      example.Metadata,
			Tags:          example.Tags,
		})
	}
	return result, nil
}

func encodeJSON(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case json.RawMessage:
		return typed
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		if json.Valid([]byte(typed)) {
			return json.RawMessage(typed)
		}
		encoded, _ := json.Marshal(typed)
		return encoded
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		return encoded
	}
}

func candidateMetadata(fields map[string]any) json.RawMessage {
	if len(fields) == 0 {
		return json.RawMessage(`{}`)
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}

func parseError(row int, field, message string) ParseError {
	return ParseError{Row: row, Field: field, Message: message}
}

func importError(row int, err error) ParseError {
	return ParseError{Row: row, Message: err.Error()}
}

func requireObject(value json.RawMessage, row int, field string) (map[string]any, error) {
	if len(value) == 0 {
		return nil, fmt.Errorf("%s is required", field)
	}
	var decoded map[string]any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return nil, fmt.Errorf("%s must be valid JSON object", field)
	}
	return decoded, nil
}
