package scoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// unicodeFold performs full Unicode case folding, handling edge cases that
// strings.ToLower misses (e.g. German eszett ß↔ss, Turkic İ/ı).
var unicodeFold = cases.Fold()

const maxFuzzyMatchRunes = 100_000

// --- fuzzy_match ---

// fuzzyMatchConfig is the strict contract for fuzzy_match validator config.
// Unknown fields are rejected at spec-load time and again at runtime.
type fuzzyMatchConfig struct {
	Threshold       *float64 `json:"threshold"`
	CaseInsensitive bool     `json:"case_insensitive"`
	// Normalize applies whitespace normalization only (trim + collapse).
	// For Unicode normalization (NFC), use normalized_match with the normalize_unicode step.
	Normalize bool `json:"normalize"`
}

func validateFuzzyMatch(actual string, expected string, rawConfig json.RawMessage) validatorOutcome {
	config, err := parseFuzzyMatchConfig(rawConfig)
	if err != nil {
		return validatorError("parse fuzzy_match config", err, nil)
	}

	threshold := 0.8
	if config.Threshold != nil {
		threshold = *config.Threshold
	}
	if threshold < 0 {
		threshold = 0
	} else if threshold > 1 {
		threshold = 1
	}

	a, b := actual, expected
	if config.Normalize {
		a = collapseWhitespace(strings.TrimSpace(a))
		b = collapseWhitespace(strings.TrimSpace(b))
	}
	if config.CaseInsensitive {
		a = unicodeFold.String(a)
		b = unicodeFold.String(b)
	}

	lenA, ok := runeCountWithinLimit(a, maxFuzzyMatchRunes)
	if !ok {
		return validatorError("fuzzy_match input too large", fmt.Errorf("inputs exceed %d runes", maxFuzzyMatchRunes), nil)
	}
	lenB, ok := runeCountWithinLimit(b, maxFuzzyMatchRunes)
	if !ok {
		return validatorError("fuzzy_match input too large", fmt.Errorf("inputs exceed %d runes", maxFuzzyMatchRunes), nil)
	}

	runesA := []rune(a)
	runesB := []rune(b)
	distance := levenshteinDistance(runesA, runesB)
	sumLen := lenA + lenB

	// Standard similarity ratio matching python-Levenshtein / rapidfuzz / thefuzz:
	//   ratio = (len(a) + len(b) - distance) / (len(a) + len(b))
	var similarity float64
	if sumLen == 0 {
		similarity = 1.0
	} else {
		similarity = float64(sumLen-distance) / float64(sumLen)
	}

	evidence := map[string]any{
		"similarity":       similarity,
		"threshold":        threshold,
		"distance":         distance,
		"sum_length":       sumLen,
		"case_insensitive": config.CaseInsensitive,
		"normalize":        config.Normalize,
	}

	if similarity >= threshold {
		return validatorOutcome{
			verdict:         "pass",
			normalizedScore: floatPtr(similarity),
			evidence:        evidence,
		}
	}
	return validatorOutcome{
		verdict:         "fail",
		normalizedScore: floatPtr(similarity),
		reason:          fmt.Sprintf("similarity %.4f is below threshold %.4f", similarity, threshold),
		evidence:        evidence,
	}
}

// levenshteinDistance computes the edit distance between two rune slices
// using the Wagner-Fischer algorithm with two-row optimization (O(min(m,n)) space).
func levenshteinDistance(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	if len(a) > len(b) {
		a, b = b, a
	}

	prev := make([]int, len(a)+1)
	curr := make([]int, len(a)+1)
	for i := range prev {
		prev[i] = i
	}

	for j := 1; j <= len(b); j++ {
		curr[0] = j
		for i := 1; i <= len(a); i++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[i] = min(
				curr[i-1]+1,
				min(prev[i]+1, prev[i-1]+cost),
			)
		}
		prev, curr = curr, prev
	}
	return prev[len(a)]
}

