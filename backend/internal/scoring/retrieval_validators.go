package scoring

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	defaultRetrievedChunksPath = "$.retrieved_chunks"
	defaultRetrievalPassAt     = 1.0
)

type retrievalValidatorConfig struct {
	RetrievedChunksPath string    `json:"retrieved_chunks_path"`
	ExpectedIDsPath     string    `json:"expected_ids_path"`
	IDFields            *[]string `json:"id_fields"`
	K                   *int      `json:"k"`
	PassAt              *float64  `json:"pass_at"`
}

type retrievalChunkEvidence struct {
	Index int
	IDs   []string
}

func validateRetrievalHit(actual string, expected string, rawConfig json.RawMessage) validatorOutcome {
	cfg, err := parseRetrievalValidatorConfig(rawConfig)
	if err != nil {
		return validatorError("parse retrieval_hit config", err, nil)
	}
	actualDoc, err := parseJSONValue(actual)
	if err != nil {
		return validatorError("parse actual retrieval JSON", err, nil)
	}
	expectedDoc, err := parseExpectedRetrievalEvidence(expected, cfg.ExpectedIDsPath)
	if err != nil {
		return validatorError("parse expected retrieval ids", err, nil)
	}

	chunks, outcome := retrievalChunksFromDocument(actualDoc, cfg)
	if outcome != nil {
		return *outcome
	}
	expectedIDs := retrievalIDsFromValue(expectedDoc)
	if len(expectedIDs) == 0 {
		return unavailableRetrievalOutcome("expected retrieval ids are unavailable", map[string]any{
			"expected_ids_path": cfg.ExpectedIDsPath,
		})
	}

	considered := clipRetrievalChunks(chunks, cfg.K)
	matched, missing := matchExpectedRetrievalIDs(considered, expectedIDs)
	evidence := retrievalEvidence(cfg, considered, expectedIDs, matched)
	evidence["missing_ids"] = missing

	if len(matched) > 0 {
		return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1), evidence: evidence}
	}
	return validatorOutcome{
		verdict:         "fail",
		normalizedScore: floatPtr(0),
		reason:          "none of the expected retrieval ids appeared in retrieved chunks",
		evidence:        evidence,
	}
}

func validateRetrievalPrecision(actual string, expected string, rawConfig json.RawMessage) validatorOutcome {
	cfg, err := parseRetrievalValidatorConfig(rawConfig)
	if err != nil {
		return validatorError("parse retrieval_precision config", err, nil)
	}
	actualDoc, err := parseJSONValue(actual)
	if err != nil {
		return validatorError("parse actual retrieval JSON", err, nil)
	}
	expectedDoc, err := parseExpectedRetrievalEvidence(expected, cfg.ExpectedIDsPath)
	if err != nil {
		return validatorError("parse expected retrieval ids", err, nil)
	}

	chunks, outcome := retrievalChunksFromDocument(actualDoc, cfg)
	if outcome != nil {
		return *outcome
	}
	expectedIDs := retrievalIDsFromValue(expectedDoc)
	if len(expectedIDs) == 0 {
		return unavailableRetrievalOutcome("expected retrieval ids are unavailable", map[string]any{
			"expected_ids_path": cfg.ExpectedIDsPath,
		})
	}

	considered := clipRetrievalChunks(chunks, cfg.K)
	if len(considered) == 0 {
		return unavailableRetrievalOutcome("retrieved chunks are unavailable", map[string]any{
			"retrieved_chunks_path": cfg.RetrievedChunksPath,
		})
	}

	expectedSet := stringSet(expectedIDs)
	matchedChunks := 0
	matchedIDs := make([]string, 0)
	for _, chunk := range considered {
		if chunkMatchesAnyExpectedID(chunk, expectedSet) {
			matchedChunks++
			matchedIDs = append(matchedIDs, matchingChunkIDs(chunk, expectedSet)...)
		}
	}
	precision := float64(matchedChunks) / float64(len(considered))
	passAt := cfg.effectivePassAt()
	evidence := retrievalEvidence(cfg, considered, expectedIDs, uniqueStrings(matchedIDs))
	evidence["matched_chunks"] = matchedChunks
	evidence["considered_chunks"] = len(considered)
	evidence["precision"] = precision
	evidence["pass_at"] = passAt

	if precision >= passAt {
		return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(precision), evidence: evidence}
	}
	return validatorOutcome{
		verdict:         "fail",
		normalizedScore: floatPtr(precision),
		reason:          fmt.Sprintf("retrieval precision %.4f is below pass_at %.4f", precision, passAt),
		evidence:        evidence,
	}
}

