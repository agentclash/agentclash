package judge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
)

// modelStats is the per-model aggregated view used during rubric
// aggregation. Lives at package scope so rubricStatsPayloadsFromStats
// can take a concrete slice type instead of a brittle type assertion.
// Unexported because it's purely an implementation detail of
// aggregateRubric and the payload converter.
type modelStats struct {
	model          string
	samples        []float64
	abstainCount   int
	errorCount     int
	median         float64
	variance       float64
	hasValidSample bool
}

// Phase 5 of issue #148 — rubric and reference mode dispatch.
//
// Rubric and reference share one evaluator: reference is rubric with
// an additional REFERENCE ANSWER block injected into the prompt
// envelope. See backend/.claude/analysis/issue-148-deep-analysis.md
// Part 5 lines 519-525 for the rationale.
//
// Pipeline:
//
//  1. Resolve effective scale + output schema
//  2. Emit rubric quality warning if rubric is < 15 words
//  3. Resolve reference text (reference mode only) — state=unavailable
//     if the declared ReferenceFrom fails to resolve
//  4. Build (models × samples) provider calls via shared fanOut helper
//  5. Each call: prompt -> InvokeModel -> parseRubricResponse -> normalize
//  6. Aggregate per-model via median, variance, confidence binning
//  7. Cross-model consensus (median / mean / unanimous)
//  8. Final normalized score or OutputStateUnavailable

// evaluateRubric handles both JudgeMethodRubric and JudgeMethodReference.
// The dispatch lives here because the two modes share 90% of their
// implementation — only the prompt envelope and the ReferenceFrom
// resolution differ.
func (e *Evaluator) evaluateRubric(ctx context.Context, judge scoring.LLMJudgeDeclaration, in Input) scoring.JudgeResult {
	result := scoring.JudgeResult{
		Key:  judge.Key,
		Mode: judge.Mode,
	}

	// Resolve the reference text for reference mode. Must happen
	// BEFORE we run any LLM calls so a misconfigured pack fails fast
	// and returns OutputStateUnavailable (Phase 5 plan Q3 resolution:
	// missing evidence is "unavailable", not "error").
	var referenceText string
	if judge.Mode == scoring.JudgeMethodReference {
		ref := strings.TrimSpace(judge.ReferenceFrom)
		if ref == "" {
			result.State = scoring.OutputStateError
			result.Reason = "reference mode judge has no reference_from field"
			return result
		}
		if in.ResolvedReferences == nil {
			result.State = scoring.OutputStateUnavailable
			result.Reason = fmt.Sprintf("reference text unavailable for %q", ref)
			return result
		}
		referenceText = strings.TrimSpace(in.ResolvedReferences[ref])
		if referenceText == "" {
			result.State = scoring.OutputStateUnavailable
			result.Reason = fmt.Sprintf("reference text unavailable for %q", ref)
			return result
		}
	}

	schema, schemaErr := resolveJudgeSchema(judge, defaultRubricSchema)
	if schemaErr != nil {
		result.State = scoring.OutputStateError
		result.Reason = fmt.Sprintf("parse judge output schema: %v", schemaErr)
		return result
	}

	calls, buildErr := buildRubricCalls(judge, in, referenceText, e.cfg)
	if buildErr != nil {
		result.State = scoring.OutputStateError
		result.Reason = fmt.Sprintf("build rubric calls: %v", buildErr)
		return result
	}

	scale := effectiveScoreScale(judge)

	outcomes := e.fanOut(ctx, calls, func(ctx context.Context, call providerCall) sampleOutcome {
		return e.runRubricCall(ctx, call, schema, scale)
	})

	return aggregateRubric(judge, outcomes, scale)
}