// --- numeric_match ---

// numericMatchConfig keeps the current config contract while also accepting the
// issue-era tolerance alias fields so older specs fail less surprisingly.
type numericMatchConfig struct {
	AbsoluteTolerance *float64 `json:"absolute_tolerance"`
	RelativeTolerance *float64 `json:"relative_tolerance"`
	ExtractNumber     bool     `json:"extract_number"`
	SignificantDigits *int     `json:"significant_digits"`
	ToleranceMode     string   `json:"tolerance_mode"`
	Tolerance         *float64 `json:"tolerance"`
}

func validateNumericMatch(actual string, expected string, rawConfig json.RawMessage) validatorOutcome {
	config, err := parseNumericMatchConfig(rawConfig)
	if err != nil {
		return validatorError("parse numeric_match config", err, nil)
	}

	expectedNum, expectedParsedText, err := parseNumericValue(expected, config.ExtractNumber)
	if err != nil {
		return validatorError("parse expected numeric value", err, map[string]any{
			"expected_raw":    expected,
			"extract_number":  config.ExtractNumber,
			"expected_parsed": expectedParsedText,
		})
	}

	actualNum, actualParsedText, err := parseNumericValue(actual, config.ExtractNumber)
	if err != nil {
		return validatorError("parse actual numeric value", err, map[string]any{
			"actual_raw":     actual,
			"extract_number": config.ExtractNumber,
			"actual_parsed":  actualParsedText,
		})
	}

	if config.SignificantDigits != nil && *config.SignificantDigits > 0 {
		actualNum = roundToSignificantDigits(actualNum, *config.SignificantDigits)
		expectedNum = roundToSignificantDigits(expectedNum, *config.SignificantDigits)
	}

	absDiff := math.Abs(actualNum - expectedNum)
	var relDiff float64
	if expectedNum != 0 {
		relDiff = absDiff / math.Abs(expectedNum)
	} else if absDiff != 0 {
		relDiff = math.Inf(1)
	}

	evidence := map[string]any{
		"actual_raw":          actual,
		"expected_raw":        expected,
		"actual_parsed":       actualParsedText,
		"expected_parsed":     expectedParsedText,
		"actual_numeric":      actualNum,
		"expected_numeric":    expectedNum,
		"absolute_difference": absDiff,
		"relative_difference": relDiff,
		"extract_number":      config.ExtractNumber,
	}

	pass := false
	if config.AbsoluteTolerance == nil && config.RelativeTolerance == nil {
		pass = actualNum == expectedNum
	} else {
		if config.AbsoluteTolerance != nil {
			evidence["absolute_tolerance"] = *config.AbsoluteTolerance
			if absDiff <= *config.AbsoluteTolerance {
				pass = true
			}
		}
		if config.RelativeTolerance != nil {
			evidence["relative_tolerance"] = *config.RelativeTolerance
			if relDiff <= *config.RelativeTolerance {
				pass = true
			}
		}
	}

	if pass {
		return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1), evidence: evidence}
	}
	return validatorOutcome{
		verdict:         "fail",
		normalizedScore: floatPtr(0),
		reason:          fmt.Sprintf("actual %g is not within tolerance of expected %g", actualNum, expectedNum),
		evidence:        evidence,
	}
}

var numberExtractRegex = regexp.MustCompile(`[+-]?(?:\d+(?:\.\d+)?|\.\d+)(?:[eE][+-]?\d+)?`)

// extractNumber strips currency symbols, commas, and percent signs, then returns
// the first numeric value found. When multiple numbers are present (e.g. "10 out of 100"),
// it returns the leftmost match — callers should structure prompts to put the target number first.
func extractNumber(s string) (float64, error) {
	match, err := extractNumberToken(s)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(match, 64)
}