func parseRetrievalValidatorConfig(raw json.RawMessage) (retrievalValidatorConfig, error) {
	cfg := retrievalValidatorConfig{RetrievedChunksPath: defaultRetrievedChunksPath}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return cfg, nil
	}
	if err := strictUnmarshal(raw, &cfg); err != nil {
		return retrievalValidatorConfig{}, err
	}
	if strings.TrimSpace(cfg.RetrievedChunksPath) == "" {
		cfg.RetrievedChunksPath = defaultRetrievedChunksPath
	} else {
		cfg.RetrievedChunksPath = strings.TrimSpace(cfg.RetrievedChunksPath)
	}
	cfg.ExpectedIDsPath = strings.TrimSpace(cfg.ExpectedIDsPath)
	if cfg.IDFields != nil {
		for i := range *cfg.IDFields {
			(*cfg.IDFields)[i] = strings.TrimSpace((*cfg.IDFields)[i])
		}
	}
	return cfg, nil
}

func (c retrievalValidatorConfig) idFields() []string {
	if c.IDFields == nil {
		return []string{"chunk_id", "document_id"}
	}
	fields := make([]string, 0, len(*c.IDFields))
	for _, field := range *c.IDFields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			fields = append(fields, trimmed)
		}
	}
	return fields
}

func (c retrievalValidatorConfig) effectivePassAt() float64 {
	if c.PassAt == nil {
		return defaultRetrievalPassAt
	}
	return *c.PassAt
}

func parseExpectedRetrievalEvidence(expected string, path string) (any, error) {
	if strings.TrimSpace(path) == "" {
		parsed, err := parseJSONValue(expected)
		if err == nil {
			return parsed, nil
		}
		return strings.TrimSpace(expected), nil
	}
	document, err := parseJSONValue(expected)
	if err != nil {
		return nil, err
	}
	value, exists, err := extractJSONPathValue(document, path)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return value, nil
}

func retrievalChunksFromDocument(document any, cfg retrievalValidatorConfig) ([]retrievalChunkEvidence, *validatorOutcome) {
	value, exists, err := extractJSONPathValue(document, cfg.RetrievedChunksPath)
	if err != nil {
		outcome := validatorError("evaluate retrieved_chunks_path", err, map[string]any{
			"retrieved_chunks_path": cfg.RetrievedChunksPath,
		})
		return nil, &outcome
	}
	if !exists {
		outcome := unavailableRetrievalOutcome("retrieved chunks are unavailable", map[string]any{
			"retrieved_chunks_path": cfg.RetrievedChunksPath,
		})
		return nil, &outcome
	}
	rawChunks, ok := value.([]any)
	if !ok {
		outcome := validatorError("parse retrieved chunks", fmt.Errorf("retrieved_chunks_path must resolve to an array"), map[string]any{
			"retrieved_chunks_path": cfg.RetrievedChunksPath,
		})
		return nil, &outcome
	}
	if len(rawChunks) == 0 {
		outcome := unavailableRetrievalOutcome("retrieved chunks are unavailable", map[string]any{
			"retrieved_chunks_path": cfg.RetrievedChunksPath,
		})
		return nil, &outcome
	}

	fields := cfg.idFields()
	chunks := make([]retrievalChunkEvidence, 0, len(rawChunks))
	for i, raw := range rawChunks {
		object, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		ids := make([]string, 0, len(fields))
		for _, field := range fields {
			if id, ok := retrievalIDString(object[field]); ok {
				ids = append(ids, id)
			}
		}
		chunks = append(chunks, retrievalChunkEvidence{Index: i, IDs: uniqueStrings(ids)})
	}
	if len(chunks) == 0 {
		outcome := unavailableRetrievalOutcome("retrieved chunk ids are unavailable", map[string]any{
			"retrieved_chunks_path": cfg.RetrievedChunksPath,
			"id_fields":             fields,
		})
		return nil, &outcome
	}
	return chunks, nil
}