// buildRubricCalls plans every provider.Request for a rubric/reference
// judge. One call per (model, sample_index) pair. The prompt envelope
// is fixed across samples for a given judge — only the sampled
// response varies — so we assemble it once and reuse.
func buildRubricCalls(
	judge scoring.LLMJudgeDeclaration,
	in Input,
	referenceText string,
	cfg Config,
) ([]providerCall, error) {
	models := resolveRubricModels(judge, cfg)
	if len(models) == 0 {
		return nil, errors.New("judge has no models")
	}

	samples := judge.Samples
	if samples <= 0 {
		samples = scoring.JudgeDefaultSamples
	}
	if samples > scoring.JudgeMaxSamplesCeiling {
		samples = scoring.JudgeMaxSamplesCeiling
	}

	systemPrompt, userPrompt := buildRubricPrompt(judge, in.FinalOutput, referenceText, in.ResolvedReferences)

	timeout := cfg.DefaultTimeout
	if judge.TimeoutMS != nil && *judge.TimeoutMS > 0 {
		timeout = time.Duration(*judge.TimeoutMS) * time.Millisecond
	}

	calls := make([]providerCall, 0, len(models)*samples)
	for _, model := range models {
		providerKey, resolveErr := resolveProviderKey(model, cfg)
		if resolveErr != nil {
			return nil, resolveErr
		}
		for sampleIdx := 0; sampleIdx < samples; sampleIdx++ {
			calls = append(calls, providerCall{
				Model:       model,
				SampleIndex: sampleIdx,
				Request: provider.Request{
					ProviderKey:         providerKey,
					CredentialReference: cfg.CredentialReference,
					Model:               model,
					StepTimeout:         timeout,
					Messages: []provider.Message{
						{Role: "system", Content: systemPrompt},
						{Role: "user", Content: userPrompt},
					},
				},
			})
		}
	}
	return calls, nil
}

// resolveRubricModels mirrors resolveJudgeModels from assertion.go but
// falls back to the rubric-specific default (claude-sonnet-4-6) when
// the judge declares no model. Rubric benefits from a stronger model
// than assertion because it has to reason over numeric calibration
// and structured output, whereas assertion is a yes/no decision that
// Haiku handles well.
func resolveRubricModels(judge scoring.LLMJudgeDeclaration, cfg Config) []string {
	switch {
	case strings.TrimSpace(judge.Model) != "":
		return []string{strings.TrimSpace(judge.Model)}
	case len(judge.Models) > 0:
		seen := make(map[string]struct{}, len(judge.Models))
		out := make([]string, 0, len(judge.Models))
		for _, m := range judge.Models {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			out = append(out, m)
		}
		return out
	default:
		return []string{cfg.DefaultRubricModel}
	}
}

// runRubricCall invokes the provider for a single (model, sample)
// tuple and parses the response. Per-call error handling:
//
//   - Provider returns error → sampleOutcome.Error set, Score nil.
//     Counted as "error" by the aggregator.
//   - Response JSON parse fails → Score nil, Reason explains.
//     Counted as "abstain" by the aggregator.
//   - Response is an unable_to_judge abstain → Score nil, Reason
//     prefixed with "unknown:" to distinguish from parse failures.
//     Also counted as abstain.
//   - Response parses cleanly with a numeric score → Score populated
//     with the NORMALIZED [0, 1] value after scale mapping.
func (e *Evaluator) runRubricCall(
	ctx context.Context,
	call providerCall,
	schema *jsonschema.Schema,
	scale scoring.ScoreScale,
) sampleOutcome {
	response, err := e.router.InvokeModel(ctx, call.Request)
	if err != nil {
		return sampleOutcome{
			Model:       call.Model,
			SampleIndex: call.SampleIndex,
			Error:       err,
			Reason:      fmt.Sprintf("provider call failed: %v", err),
		}
	}

	parsed, ok := parseRubricResponse(response.OutputText, schema)
	if !ok {
		return sampleOutcome{
			Model:       call.Model,
			SampleIndex: call.SampleIndex,
			Reason:      "response did not parse as valid rubric JSON",
			Usage:       response.Usage,
			RawOutput:   response.OutputText,
		}
	}

	if parsed.UnableToJudge {
		return sampleOutcome{
			Model:       call.Model,
			SampleIndex: call.SampleIndex,
			Reason:      "unknown: " + parsed.AbstainReason,
			Usage:       response.Usage,
			RawOutput:   response.OutputText,
		}
	}

	if parsed.Score == nil {
		// Defensive: parseRubricResponse should never return ok=true
		// without either UnableToJudge or Score set. Guard against
		// future edits breaking the invariant.
		return sampleOutcome{
			Model:       call.Model,
			SampleIndex: call.SampleIndex,
			Reason:      "rubric response has no score and no abstain flag",
			Usage:       response.Usage,
			RawOutput:   response.OutputText,
		}
	}

	normalized := scaleNormalize(*parsed.Score, scale)
	return sampleOutcome{
		Model:       call.Model,
		SampleIndex: call.SampleIndex,
		Score:       &normalized,
		Reason:      parsed.Reasoning,
		Usage:       response.Usage,
		RawOutput:   response.OutputText,
	}
}

