package judge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
)

// Phase 6 of issue #148 — n_wise mode dispatch. Unlike per-agent
// modes, n_wise operates at the run level: a single LLM call sees
// ALL agents' final outputs and produces a relative ranking. The
// scoring layer then converts the ranking to per-agent normalized
// scores via Borda count.
//
// Design decisions captured in backend/.claude/analysis/issue-148-
// deep-analysis.md Part 5 lines 478-516 and Phase 6 plan questions
// Q1-Q5 (locked in before implementation).
//
// Scope cuts deliberately deferred to Phase 7:
//   - Multi-model consensus (Phase 6 uses the first Model and warns)
//   - MaxCallsUSD budget enforcement
//
// Scope cuts deliberately deferred to a follow-up issue:
//   - run_scorecards.scorecard jsonb denormalization for cheap
//     ranking API reads

// nwiseMaxAgents is the hard upper bound on the number of agents an
// n_wise judge can rank. Labels use letters A..Z (see
// prompts.go:nwiseLabelAt), so 26 is the natural ceiling. Runs with
// more agents produce state=error with a clear reason.
const nwiseMaxAgents = 26

// nwiseMinAgents is the minimum agent count for meaningful ranking.
// With one agent there is nothing to compare against. Runs with
// fewer agents produce state=unavailable, matching the Phase 6 plan
// Q5 resolution (partial scorecard, not hard error).
const nwiseMinAgents = 2

// NWiseAgent is one agent's identity + output for a run-level n_wise
// evaluation. The workflow-side JudgeRun activity constructs these
// by reading each run_agent's events and extracting final_output via
// scoring.ExtractFinalOutputFromEvents. Label is a free-form string
// that the caller can use for its own bookkeeping (e.g., match it
// against run_agents.label) — it does NOT appear in the prompt
// envelope, which uses opaque letters A..Z to prevent model-name
// bias.
type NWiseAgent struct {
	RunAgentID  uuid.UUID
	Label       string
	FinalOutput string
}

// NWiseInput is the run-level input to Evaluator.EvaluateNWise. The
// Phase 6 JudgeRun activity populates this once per run from the
// execution context. Judges is the subset of spec.LLMJudges filtered
// to n_wise mode only; ResolvedReferences is the same workflow-side
// context resolution used by per-agent evaluate (one shared map per
// run, keyed by evidence reference).
type NWiseInput struct {
	RunID              uuid.UUID
	EvaluationSpecID   uuid.UUID
	Judges             []scoring.LLMJudgeDeclaration
	Agents             []NWiseAgent
	ResolvedReferences map[string]string
}

// NWiseResult is the aggregated output of a run-level n_wise pass.
// PerAgent maps run_agent_id to the list of JudgeResult entries
// produced for that agent (one per n_wise judge). The JudgeRun
// activity persists these via UpsertLLMJudgeResult (one row per
// (agent, judge_key)) and threads them to the per-agent
// JudgeRunAgent activity as finalize input.
//
// Warnings aggregates cross-judge warnings (e.g., "judge foo:
// truncated output for Agent B", "judge bar: multi-model requested
// but Phase 6 uses first model only"). Phase 4's JudgeRunAgent
// merge path forwards them into RunAgentEvaluation.Warnings.
type NWiseResult struct {
	PerAgent map[uuid.UUID][]scoring.JudgeResult
	Warnings []string
}