func extractNumberToken(s string) (string, error) {
	// Reuse the shared currencySymbols replacer, then strip commas and percent.
	cleaned := currencySymbols.Replace(s)
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = strings.ReplaceAll(cleaned, "%", "")
	cleaned = strings.TrimSpace(cleaned)

	match := numberExtractRegex.FindString(cleaned)
	if match == "" {
		return "", fmt.Errorf("no numeric value found in %q", s)
	}
	return match, nil
}

func roundToSignificantDigits(val float64, digits int) float64 {
	if val == 0 {
		return 0
	}
	d := math.Floor(math.Log10(math.Abs(val))) + 1
	pow := math.Pow(10, float64(digits)-d)
	return math.Round(val*pow) / pow
}

// --- normalized_match ---

// normalizedMatchConfig accepts both the current `pipeline` key and the
// issue-era `normalizations` alias, but rejects mixing both in one config.
type normalizedMatchConfig struct {
	Pipeline       []string `json:"pipeline"`
	Normalizations []string `json:"normalizations"`
}

type tokenF1Config struct {
	Threshold         *float64 `json:"threshold"`
	Normalize         bool     `json:"normalize"`
	RemoveArticles    bool     `json:"remove_articles"`
	RemovePunctuation bool     `json:"remove_punctuation"`
}

var knownPipelineSteps = map[string]bool{
	"trim":                true,
	"lowercase":           true,
	"collapse_whitespace": true,
	"strip_punctuation":   true,
	"strip_currency":      true,
	"strip_formatting":    true,
	"normalize_unicode":   true,
	"remove_articles":     true,
	"sort_words":          true,
	"sort_lines":          true,
}

var defaultPipeline = []string{"trim", "lowercase", "collapse_whitespace"}

func validateNormalizedMatch(actual string, expected string, rawConfig json.RawMessage) validatorOutcome {
	config, err := parseNormalizedMatchConfig(rawConfig)
	if err != nil {
		return validatorError("parse normalized_match config", err, nil)
	}

	pipeline, err := config.pipeline()
	if err != nil {
		return validatorError("parse normalized_match config", err, nil)
	}

	normalizedActual, err := applyNormalizationPipeline(actual, pipeline)
	if err != nil {
		return validatorError("normalize actual value", err, nil)
	}
	normalizedExpected, err := applyNormalizationPipeline(expected, pipeline)
	if err != nil {
		return validatorError("normalize expected value", err, nil)
	}

	evidence := map[string]any{
		"normalized_actual":   normalizedActual,
		"normalized_expected": normalizedExpected,
		"pipeline":            pipeline,
	}

	if normalizedActual == normalizedExpected {
		return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1), evidence: evidence}
	}
	return validatorOutcome{
		verdict:         "fail",
		normalizedScore: floatPtr(0),
		reason:          "normalized values do not match",
		evidence:        evidence,
	}
}

// --- token_f1 ---

const defaultTokenF1Threshold = 0.5

func validateTokenF1(actual string, expected string, rawConfig json.RawMessage) validatorOutcome {
	config, err := parseTokenF1Config(rawConfig)
	if err != nil {
		return validatorError("parse token_f1 config", err, nil)
	}

	pipeline := config.pipeline()
	normalizedActual, err := applyNormalizationPipeline(actual, pipeline)
	if err != nil {
		return validatorError("normalize token_f1 actual value", err, nil)
	}
	normalizedExpected, err := applyNormalizationPipeline(expected, pipeline)
	if err != nil {
		return validatorError("normalize token_f1 expected value", err, nil)
	}

	predictionTokens := strings.Fields(normalizedActual)
	referenceTokens := strings.Fields(normalizedExpected)
	precision, recall, score, overlap := tokenF1Score(predictionTokens, referenceTokens)

	return thresholdedOutcome(score, thresholdOrDefault(config.Threshold, defaultTokenF1Threshold), map[string]any{
		"normalized_actual":      normalizedActual,
		"normalized_expected":    normalizedExpected,
		"pipeline":               pipeline,
		"prediction_tokens":      predictionTokens,
		"reference_tokens":       referenceTokens,
		"prediction_token_count": len(predictionTokens),
		"reference_token_count":  len(referenceTokens),
		"overlap_token_count":    overlap,
		"precision":              precision,
		"recall":                 recall,
	})
}

