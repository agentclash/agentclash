package scoring

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
)

const (
	bleuSmoothingNone    = "none"
	bleuSmoothingMethod1 = "method1"

	rougeVariant1 = "rouge-1"
	rougeVariant2 = "rouge-2"
	rougeVariantL = "rouge-l"

	defaultGenerationThreshold = 0.3
)

type bleuScoreConfig struct {
	Threshold       *float64 `json:"threshold"`
	MaxNGram        *int     `json:"max_ngram"`
	Smoothing       string   `json:"smoothing"`
	CaseInsensitive bool     `json:"case_insensitive"`
	Normalize       bool     `json:"normalize"`
}

type rougeScoreConfig struct {
	Threshold       *float64 `json:"threshold"`
	Variant         string   `json:"variant"`
	Beta            *float64 `json:"beta"`
	CaseInsensitive bool     `json:"case_insensitive"`
	Normalize       bool     `json:"normalize"`
}

type chrfScoreConfig struct {
	Threshold       *float64 `json:"threshold"`
	CharOrder       *int     `json:"char_order"`
	Beta            *float64 `json:"beta"`
	CaseInsensitive bool     `json:"case_insensitive"`
	Normalize       bool     `json:"normalize"`
}

func validateBLEUScore(actual string, expected string, rawConfig json.RawMessage) validatorOutcome {
	config, err := parseBLEUScoreConfig(rawConfig)
	if err != nil {
		return validatorError("parse bleu_score config", err, nil)
	}
	maxNGram := *config.MaxNGram

	candidateText := normalizeGenerationText(actual, config.CaseInsensitive, config.Normalize)
	candidateTokens := strings.Fields(candidateText)
	references, err := parseGenerationReferences(expected, config.CaseInsensitive, config.Normalize)
	if err != nil {
		return validatorError("parse bleu_score references", err, map[string]any{"expected_raw": expected})
	}
	if len(references) == 0 {
		return validatorError("parse bleu_score references", fmt.Errorf("at least one reference is required"), nil)
	}

	refTokens := make([][]string, 0, len(references))
	refLengths := make([]int, 0, len(references))
	for _, reference := range references {
		tokens := strings.Fields(reference)
		refTokens = append(refTokens, tokens)
		refLengths = append(refLengths, len(tokens))
	}

	candidateLength := len(candidateTokens)
	if candidateLength == 0 {
		score := 0.0
		if len(refTokens) == 1 && len(refTokens[0]) == 0 {
			score = 1.0
		}
		return thresholdedOutcome(score, effectiveThreshold(config.Threshold), map[string]any{
			"candidate_length":  candidateLength,
			"reference_lengths": refLengths,
			"max_ngram":         maxNGram,
			"smoothing":         config.Smoothing,
			"brevity_penalty":   score,
			"precisions":        []float64{},
		})
	}

	maxOrder := min(maxNGram, candidateLength)
	precisions := make([]float64, 0, maxOrder)
	sumLogPrecisions := 0.0
	for n := 1; n <= maxOrder; n++ {
		total := len(candidateTokens) - n + 1
		if total <= 0 {
			precisions = append(precisions, 0)
			sumLogPrecisions = math.Inf(-1)
			continue
		}

		candidateCounts := tokenNGramCounts(candidateTokens, n)
		maxReferenceCounts := map[string]int{}
		for _, reference := range refTokens {
			for gram, count := range tokenNGramCounts(reference, n) {
				if count > maxReferenceCounts[gram] {
					maxReferenceCounts[gram] = count
				}
			}
		}

		matches := 0
		for gram, count := range candidateCounts {
			matches += min(count, maxReferenceCounts[gram])
		}

		precision := 0.0
		switch {
		case matches > 0:
			precision = float64(matches) / float64(total)
		case config.Smoothing == bleuSmoothingMethod1 && n > 1:
			// Match Chen & Cherry's method1 behavior: only smooth higher-order
			// precisions after unigram overlap has already gone to zero.
			precision = 1.0 / float64(total+1)
		}
		precisions = append(precisions, precision)
		if precision == 0 {
			sumLogPrecisions = math.Inf(-1)
		} else if !math.IsInf(sumLogPrecisions, -1) {
			sumLogPrecisions += math.Log(precision)
		}
	}

	geoMean := 0.0
	if !math.IsInf(sumLogPrecisions, -1) {
		geoMean = math.Exp(sumLogPrecisions / float64(maxOrder))
	}

	referenceLength := chooseBLEUReferenceLength(candidateLength, refLengths)
	brevityPenalty := 1.0
	if candidateLength < referenceLength {
		brevityPenalty = math.Exp(1 - float64(referenceLength)/float64(candidateLength))
	}
	score := brevityPenalty * geoMean

	return thresholdedOutcome(score, effectiveThreshold(config.Threshold), map[string]any{
		"candidate_length":  candidateLength,
		"reference_length":  referenceLength,
		"reference_lengths": refLengths,
		"max_ngram":         maxNGram,
		"computed_orders":   maxOrder,
		"smoothing":         config.Smoothing,
		"brevity_penalty":   brevityPenalty,
		"precisions":        precisions,
	})
}