// defaultNWiseSchemaJSON is the fallback schema when an n_wise judge
// declares no output_schema. Matches the Method 3 example in issue
// #148 (lines 146-158). Required fields: ranking array with
// agent_label + rank per entry.
const defaultNWiseSchemaJSON = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "ranking": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "agent_label": {"type": "string"},
          "rank": {"type": "integer", "minimum": 1},
          "reasoning": {"type": "string"}
        },
        "required": ["agent_label", "rank"]
      }
    },
    "unable_to_judge": {"type": "boolean"},
    "reason": {"type": "string"}
  }
}`

var defaultNWiseSchema *jsonschema.Schema

func init() {
	var schema jsonschema.Schema
	if err := json.Unmarshal([]byte(defaultNWiseSchemaJSON), &schema); err != nil {
		panic(fmt.Sprintf("judge: invalid default n_wise schema: %v", err))
	}
	schema.Schema = ""
	defaultNWiseSchema = &schema
}

// nwiseRankEntry is one entry in the parsed LLM ranking response.
// The parser decodes the "ranking" array into a slice of these so
// the aggregator can walk them without re-parsing.
type nwiseRankEntry struct {
	AgentLabel string `json:"agent_label"`
	Rank       int    `json:"rank"`
	Reasoning  string `json:"reasoning"`
}

// nwiseResponse is the canonical shape extracted from a single
// n_wise LLM call. Mirrors the rubricResponse pattern from Phase 5
// but for cross-agent ranking data instead of scalar scores.
type nwiseResponse struct {
	Ranking       []nwiseRankEntry
	UnableToJudge bool
	AbstainReason string
	RawJSON       json.RawMessage
}

// parseNWiseResponse runs the three-tier JSON extractor from Phase 5
// (extractJSONObject) plus schema validation, then maps the parsed
// JSON into an nwiseResponse struct.
//
// Returns (response, ok). ok=false means the response couldn't be
// parsed, schema validation failed, or the ranking array was empty.
// The caller marks the sample as abstain and multi-sample Borda
// aggregation absorbs the loss.
//
// Abstain fast-path: if unable_to_judge is true, returns ok=true
// with UnableToJudge set, bypassing schema validation (the judge
// explicitly told us it can't decide; forcing it through a
// ranking-schema check would be counterproductive).
func parseNWiseResponse(text string, schema *jsonschema.Schema) (nwiseResponse, bool) {
	extracted, ok := extractJSONObject(text)
	if !ok {
		return nwiseResponse{}, false
	}
	raw := json.RawMessage(extracted)

	decoder := json.NewDecoder(strings.NewReader(extracted))
	var generic map[string]any
	if err := decoder.Decode(&generic); err != nil {
		return nwiseResponse{}, false
	}

	// Abstain fast-path — honours the judge's explicit escape hatch
	// regardless of schema shape.
	if abstain, ok := generic["unable_to_judge"].(bool); ok && abstain {
		reason, _ := generic["reason"].(string)
		return nwiseResponse{
			UnableToJudge: true,
			AbstainReason: strings.TrimSpace(reason),
			RawJSON:       raw,
		}, true
	}

	// Schema validation.
	resolved, resolveErr := schema.Resolve(nil)
	if resolveErr != nil {
		return nwiseResponse{}, false
	}
	if err := resolved.Validate(generic); err != nil {
		return nwiseResponse{}, false
	}

	// Extract the ranking array. Decoded via json.Decoder into the
	// generic map, then re-decoded into the typed struct. The
	// re-decode avoids reflection-heavy manual walking.
	var shaped struct {
		Ranking []nwiseRankEntry `json:"ranking"`
	}
	if err := json.Unmarshal([]byte(extracted), &shaped); err != nil {
		return nwiseResponse{}, false
	}
	if len(shaped.Ranking) == 0 {
		return nwiseResponse{}, false
	}

	return nwiseResponse{
		Ranking: shaped.Ranking,
		RawJSON: raw,
	}, true
}

// EvaluateNWise runs every declared n_wise judge across the run's
// agents and returns one JudgeResult per (agent, judge_key). The
// workflow-side JudgeRun activity (Phase 6) calls this after
// collecting all agents' final outputs; the returned per-agent
// results flow into the JudgeRunAgent activity for finalization.
//
// Per-judge failures NEVER abort the run — they surface as
// state=unavailable for the affected agents, and other judges still
// run. Cross-sample ranking noise is smoothed by Borda count, which
// preserves signal from every valid sample even when some disagree.
//
// Filters the input:
//   - Judges slice: takes ONLY n_wise mode judges (skips assertion/
//     rubric/reference/unknown modes). This lets the caller pass
//     spec.LLMJudges unchanged.
//   - Agents slice: drops agents with empty final_output; the
//     remaining set must have at least nwiseMinAgents entries (2)
//     for any meaningful ranking.
//
// The error return is reserved for Evaluate-wide failures that
// prevent ANY judge from running (typically ctx cancellation). In
// practice it is always nil — per-judge errors are captured as
// error-state JudgeResults inside PerAgent.
func (e *Evaluator) EvaluateNWise(ctx context.Context, in NWiseInput) (NWiseResult, error) {
	perAgent := make(map[uuid.UUID][]scoring.JudgeResult)
	warnings := make([]string, 0)

	// Filter judges to n_wise mode only. Most runs won't have any,
	// in which case this function is a no-op.
	nwiseJudges := make([]scoring.LLMJudgeDeclaration, 0, len(in.Judges))
	for _, judge := range in.Judges {
		if judge.Mode == scoring.JudgeMethodNWise {
			nwiseJudges = append(nwiseJudges, judge)
		}
	}
	if len(nwiseJudges) == 0 {
		return NWiseResult{PerAgent: perAgent, Warnings: warnings}, nil
	}

	// Pre-seed perAgent with the ORIGINAL agent set so every
	// run_agent gets a map entry (possibly empty) even when n_wise
	// filters them out later. This keeps the caller's iteration
	// pattern clean: range over perAgent to find results for each
	// run_agent.
	for _, agent := range in.Agents {
		perAgent[agent.RunAgentID] = nil
	}

	// Filter out agents with empty final_output. Empty outputs
	// can't be ranked and would confuse the LLM. Emit a warning
	// per filtered agent so operators see the signal.
	filteredAgents := make([]NWiseAgent, 0, len(in.Agents))
	for _, agent := range in.Agents {
		if strings.TrimSpace(agent.FinalOutput) == "" {
			warnings = append(warnings, fmt.Sprintf("n_wise: agent %s has empty final output and is excluded from ranking", agent.RunAgentID))
			continue
		}
		filteredAgents = append(filteredAgents, agent)
	}

	// N=1 or N>26 → state=unavailable for every judge. Matches
	// Phase 6 plan Q5 (partial scorecard principle).
	if len(filteredAgents) < nwiseMinAgents {
		for _, judge := range nwiseJudges {
			unavailableReason := fmt.Sprintf("n_wise requires at least %d agents (got %d)", nwiseMinAgents, len(filteredAgents))
			result := scoring.JudgeResult{
				Key:    judge.Key,
				Mode:   scoring.JudgeMethodNWise,
				State:  scoring.OutputStateUnavailable,
				Reason: unavailableReason,
			}
			for _, agent := range in.Agents {
				perAgent[agent.RunAgentID] = append(perAgent[agent.RunAgentID], result)
			}
			warnings = append(warnings, fmt.Sprintf("judge %q: %s", judge.Key, unavailableReason))
		}
		return NWiseResult{PerAgent: perAgent, Warnings: warnings}, nil
	}
	if len(filteredAgents) > nwiseMaxAgents {
		for _, judge := range nwiseJudges {
			errorReason := fmt.Sprintf("n_wise supports at most %d agents per run (got %d)", nwiseMaxAgents, len(filteredAgents))
			result := scoring.JudgeResult{
				Key:    judge.Key,
				Mode:   scoring.JudgeMethodNWise,
				State:  scoring.OutputStateError,
				Reason: errorReason,
			}
			for _, agent := range in.Agents {
				perAgent[agent.RunAgentID] = append(perAgent[agent.RunAgentID], result)
			}
			warnings = append(warnings, fmt.Sprintf("judge %q: %s", judge.Key, errorReason))
		}
		return NWiseResult{PerAgent: perAgent, Warnings: warnings}, nil
	}

	// Run each n_wise judge independently. Judges don't interact,
	// so we evaluate them sequentially — the fan-out inside each
	// judge (samples) is already parallel via fanOut.
	for _, judge := range nwiseJudges {
		if ctx.Err() != nil {
			for _, agent := range in.Agents {
				perAgent[agent.RunAgentID] = append(perAgent[agent.RunAgentID], scoring.JudgeResult{
					Key:    judge.Key,
					Mode:   scoring.JudgeMethodNWise,
					State:  scoring.OutputStateError,
					Reason: fmt.Sprintf("judge %q: context cancelled before dispatch: %v", judge.Key, ctx.Err()),
				})
			}
			warnings = append(warnings, fmt.Sprintf("judge %q: cancelled", judge.Key))
			continue
		}

		agentResults, judgeWarnings := e.evaluateOneNWise(ctx, judge, filteredAgents, in.ResolvedReferences)
		warnings = append(warnings, judgeWarnings...)

		// Merge judge-result map into the run-level perAgent map.
		// Agents in in.Agents but not in filteredAgents still exist
		// in perAgent with no entries — the caller sees empty result
		// slices, which downstream treats as "this agent has no
		// n_wise score" (an empty judges list, not an unavailable
		// stub).
		for agentID, jr := range agentResults {
			perAgent[agentID] = append(perAgent[agentID], jr)
		}
	}

	return NWiseResult{PerAgent: perAgent, Warnings: warnings}, nil
}

// evaluateOneNWise runs a single n_wise judge across the filtered
// agent set. Returns a per-agent map of JudgeResult and a slice of
// warnings. Never panics — every failure path produces either a
// usable result or an explicit unavailable/error state.
//
// Pipeline:
//  1. Resolve schema (custom or defaultNWiseSchema)
//  2. Resolve model list (use first if multi-model — Phase 6 scope)
//  3. Generate sample orderings (cyclic shifts or fixed)
//  4. Build and dispatch provider calls via shared fanOut
//  5. Parse each sample's ranking, mapping label back to agent index
//  6. Aggregate via Borda count per agent
//  7. Derive confidence from rank consistency
//  8. Emit one JudgeResult per agent with the per-agent payload
func (e *Evaluator) evaluateOneNWise(
	ctx context.Context,
	judge scoring.LLMJudgeDeclaration,
	agents []NWiseAgent,
	resolvedRefs map[string]string,
) (map[uuid.UUID]scoring.JudgeResult, []string) {
	warnings := make([]string, 0)
	results := make(map[uuid.UUID]scoring.JudgeResult, len(agents))

	schema, schemaErr := resolveJudgeSchema(judge, defaultNWiseSchema)
	if schemaErr != nil {
		for _, agent := range agents {
			results[agent.RunAgentID] = scoring.JudgeResult{
				Key:    judge.Key,
				Mode:   scoring.JudgeMethodNWise,
				State:  scoring.OutputStateError,
				Reason: fmt.Sprintf("parse judge output schema: %v", schemaErr),
			}
		}
		return results, warnings
	}

	// Resolve models. Phase 6 uses a single model per n_wise judge;
	// if the pack declared multi-model, use the first and warn.
	models := resolveNWiseModels(judge, e.cfg)
	if len(models) == 0 {
		for _, agent := range agents {
			results[agent.RunAgentID] = scoring.JudgeResult{
				Key:    judge.Key,
				Mode:   scoring.JudgeMethodNWise,
				State:  scoring.OutputStateError,
				Reason: "judge has no models",
			}
		}
		return results, warnings
	}
	if len(judge.Models) > 1 {
		warnings = append(warnings, fmt.Sprintf("judge %q: multi-model n_wise is a Phase 7 feature; using first model %q", judge.Key, models[0]))
	}
	model := models[0]

	// Determine sample count and generate orderings.
	samples := judge.Samples
	if samples <= 0 {
		samples = scoring.JudgeDefaultSamples
	}
	if samples > scoring.JudgeMaxSamplesCeiling {
		samples = scoring.JudgeMaxSamplesCeiling
	}
	orderings, orderingsWarn := generateSampleOrderings(len(agents), samples, judge.PositionDebiasing)
	if orderingsWarn != "" {
		warnings = append(warnings, fmt.Sprintf("judge %q: %s", judge.Key, orderingsWarn))
	}

	providerKey, resolveErr := resolveProviderKey(model, e.cfg)
	if resolveErr != nil {
		for _, agent := range agents {
			results[agent.RunAgentID] = scoring.JudgeResult{
				Key:    judge.Key,
				Mode:   scoring.JudgeMethodNWise,
				State:  scoring.OutputStateError,
				Reason: resolveErr.Error(),
			}
		}
		return results, warnings
	}

	timeout := e.cfg.DefaultTimeout
	if judge.TimeoutMS != nil && *judge.TimeoutMS > 0 {
		timeout = time.Duration(*judge.TimeoutMS) * time.Millisecond
	}

	// Build one provider call per sample. Each call gets a DIFFERENT
	// prompt (different ordering) so we can't share the request
	// across samples the way rubric/assertion do. The ordering per
	// sample is captured in callOrderings so the parser can map
	// labels back to agent indices after the sample completes.
	calls := make([]providerCall, 0, len(orderings))
	callOrderings := make([][]int, 0, len(orderings))
	truncationWarnings := make(map[string]struct{})
	for sampleIdx, ordering := range orderings {
		sysPrompt, userPrompt, truncatedLabels := buildNWisePrompt(judge, agents, ordering, resolvedRefs, e.cfg.NWiseMaxOutputChars)
		for _, label := range truncatedLabels {
			// Track agent IDs whose outputs got truncated so we can
			// emit a single warning per agent rather than per sample.
			agentIdx := ordering[labelIndex(label)]
			if agentIdx >= 0 && agentIdx < len(agents) {
				key := fmt.Sprintf("%s:%s", judge.Key, agents[agentIdx].RunAgentID)
				truncationWarnings[key] = struct{}{}
			}
		}
		calls = append(calls, providerCall{
			Model:       model,
			SampleIndex: sampleIdx,
			Request: provider.Request{
				ProviderKey:         providerKey,
				CredentialReference: e.cfg.CredentialReference,
				Model:               model,
				StepTimeout:         timeout,
				Messages: []provider.Message{
					{Role: "system", Content: sysPrompt},
					{Role: "user", Content: userPrompt},
				},
			},
		})
		callOrderings = append(callOrderings, ordering)
	}

	// Emit truncation warnings once per affected agent per judge.
	for key := range truncationWarnings {
		warnings = append(warnings, fmt.Sprintf("n_wise: truncated final output for %s (exceeded NWiseMaxOutputChars)", key))
	}

	outcomes := e.fanOut(ctx, calls, func(ctx context.Context, call providerCall) sampleOutcome {
		return e.runNWiseCall(ctx, call, schema)
	})

	// Parse each sample's ranking and collect per-agent rank info.
	// sampleRankings[sampleIdx][agentIdx] = rank, or 0 when the
	// sample failed or the agent was missing from the response.
	sampleRankings := make([][]int, len(outcomes))
	validSamples := 0
	abstainCount := 0
	errorCount := 0
	reasoningPerSample := make([]string, len(outcomes))
	for i, outcome := range outcomes {
		if outcome.Error != nil {
			errorCount++
			continue
		}
		if outcome.RawOutput == "" {
			abstainCount++
			continue
		}
		parsed, ok := parseNWiseResponse(outcome.RawOutput, schema)
		if !ok {
			abstainCount++
			continue
		}
		if parsed.UnableToJudge {
			abstainCount++
			reasoningPerSample[i] = "unable_to_judge: " + parsed.AbstainReason
			continue
		}

		ranks, reasoning := mapRankingToAgentRanks(parsed.Ranking, len(agents), callOrderings[i])
		if ranks == nil {
			// Parsed response didn't contain a valid rank for every
			// agent. Counts as abstain — multi-sample averaging
			// absorbs it.
			abstainCount++
			continue
		}
		sampleRankings[i] = ranks
		reasoningPerSample[i] = reasoning
		validSamples++
	}

	// Aggregate via Borda count.
	if validSamples == 0 {
		// Every sample failed — state=unavailable for every agent.
		for _, agent := range agents {
			results[agent.RunAgentID] = scoring.JudgeResult{
				Key:         judge.Key,
				Mode:        scoring.JudgeMethodNWise,
				State:       scoring.OutputStateUnavailable,
				Reason:      "no valid n_wise samples",
				SampleCount: len(outcomes),
				ModelCount:  1,
				Confidence:  "low",
			}
		}
		return results, warnings
	}

	bordaPoints := computeBordaPoints(sampleRankings, len(agents))
	agentRanks := finalRanksFromBorda(bordaPoints)

	// Per-agent confidence: based on rank stability across samples.
	for agentIdx, agent := range agents {
		normalized := normalizeBorda(bordaPoints[agentIdx], validSamples, len(agents))
		rankInRun := agentRanks[agentIdx]
		confidence := deriveNWiseConfidence(sampleRankings, agentIdx, validSamples)
		payload := mustMarshalNWisePayload(judge, agentIdx, rankInRun, bordaPoints[agentIdx], sampleRankings, reasoningPerSample, callOrderings, validSamples)

		nScore := normalized
		results[agent.RunAgentID] = scoring.JudgeResult{
			Key:             judge.Key,
			Mode:            scoring.JudgeMethodNWise,
			State:           scoring.OutputStateAvailable,
			NormalizedScore: &nScore,
			Confidence:      confidence,
			SampleCount:     len(outcomes),
			ModelCount:      1,
			Payload:         payload,
		}
	}
	return results, warnings
}

// resolveNWiseModels returns the ordered list of models for an
// n_wise judge. Phase 6 uses a single model per judge; multi-model
// consensus is a Phase 7 feature. The caller emits a warning when
// multi-model was requested so operators know the pack's intent
// didn't match the Phase 6 scope.
func resolveNWiseModels(judge scoring.LLMJudgeDeclaration, cfg Config) []string {
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
		return []string{cfg.DefaultNWiseModel}
	}
}

// generateSampleOrderings returns a slice of agent index orderings,
// one per sample. Each ordering is a permutation of [0, n-1] that
// tells buildNWisePrompt how to map label slots (A, B, C, ...) to
// agents.
//
// Cyclic-shift algorithm (Phase 6 plan D2 from the analysis doc):
//
//   samples=0   → samples = JudgeDefaultSamples (3)
//   debias=false → all samples use the identity ordering
//   debias=true && samples >= n → sample i uses shift i % n, so
//     each agent occupies each position exactly once when samples=n
//   debias=true && samples < n → fall back to identity ordering and
//     return a warning explaining the fallback
//
// Returns (orderings, warning). warning is empty when there's
// nothing to report.
func generateSampleOrderings(n, samples int, positionDebiasing bool) ([][]int, string) {
	if samples <= 0 {
		samples = scoring.JudgeDefaultSamples
	}
	orderings := make([][]int, samples)
	identity := make([]int, n)
	for i := 0; i < n; i++ {
		identity[i] = i
	}

	if !positionDebiasing {
		for i := range orderings {
			orderings[i] = append([]int(nil), identity...)
		}
		return orderings, ""
	}

	var warning string
	if samples < n {
		warning = fmt.Sprintf("position_debiasing requires samples >= N_agents (got samples=%d, N=%d); falling back to fixed ordering", samples, n)
		for i := range orderings {
			orderings[i] = append([]int(nil), identity...)
		}
		return orderings, warning
	}

	for s := 0; s < samples; s++ {
		shift := s % n
		order := make([]int, n)
		for slot := 0; slot < n; slot++ {
			order[slot] = (slot + shift) % n
		}
		orderings[s] = order
	}
	return orderings, warning
}

// runNWiseCall invokes the provider for one (judge, sample) tuple.
// Schema validation happens inside parseNWiseResponse — here we
// only capture the raw response text so the caller can parse it
// with the per-call ordering context.
func (e *Evaluator) runNWiseCall(ctx context.Context, call providerCall, _ *jsonschema.Schema) sampleOutcome {
	response, err := e.router.InvokeModel(ctx, call.Request)
	if err != nil {
		return sampleOutcome{
			Model:       call.Model,
			SampleIndex: call.SampleIndex,
			Error:       err,
			Reason:      fmt.Sprintf("provider call failed: %v", err),
		}
	}
	return sampleOutcome{
		Model:       call.Model,
		SampleIndex: call.SampleIndex,
		Usage:       response.Usage,
		RawOutput:   response.OutputText,
	}
}

// mapRankingToAgentRanks takes a parsed ranking from the LLM and
// returns a []int where out[agentIdx] = rank (1..n). Returns nil
// when the ranking is missing or malformed for this sample.
//
// The sample's ordering tells us which agent index appears under
// each label. For example, if ordering = [1, 2, 0], then label A
// was shown with agents[1], label B with agents[2], label C with
// agents[0]. When the LLM responds with agent_label="A", rank=2
// for this sample, we assign rank 2 to agentIdx=1.
//
// Validation:
//   - Every entry must have a known label (A..Z, within n range)
//   - Every agent must appear exactly once (no missing agents,
//     no duplicates, no unknown labels)
//
// Returns (nil, "") when validation fails. Caller treats that as
// an abstain sample. The reasoning string is built from the
// per-entry reasoning fields joined in rank order.
func mapRankingToAgentRanks(ranking []nwiseRankEntry, n int, ordering []int) ([]int, string) {
	if len(ranking) == 0 || n == 0 {
		return nil, ""
	}
	agentRanks := make([]int, n)
	seenAgent := make([]bool, n)
	reasoningParts := make([]string, 0, len(ranking))

	for _, entry := range ranking {
		labelIdx := labelIndex(entry.AgentLabel)
		if labelIdx < 0 || labelIdx >= len(ordering) {
			return nil, ""
		}
		agentIdx := ordering[labelIdx]
		if agentIdx < 0 || agentIdx >= n {
			return nil, ""
		}
		if seenAgent[agentIdx] {
			return nil, ""
		}
		if entry.Rank < 1 || entry.Rank > n {
			return nil, ""
		}
		agentRanks[agentIdx] = entry.Rank
		seenAgent[agentIdx] = true
		if entry.Reasoning != "" {
			reasoningParts = append(reasoningParts, fmt.Sprintf("%s(#%d): %s", entry.AgentLabel, entry.Rank, entry.Reasoning))
		}
	}
	for _, seen := range seenAgent {
		if !seen {
			return nil, ""
		}
	}
	return agentRanks, strings.Join(reasoningParts, " | ")
}

// labelIndex maps an agent label string back to its slot index.
// "A" → 0, "B" → 1, ..., "Z" → 25. Case-insensitive, trims
// whitespace. Returns -1 for invalid labels so callers can treat
// the sample as malformed.
func labelIndex(label string) int {
	trimmed := strings.ToUpper(strings.TrimSpace(label))
	if len(trimmed) != 1 {
		return -1
	}
	c := trimmed[0]
	if c < 'A' || c > 'Z' {
		return -1
	}
	return int(c - 'A')
}

// computeBordaPoints tallies Borda points per agent across all
// valid samples. Nil rows (failed samples) are skipped. For n agents
// and rank r, the agent earns (n - r) points for that sample. Sum
// across samples gives total Borda points per agent.
func computeBordaPoints(sampleRankings [][]int, n int) []int {
	points := make([]int, n)
	for _, ranking := range sampleRankings {
		if ranking == nil {
			continue
		}
		for agentIdx, rank := range ranking {
			if rank < 1 || rank > n {
				continue
			}
			points[agentIdx] += n - rank
		}
	}
	return points
}

// finalRanksFromBorda converts Borda points to 1-based ranks. Ties
// get the same rank number, next distinct score skips by the tied
// count (standard competition ranking: 1, 2, 2, 4). The output is
// indexed by agentIdx.
func finalRanksFromBorda(points []int) []int {
	type agentPoint struct {
		idx int
		pts int
	}
	sorted := make([]agentPoint, len(points))
	for i, p := range points {
		sorted[i] = agentPoint{idx: i, pts: p}
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].pts > sorted[j].pts
	})

	ranks := make([]int, len(points))
	currentRank := 1
	for i, ap := range sorted {
		if i > 0 && sorted[i-1].pts != ap.pts {
			currentRank = i + 1
		}
		ranks[ap.idx] = currentRank
	}
	return ranks
}

// normalizeBorda maps a per-agent Borda total to a [0, 1]
// normalized score via:
//
//	normalized = points / (samples * (n - 1))
//
// Always-first agent (rank 1 in every sample) → normalized = 1.
// Always-last agent (rank n in every sample) → normalized = 0.
//
// Edge cases:
//   - n < 2: undefined (divide by zero) → return 0. Caller should
//     have short-circuited to state=unavailable before reaching here.
//   - samples = 0: same, return 0.
func normalizeBorda(points, samples, n int) float64 {
	if samples <= 0 || n < 2 {
		return 0
	}
	denom := float64(samples * (n - 1))
	return float64(points) / denom
}

// deriveNWiseConfidence bins the rank consistency of a single agent
// across samples:
//
//   - single-sample run → "medium" (N=1 samples overclaims if high)
//   - agent always same rank → "high"
//   - modal rank ≥ 67% of samples → "medium"
//   - else → "low"
//
// Matches Phase 6 plan D10. Rank consistency is a better proxy for
// rubric variance on ranking data: an agent that moved between
// rank 1 and rank 3 across samples has a noisy signal even if the
// Borda average is stable.
func deriveNWiseConfidence(sampleRankings [][]int, agentIdx, validSamples int) string {
	if validSamples <= 0 {
		return "low"
	}
	if validSamples == 1 {
		return "medium"
	}
	rankCounts := make(map[int]int)
	for _, ranking := range sampleRankings {
		if ranking == nil {
			continue
		}
		rankCounts[ranking[agentIdx]]++
	}
	modalCount := 0
	for _, c := range rankCounts {
		if c > modalCount {
			modalCount = c
		}
	}
	if modalCount == validSamples {
		return "high"
	}
	// modalCount / validSamples >= 2/3, via integer math to avoid
	// rounding (2/3 ≈ 0.666... < 0.67 would reject 2-of-3 consistency).
	if 3*modalCount >= 2*validSamples {
		return "medium"
	}
	return "low"
}

// nwiseAgentPayload is the per-agent jsonb shape persisted in
// llm_judge_results.payload for an n_wise judge. Captures the
// agent's rank, Borda breakdown, and cross-sample rank history.
type nwiseAgentPayload struct {
	Mode              string   `json:"mode"`
	Judge             string   `json:"judge"`
	Available         bool     `json:"available"`
	FinalRank         int      `json:"final_rank"`
	BordaPoints       int      `json:"borda_points"`
	TotalSamples      int      `json:"total_samples"`
	ValidSamples      int      `json:"valid_samples"`
	AgentLabelHistory []string `json:"agent_label_history"`
	RankHistory       []int    `json:"rank_history"`
	PositionDebiasing bool     `json:"position_debiasing"`
	Orderings         []string `json:"orderings"`
	ReasoningSamples  []string `json:"reasoning_samples,omitempty"`
}

// mustMarshalNWisePayload serialises the per-agent breakdown into
// the jsonb payload. Never errors in practice — inputs are simple
// types. Returns {} on any failure so callers never have to special
// case a nil payload.
//
// agentIdx is the agent's position in the canonical agents[] slice
// the aggregator is iterating. Used to index into each sample's
// ranking slice (which is also agentIdx-keyed) and to compute
// which label slot this agent occupied in each sample's ordering
// (so operators can diagnose position-bias issues from the payload).
func mustMarshalNWisePayload(
	judge scoring.LLMJudgeDeclaration,
	agentIdx int,
	finalRank, bordaPoints int,
	sampleRankings [][]int,
	reasoningPerSample []string,
	orderings [][]int,
	validSamples int,
) json.RawMessage {
	rankHistory := make([]int, 0, len(sampleRankings))
	labelHistory := make([]string, 0, len(sampleRankings))
	for i, ranking := range sampleRankings {
		if ranking == nil {
			rankHistory = append(rankHistory, 0)
			labelHistory = append(labelHistory, "")
			continue
		}
		rankHistory = append(rankHistory, ranking[agentIdx])
		labelHistory = append(labelHistory, labelForAgentIndexInOrdering(agentIdx, orderings[i]))
	}

	payload := nwiseAgentPayload{
		Mode:              string(scoring.JudgeMethodNWise),
		Judge:             judge.Key,
		Available:         true,
		FinalRank:         finalRank,
		BordaPoints:       bordaPoints,
		TotalSamples:      len(sampleRankings),
		ValidSamples:      validSamples,
		AgentLabelHistory: labelHistory,
		RankHistory:       rankHistory,
		PositionDebiasing: judge.PositionDebiasing,
		Orderings:         renderOrderings(orderings),
		ReasoningSamples:  cleanReasoning(reasoningPerSample),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}

// labelForAgentIndexInOrdering returns the label string ("A", "B",
// ...) that the given agent index occupied in a single sample's
// ordering. The ordering is a slot-index → agent-index permutation,
// so we invert: find the slot j where ordering[j] == agentIdx and
// return nwiseLabelAt(j). Returns empty string when the agent
// doesn't appear in the ordering (shouldn't happen in practice —
// orderings are always full permutations — but defended against).
func labelForAgentIndexInOrdering(agentIdx int, ordering []int) string {
	for slot, idx := range ordering {
		if idx == agentIdx {
			return nwiseLabelAt(slot)
		}
	}
	return ""
}

// renderOrderings converts ordering index slices into human-readable
// label sequences ("A,B,C", "B,C,A", ...) for the payload.
func renderOrderings(orderings [][]int) []string {
	out := make([]string, len(orderings))
	for i, ordering := range orderings {
		parts := make([]string, len(ordering))
		// In the prompt, slot j is rendered as label nwiseLabelAt(j)
		// and maps to agents[ordering[j]]. So the displayed sequence
		// at slot j is nwiseLabelAt(j) pointing at agent ordering[j].
		// For the payload, we render the displayed label sequence
		// so operators can reproduce what the model saw: "A→agent[1],
		// B→agent[2], C→agent[0]" → "ABC".
		for j := range ordering {
			parts[j] = nwiseLabelAt(j)
		}
		out[i] = strings.Join(parts, "")
	}
	return out
}

// cleanReasoning filters out empty reasoning strings and truncates
// each surviving entry to 200 chars so the payload stays small.
func cleanReasoning(reasoning []string) []string {
	out := make([]string, 0, len(reasoning))
	for _, r := range reasoning {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if len(r) > 200 {
			runes := []rune(r)
			if len(runes) > 200 {
				r = string(runes[:200]) + "..."
			}
		}
		out = append(out, r)
	}
	return out
}

// sentinelErr for future use when EvaluateNWise wants to surface a
// terminal Evaluate-wide error (e.g., spec load failure). Currently
// unused — all failures are captured per-agent.
var errNWiseTerminal = errors.New("n_wise evaluation terminal failure")

var _ = errNWiseTerminal
