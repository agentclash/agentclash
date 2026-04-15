package judge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
)

// Phase 5 of issue #148 — shared parsing + schema + scale helpers for
// rubric and reference modes. See backend/.claude/analysis/issue-148-
// deep-analysis.md Part 5 lines 420-451 for the execution semantics
// these helpers implement.

// rubricResponse is the canonical shape the evaluator extracts from a
// rubric/reference judge's LLM response. The raw JSON is parsed into
// this struct AFTER the optional custom OutputSchema validation so
// pack authors can still enforce stricter schemas (stricter types,
// additional required fields, rubric-specific extras like a
// "hallucinations" array from issue Method 1 example) while the
// evaluator reads only the fields it needs for scoring.
//
// UnableToJudge short-circuits to abstain without schema validation —
// the judge explicitly told us it can't decide, so requiring the
// response to also conform to a score-bearing schema would be rude
// and self-defeating.
type rubricResponse struct {
	Score          *float64
	Reasoning      string
	UnableToJudge  bool
	AbstainReason  string
	RawJSON        json.RawMessage
}

// defaultRubricSchemaJSON is the fallback schema when a judge declares
// no output_schema. Matches the definition pinned in backend/.claude/
// analysis/issue-148-deep-analysis.md Part 8 Q4 (Phase 5 plan question
// Q1 resolution): score is a number (NOT nullable), reasoning is a
// string, unable_to_judge is a boolean. Pack authors who want to
// forbid the escape hatch can override with a stricter schema; packs
// that want even more structured output (e.g., a hallucinations
// array) add custom fields via their own output_schema.
const defaultRubricSchemaJSON = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "score": {"type": "number"},
    "reasoning": {"type": "string"},
    "unable_to_judge": {"type": "boolean"},
    "reason": {"type": "string"}
  }
}`

// defaultRubricSchema is parsed at package init time so every
// evaluateRubric call reuses the same compiled schema instance.
// Compilation failures would panic at init which is exactly what we
// want — a broken default schema is a programming error, not a
// runtime condition, and must not be caught.
var defaultRubricSchema *jsonschema.Schema

func init() {
	var schema jsonschema.Schema
	if err := json.Unmarshal([]byte(defaultRubricSchemaJSON), &schema); err != nil {
		panic(fmt.Sprintf("judge: invalid default rubric schema: %v", err))
	}
	// jsonschema-go validates against 2020-12 only; draft-07 schemas
	// are accepted for the overlapping keyword subset (same treatment
	// as scoring/json_validators.go:163).
	schema.Schema = ""
	defaultRubricSchema = &schema
}

// resolveRubricSchema returns the schema the evaluator should use when
// validating a rubric/reference response. Priority:
//
//  1. judge.OutputSchema when non-empty — parsed via the existing
//     scoring.parseJSONSchema helper (not reachable from here — we
//     re-implement a tiny equivalent because judge is its own package).
//  2. defaultRubricSchema otherwise.
//
// Schema parse failures at runtime are unlikely — validation already
// rejects malformed output_schema values at spec load time (Phase 1
// rule 8 in validation_judges.go). Defensive parse-retry here just in
// case a schema slips through.
func resolveRubricSchema(judge scoring.LLMJudgeDeclaration) (*jsonschema.Schema, error) {
	raw := bytes.TrimSpace(judge.OutputSchema)
	if len(raw) == 0 {
		return defaultRubricSchema, nil
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("parse judge output_schema: %w", err)
	}
	// Match the json_validators.go draft handling: accept draft-07 by
	// clearing the $schema URI so the 2020-12-only validator walks
	// the overlapping keyword subset.
	schema.Schema = ""
	return &schema, nil
}

// jsonCodeFencePattern matches optional markdown code fences so the
// extractor can strip ` ```json ... ``` ` wrappers before attempting a
// strict parse. (?s) enables dotall so the middle can span newlines.
var jsonCodeFencePattern = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

// extractJSONObject is a three-tier prose-to-JSON extractor. Returns
// the JSON substring and true on success, empty string and false on
// failure. Used by parseRubricResponse to pull a JSON object out of an
// LLM response that may have surrounding prose or markdown wrapping.
//
// Tiers, in order:
//
//  1. Strict: the entire response is already a JSON object. Wins for
//     models that follow the "respond ONLY with JSON" prompt.
//  2. Code fence: ``` or ```json ... ``` markdown wrappers. Common for
//     models that default to markdown output.
//  3. Brace balance: find the first '{', scan forward tracking depth,
//     return the substring ending at the matching '}'. Catches "Here
//     is my response: { ... }" prose prologs.
//
// The function does NOT itself parse the JSON — it only isolates the
// substring. parseRubricResponse then runs json.Decoder on the result.
// Splitting extraction from parsing keeps each step testable and
// allows parseRubricResponse to surface better error messages.
func extractJSONObject(text string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", false
	}

	// Tier 1: strict — the whole response is a JSON object.
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return trimmed, true
	}

	// Tier 2: markdown code fence.
	if match := jsonCodeFencePattern.FindStringSubmatch(trimmed); len(match) == 2 {
		inner := strings.TrimSpace(match[1])
		if strings.HasPrefix(inner, "{") {
			return inner, true
		}
	}

	// Tier 3: brace balance scan. Find the first '{', walk forward
	// counting depth, return when depth returns to zero. Naive string
	// handling is adequate here because LLM responses rarely contain
	// literal '{' inside strings when also wrapping in code fences —
	// and even if they do, the multi-sample aggregation absorbs the
	// noise. Strict handling would require a full JSON tokenizer.
	start := strings.IndexByte(trimmed, '{')
	if start < 0 {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(trimmed); i++ {
		c := trimmed[i]
		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return trimmed[start : i+1], true
			}
		}
	}
	return "", false
}

