package scoring

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const maxFuzzyMatchRunes = 100_000

// --- fuzzy_match ---

type fuzzyMatchConfig struct {
	Threshold       *float64 `json:"threshold"`
	CaseInsensitive bool     `json:"case_insensitive"`
	// Normalize applies whitespace normalization only (trim + collapse).
	// For Unicode normalization (NFC), use normalized_match with the normalize_unicode step.
	Normalize bool `json:"normalize"`
}

func validateFuzzyMatch(actual string, expected string, rawConfig json.RawMessage) validatorOutcome {
	config := fuzzyMatchConfig{}
	if len(rawConfig) > 0 {
		if err := json.Unmarshal(rawConfig, &config); err != nil {
			return validatorError("parse fuzzy_match config", err, nil)
		}
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
		a = strings.ToLower(a)
		b = strings.ToLower(b)
	}

	runesA := []rune(a)
	runesB := []rune(b)
	if len(runesA) > maxFuzzyMatchRunes || len(runesB) > maxFuzzyMatchRunes {
		return validatorError("fuzzy_match input too large", fmt.Errorf("inputs exceed %d runes", maxFuzzyMatchRunes), nil)
	}

	distance := levenshteinDistance(runesA, runesB)
	maxLen := len(runesA)
	if len(runesB) > maxLen {
		maxLen = len(runesB)
	}

	var similarity float64
	if maxLen == 0 {
		similarity = 1.0
	} else {
		similarity = 1.0 - float64(distance)/float64(maxLen)
	}

	evidence := map[string]any{
		"similarity":       similarity,
		"threshold":        threshold,
		"distance":         distance,
		"max_length":       maxLen,
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

type numericMatchConfig struct {
	AbsoluteTolerance *float64 `json:"absolute_tolerance"`
	RelativeTolerance *float64 `json:"relative_tolerance"`
	ExtractNumber     bool     `json:"extract_number"`
	SignificantDigits *int     `json:"significant_digits"`
}

func validateNumericMatch(actual string, expected string, rawConfig json.RawMessage) validatorOutcome {
	config := numericMatchConfig{}
	if len(rawConfig) > 0 {
		if err := json.Unmarshal(rawConfig, &config); err != nil {
			return validatorError("parse numeric_match config", err, nil)
		}
	}

	expectedNum, err := strconv.ParseFloat(strings.TrimSpace(expected), 64)
	if err != nil {
		return validatorError("parse expected numeric value", err, map[string]any{
			"expected_raw": expected,
		})
	}

	var actualNum float64
	if config.ExtractNumber {
		actualNum, err = extractNumber(actual)
	} else {
		actualNum, err = strconv.ParseFloat(strings.TrimSpace(actual), 64)
	}
	if err != nil {
		return validatorError("parse actual numeric value", err, map[string]any{
			"actual_raw":     actual,
			"extract_number": config.ExtractNumber,
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
		"actual_numeric":      actualNum,
		"expected_numeric":    expectedNum,
		"absolute_difference": absDiff,
		"relative_difference": relDiff,
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

var numberCleanerReplacer = strings.NewReplacer(
	"$", "", "€", "", "£", "", "¥", "", "₹", "",
	"EUR", "", "GBP", "", "USD", "", "JPY", "", "INR", "",
	",", "", "%", "",
)

var numberExtractRegex = regexp.MustCompile(`-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?`)

// extractNumber strips currency symbols, commas, and percent signs, then returns
// the first numeric value found. When multiple numbers are present (e.g. "10 out of 100"),
// it returns the leftmost match — callers should structure prompts to put the target number first.
func extractNumber(s string) (float64, error) {
	cleaned := numberCleanerReplacer.Replace(s)
	cleaned = strings.TrimSpace(cleaned)

	match := numberExtractRegex.FindString(cleaned)
	if match == "" {
		return 0, fmt.Errorf("no numeric value found in %q", s)
	}
	return strconv.ParseFloat(match, 64)
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

type normalizedMatchConfig struct {
	Pipeline []string `json:"pipeline"`
}

var knownPipelineSteps = map[string]bool{
	"trim":                true,
	"lowercase":           true,
	"collapse_whitespace": true,
	"strip_punctuation":   true,
	"strip_currency":      true,
	"strip_formatting":    true,
	"normalize_unicode":   true,
	"sort_words":          true,
	"sort_lines":          true,
}

var defaultPipeline = []string{"trim", "lowercase", "collapse_whitespace"}

func validateNormalizedMatch(actual string, expected string, rawConfig json.RawMessage) validatorOutcome {
	config := normalizedMatchConfig{}
	if len(rawConfig) > 0 {
		if err := json.Unmarshal(rawConfig, &config); err != nil {
			return validatorError("parse normalized_match config", err, nil)
		}
	}

	pipeline := config.Pipeline
	if len(pipeline) == 0 {
		pipeline = defaultPipeline
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

func applyNormalizationPipeline(s string, pipeline []string) (string, error) {
	for _, step := range pipeline {
		switch step {
		case "trim":
			s = strings.TrimSpace(s)
		case "lowercase":
			s = strings.ToLower(s)
		case "collapse_whitespace":
			s = collapseWhitespace(s)
		case "strip_punctuation":
			s = stripPunctuation(s)
		case "strip_currency":
			s = stripCurrency(s)
		case "strip_formatting":
			s = stripFormatting(s)
		case "normalize_unicode":
			s = norm.NFC.String(s)
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