// aggregateRubric collapses per-sample outcomes into a single
// scoring.JudgeResult. Pipeline:
//
//  1. Bucket outcomes by model.
//  2. Per-model: median of normalized samples, variance, abstain count.
//  3. Cross-model: single model → its median. Multi-model →
//     applyNumericConsensus with the judge's ConsensusConfig.
//  4. Overall variance = max per-model variance (conservative).
//  5. Confidence = deriveRubricConfidence(total, abstain, variance).
//  6. Rubric quality warning embedded in Reason when rubric is vague.
//  7. Payload jsonb built from the per-model breakdown.
//
// Ties in multi-model consensus are NOT broken here — the consensus
// helper encodes the specific tie semantics (median of evens averages
// the two middles, etc).
func aggregateRubric(
	judge scoring.LLMJudgeDeclaration,
	outcomes []sampleOutcome,
	scale scoring.ScoreScale,
) scoring.JudgeResult {
	result := scoring.JudgeResult{
		Key:  judge.Key,
		Mode: judge.Mode,
	}
	if qualityWarning := checkRubricQuality(judge); qualityWarning != "" {
		result.Reason = qualityWarning
	}

	if len(outcomes) == 0 {
		result.State = scoring.OutputStateUnavailable
		if result.Reason != "" {
			result.Reason = result.Reason + "; no rubric samples executed"
		} else {
			result.Reason = "no rubric samples executed"
		}
		return result
	}

	// Bucket by model in appearance order for stable test output.
	byModel := make(map[string][]sampleOutcome)
	modelOrder := make([]string, 0)
	for _, o := range outcomes {
		if _, seen := byModel[o.Model]; !seen {
			modelOrder = append(modelOrder, o.Model)
		}
		byModel[o.Model] = append(byModel[o.Model], o)
	}

	stats := make([]modelStats, 0, len(modelOrder))
	totalSamples := 0
	totalAbstain := 0
	totalErrors := 0
	for _, model := range modelOrder {
		s := modelStats{model: model}
		for _, o := range byModel[model] {
			totalSamples++
			switch {
			case o.Error != nil:
				s.errorCount++
				totalErrors++
			case o.Score == nil:
				s.abstainCount++
				totalAbstain++
			default:
				s.samples = append(s.samples, *o.Score)
			}
		}
		if len(s.samples) > 0 {
			s.median = medianFloats(s.samples)
			s.variance = populationVariance(s.samples)
			s.hasValidSample = true
		}
		stats = append(stats, s)
	}

	result.SampleCount = totalSamples
	result.ModelCount = len(modelOrder)

	// Collect per-model medians for cross-model consensus.
	modelScores := make(map[string]float64, len(stats))
	maxVariance := 0.0
	for _, s := range stats {
		if !s.hasValidSample {
			continue
		}
		modelScores[s.model] = s.median
		if s.variance > maxVariance {
			maxVariance = s.variance
		}
	}

	payloadStats := rubricStatsPayloadsFromStats(stats)

	if len(modelScores) == 0 {
		// Every model abstained or errored across every sample.
		result.State = scoring.OutputStateUnavailable
		if result.Reason == "" {
			result.Reason = "every model abstained or errored on rubric"
		} else {
			result.Reason = result.Reason + "; every model abstained or errored on rubric"
		}
		result.Confidence = "low"
		result.Variance = 0
		result.Payload = mustMarshalRubricPayload(judge, payloadStats, nil, scale, false)
		return result
	}

	var finalScore float64
	var consensusReason string
	switch {
	case len(modelScores) == 1:
		for _, v := range modelScores {
			finalScore = v
		}
	case judge.Consensus != nil:
		decided, reason, ok := applyNumericConsensus(modelScores, *judge.Consensus)
		if !ok {
			result.State = scoring.OutputStateUnavailable
			result.Reason = reason
			result.Confidence = "low"
			result.Variance = maxVariance
			result.Payload = mustMarshalRubricPayload(judge, payloadStats, nil, scale, false)
			return result
		}
		finalScore = decided
		consensusReason = reason
	default:
		// Validation rejects multi-model numeric judges without
		// consensus config. Defensive fallback for programmatic
		// constructions (mostly tests).
		result.State = scoring.OutputStateError
		result.Reason = "multi-model rubric judge requires consensus config"
		result.Payload = mustMarshalRubricPayload(judge, payloadStats, nil, scale, false)
		return result
	}

	// Abstain-majority gate: if more than half the samples abstained
	// or errored, the signal is too weak to trust even if the
	// surviving samples agreed. Surface as unavailable.
	if totalSamples > 0 && float64(totalAbstain+totalErrors)/float64(totalSamples) > 0.5 {
		result.State = scoring.OutputStateUnavailable
		if result.Reason == "" {
			result.Reason = "more than half of rubric samples abstained or errored"
		} else {
			result.Reason = result.Reason + "; more than half of rubric samples abstained or errored"
		}
		result.Confidence = "low"
		result.Variance = maxVariance
		result.Payload = mustMarshalRubricPayload(judge, payloadStats, nil, scale, false)
		return result
	}

	scoreCopy := finalScore
	result.State = scoring.OutputStateAvailable
	result.NormalizedScore = &scoreCopy
	result.Variance = maxVariance
	result.Confidence = deriveRubricConfidence(totalSamples, totalAbstain+totalErrors, maxVariance)
	if consensusReason != "" {
		if result.Reason == "" {
			result.Reason = consensusReason
		} else {
			result.Reason = result.Reason + "; " + consensusReason
		}
	}
	result.Payload = mustMarshalRubricPayload(judge, payloadStats, &scoreCopy, scale, true)
	return result
}