func validateROUGEScore(actual string, expected string, rawConfig json.RawMessage) validatorOutcome {
	config, err := parseROUGEScoreConfig(rawConfig)
	if err != nil {
		return validatorError("parse rouge_score config", err, nil)
	}

	candidateTokens := strings.Fields(normalizeGenerationText(actual, config.CaseInsensitive, config.Normalize))
	references, err := parseGenerationReferences(expected, config.CaseInsensitive, config.Normalize)
	if err != nil {
		return validatorError("parse rouge_score references", err, map[string]any{"expected_raw": expected})
	}
	if len(references) == 0 {
		return validatorError("parse rouge_score references", fmt.Errorf("at least one reference is required"), nil)
	}

	beta := 1.0
	if config.Beta != nil {
		beta = *config.Beta
	}

	bestScore := -1.0
	bestEvidence := map[string]any{}
	for _, reference := range references {
		referenceTokens := strings.Fields(reference)
		precision, recall, fScore := rougePrecisionRecallFScore(candidateTokens, referenceTokens, config.Variant, beta)
		if fScore > bestScore {
			bestScore = fScore
			bestEvidence = map[string]any{
				"variant":          config.Variant,
				"precision":        precision,
				"recall":           recall,
				"f_score":          fScore,
				"candidate_length": len(candidateTokens),
				"reference_length": len(referenceTokens),
				"beta":             beta,
			}
		}
	}

	return thresholdedOutcome(bestScore, effectiveThreshold(config.Threshold), bestEvidence)
}

func validateChrFScore(actual string, expected string, rawConfig json.RawMessage) validatorOutcome {
	config, err := parseChrFScoreConfig(rawConfig)
	if err != nil {
		return validatorError("parse chrf_score config", err, nil)
	}
	charOrder := *config.CharOrder

	candidateRunes := chrfRunes(normalizeGenerationText(actual, config.CaseInsensitive, config.Normalize))
	references, err := parseGenerationReferences(expected, config.CaseInsensitive, config.Normalize)
	if err != nil {
		return validatorError("parse chrf_score references", err, map[string]any{"expected_raw": expected})
	}
	if len(references) == 0 {
		return validatorError("parse chrf_score references", fmt.Errorf("at least one reference is required"), nil)
	}

	beta := 2.0
	if config.Beta != nil {
		beta = *config.Beta
	}

	bestScore := -1.0
	bestEvidence := map[string]any{}
	for _, reference := range references {
		referenceRunes := chrfRunes(reference)
		maxOrder := min(charOrder, min(len(candidateRunes), len(referenceRunes)))
		if maxOrder == 0 {
			score := 0.0
			if len(candidateRunes) == 0 && len(referenceRunes) == 0 {
				score = 1.0
			}
			if score > bestScore {
				bestScore = score
				bestEvidence = map[string]any{
					"char_order":       charOrder,
					"computed_orders":  0,
					"beta":             beta,
					"precision":        score,
					"recall":           score,
					"f_score":          score,
					"candidate_length": len(candidateRunes),
					"reference_length": len(referenceRunes),
					"precisions":       []float64{},
					"recalls":          []float64{},
				}
			}
			continue
		}
		precisions := make([]float64, 0, maxOrder)
		recalls := make([]float64, 0, maxOrder)
		for n := 1; n <= maxOrder; n++ {
			precision, recall := charNGramPrecisionRecall(candidateRunes, referenceRunes, n)
			precisions = append(precisions, precision)
			recalls = append(recalls, recall)
		}
		meanPrecision := averageFloatSlice(precisions)
		meanRecall := averageFloatSlice(recalls)
		score := fScore(meanPrecision, meanRecall, beta)
		if score > bestScore {
			bestScore = score
			bestEvidence = map[string]any{
				"char_order":       charOrder,
				"computed_orders":  maxOrder,
				"beta":             beta,
				"precision":        meanPrecision,
				"recall":           meanRecall,
				"f_score":          score,
				"candidate_length": len(candidateRunes),
				"reference_length": len(referenceRunes),
				"precisions":       precisions,
				"recalls":          recalls,
			}
		}
	}

	return thresholdedOutcome(bestScore, effectiveThreshold(config.Threshold), bestEvidence)
}