func retrievalIDsFromValue(value any) []string {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		if trimmed := strings.TrimSpace(typed); trimmed != "" {
			return []string{trimmed}
		}
	case []any:
		ids := make([]string, 0, len(typed))
		for _, item := range typed {
			if id, ok := retrievalIDString(item); ok {
				ids = append(ids, id)
			}
		}
		return uniqueStrings(ids)
	case map[string]any:
		for _, key := range []string{"chunk_id", "document_id", "id"} {
			if id, ok := retrievalIDString(typed[key]); ok {
				return []string{id}
			}
		}
	}
	if id, ok := retrievalIDString(value); ok {
		return []string{id}
	}
	return nil
}

func retrievalIDString(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		return trimmed, trimmed != ""
	case json.Number:
		return typed.String(), typed.String() != ""
	default:
		return "", false
	}
}

func clipRetrievalChunks(chunks []retrievalChunkEvidence, k *int) []retrievalChunkEvidence {
	limit := len(chunks)
	if k != nil && *k < limit {
		limit = *k
	}
	return append([]retrievalChunkEvidence(nil), chunks[:limit]...)
}

func matchExpectedRetrievalIDs(chunks []retrievalChunkEvidence, expectedIDs []string) ([]string, []string) {
	retrieved := map[string]struct{}{}
	for _, chunk := range chunks {
		for _, id := range chunk.IDs {
			retrieved[id] = struct{}{}
		}
	}
	matched := make([]string, 0)
	missing := make([]string, 0)
	for _, id := range uniqueStrings(expectedIDs) {
		if _, ok := retrieved[id]; ok {
			matched = append(matched, id)
		} else {
			missing = append(missing, id)
		}
	}
	return matched, missing
}

func retrievalEvidence(cfg retrievalValidatorConfig, chunks []retrievalChunkEvidence, expectedIDs []string, matchedIDs []string) map[string]any {
	return map[string]any{
		"retrieved_chunks_path": cfg.RetrievedChunksPath,
		"expected_ids_path":     cfg.ExpectedIDsPath,
		"id_fields":             cfg.idFields(),
		"k":                     cfg.K,
		"expected_ids":          uniqueStrings(expectedIDs),
		"considered_ids":        consideredRetrievalIDs(chunks),
		"matched_ids":           uniqueStrings(matchedIDs),
	}
}

func consideredRetrievalIDs(chunks []retrievalChunkEvidence) []string {
	ids := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		ids = append(ids, chunk.IDs...)
	}
	return uniqueStrings(ids)
}

func chunkMatchesAnyExpectedID(chunk retrievalChunkEvidence, expected map[string]struct{}) bool {
	for _, id := range chunk.IDs {
		if _, ok := expected[id]; ok {
			return true
		}
	}
	return false
}

func matchingChunkIDs(chunk retrievalChunkEvidence, expected map[string]struct{}) []string {
	matched := make([]string, 0)
	for _, id := range chunk.IDs {
		if _, ok := expected[id]; ok {
			matched = append(matched, id)
		}
	}
	return matched
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out[trimmed] = struct{}{}
		}
	}
	return out
}

func unavailableRetrievalOutcome(reason string, evidence map[string]any) validatorOutcome {
	if evidence == nil {
		evidence = map[string]any{}
	}
	return validatorOutcome{
		state:    OutputStateUnavailable,
		reason:   reason,
		evidence: evidence,
	}
}

func validateJSONPathSyntax(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	if path[0] != '$' {
		return fmt.Errorf("path must start with '$'")
	}
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
				return fmt.Errorf("missing property name at position %d", start)
			}
		case '[':
			closeIndex, _, err := parseJSONPathBracket(path, index)
			if err != nil {
				return err
			}
			index = closeIndex + 1
		default:
			return fmt.Errorf("unexpected token %q at position %d", path[index], index)
		}
	}
	return nil
}
