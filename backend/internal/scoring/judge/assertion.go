package judge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
)

// evaluateAssertion runs an assertion-mode judge end-to-end: build the
// call list (models × samples), fan out via the shared helper, parse
// each verdict, and aggregate with per-model majority vote + optional
// cross-model consensus.
//
// Failure mode matrix:
//
//   - Every call errors or abstains → state=unavailable, NormalizedScore=nil.
//   - Some calls error/abstain → remaining valid calls vote. Confidence
//     drops with abstain/error rate.
//   - All calls vote cleanly but models disagree under unanimous
//     consensus → state=unavailable, reason cites disagreement.
//   - Ties break toward NO (safer default for safety assertions).
//   - Final verdict compared against judge.Expect (default true);
//     matching → score=1.0, non-matching → score=0.0.
func (e *Evaluator) evaluateAssertion(ctx context.Context, judge scoring.LLMJudgeDeclaration, in Input) scoring.JudgeResult {
	calls, err := buildAssertionCalls(judge, in, e.cfg)
	if err != nil {
		return scoring.JudgeResult{
			Key:    judge.Key,
			Mode:   scoring.JudgeMethodAssertion,
			State:  scoring.OutputStateError,
			Reason: fmt.Sprintf("build assertion calls: %v", err),
		}
	}

	outcomes := e.fanOut(ctx, calls, e.runAssertionCall)
	return aggregateAssertion(judge, outcomes)
}

// buildAssertionCalls plans every provider.Request the judge needs to
// dispatch. One call per (model, sample_index) pair. Errors when the
// judge references a model with no resolvable provider or when
// validation invariants are violated (should have been caught earlier
// but we guard defensively — judges can be constructed programmatically
// in tests).
func buildAssertionCalls(judge scoring.LLMJudgeDeclaration, in Input, cfg Config) ([]providerCall, error) {
	models := resolveJudgeModels(judge, cfg)
	if len(models) == 0 {
		return nil, errors.New("judge has no models")
	}

	samples := judge.Samples
	if samples <= 0 {
		samples = scoring.JudgeDefaultSamples
	}

	systemPrompt, userPrompt := buildAssertionPrompt(judge, in.FinalOutput, in.ChallengeInput)

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

// resolveJudgeModels returns the ordered list of models the judge
// should invoke. Explicit judge.Model wins over judge.Models (which
// validation rejects if both are set). When neither is set the
// evaluator falls back to Config.DefaultAssertionModel.
//
// Deduplication is intentional: a Models slice with duplicates would
// multiply the call count unnecessarily, and multi-model consensus
// semantics assume distinct models.
func resolveJudgeModels(judge scoring.LLMJudgeDeclaration, cfg Config) []string {
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
		return []string{cfg.DefaultAssertionModel}
	}
}

// runAssertionCall is the fanOut callback — it invokes the provider
// router for one (model, sample) tuple and parses the response.
//
// Per-call error handling:
//   - Provider returns an error → outcome.Error set, outcome.Verdict
//     nil. The aggregator counts this as "error" not "abstain".
//   - Parse fails entirely → outcome.Verdict nil, Reason explains.
//     The aggregator counts this as "abstain".
//   - Parse yields UNKNOWN → outcome.Verdict nil (ok=true path),
//     distinguished from parse-fail by Reason starting with
//     "unknown:". Also counted as abstain.
//   - Parse yields YES/NO → outcome.Verdict set.
func (e *Evaluator) runAssertionCall(ctx context.Context, call providerCall) sampleOutcome {
	response, err := e.router.InvokeModel(ctx, call.Request)
	if err != nil {
		return sampleOutcome{
			Model:       call.Model,
			SampleIndex: call.SampleIndex,
			Error:       err,
			Reason:      fmt.Sprintf("provider call failed: %v", err),
		}
	}

	verdict, reason, ok := parseYesNo(response.OutputText)
	if !ok {
		return sampleOutcome{
			Model:       call.Model,
			SampleIndex: call.SampleIndex,
			Verdict:     nil,
			Reason:      "response did not contain YES, NO, or UNKNOWN",
			Usage:       response.Usage,
			RawOutput:   response.OutputText,
		}
	}

	if verdict == nil {
		// UNKNOWN / abstain path. Reason is prefixed so the aggregator
		// and downstream inspectors can distinguish this from a
		// failed parse.
		return sampleOutcome{
			Model:       call.Model,
			SampleIndex: call.SampleIndex,
			Verdict:     nil,
			Reason:      "unknown: " + reason,
			Usage:       response.Usage,
			RawOutput:   response.OutputText,
		}
	}

	return sampleOutcome{
		Model:       call.Model,
		SampleIndex: call.SampleIndex,
		Verdict:     verdict,
		Reason:      reason,
		Usage:       response.Usage,
		RawOutput:   response.OutputText,
	}
}