// parseRubricResponse runs the full parse pipeline for a single
// rubric/reference judge response: prose extraction, JSON decode,
// schema validation, rubricResponse mapping.
//
// Returns (result, ok). ok=false means the response couldn't be
// parsed or failed schema validation — the caller marks the sample
// as abstain and continues. Multi-sample averaging absorbs the loss.
//
// The UnableToJudge short-circuit happens AFTER JSON decode but
// BEFORE schema validation: if the judge explicitly abstained, we
// honour that regardless of whether the rest of the response
// conforms to the score-bearing schema. Matches Q1 from the Phase 5
// plan resolution.
func parseRubricResponse(text string, schema *jsonschema.Schema) (rubricResponse, bool) {
	extracted, ok := extractJSONObject(text)
	if !ok {
		return rubricResponse{}, false
	}

	raw := json.RawMessage(extracted)

	// Decode into a generic map first so we can check for the abstain
	// flag before running schema validation. Deliberately NOT using
	// UseNumber: jsonschema-go validates "type: number" against Go
	// float64 (not json.Number), and integer-vs-float discrimination
	// is handled below by coerceNumber looking at the raw string form
	// when needed.
	decoder := json.NewDecoder(strings.NewReader(extracted))
	var generic map[string]any
	if err := decoder.Decode(&generic); err != nil {
		return rubricResponse{}, false
	}

	// Abstain fast-path.
	if abstain, ok := generic["unable_to_judge"].(bool); ok && abstain {
		reason, _ := generic["reason"].(string)
		if reason == "" {
			reason, _ = generic["reasoning"].(string)
		}
		return rubricResponse{
			UnableToJudge: true,
			AbstainReason: strings.TrimSpace(reason),
			RawJSON:       raw,
		}, true
	}

	// Schema validation. Resolved outside the hot path by the caller.
	resolved, resolveErr := schema.Resolve(nil)
	if resolveErr != nil {
		return rubricResponse{}, false
	}
	if err := resolved.Validate(generic); err != nil {
		return rubricResponse{}, false
	}

	// Pull score out of the generic map. JSON numbers via UseNumber
	// arrive as json.Number which preserves the original textual form —
	// parse it as float64 for normalization math.
	rawScore, present := generic["score"]
	if !present {
		return rubricResponse{}, false
	}
	score, ok := coerceNumber(rawScore)
	if !ok {
		// Score field present but not a number. Schema should have
		// caught this, but defend against exotic pack schemas that
		// accept non-number types.
		return rubricResponse{}, false
	}

	reasoning, _ := generic["reasoning"].(string)
	return rubricResponse{
		Score:     &score,
		Reasoning: strings.TrimSpace(reasoning),
		RawJSON:   raw,
	}, true
}

// coerceNumber extracts a float64 from a JSON value that may arrive
// as json.Number (UseNumber) or the default float64. Returns
// (value, true) on success.
func coerceNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case json.Number:
		f, err := strconv.ParseFloat(v.String(), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

// scaleNormalize maps a raw rubric score from the declared scale into
// [0, 1]. The caller supplies the scale from judge.ScoreScale (or the
// default 1..5 when nil). Values outside the scale are CLAMPED, not
// rejected — a model returning 6 on a 1..5 scale gets normalized to
// 1.0 and the variance-based confidence bin absorbs the signal that
// the model struggled to calibrate. Rejecting out-of-range values
// here would just turn a slightly-confused sample into an abstain,
// which is worse for aggregation.
//
// When scale.Min >= scale.Max (invariant violation that validation
// should have caught at spec load time), the function defensively
// returns 0 so the judge dim fails open rather than panicking.
func scaleNormalize(raw float64, scale scoring.ScoreScale) float64 {
	if scale.Min >= scale.Max {
		return 0
	}
	if raw <= scale.Min {
		return 0
	}
	if raw >= scale.Max {
		return 1
	}
	return (raw - scale.Min) / (scale.Max - scale.Min)
}

// effectiveScoreScale returns the ScoreScale to use for normalization.
// Falls back to the issue default of 1..5 (see issue #148 Method 1
// example at lines 56 and 68) when the judge omits score_scale.
func effectiveScoreScale(judge scoring.LLMJudgeDeclaration) scoring.ScoreScale {
	if judge.ScoreScale != nil && judge.ScoreScale.Max > judge.ScoreScale.Min {
		return *judge.ScoreScale
	}
	return scoring.ScoreScale{Min: 1, Max: 5}
}