// applyNumericConsensus collapses per-model numeric scores into a
// single cross-model score according to the ConsensusConfig. Only
// median, mean, and unanimous apply to numeric modes; majority_vote
// is rejected at validation for numeric modes (Phase 1 rule 6 in
// validation_judges.go).
//
// unanimous for numeric modes means "per-model spread ≤
// MinAgreementThreshold." When MinAgreementThreshold is 0, any spread
// > 0 fails — matching the intuitive "models must agree exactly."
// Returns the mean of the per-model scores when the agreement check
// passes (a single representative point for the "unanimous cluster").
func applyNumericConsensus(modelScores map[string]float64, cfg scoring.ConsensusConfig) (float64, string, bool) {
	if len(modelScores) == 0 {
		return 0, "no per-model scores available for consensus", false
	}
	values := make([]float64, 0, len(modelScores))
	for _, v := range modelScores {
		values = append(values, v)
	}

	switch cfg.Aggregation {
	case scoring.ConsensusAggMedian:
		return medianFloats(values), fmt.Sprintf("median across %d models", len(values)), true
	case scoring.ConsensusAggMean:
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values)), fmt.Sprintf("mean across %d models", len(values)), true
	case scoring.ConsensusAggUnanimous:
		spread := spreadOf(values)
		threshold := cfg.MinAgreementThreshold
		if threshold <= 0 {
			// Default tolerance when the pack doesn't specify: 0.05
			// on the normalized [0, 1] scale. This is permissive
			// enough for rubric noise but still catches real
			// disagreements.
			threshold = 0.05
		}
		if spread > threshold {
			return 0, fmt.Sprintf("unanimous required but models disagree: spread %.4f > threshold %.4f", spread, threshold), false
		}
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values)), fmt.Sprintf("unanimous (spread %.4f ≤ %.4f)", spread, threshold), true
	default:
		return 0, fmt.Sprintf("consensus aggregation %q is not supported for rubric mode", cfg.Aggregation), false
	}
}

// spreadOf returns max - min over a non-empty float slice. The rubric
// unanimous consensus uses this as the agreement metric: all per-model
// scores must fit within a MinAgreementThreshold band.
func spreadOf(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	max := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return max - min
}