// assertionVerdictPattern matches YES/NO/UNKNOWN at the start of a line
// (after optional whitespace). Case-insensitive. \b ensures "YESSIR"
// doesn't match but "YES." or "YES " does. Applied per-line by
// parseYesNo after splitting the response on \n.
var assertionVerdictPattern = regexp.MustCompile(`(?i)^\s*(YES|NO|UNKNOWN)\b`)

// parseYesNo extracts the verdict from a judge response. It walks the
// lines of the response and returns the first line that begins with
// YES, NO, or UNKNOWN (case-insensitive). Returns:
//
//   - (pointer to true, reason, true)  for YES
//   - (pointer to false, reason, true) for NO
//   - (nil, reason, true)              for UNKNOWN (abstain)
//   - (nil, "", false)                 for no match (abstain, parse fail)
//
// The reason is the trailer of the matched line (everything after the
// verdict word, with ":", "-", or "—" separators stripped) optionally
// followed by subsequent lines joined with spaces. Multi-line
// reasoning is preserved because the prompt explicitly permits it on
// line 2.
//
// Line-walking handles common noise: code-fence wrappers (```), prose
// prefixes ("Answer: YES"), and leading blank lines.
func parseYesNo(text string) (verdict *bool, reason string, ok bool) {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		match := assertionVerdictPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		// Normalise the case of the captured word so downstream
		// comparisons are stable. match[1] preserves the original
		// case of the input.
		word := strings.ToUpper(match[1])

		// Build the trailer by stripping leading whitespace from the
		// line (so it starts with the verdict word), then slicing off
		// the verdict word itself. Case-preserving because we slice
		// the original text, not the upper-cased word.
		trimmed := strings.TrimLeft(line, " \t")
		trailer := strings.TrimSpace(trimmed[len(match[1]):])
		trailer = strings.TrimPrefix(trailer, ":")
		trailer = strings.TrimPrefix(trailer, "-")
		trailer = strings.TrimPrefix(trailer, "—")
		trailer = strings.TrimSpace(trailer)

		if i+1 < len(lines) {
			rest := strings.TrimSpace(strings.Join(lines[i+1:], " "))
			switch {
			case trailer == "" && rest != "":
				trailer = rest
			case trailer != "" && rest != "":
				trailer = trailer + " " + rest
			}
		}

		switch word {
		case "YES":
			t := true
			return &t, trailer, true
		case "NO":
			f := false
			return &f, trailer, true
		case "UNKNOWN":
			return nil, trailer, true
		}
	}
	return nil, "", false
}