func (c tokenF1Config) pipeline() []string {
	pipeline := make([]string, 0, len(defaultPipeline)+4)
	if c.Normalize {
		pipeline = append(pipeline, defaultPipeline...)
	}
	if c.RemovePunctuation {
		pipeline = append(pipeline, "strip_punctuation")
	}
	if c.RemoveArticles {
		pipeline = append(pipeline, "remove_articles")
	}
	if len(pipeline) == 0 {
		return nil
	}
	return append(pipeline, "collapse_whitespace", "trim")
}

func tokenF1Score(predictionTokens []string, referenceTokens []string) (precision float64, recall float64, score float64, overlap int) {
	switch {
	case len(predictionTokens) == 0 && len(referenceTokens) == 0:
		return 1, 1, 1, 0
	case len(predictionTokens) == 0 || len(referenceTokens) == 0:
		return 0, 0, 0, 0
	}

	predictionCounts := tokenCounts(predictionTokens)
	referenceCounts := tokenCounts(referenceTokens)
	for token, predictionCount := range predictionCounts {
		overlap += min(predictionCount, referenceCounts[token])
	}

	precision = float64(overlap) / float64(len(predictionTokens))
	recall = float64(overlap) / float64(len(referenceTokens))
	if precision == 0 || recall == 0 {
		return precision, recall, 0, overlap
	}
	score = (2 * precision * recall) / (precision + recall)
	return precision, recall, score, overlap
}

func tokenCounts(tokens []string) map[string]int {
	counts := make(map[string]int, len(tokens))
	for _, token := range tokens {
		counts[token]++
	}
	return counts
}

func applyNormalizationPipeline(s string, pipeline []string) (string, error) {
	for _, step := range pipeline {
		switch step {
		case "trim":
			s = strings.TrimSpace(s)
		case "lowercase":
			s = unicodeFold.String(s)
		case "collapse_whitespace":
			s = collapseWhitespace(s)
		case "strip_punctuation":
			s = stripPunctuation(s)
		case "strip_currency":
			s = stripCurrency(s)
		case "strip_formatting":
			s = stripFormatting(s)
		case "normalize_unicode":
			s = norm.NFKC.String(s)
		case "remove_articles":
			s = removeArticles(s)
		case "sort_words":
			words := strings.Fields(s)
			sort.Strings(words)
			s = strings.Join(words, " ")
		case "sort_lines":
			lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
			sort.Strings(lines)
			s = strings.Join(lines, "\n")
		default:
			return "", fmt.Errorf("unknown normalization step %q", step)
		}
	}
	return s, nil
}

// --- shared helpers ---

// articlesRegex matches English articles (a, an, the) at word boundaries,
// following the SQuAD / HELM / lm-evaluation-harness normalization convention.
var articlesRegex = regexp.MustCompile(`\b(a|an|the)\b`)

func removeArticles(s string) string {
	return articlesRegex.ReplaceAllString(s, " ")
}

var whitespaceRegex = regexp.MustCompile(`\s+`)

func collapseWhitespace(s string) string {
	return whitespaceRegex.ReplaceAllString(s, " ")
}

