package traces

import (
	"encoding/json"
	"fmt"
	"strings"
)

type otlpDocument struct {
	ResourceSpans []otlpResourceSpan `json:"resourceSpans"`
}

type otlpResourceSpan struct {
	ScopeSpans []otlpScopeSpan `json:"scopeSpans"`
}

type otlpScopeSpan struct {
	Spans []otlpSpan `json:"spans"`
}

type otlpSpan struct {
	TraceID    string         `json:"traceId"`
	SpanID     string         `json:"spanId"`
	Name       string         `json:"name"`
	Attributes []otlpAttribute `json:"attributes"`
}

type otlpAttribute struct {
	Key   string      `json:"key"`
	Value otlpAnyValue `json:"value"`
}

type otlpAnyValue struct {
	StringValue *string         `json:"stringValue"`
	DoubleValue *float64        `json:"doubleValue"`
	IntValue    *json.Number    `json:"intValue"`
	BoolValue   *bool           `json:"boolValue"`
	ArrayValue  *otlpArrayValue `json:"arrayValue"`
}

type otlpArrayValue struct {
	Values []otlpAnyValue `json:"values"`
}

func parseOTLPJSON(data []byte) (ImportResult, error) {
	var doc otlpDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return ImportResult{}, fmt.Errorf("parse otlp json: %w", err)
	}
	result := ImportResult{Platform: SourceOTel}
	row := 0
	for _, resource := range doc.ResourceSpans {
		for _, scope := range resource.ScopeSpans {
			for _, span := range scope.Spans {
				row++
				candidate, err := candidateFromOTLPSpan(span)
				if err != nil {
					result.Errors = append(result.Errors, parseError(row, "span", err.Error()))
					continue
				}
				result.Candidates = append(result.Candidates, candidate)
			}
		}
	}
	if len(result.Candidates) == 0 && len(result.Errors) == 0 {
		result.Errors = append(result.Errors, parseError(0, "resourceSpans", "no spans found"))
	}
	return result, nil
}

func candidateFromOTLPSpan(span otlpSpan) (Candidate, error) {
	attrs := attributeMap(span.Attributes)
	inputRaw := firstNonEmptyAttr(attrs,
		"gen_ai.input.messages",
		"gen_ai.prompt",
		"input",
	)
	outputRaw := firstNonEmptyAttr(attrs,
		"gen_ai.output.messages",
		"gen_ai.completion",
		"output",
	)
	if inputRaw == "" && outputRaw == "" {
		return Candidate{}, fmt.Errorf("span %q missing gen_ai input/output attributes", span.SpanID)
	}
	traceID := strings.TrimSpace(span.TraceID)
	if traceID == "" {
		traceID = strings.TrimSpace(span.SpanID)
	}
	metadata := map[string]any{
		"span_name": span.Name,
	}
	copyGenAIMetadata(attrs, metadata)
	input := encodeJSON(parseJSONish(inputRaw))
	output := encodeJSON(parseJSONish(outputRaw))
	return Candidate{
		SourceTraceID: traceID,
		ExternalID:    optionalString(traceID),
		Input:         input,
		Output:        output,
		Expected:      output,
		Metadata:      candidateMetadata(metadata),
	}, nil
}

func attributeMap(attrs []otlpAttribute) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		out[attr.Key] = anyValueString(attr.Value)
	}
	return out
}

func anyValueString(value otlpAnyValue) string {
	switch {
	case value.StringValue != nil:
		return strings.TrimSpace(*value.StringValue)
	case value.DoubleValue != nil:
		return fmt.Sprintf("%v", *value.DoubleValue)
	case value.IntValue != nil:
		return value.IntValue.String()
	case value.BoolValue != nil:
		return fmt.Sprintf("%t", *value.BoolValue)
	default:
		return ""
	}
}

func firstNonEmptyAttr(attrs map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(attrs[key]); value != "" {
			return value
		}
	}
	return ""
}

func copyGenAIMetadata(attrs map[string]string, metadata map[string]any) {
	for key, value := range attrs {
		if !strings.HasPrefix(key, "gen_ai.") {
			continue
		}
		switch key {
		case "gen_ai.input.messages", "gen_ai.output.messages", "gen_ai.prompt", "gen_ai.completion":
			continue
		}
		metadata[key] = parseJSONish(value)
	}
}

func parseJSONish(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if json.Valid([]byte(raw)) {
		var decoded any
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			return decoded
		}
	}
	return raw
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