// aggregateAssertion collapses the per-sample outcomes into a single
// scoring.JudgeResult. Pipeline:
//
//  1. Bucket outcomes by model.
//  2. Per-model: majority vote over non-error, non-abstain verdicts.
//     Ties break to NO. All-abstain/error for a model → model
//     contributes nothing to cross-model consensus.
//  3. Cross-model: single model → its verdict wins. Multi-model →
//     applyAssertionConsensus with the judge's ConsensusConfig.
//  4. Compare final verdict against judge.Expect (default true);
//     score = 1.0 on match, 0.0 on mismatch.
//  5. Derive confidence from the abstain/error rate.
//  6. Build the payload jsonb for persistence.
func aggregateAssertion(judge scoring.LLMJudgeDeclaration, outcomes []sampleOutcome) scoring.JudgeResult {
	result := scoring.JudgeResult{
		Key:  judge.Key,
		Mode: scoring.JudgeMethodAssertion,
	}
	if len(outcomes) == 0 {
		result.State = scoring.OutputStateUnavailable
		result.Reason = "no assertion samples executed"
		return result
	}

	// Bucket by model in the order they first appear so deterministic
	// test assertions don't have to sort.
	byModel := make(map[string][]sampleOutcome)
	modelOrder := make([]string, 0)
	for _, o := range outcomes {
		if _, seen := byModel[o.Model]; !seen {
			modelOrder = append(modelOrder, o.Model)
		}
		byModel[o.Model] = append(byModel[o.Model], o)
	}

	// Per-model majority vote. modelTally is the in-memory pipeline
	// shape; tallyPayloadsFromTallies converts it to the serialisable
	// modelTallyPayload for persistence.
	tallies := make([]modelTally, 0, len(modelOrder))
	totalSamples := 0
	totalAbstain := 0
	totalErrors := 0
	for _, model := range modelOrder {
		modelOutcomes := byModel[model]
		t := modelTally{model: model}
		for _, o := range modelOutcomes {
			totalSamples++
			switch {
			case o.Error != nil:
				t.errs++
				totalErrors++
			case o.Verdict == nil:
				t.abstain++
				totalAbstain++
			case *o.Verdict:
				t.yes++
			default:
				t.no++
			}
		}
		if t.yes == 0 && t.no == 0 {
			// All samples for this model abstained or errored.
			tallies = append(tallies, t)
			continue
		}
		// Ties break toward NO (safer default for safety assertions).
		if t.yes > t.no {
			v := true
			t.verdict = &v
		} else {
			v := false
			t.verdict = &v
		}
		tallies = append(tallies, t)
	}

	result.SampleCount = totalSamples
	result.ModelCount = len(modelOrder)

	// Cross-model consensus. Collect non-nil per-model verdicts.
	modelVerdicts := make(map[string]bool, len(tallies))
	for _, t := range tallies {
		if t.verdict != nil {
			modelVerdicts[t.model] = *t.verdict
		}
	}
	tallyPayloads := tallyPayloadsFromTallies(tallies)

	if len(modelVerdicts) == 0 {
		result.State = scoring.OutputStateUnavailable
		result.Reason = "every model abstained or errored on assertion"
		result.Confidence = "low"
		result.Payload = mustMarshalAssertionPayload(judge, tallyPayloads, nil, deriveExpect(judge), false)
		return result
	}

	var finalVerdict bool
	var consensusReason string
	switch {
	case len(modelVerdicts) == 1:
		for _, v := range modelVerdicts {
			finalVerdict = v
		}
	case judge.Consensus != nil:
		decided, reason, ok := applyAssertionConsensus(modelVerdicts, *judge.Consensus)
		if !ok {
			result.State = scoring.OutputStateUnavailable
			result.Reason = reason
			result.Confidence = "low"
			result.Payload = mustMarshalAssertionPayload(judge, tallyPayloads, nil, deriveExpect(judge), false)
			return result
		}
		finalVerdict = decided
		consensusReason = reason
	default:
		// Defensive. Validation rejects multi-model assertion without
		// consensus config, but handle it here so a programmatically
		// constructed judge doesn't panic.
		result.State = scoring.OutputStateError
		result.Reason = "multi-model assertion judge requires consensus config"
		result.Payload = mustMarshalAssertionPayload(judge, tallyPayloads, nil, deriveExpect(judge), false)
		return result
	}

	expected := deriveExpect(judge)
	scoreValue := 0.0
	if finalVerdict == expected {
		scoreValue = 1.0
	}
	result.State = scoring.OutputStateAvailable
	result.NormalizedScore = &scoreValue
	result.Confidence = deriveAssertionConfidence(totalSamples, totalAbstain+totalErrors)
	if consensusReason != "" {
		result.Reason = consensusReason
	}
	result.Payload = mustMarshalAssertionPayload(judge, tallyPayloads, &finalVerdict, expected, true)
	return result
}

// modelTally is the in-memory per-model vote breakdown used during
// aggregation. Not serialised directly — tallyPayloadsFromTallies
// converts it to modelTallyPayload for the jsonb payload column.
type modelTally struct {
	model   string
	yes     int
	no      int
	abstain int
	errs    int
	verdict *bool // nil = no majority (all abstained/errored)
}

// tallyPayloadsFromTallies converts the internal pipeline type into
// the serialisable payload shape. Kept tiny and pure so the
// aggregator reads as a straight-line pipeline.
func tallyPayloadsFromTallies(tallies []modelTally) []modelTallyPayload {
	out := make([]modelTallyPayload, len(tallies))
	for i, t := range tallies {
		var verdict *bool
		if t.verdict != nil {
			v := *t.verdict
			verdict = &v
		}
		out[i] = modelTallyPayload{
			Model:   t.model,
			Yes:     t.yes,
			No:      t.no,
			Abstain: t.abstain,
			Errors:  t.errs,
			Verdict: verdict,
		}
	}
	return out
}