// medianFloats returns the median of a non-empty float slice. Even-
// count slices return the arithmetic mean of the two middles
// (standard mathematical convention). Sorts a copy so the caller's
// slice is not mutated.
func medianFloats(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// populationVariance returns the population variance of a slice:
//
//	variance = sum((x_i - mean)^2) / n
//
// Population (not sample) variance matches the analysis doc Part 5
// line 446 specification. Single-sample inputs return 0 (the
// distribution has no spread), and the caller's confidence binning
// downgrades that to "medium" to avoid overclaiming.
func populationVariance(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	sumSquares := 0.0
	for _, v := range values {
		d := v - mean
		sumSquares += d * d
	}
	variance := sumSquares / float64(len(values))
	// Guard against floating-point noise producing tiny negative
	// variances from near-identical inputs.
	if variance < 0 {
		return 0
	}
	// Guard against NaN / Inf from pathological inputs (shouldn't
	// happen with clamped [0, 1] values, but defend anyway).
	if math.IsNaN(variance) || math.IsInf(variance, 0) {
		return 0
	}
	return variance
}

// deriveRubricConfidence bins the (sample count, abstain rate,
// variance) triple into high/medium/low:
//
//   - Zero valid samples                   → "low"
//   - Single-shot (1 valid sample)         → "medium"
//   - variance <  0.01                     → "high"
//   - variance <  0.05                     → "medium"
//   - else                                 → "low"
//
// The variance thresholds come from analysis doc Part 5 lines 447-449.
// Single-shot is explicitly downgraded from "high" to "medium"
// because a single sample has no measurable variance — reporting
// "high confidence" on one datapoint would overclaim.
func deriveRubricConfidence(total, abstainOrError int, variance float64) string {
	validSamples := total - abstainOrError
	if validSamples <= 0 {
		return "low"
	}
	if validSamples == 1 {
		return "medium"
	}
	if variance < 0.01 {
		return "high"
	}
	if variance < 0.05 {
		return "medium"
	}
	return "low"
}

// checkRubricQuality runs the vague-rubric check documented in the
// analysis doc Part 7 rule 9 (Phase 5 plan question Q2 resolution:
// threshold is 15 words). Returns a non-empty warning string when
// the rubric text is shorter than 15 whitespace-separated words;
// empty string when the rubric is healthy.
//
// The warning is surfaced via the JudgeResult.Reason field (appended
// to any consensus/abstain reasons via "; " separators) so operators
// see it in both the scoring events and the finalize-path warnings.
// Non-blocking — vague rubrics still run, they just flag themselves.
func checkRubricQuality(judge scoring.LLMJudgeDeclaration) string {
	wordCount := len(strings.Fields(judge.Rubric))
	if wordCount >= 15 {
		return ""
	}
	return fmt.Sprintf("judge %q rubric is fewer than 15 words (%d); vague rubrics produce inconsistent judgments — consider expanding", judge.Key, wordCount)
}

// modelRubricStatsPayload is the jsonb-serialisable shape for one
// model's aggregated breakdown. Mirrors modelTallyPayload from
// assertion.go but with numeric fields instead of boolean.
type modelRubricStatsPayload struct {
	Model        string    `json:"model"`
	Samples      []float64 `json:"samples"`
	SampleCount  int       `json:"sample_count"`
	Median       *float64  `json:"median,omitempty"`
	Variance     float64   `json:"variance"`
	AbstainCount int       `json:"abstain_count"`
	ErrorCount   int       `json:"error_count"`
}

// rubricStatsPayloadsFromStats converts the internal pipeline type
// into the serialisable jsonb shape. Straight field-by-field copy;
// kept as a helper so the aggregator reads as a clean pipeline.
func rubricStatsPayloadsFromStats(stats []modelStats) []modelRubricStatsPayload {
	out := make([]modelRubricStatsPayload, 0, len(stats))
	for _, s := range stats {
		payload := modelRubricStatsPayload{
			Model:        s.model,
			Samples:      append([]float64(nil), s.samples...),
			SampleCount:  len(s.samples) + s.abstainCount + s.errorCount,
			Variance:     s.variance,
			AbstainCount: s.abstainCount,
			ErrorCount:   s.errorCount,
		}
		if s.hasValidSample {
			median := s.median
			payload.Median = &median
		}
		out = append(out, payload)
	}
	// Sort by model name for deterministic JSON output across test
	// runs (map iteration order is nondeterministic in Go, so
	// upstream consumers can't rely on any stable order otherwise).
	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return out
}

// mustMarshalRubricPayload serialises the per-model breakdown into
// the jsonb payload shape persisted in llm_judge_results.payload.
// Never errors in practice — the inputs are simple types — but
// returns an empty {} fallback so callers never have to special-
// case a nil payload.
func mustMarshalRubricPayload(
	judge scoring.LLMJudgeDeclaration,
	stats []modelRubricStatsPayload,
	finalScore *float64,
	scale scoring.ScoreScale,
	available bool,
) json.RawMessage {
	payload := struct {
		Mode       string                    `json:"mode"`
		Judge      string                    `json:"judge"`
		Available  bool                      `json:"available"`
		FinalScore *float64                  `json:"final_score,omitempty"`
		ScoreScale scoring.ScoreScale        `json:"score_scale"`
		Stats      []modelRubricStatsPayload `json:"stats"`
		Consensus  *scoring.ConsensusConfig  `json:"consensus,omitempty"`
	}{
		Mode:       string(judge.Mode),
		Judge:      judge.Key,
		Available:  available,
		FinalScore: finalScore,
		ScoreScale: scale,
		Stats:      stats,
		Consensus:  judge.Consensus,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}