func parseBLEUScoreConfig(rawConfig json.RawMessage) (bleuScoreConfig, error) {
	cfg := bleuScoreConfig{
		Smoothing: bleuSmoothingNone,
	}
	defaultMaxNGram := 4
	cfg.MaxNGram = &defaultMaxNGram
	if len(rawConfig) == 0 {
		return cfg, nil
	}
	if err := decodeStrictJSON(rawConfig, &cfg); err != nil {
		return bleuScoreConfig{}, err
	}
	if cfg.Smoothing == "" {
		cfg.Smoothing = bleuSmoothingNone
	}
	return cfg, nil
}

func parseROUGEScoreConfig(rawConfig json.RawMessage) (rougeScoreConfig, error) {
	cfg := rougeScoreConfig{
		Variant: rougeVariantL,
	}
	if len(rawConfig) == 0 {
		return cfg, nil
	}
	if err := decodeStrictJSON(rawConfig, &cfg); err != nil {
		return rougeScoreConfig{}, err
	}
	if cfg.Variant == "" {
		cfg.Variant = rougeVariantL
	}
	return cfg, nil
}

func parseChrFScoreConfig(rawConfig json.RawMessage) (chrfScoreConfig, error) {
	defaultCharOrder := 6
	cfg := chrfScoreConfig{CharOrder: &defaultCharOrder}
	if len(rawConfig) == 0 {
		return cfg, nil
	}
	if err := decodeStrictJSON(rawConfig, &cfg); err != nil {
		return chrfScoreConfig{}, err
	}
	return cfg, nil
}

func parseGenerationReferences(expected string, caseInsensitive bool, normalize bool) ([]string, error) {
	trimmed := strings.TrimSpace(expected)
	if trimmed == "" {
		return []string{normalizeGenerationText("", caseInsensitive, normalize)}, nil
	}

	var references []string
	if strings.HasPrefix(trimmed, "[") {
		if err := decodeStrictJSON([]byte(trimmed), &references); err != nil {
			// Fall back to a single literal reference so inputs that happen to
			// start with "[" do not fail unexpectedly unless they decode to the
			// wrong JSON shape.
			var typedReferences []any
			if typedErr := decodeStrictJSON([]byte(trimmed), &typedReferences); typedErr == nil {
				return nil, fmt.Errorf("expected a JSON array of reference strings")
			}
			references = []string{expected}
		}
	} else {
		references = []string{expected}
	}

	normalized := make([]string, 0, len(references))
	for _, reference := range references {
		normalized = append(normalized, normalizeGenerationText(reference, caseInsensitive, normalize))
	}
	return normalized, nil
}

func normalizeGenerationText(text string, caseInsensitive bool, normalize bool) string {
	if normalize {
		text = collapseWhitespace(strings.TrimSpace(text))
	}
	if caseInsensitive {
		text = cases.Fold().String(text)
	}
	return text
}

func chooseBLEUReferenceLength(candidateLength int, referenceLengths []int) int {
	bestLength := 0
	bestDistance := math.MaxInt
	found := false
	for _, referenceLength := range referenceLengths {
		distance := absInt(candidateLength - referenceLength)
		if !found || distance < bestDistance || (distance == bestDistance && referenceLength < bestLength) {
			bestDistance = distance
			bestLength = referenceLength
			found = true
		}
	}
	return bestLength
}