// applyAssertionConsensus collapses per-model boolean verdicts into a
// single cross-model verdict according to the ConsensusConfig. Only
// majority_vote and unanimous apply to assertion mode (median and mean
// are numeric-only, rejected at validation time).
//
// Returns (verdict, reason, ok). ok=false means consensus failed and
// the judge should surface as unavailable.
func applyAssertionConsensus(modelVerdicts map[string]bool, cfg scoring.ConsensusConfig) (bool, string, bool) {
	if len(modelVerdicts) == 0 {
		return false, "no per-model verdicts available for consensus", false
	}

	yes := 0
	no := 0
	for _, v := range modelVerdicts {
		if v {
			yes++
		} else {
			no++
		}
	}

	switch cfg.Aggregation {
	case scoring.ConsensusAggMajorityVote:
		if yes > no {
			return true, fmt.Sprintf("majority_vote: %d/%d models YES", yes, len(modelVerdicts)), true
		}
		// Ties in cross-model vote also break toward NO for the same
		// reason as intra-model ties.
		return false, fmt.Sprintf("majority_vote: %d/%d models YES (tie or majority NO → NO)", yes, len(modelVerdicts)), true
	case scoring.ConsensusAggUnanimous:
		if yes == len(modelVerdicts) {
			return true, "unanimous: all models YES", true
		}
		if no == len(modelVerdicts) {
			return false, "unanimous: all models NO", true
		}
		return false, fmt.Sprintf("unanimous required but models disagree: %d YES, %d NO", yes, no), false
	default:
		// Numeric aggregations (median/mean) are rejected at validation
		// for assertion mode. Defensive fallback.
		return false, fmt.Sprintf("consensus aggregation %q is not supported for assertion mode", cfg.Aggregation), false
	}
}

// deriveExpect returns the expected verdict for an assertion judge.
// Defaults to true (the assertion should be satisfied) when
// judge.Expect is nil.
func deriveExpect(judge scoring.LLMJudgeDeclaration) bool {
	if judge.Expect == nil {
		return true
	}
	return *judge.Expect
}

// deriveAssertionConfidence buckets the abstain+error rate into
// high/medium/low confidence. Thresholds are chosen so a single
// abstain out of three samples (33%) still counts as medium, not low.
//
//	abstain+error rate = 0   → high
//	abstain+error rate < 0.5 → medium
//	abstain+error rate >= 0.5 → low
func deriveAssertionConfidence(total, abstainOrError int) string {
	if total <= 0 {
		return "low"
	}
	if abstainOrError == 0 {
		return "high"
	}
	rate := float64(abstainOrError) / float64(total)
	if rate < 0.5 {
		return "medium"
	}
	return "low"
}

// mustMarshalAssertionPayload serialises the per-sample / per-model
// breakdown into the jsonb payload shape defined in the Phase 2
// llm_judge_results migration. Never errors in practice — the inputs
// are simple types — but returns an empty {} fallback on the
// impossible json.Marshal failure so callers never have to special-
// case a nil payload.
func mustMarshalAssertionPayload(
	judge scoring.LLMJudgeDeclaration,
	tallies []modelTallyPayload,
	finalVerdict *bool,
	expected bool,
	available bool,
) json.RawMessage {
	payload := struct {
		Mode            string                     `json:"mode"`
		Judge           string                     `json:"judge"`
		Available       bool                       `json:"available"`
		Expected        bool                       `json:"expected"`
		FinalVerdict    *bool                      `json:"final_verdict,omitempty"`
		Tallies         []modelTallyPayload        `json:"tallies"`
		Consensus       *scoring.ConsensusConfig   `json:"consensus,omitempty"`
	}{
		Mode:         string(scoring.JudgeMethodAssertion),
		Judge:        judge.Key,
		Available:    available,
		Expected:     expected,
		FinalVerdict: finalVerdict,
		Tallies:      sortedTalliesCopy(tallies),
		Consensus:    judge.Consensus,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}

// modelTallyPayload is the public-facing serialised shape for one
// model's vote breakdown. Distinct from the internal modelTally struct
// which carries a nullable pointer for the in-memory pipeline.
type modelTallyPayload struct {
	Model   string `json:"model"`
	Yes     int    `json:"yes"`
	No      int    `json:"no"`
	Abstain int    `json:"abstain"`
	Errors  int    `json:"errors"`
	Verdict *bool  `json:"verdict,omitempty"`
}

// sortedTalliesCopy deep-copies the tally list and sorts it by model
// name for deterministic JSON output across test runs.
func sortedTalliesCopy(src []modelTallyPayload) []modelTallyPayload {
	out := make([]modelTallyPayload, len(src))
	copy(out, src)
	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return out
}
