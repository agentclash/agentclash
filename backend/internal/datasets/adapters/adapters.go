package adapters

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

type Format string

const (
	FormatOpenAI     Format = "openai"
	FormatBraintrust Format = "braintrust"
	FormatLangSmith  Format = "langsmith"
	FormatPhoenix    Format = "phoenix"
	FormatJSONL      Format = "jsonl"
	FormatCSV        Format = "csv"
)

type Mapping struct {
	InputKeys    []string `json:"input_keys,omitempty"`
	OutputKeys   []string `json:"output_keys,omitempty"`
	MetadataKeys []string `json:"metadata_keys,omitempty"`
	TagsKey      string   `json:"tags_key,omitempty"`
	IDKey        string   `json:"id_key,omitempty"`
	ExampleIDKey string   `json:"example_id_key,omitempty"`
}

type Example struct {
	ExternalID *string         `json:"external_id,omitempty"`
	Input      json.RawMessage `json:"input"`
	Expected   json.RawMessage `json:"expected,omitempty"`
	Metadata   json.RawMessage `json:"metadata"`
	Tags       []string        `json:"tags,omitempty"`
}

type RowError struct {
	Row     int    `json:"row"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type ImportResult struct {
	Format   Format     `json:"format"`
	Examples []Example  `json:"examples"`
	Errors   []RowError `json:"errors,omitempty"`
}

func Import(formatHint string, data []byte, mapping Mapping) (ImportResult, error) {
	format, err := normalizeFormat(formatHint, data)
	if err != nil {
		return ImportResult{}, err
	}
	switch format {
	case FormatCSV:
		return importCSV(data, mapping)
	case FormatOpenAI, FormatBraintrust, FormatLangSmith, FormatPhoenix, FormatJSONL:
		return importJSONL(format, data, mapping)
	default:
		return ImportResult{}, fmt.Errorf("unsupported dataset format %q", format)
	}
}

func Export(formatHint string, examples []Example) ([]byte, string, error) {
	format, err := normalizeFormat(formatHint, nil)
	if err != nil {
		return nil, "", err
	}
	switch format {
	case FormatCSV:
		data, err := exportCSV(examples)
		return data, "text/csv", err
	case FormatOpenAI, FormatBraintrust, FormatLangSmith, FormatPhoenix, FormatJSONL:
		data, err := exportJSONL(format, examples)
		return data, "application/x-ndjson", err
	default:
		return nil, "", fmt.Errorf("unsupported dataset format %q", format)
	}
}

func normalizeFormat(formatHint string, data []byte) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(formatHint))) {
	case "":
		return sniffFormat(data), nil
	case FormatOpenAI:
		return FormatOpenAI, nil
	case FormatBraintrust:
		return FormatBraintrust, nil
	case FormatLangSmith:
		return FormatLangSmith, nil
	case FormatPhoenix:
		return FormatPhoenix, nil
	case FormatJSONL, "generic":
		return FormatJSONL, nil
	case FormatCSV:
		return FormatCSV, nil
	default:
		return "", fmt.Errorf("unsupported dataset format %q", formatHint)
	}
}

func sniffFormat(data []byte) Format {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return FormatJSONL
	}
	if bytes.Contains(bytes.SplitN(trimmed, []byte("\n"), 2)[0], []byte(",")) && !bytes.HasPrefix(trimmed, []byte("{")) {
		return FormatCSV
	}
	return FormatJSONL
}

func importJSONL(format Format, data []byte, mapping Mapping) (ImportResult, error) {
	var result ImportResult
	result.Format = format
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	row := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		row++
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			result.Errors = append(result.Errors, RowError{Row: row, Message: "row must be valid JSON"})
			continue
		}
		example, rowErrors := normalizeJSONRow(format, raw, mapping, row)
		if len(rowErrors) > 0 {
			result.Errors = append(result.Errors, rowErrors...)
			continue
		}
		result.Examples = append(result.Examples, example)
	}
	if err := scanner.Err(); err != nil {
		return ImportResult{}, err
	}
	return result, nil
}

func normalizeJSONRow(format Format, raw map[string]any, mapping Mapping, row int) (Example, []RowError) {
	if hasFieldMapping(mapping) {
		return normalizeMappedRow(raw, mapping, row)
	}
	switch format {
	case FormatOpenAI:
		if item, ok := raw["item"]; ok {
			return buildExample(raw, item, firstPresent(raw, "ideal", "expected"), raw["metadata"], raw["tags"], externalIDFrom(raw, "external_id", "id"), row)
		}
		return buildExample(raw, raw["input"], firstPresent(raw, "ideal", "expected"), raw["metadata"], raw["tags"], externalIDFrom(raw, "external_id", "id"), row)
	case FormatBraintrust:
		return buildExample(raw, raw["input"], raw["expected"], raw["metadata"], raw["tags"], externalIDFrom(raw, "external_id", "id"), row)
	case FormatLangSmith:
		return buildExample(raw, raw["inputs"], raw["outputs"], firstPresent(raw, "metadata", "run_metadata"), raw["tags"], externalIDFrom(raw, "external_id", "id"), row)
	case FormatPhoenix:
		id := externalIDFrom(raw, "external_id", "id", "example_id")
		if mapping.ExampleIDKey != "" {
			id = externalIDFromNested(raw, mapping.ExampleIDKey)
		}
		return buildExample(raw, raw["input"], raw["output"], raw["metadata"], raw["tags"], id, row)
	default:
		return normalizeMappedRow(raw, Mapping{InputKeys: []string{"input"}, OutputKeys: []string{"expected"}, MetadataKeys: []string{"metadata"}, TagsKey: "tags", IDKey: "external_id"}, row)
	}
}

func normalizeMappedRow(raw map[string]any, mapping Mapping, row int) (Example, []RowError) {
	input := selectMappedValue(raw, mapping.InputKeys)
	if input == nil {
		input = raw["input"]
	}
	expected := selectMappedValue(raw, mapping.OutputKeys)
	metadata := selectMappedValue(raw, mapping.MetadataKeys)
	tags := raw[mapping.TagsKey]
	id := externalIDFrom(raw, mapping.IDKey)
	if id == nil && mapping.ExampleIDKey != "" {
		id = externalIDFromNested(raw, mapping.ExampleIDKey)
	}
	return buildExample(raw, input, expected, metadata, tags, id, row)
}

func importCSV(data []byte, mapping Mapping) (ImportResult, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.TrimLeadingSpace = true
	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return ImportResult{Format: FormatCSV}, nil
		}
		return ImportResult{}, err
	}
	index := make(map[string]int, len(header))
	for i, name := range header {
		index[strings.TrimSpace(name)] = i
	}
	var result ImportResult
	result.Format = FormatCSV
	row := 1
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		row++
		if err != nil {
			result.Errors = append(result.Errors, RowError{Row: row, Message: err.Error()})
			continue
		}
		raw := make(map[string]any, len(index))
		for name, i := range index {
			if i < len(record) {
				raw[name] = decodeCell(record[i])
			}
		}
		example, rowErrors := normalizeMappedRow(raw, mappingWithDefaults(mapping), row)
		if len(rowErrors) > 0 {
			result.Errors = append(result.Errors, rowErrors...)
			continue
		}
		result.Examples = append(result.Examples, example)
	}
	return result, nil
}

func mappingWithDefaults(mapping Mapping) Mapping {
	if len(mapping.InputKeys) == 0 {
		mapping.InputKeys = []string{"input"}
	}
	if len(mapping.OutputKeys) == 0 {
		mapping.OutputKeys = []string{"expected", "output", "outputs"}
	}
	if len(mapping.MetadataKeys) == 0 {
		mapping.MetadataKeys = []string{"metadata"}
	}
	if mapping.TagsKey == "" {
		mapping.TagsKey = "tags"
	}
	if mapping.IDKey == "" {
		mapping.IDKey = "external_id"
	}
	return mapping
}

func buildExample(raw map[string]any, input, expected, metadata, tags any, externalID *string, row int) (Example, []RowError) {
	if input == nil {
		return Example{}, []RowError{{Row: row, Field: "input", Message: "input is required"}}
	}
	inputJSON, err := marshalCanonical(input)
	if err != nil {
		return Example{}, []RowError{{Row: row, Field: "input", Message: err.Error()}}
	}
	expectedJSON, err := marshalOptionalCanonical(expected)
	if err != nil {
		return Example{}, []RowError{{Row: row, Field: "expected", Message: err.Error()}}
	}
	metadataJSON, err := marshalMetadata(metadata)
	if err != nil {
		return Example{}, []RowError{{Row: row, Field: "metadata", Message: err.Error()}}
	}
	return Example{ExternalID: externalID, Input: inputJSON, Expected: expectedJSON, Metadata: metadataJSON, Tags: normalizeTags(tags, raw)}, nil
}

func exportJSONL(format Format, examples []Example) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	for _, example := range examples {
		row, err := exportJSONRow(format, example)
		if err != nil {
			return nil, err
		}
		if err := encoder.Encode(row); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func exportJSONRow(format Format, example Example) (map[string]any, error) {
	input, err := rawToAny(example.Input)
	if err != nil {
		return nil, err
	}
	expected, err := rawToOptionalAny(example.Expected)
	if err != nil {
		return nil, err
	}
	metadata, err := rawToAny(defaultJSONObject(example.Metadata))
	if err != nil {
		return nil, err
	}
	row := map[string]any{}
	switch format {
	case FormatOpenAI:
		row["input"] = input
		if expected != nil {
			row["ideal"] = expected
		}
	case FormatBraintrust:
		row["input"] = input
		if expected != nil {
			row["expected"] = expected
		}
		row["metadata"] = metadata
		row["tags"] = example.Tags
	case FormatLangSmith:
		row["inputs"] = input
		if expected != nil {
			row["outputs"] = expected
		}
		row["metadata"] = metadata
		row["tags"] = example.Tags
	case FormatPhoenix:
		row["input"] = input
		if expected != nil {
			row["output"] = expected
		}
		row["metadata"] = metadata
	case FormatJSONL:
		row["input"] = input
		if expected != nil {
			row["expected"] = expected
		}
		row["metadata"] = metadata
		row["tags"] = example.Tags
	}
	if example.ExternalID != nil {
		row["external_id"] = *example.ExternalID
	}
	return row, nil
}

func exportCSV(examples []Example) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"external_id", "input", "expected", "metadata", "tags"}); err != nil {
		return nil, err
	}
	for _, example := range examples {
		externalID := ""
		if example.ExternalID != nil {
			externalID = *example.ExternalID
		}
		if err := writer.Write([]string{
			externalID,
			string(example.Input),
			string(example.Expected),
			string(defaultJSONObject(example.Metadata)),
			strings.Join(example.Tags, ","),
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buf.Bytes(), writer.Error()
}

func hasMapping(mapping Mapping) bool {
	return len(mapping.InputKeys) > 0 || len(mapping.OutputKeys) > 0 || len(mapping.MetadataKeys) > 0 || mapping.TagsKey != "" || mapping.IDKey != "" || mapping.ExampleIDKey != ""
}

func hasFieldMapping(mapping Mapping) bool {
	return len(mapping.InputKeys) > 0 || len(mapping.OutputKeys) > 0 || len(mapping.MetadataKeys) > 0 || mapping.TagsKey != "" || mapping.IDKey != ""
}

func selectMappedValue(raw map[string]any, keys []string) any {
	if len(keys) == 0 {
		return nil
	}
	if len(keys) == 1 {
		return valueAtPath(raw, keys[0])
	}
	out := make(map[string]any)
	for _, key := range keys {
		if value := valueAtPath(raw, key); value != nil {
			out[lastPathSegment(key)] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func firstPresent(raw map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			return value
		}
	}
	return nil
}

func externalIDFrom(raw map[string]any, keys ...string) *string {
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if value := valueAtPath(raw, key); value != nil {
			if id := stringify(value); id != "" {
				return &id
			}
		}
	}
	return nil
}

func externalIDFromNested(raw map[string]any, key string) *string {
	return externalIDFrom(raw, key, "metadata."+key)
}

func valueAtPath(raw map[string]any, path string) any {
	if path == "" {
		return nil
	}
	parts := strings.Split(path, ".")
	var current any = raw
	for _, part := range parts {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = object[part]
		if !ok {
			return nil
		}
	}
	return current
}

func lastPathSegment(path string) string {
	parts := strings.Split(path, ".")
	return parts[len(parts)-1]
}

func marshalCanonical(value any) (json.RawMessage, error) {
	if raw, ok := value.(json.RawMessage); ok {
		return raw, nil
	}
	data, err := json.Marshal(value)
	return json.RawMessage(data), err
}

func marshalOptionalCanonical(value any) (json.RawMessage, error) {
	if value == nil {
		return nil, nil
	}
	return marshalCanonical(value)
}

func marshalMetadata(value any) (json.RawMessage, error) {
	if value == nil {
		return json.RawMessage(`{}`), nil
	}
	return marshalCanonical(value)
}

func defaultJSONObject(raw json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return json.RawMessage(`{}`)
	}
	return raw
}

func rawToAny(raw json.RawMessage) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func rawToOptionalAny(raw json.RawMessage) (any, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	return rawToAny(raw)
}

func normalizeTags(value any, raw map[string]any) []string {
	tags := tagsFromValue(value)
	if len(tags) == 0 {
		tags = tagsFromValue(raw["metadata.tags"])
	}
	sort.Strings(tags)
	return tags
}

func tagsFromValue(value any) []string {
	switch v := value.(type) {
	case []string:
		return compactStrings(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s := stringify(item); s != "" {
				out = append(out, s)
			}
		}
		return compactStrings(out)
	case string:
		return compactStrings(strings.Split(v, ","))
	default:
		return nil
	}
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func stringify(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case float64:
		return strings.TrimSpace(strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", v), "0"), "."))
	case nil:
		return ""
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(data)
	}
}

func decodeCell(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	var decoded any
	if json.Unmarshal([]byte(trimmed), &decoded) == nil {
		return decoded
	}
	return trimmed
}