func tokenNGramCounts(tokens []string, n int) map[string]int {
	counts := map[string]int{}
	for i := 0; i+n <= len(tokens); i++ {
		gram := strings.Join(tokens[i:i+n], "\x00")
		counts[gram]++
	}
	return counts
}

func rougePrecisionRecallFScore(candidate []string, reference []string, variant string, beta float64) (float64, float64, float64) {
	switch variant {
	case rougeVariant1:
		return overlapPrecisionRecallFScore(candidate, reference, 1, beta)
	case rougeVariant2:
		return overlapPrecisionRecallFScore(candidate, reference, 2, beta)
	case rougeVariantL:
		lcs := longestCommonSubsequence(candidate, reference)
		precision := safeRatio(float64(lcs), float64(len(candidate)))
		recall := safeRatio(float64(lcs), float64(len(reference)))
		return precision, recall, fScore(precision, recall, beta)
	default:
		return 0, 0, 0
	}
}

func overlapPrecisionRecallFScore(candidate []string, reference []string, n int, beta float64) (float64, float64, float64) {
	candidateCounts := tokenNGramCounts(candidate, n)
	referenceCounts := tokenNGramCounts(reference, n)
	matches := 0
	for gram, count := range candidateCounts {
		matches += min(count, referenceCounts[gram])
	}
	candidateTotal := max(len(candidate)-n+1, 0)
	referenceTotal := max(len(reference)-n+1, 0)
	precision := safeRatio(float64(matches), float64(candidateTotal))
	recall := safeRatio(float64(matches), float64(referenceTotal))
	return precision, recall, fScore(precision, recall, beta)
}

func longestCommonSubsequence(a []string, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1] + 1
			} else {
				curr[j] = max(prev[j], curr[j-1])
			}
		}
		prev, curr = curr, prev
		clear(curr)
	}
	return prev[len(b)]
}

func chrfRunes(text string) []rune {
	runes := make([]rune, 0, len(text))
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		runes = append(runes, r)
	}
	return runes
}

func charNGramPrecisionRecall(candidate []rune, reference []rune, n int) (float64, float64) {
	candidateCounts := runeNGramCounts(candidate, n)
	referenceCounts := runeNGramCounts(reference, n)
	matches := 0
	for gram, count := range candidateCounts {
		matches += min(count, referenceCounts[gram])
	}
	candidateTotal := max(len(candidate)-n+1, 0)
	referenceTotal := max(len(reference)-n+1, 0)
	return safeRatio(float64(matches), float64(candidateTotal)), safeRatio(float64(matches), float64(referenceTotal))
}

func runeNGramCounts(runes []rune, n int) map[string]int {
	counts := map[string]int{}
	for i := 0; i+n <= len(runes); i++ {
		gram := string(runes[i : i+n])
		counts[gram]++
	}
	return counts
}

func averageFloatSlice(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func fScore(precision float64, recall float64, beta float64) float64 {
	if precision == 0 && recall == 0 {
		return 0
	}
	betaSquared := beta * beta
	return (1 + betaSquared) * precision * recall / ((betaSquared * precision) + recall)
}

func safeRatio(numerator float64, denominator float64) float64 {
	if denominator <= 0 {
		// "Positive matches with zero denominator" is unreachable for the overlap
		// metrics here because matches are bounded by both n-gram totals.
		return 0
	}
	return numerator / denominator
}

func effectiveThreshold(value *float64) float64 {
	return thresholdOrDefault(value, defaultGenerationThreshold)
}

func thresholdOrDefault(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	if *value < 0 {
		return 0
	}
	if *value > 1 {
		return 1
	}
	return *value
}

func thresholdedOutcome(score float64, threshold float64, evidence map[string]any) validatorOutcome {
	evidence["score"] = score
	evidence["threshold"] = threshold
	if score >= threshold {
		return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(score), evidence: evidence}
	}
	return validatorOutcome{
		verdict:         "fail",
		normalizedScore: floatPtr(score),
		reason:          fmt.Sprintf("score %.4f is below threshold %.4f", score, threshold),
		evidence:        evidence,
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