func stripPunctuation(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !unicode.IsPunct(r) && !unicode.IsSymbol(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func decodeStrictJSON(raw json.RawMessage, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("unexpected trailing data")
	}
	return nil
}

func parseFuzzyMatchConfig(rawConfig json.RawMessage) (fuzzyMatchConfig, error) {
	var config fuzzyMatchConfig
	if len(rawConfig) == 0 {
		return config, nil
	}
	if err := decodeStrictJSON(rawConfig, &config); err != nil {
		return fuzzyMatchConfig{}, err
	}
	return config, nil
}

func parseNumericMatchConfig(rawConfig json.RawMessage) (numericMatchConfig, error) {
	var config numericMatchConfig
	if len(rawConfig) == 0 {
		return config, nil
	}
	if err := decodeStrictJSON(rawConfig, &config); err != nil {
		return numericMatchConfig{}, err
	}

	hasLegacyTolerance := config.Tolerance != nil || strings.TrimSpace(config.ToleranceMode) != ""
	hasCurrentTolerance := config.AbsoluteTolerance != nil || config.RelativeTolerance != nil
	if hasLegacyTolerance && hasCurrentTolerance {
		return numericMatchConfig{}, fmt.Errorf("cannot mix tolerance_mode/tolerance with absolute_tolerance/relative_tolerance")
	}
	if hasLegacyTolerance {
		mode := strings.TrimSpace(config.ToleranceMode)
		if mode == "" {
			mode = "relative"
		}
		tolerance := 0.001
		if config.Tolerance != nil {
			tolerance = *config.Tolerance
		}

		switch mode {
		case "absolute":
			config.AbsoluteTolerance = &tolerance
		case "relative":
			config.RelativeTolerance = &tolerance
		default:
			return numericMatchConfig{}, fmt.Errorf("tolerance_mode must be either %q or %q", "absolute", "relative")
		}
	}

	return config, nil
}

func parseNumericValue(raw string, extract bool) (float64, string, error) {
	if extract {
		token, err := extractNumberToken(raw)
		if err != nil {
			return 0, "", err
		}
		value, err := strconv.ParseFloat(token, 64)
		if err != nil {
			return 0, token, err
		}
		return value, token, nil
	}

	token := strings.TrimSpace(raw)
	value, err := strconv.ParseFloat(token, 64)
	if err != nil {
		return 0, token, err
	}
	return value, token, nil
}

func parseNormalizedMatchConfig(rawConfig json.RawMessage) (normalizedMatchConfig, error) {
	var config normalizedMatchConfig
	if len(rawConfig) == 0 {
		return config, nil
	}
	if err := decodeStrictJSON(rawConfig, &config); err != nil {
		return normalizedMatchConfig{}, err
	}
	if _, err := config.pipeline(); err != nil {
		return normalizedMatchConfig{}, err
	}
	return config, nil
}

func (c normalizedMatchConfig) pipeline() ([]string, error) {
	if len(c.Pipeline) > 0 && len(c.Normalizations) > 0 {
		return nil, fmt.Errorf("cannot mix pipeline with normalizations")
	}
	if len(c.Pipeline) > 0 {
		return c.Pipeline, nil
	}
	if len(c.Normalizations) > 0 {
		return c.Normalizations, nil
	}
	return defaultPipeline, nil
}

func parseTokenF1Config(rawConfig json.RawMessage) (tokenF1Config, error) {
	var config tokenF1Config
	if len(rawConfig) == 0 {
		return config, nil
	}
	if err := decodeStrictJSON(rawConfig, &config); err != nil {
		return tokenF1Config{}, err
	}
	return config, nil
}

func runeCountWithinLimit(s string, limit int) (int, bool) {
	count := 0
	for range s {
		count++
		if count > limit {
			return count, false
		}
	}
	return count, true
}

var currencySymbols = strings.NewReplacer(
	"$", "", "€", "", "£", "", "¥", "", "₹", "",
	"USD", "", "EUR", "", "GBP", "", "JPY", "", "INR", "",
)

func stripCurrency(s string) string {
	return currencySymbols.Replace(s)
}

var formattingReplacer = strings.NewReplacer(
	",", "", "(", "", ")", "", "[", "", "]", "",
)

func stripFormatting(s string) string {
	return formattingReplacer.Replace(s)
}
