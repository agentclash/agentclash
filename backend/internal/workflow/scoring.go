package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring/judge"
	"github.com/google/uuid"
)

func executeRunAgentEvaluation(ctx context.Context, repo RunRepository, runAgentID uuid.UUID) (scoring.RunAgentEvaluation, error) {
	executionContext, err := repo.GetRunAgentExecutionContextByID(ctx, runAgentID)
	if err != nil {
		return scoring.RunAgentEvaluation{}, err
	}

	manifestSpec, err := scoring.LoadEvaluationSpec(executionContext.ChallengePackVersion.Manifest)
	if err != nil {
		emitErr := recordScoringFailedEvent(ctx, repo, executionContext.Run.ID, runAgentID, fmt.Sprintf("load evaluation spec from manifest: %v", err))
		if emitErr != nil {
			return scoring.RunAgentEvaluation{}, fmt.Errorf("load evaluation spec from manifest: %w; additionally failed to record scoring failure: %v", err, emitErr)
		}
		return scoring.RunAgentEvaluation{}, fmt.Errorf("load evaluation spec from manifest: %w", err)
	}

	specRecord, err := ensurePersistedEvaluationSpec(ctx, repo, executionContext.ChallengePackVersion.ID, manifestSpec)
	if err != nil {
		emitErr := recordScoringFailedEvent(ctx, repo, executionContext.Run.ID, runAgentID, fmt.Sprintf("load persisted evaluation spec: %v", err))
		if emitErr != nil {
			return scoring.RunAgentEvaluation{}, fmt.Errorf("load persisted evaluation spec: %w; additionally failed to record scoring failure: %v", err, emitErr)
		}
		return scoring.RunAgentEvaluation{}, fmt.Errorf("load persisted evaluation spec: %w", err)
	}

	persistedSpec, err := scoring.DecodeDefinition(specRecord.Definition)
	if err != nil {
		emitErr := recordScoringFailedEvent(ctx, repo, executionContext.Run.ID, runAgentID, fmt.Sprintf("decode persisted evaluation spec: %v", err))
		if emitErr != nil {
			return scoring.RunAgentEvaluation{}, fmt.Errorf("decode persisted evaluation spec: %w; additionally failed to record scoring failure: %v", err, emitErr)
		}
		return scoring.RunAgentEvaluation{}, fmt.Errorf("decode persisted evaluation spec: %w", err)
	}

	events, err := repo.ListRunEventsByRunAgentID(ctx, runAgentID)
	if err != nil {
		emitErr := recordScoringFailedEvent(ctx, repo, executionContext.Run.ID, runAgentID, fmt.Sprintf("list run events: %v", err))
		if emitErr != nil {
			return scoring.RunAgentEvaluation{}, fmt.Errorf("list run events: %w; additionally failed to record scoring failure: %v", err, emitErr)
		}
		return scoring.RunAgentEvaluation{}, fmt.Errorf("list run events: %w", err)
	}

	challengeInputs, err := mapChallengeInputs(executionContext.ChallengePackVersion.Manifest, executionContext.ChallengeInputSet)
	if err != nil {
		emitErr := recordScoringFailedEvent(ctx, repo, executionContext.Run.ID, runAgentID, fmt.Sprintf("map challenge inputs: %v", err))
		if emitErr != nil {
			return scoring.RunAgentEvaluation{}, fmt.Errorf("map challenge inputs: %w; additionally failed to record scoring failure: %v", err, emitErr)
		}
		return scoring.RunAgentEvaluation{}, fmt.Errorf("map challenge inputs: %w", err)
	}

	evaluation, err := scoring.EvaluateRunAgent(scoring.EvaluationInput{
		RunAgentID:       runAgentID,
		EvaluationSpecID: specRecord.ID,
		ChallengeInputs:  challengeInputs,
		Events:           mapRunEvents(events),
	}, persistedSpec)
	if err != nil {
		emitErr := recordScoringFailedEvent(ctx, repo, executionContext.Run.ID, runAgentID, fmt.Sprintf("evaluate run agent: %v", err))
		if emitErr != nil {
			return scoring.RunAgentEvaluation{}, fmt.Errorf("evaluate run agent: %w; additionally failed to record scoring failure: %v", err, emitErr)
		}
		return scoring.RunAgentEvaluation{}, fmt.Errorf("evaluate run agent: %w", err)
	}

	// Log individual validator results for debugging — especially errors.
	for _, vr := range evaluation.ValidatorResults {
		if vr.State == scoring.OutputStateError || vr.Verdict == "error" {
			slog.Error("validator error",
				"run_agent_id", runAgentID,
				"validator_key", vr.Key,
				"validator_type", vr.Type,
				"state", vr.State,
				"verdict", vr.Verdict,
				"reason", vr.Reason,
			)
		} else {
			slog.Info("validator result",
				"run_agent_id", runAgentID,
				"validator_key", vr.Key,
				"validator_type", vr.Type,
				"verdict", vr.Verdict,
			)
		}
	}

	if err := repo.StoreRunAgentEvaluationResults(ctx, evaluation); err != nil {
		emitErr := recordScoringFailedEvent(ctx, repo, executionContext.Run.ID, runAgentID, fmt.Sprintf("persist evaluation results: %v", err))
		if emitErr != nil {
			return scoring.RunAgentEvaluation{}, fmt.Errorf("persist evaluation results: %w; additionally failed to record scoring failure: %v", err, emitErr)
		}
		return scoring.RunAgentEvaluation{}, fmt.Errorf("persist evaluation results: %w", err)
	}

	if err := recordScoringEvents(ctx, repo, executionContext.Run.ID, evaluation); err != nil {
		// Persisted judge/metric rows are the source of truth. A failure to emit
		// derived replay events should not flip an otherwise successful run-agent
		// into failed after evaluation results are already durable.
		evaluation.Warnings = append(evaluation.Warnings, fmt.Sprintf("record scoring events: %v", err))
	}

	return evaluation, nil
}

func ensurePersistedEvaluationSpec(
	ctx context.Context,
	repo RunRepository,
	challengePackVersionID uuid.UUID,
	manifestSpec scoring.EvaluationSpec,
) (repository.EvaluationSpecRecord, error) {
	specRecord, err := repo.GetEvaluationSpecByChallengePackVersionAndVersion(
		ctx,
		challengePackVersionID,
		manifestSpec.Name,
		manifestSpec.VersionNumber,
	)
	if err == nil {
		return specRecord, nil
	}
	if !isEvaluationSpecNotFound(err) {
		return repository.EvaluationSpecRecord{}, err
	}

	definition, err := scoring.MarshalDefinition(manifestSpec)
	if err != nil {
		return repository.EvaluationSpecRecord{}, fmt.Errorf("marshal manifest evaluation spec: %w", err)
	}

	created, createErr := repo.CreateEvaluationSpec(ctx, repository.CreateEvaluationSpecParams{
		ChallengePackVersionID: challengePackVersionID,
		Name:                   manifestSpec.Name,
		VersionNumber:          manifestSpec.VersionNumber,
		JudgeMode:              string(manifestSpec.JudgeMode),
		Definition:             definition,
	})
	if createErr == nil {
		return created, nil
	}

	// Another concurrent scoring activity may have inserted the same spec first.
	refetched, refetchErr := repo.GetEvaluationSpecByChallengePackVersionAndVersion(
		ctx,
		challengePackVersionID,
		manifestSpec.Name,
		manifestSpec.VersionNumber,
	)
	if refetchErr == nil {
		return refetched, nil
	}

	return repository.EvaluationSpecRecord{}, createErr
}

func isEvaluationSpecNotFound(err error) bool {
	return errors.Is(err, repository.ErrEvaluationSpecNotFound)
}

func mapChallengeInputs(manifest []byte, inputSet *repository.ChallengeInputSetExecutionContext) ([]scoring.EvidenceInput, error) {
	return repository.BuildScoringEvidenceInputs(manifest, inputSet)
}

func mapRunEvents(events []repository.RunEvent) []scoring.Event {
	mapped := make([]scoring.Event, 0, len(events))
	for _, event := range events {
		mapped = append(mapped, scoring.Event{
			Type:       string(event.EventType),
			Source:     string(event.Source),
			OccurredAt: event.OccurredAt.UTC(),
			Payload:    cloneJSON(event.Payload),
		})
	}
	return mapped
}

func recordScoringEvents(ctx context.Context, repo RunRepository, runID uuid.UUID, evaluation scoring.RunAgentEvaluation) error {
	now := time.Now().UTC()
	if _, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       fmt.Sprintf("scoring:%s:%s:started", evaluation.RunAgentID, evaluation.EvaluationSpecID),
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         runID,
			RunAgentID:    evaluation.RunAgentID,
			EventType:     runevents.EventTypeScoringStarted,
			Source:        runevents.SourceWorkerScoring,
			OccurredAt:    now,
			Payload: mustMarshalJSON(map[string]any{
				"evaluation_spec_id": evaluation.EvaluationSpecID,
			}),
			Summary: runevents.SummaryMetadata{
				Status:        "running",
				EvidenceLevel: runevents.EvidenceLevelDerivedSummary,
			},
		},
	}); err != nil {
		return err
	}

	for _, metric := range evaluation.MetricResults {
		payload := map[string]any{
			"evaluation_spec_id": evaluation.EvaluationSpecID,
			"metric_key":         metric.Key,
			"collector":          metric.Collector,
			"state":              metric.State,
			"reason":             metric.Reason,
		}
		if metric.NumericValue != nil {
			payload["numeric_value"] = *metric.NumericValue
		}
		if metric.TextValue != nil {
			payload["text_value"] = *metric.TextValue
		}
		if metric.BooleanValue != nil {
			payload["boolean_value"] = *metric.BooleanValue
		}

		if _, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
			Event: runevents.Envelope{
				EventID:       fmt.Sprintf("scoring:%s:%s:metric:%s", evaluation.RunAgentID, evaluation.EvaluationSpecID, metric.Key),
				SchemaVersion: runevents.SchemaVersionV1,
				RunID:         runID,
				RunAgentID:    evaluation.RunAgentID,
				EventType:     runevents.EventTypeScoringMetricRecorded,
				Source:        runevents.SourceWorkerScoring,
				OccurredAt:    time.Now().UTC(),
				Payload:       mustMarshalJSON(payload),
				Summary: runevents.SummaryMetadata{
					Status:        string(metric.State),
					MetricKey:     metric.Key,
					EvidenceLevel: runevents.EvidenceLevelDerivedSummary,
				},
			},
		}); err != nil {
			return err
		}
	}

	_, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       fmt.Sprintf("scoring:%s:%s:completed", evaluation.RunAgentID, evaluation.EvaluationSpecID),
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         runID,
			RunAgentID:    evaluation.RunAgentID,
			EventType:     runevents.EventTypeScoringCompleted,
			Source:        runevents.SourceWorkerScoring,
			OccurredAt:    time.Now().UTC(),
			Payload:       mustMarshalJSON(scoringCompletedPayload(evaluation)),
			Summary: runevents.SummaryMetadata{
				Status:        scoringTerminalStatus(evaluation.Status),
				EvidenceLevel: runevents.EvidenceLevelDerivedSummary,
			},
		},
	})
	return err
}

func recordScoringFailedEvent(ctx context.Context, repo RunRepository, runID uuid.UUID, runAgentID uuid.UUID, reason string) error {
	_, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       fmt.Sprintf("scoring:%s:failed:%d", runAgentID, time.Now().UTC().UnixNano()),
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         runID,
			RunAgentID:    runAgentID,
			EventType:     runevents.EventTypeScoringFailed,
			Source:        runevents.SourceWorkerScoring,
			OccurredAt:    time.Now().UTC(),
			Payload:       mustMarshalJSON(map[string]any{"error": reason}),
			Summary: runevents.SummaryMetadata{
				Status:        "failed",
				EvidenceLevel: runevents.EvidenceLevelDerivedSummary,
			},
		},
	})
	return err
}

func scoringCompletedPayload(evaluation scoring.RunAgentEvaluation) map[string]any {
	dimensionScores := make(map[string]any, len(evaluation.DimensionScores))
	for key, value := range evaluation.DimensionScores {
		if value == nil {
			dimensionScores[key] = nil
			continue
		}
		dimensionScores[key] = *value
	}

	return map[string]any{
		"evaluation_spec_id": evaluation.EvaluationSpecID,
		"status":             evaluation.Status,
		"dimension_scores":   dimensionScores,
		"warnings":           append([]string(nil), evaluation.Warnings...),
	}
}

func scoringTerminalStatus(status scoring.EvaluationStatus) string {
	if status == scoring.EvaluationStatusFailed {
		return "failed"
	}
	return "completed"
}

func mustMarshalJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

// executeJudgeRunAgent runs the LLM-as-judge evaluator for a single
// run-agent and persists the finalized scorecard. Called by the
// JudgeRunAgent activity after ScoreRunAgent has produced the
// deterministic evaluation.
//
// Fast-path returns leave the deterministic evaluation unchanged
// (and skip all DB writes) when:
//
//   - judgeEvaluator is nil — workers / tests not wired for judges
//   - the persisted spec declares no LLMJudges — deterministic-only packs
//   - the spec fails to load — error propagates so Temporal retries
//
// When judges are present and the evaluator is wired, the helper:
//
//  1. Loads the persisted spec via the same path ScoreRunAgent uses
//  2. Reads the run-agent's events and extracts final_output
//  3. Calls judge.Evaluator.Evaluate (bounded fan-out across
//     models × samples, anti-gaming envelope, multi-model consensus)
//  4. Calls scoring.FinalizeRunAgentEvaluation to merge the judge
//     results into the deterministic dimension dispatch and recompute
//     the overall score / passed verdict
//  5. Persists the finalized scorecard + judge result rows in one
//     transaction via repo.StoreFinalizedScoringResults
//  6. Emits scoring.judge.recorded events (one per judge) for replay
//     observability
//
// Per-judge errors NEVER abort the activity. The judge evaluator
// captures them as JudgeResults with state=error and a Reason; the
// finalize merge leaves the corresponding dim as unavailable; the
// scorecard ships with state=partial. Operators see the failures via
// the events and the persisted error-state rows.
func executeJudgeRunAgent(
	ctx context.Context,
	repo RunRepository,
	judgeEvaluator *judge.Evaluator,
	runAgentID uuid.UUID,
	deterministicEvaluation scoring.RunAgentEvaluation,
	nwiseJudgeResults []scoring.JudgeResult,
) (scoring.RunAgentEvaluation, error) {
	// Fast-path 1: no evaluator wired (test fixtures, dev workers
	// without provider credentials). Return the deterministic
	// evaluation unchanged and skip all DB / event work.
	//
	// Exception: if the workflow provided n_wise results (which come
	// from a separately-wired JudgeRun activity), we still need to
	// finalize and persist them even though the per-agent evaluator
	// is nil. In practice the two evaluators share a single bootstrap
	// path, so this branch is a defensive fallthrough rather than a
	// real execution mode.
	if judgeEvaluator == nil && len(nwiseJudgeResults) == 0 {
		return deterministicEvaluation, nil
	}

	executionContext, err := repo.GetRunAgentExecutionContextByID(ctx, runAgentID)
	if err != nil {
		return deterministicEvaluation, fmt.Errorf("load run-agent execution context for judges: %w", err)
	}

	manifestSpec, err := scoring.LoadEvaluationSpec(executionContext.ChallengePackVersion.Manifest)
	if err != nil {
		return deterministicEvaluation, fmt.Errorf("load evaluation spec for judges: %w", err)
	}

	// Fast-path 2: spec declares no judges and JudgeRun produced no
	// n_wise results. The deterministic evaluation is already final.
	if len(manifestSpec.LLMJudges) == 0 && len(nwiseJudgeResults) == 0 {
		return deterministicEvaluation, nil
	}

	specRecord, err := ensurePersistedEvaluationSpec(ctx, repo, executionContext.ChallengePackVersion.ID, manifestSpec)
	if err != nil {
		return deterministicEvaluation, fmt.Errorf("load persisted evaluation spec for judges: %w", err)
	}
	persistedSpec, err := scoring.DecodeDefinition(specRecord.Definition)
	if err != nil {
		return deterministicEvaluation, fmt.Errorf("decode persisted evaluation spec for judges: %w", err)
	}

	events, err := repo.ListRunEventsByRunAgentID(ctx, runAgentID)
	if err != nil {
		return deterministicEvaluation, fmt.Errorf("list run events for judges: %w", err)
	}
	scoringEvents := mapRunEvents(events)
	finalOutput := scoring.ExtractFinalOutputFromEvents(scoringEvents)

	// Phase 5 (#148): resolve the union of every judge's ContextFrom
	// and ReferenceFrom values so the rubric/reference prompt
	// builders can populate CONTEXT and REFERENCE ANSWER blocks
	// without the judge package touching evidence internals. The
	// rubric/reference dispatch in internal/scoring/judge/rubric.go
	// looks these up by reference string.
	challengeInputs, mapErr := mapChallengeInputs(executionContext.ChallengePackVersion.Manifest, executionContext.ChallengeInputSet)
	if mapErr != nil {
		// Challenge-input mapping failures are non-fatal for judges —
		// we can still run judges that don't need evidence refs (e.g.,
		// rubric judges with only final_output context). Log the error
		// as a warning and proceed with an empty challenge inputs set.
		deterministicEvaluation.Warnings = append(deterministicEvaluation.Warnings,
			fmt.Sprintf("map challenge inputs for judges: %v", mapErr))
		challengeInputs = nil
	}
	resolvedRefs := collectAndResolveJudgeRefs(persistedSpec.LLMJudges, challengeInputs, scoringEvents)
	challengeInput := resolvedRefs["challenge_input"]

	// Phase 6 (#148): per-agent Evaluate filters out n_wise judges
	// internally, so we always pass the full LLMJudges slice here
	// and later concatenate the pre-computed n_wise results from
	// the JudgeRun activity. When the evaluator is nil (defensive
	// branch for the n_wise-only fast path), skip per-agent Evaluate
	// entirely and finalize with just the n_wise results.
	var perAgentResults []scoring.JudgeResult
	if judgeEvaluator != nil {
		judgeResult, judgeErr := judgeEvaluator.Evaluate(ctx, judge.Input{
			RunAgentID:         runAgentID,
			EvaluationSpecID:   specRecord.ID,
			Judges:             persistedSpec.LLMJudges,
			FinalOutput:        finalOutput,
			ChallengeInput:     challengeInput,
			ResolvedReferences: resolvedRefs,
		})
		if judgeErr != nil {
			// Top-level evaluator errors are rare (the per-judge error
			// path captures provider failures cleanly). Surface as a
			// warning and continue with whatever results we got.
			deterministicEvaluation.Warnings = append(deterministicEvaluation.Warnings,
				fmt.Sprintf("judge evaluator returned error: %v", judgeErr))
		}
		if len(judgeResult.Warnings) > 0 {
			deterministicEvaluation.Warnings = append(deterministicEvaluation.Warnings, judgeResult.Warnings...)
		}
		perAgentResults = judgeResult.JudgeResults
	}

	// Merge per-agent + n_wise results. FinalizeRunAgentEvaluation
	// expects every LLMJudgeDeclaration in the spec to be represented
	// exactly once, so the combined slice becomes the authoritative
	// input for both the dim-dispatch merge and the persistence path.
	mergedResults := make([]scoring.JudgeResult, 0, len(perAgentResults)+len(nwiseJudgeResults))
	mergedResults = append(mergedResults, perAgentResults...)
	mergedResults = append(mergedResults, nwiseJudgeResults...)

	finalized := scoring.FinalizeRunAgentEvaluation(deterministicEvaluation, persistedSpec, mergedResults)

	if err := repo.StoreFinalizedScoringResults(ctx, finalized, mergedResults); err != nil {
		return deterministicEvaluation, fmt.Errorf("persist finalized scoring results: %w", err)
	}

	if err := recordJudgeEvents(ctx, repo, executionContext.Run.ID, finalized.RunAgentID, finalized.EvaluationSpecID, mergedResults); err != nil {
		// Persisted judge rows are the source of truth. Failure to
		// emit derived events should not flip an otherwise successful
		// finalize into a fatal error after writes are durable.
		finalized.Warnings = append(finalized.Warnings, fmt.Sprintf("record judge events: %v", err))
	}

	return finalized, nil
}

// executeJudgeRun runs the run-level n_wise judge pass for all agents
// in runAgentIDs. Called by the JudgeRun Temporal activity once per
// run-workflow, between ScoreRunAgent and JudgeRunAgent. Returns a
// map of per-agent n_wise results that the workflow threads into each
// JudgeRunAgentInput.NWiseJudgeResults.
//
// Fast-path returns leave the output empty and return (_, nil) when:
//
//  1. judgeEvaluator is nil — tests / dev workers without creds.
//  2. len(runAgentIDs) < 2 — n_wise needs at least 2 agents.
//  3. The persisted spec has no n_wise judges.
//  4. Every agent's final_output is empty.
//
// The activity does NOT write to the database; persistence happens
// inside JudgeRunAgent via StoreFinalizedScoringResults to keep every
// row in the same transaction as the run-agent's scorecard.
//
// Error handling: loading a single agent's events or execution
// context failing is treated as a missing agent — we warn, skip that
// agent, and proceed with the others. A failure to load the spec
// itself propagates up so Temporal retries the activity.
func executeJudgeRun(
	ctx context.Context,
	repo RunRepository,
	judgeEvaluator *judge.Evaluator,
	runID uuid.UUID,
	runAgentIDs []uuid.UUID,
) (JudgeRunOutput, error) {
	empty := JudgeRunOutput{PerAgent: map[uuid.UUID][]scoring.JudgeResult{}}

	if judgeEvaluator == nil {
		return empty, nil
	}
	if len(runAgentIDs) < 2 {
		return empty, nil
	}

	// Load the spec once from the first agent's execution context.
	// Every agent in a run shares the same challenge pack version, so
	// the persisted evaluation spec is identical across agents.
	firstContext, err := repo.GetRunAgentExecutionContextByID(ctx, runAgentIDs[0])
	if err != nil {
		return empty, fmt.Errorf("load first run-agent execution context for n_wise judges: %w", err)
	}
	manifestSpec, err := scoring.LoadEvaluationSpec(firstContext.ChallengePackVersion.Manifest)
	if err != nil {
		return empty, fmt.Errorf("load evaluation spec for n_wise judges: %w", err)
	}

	nwiseJudges := filterNWiseJudges(manifestSpec.LLMJudges)
	if len(nwiseJudges) == 0 {
		return empty, nil
	}

	specRecord, err := ensurePersistedEvaluationSpec(ctx, repo, firstContext.ChallengePackVersion.ID, manifestSpec)
	if err != nil {
		return empty, fmt.Errorf("load persisted evaluation spec for n_wise judges: %w", err)
	}
	persistedSpec, err := scoring.DecodeDefinition(specRecord.Definition)
	if err != nil {
		return empty, fmt.Errorf("decode persisted evaluation spec for n_wise judges: %w", err)
	}
	persistedNWise := filterNWiseJudges(persistedSpec.LLMJudges)
	if len(persistedNWise) == 0 {
		return empty, nil
	}

	// Collect each agent's final output. Agents with load errors or
	// empty outputs are skipped; a warning records the exclusion so
	// operators can distinguish "agent crashed early" from "judge
	// chose to ignore this agent".
	var warnings []string
	agents := make([]judge.NWiseAgent, 0, len(runAgentIDs))
	var firstEventsForRefs []scoring.Event
	for _, runAgentID := range runAgentIDs {
		events, err := repo.ListRunEventsByRunAgentID(ctx, runAgentID)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("n_wise: failed to list events for %s: %v", runAgentID, err))
			continue
		}
		scoringEvents := mapRunEvents(events)
		finalOutput := scoring.ExtractFinalOutputFromEvents(scoringEvents)
		if finalOutput == "" {
			warnings = append(warnings, fmt.Sprintf("n_wise: skipping %s (empty final output)", runAgentID))
			continue
		}
		if firstEventsForRefs == nil {
			firstEventsForRefs = scoringEvents
		}
		agents = append(agents, judge.NWiseAgent{
			RunAgentID:  runAgentID,
			FinalOutput: finalOutput,
		})
	}

	if len(agents) < 2 {
		warnings = append(warnings, fmt.Sprintf("n_wise: only %d agent(s) with non-empty output; need >= 2", len(agents)))
		return JudgeRunOutput{
			PerAgent: map[uuid.UUID][]scoring.JudgeResult{},
			Warnings: warnings,
		}, nil
	}

	// Resolve shared references (challenge_input, case.*). N_wise
	// judges rank across agents, so the reference context is
	// necessarily run-scoped — pick the first successfully-loaded
	// run-agent's evidence set.
	challengeInputs, mapErr := mapChallengeInputs(firstContext.ChallengePackVersion.Manifest, firstContext.ChallengeInputSet)
	if mapErr != nil {
		warnings = append(warnings, fmt.Sprintf("n_wise: map challenge inputs: %v", mapErr))
		challengeInputs = nil
	}
	resolvedRefs := collectAndResolveJudgeRefs(persistedNWise, challengeInputs, firstEventsForRefs)

	result, err := judgeEvaluator.EvaluateNWise(ctx, judge.NWiseInput{
		RunID:              runID,
		EvaluationSpecID:   specRecord.ID,
		Judges:             persistedNWise,
		Agents:             agents,
		ResolvedReferences: resolvedRefs,
	})
	if err != nil {
		return empty, fmt.Errorf("evaluate n_wise judges: %w", err)
	}
	if len(result.Warnings) > 0 {
		warnings = append(warnings, result.Warnings...)
	}
	if result.PerAgent == nil {
		result.PerAgent = map[uuid.UUID][]scoring.JudgeResult{}
	}
	return JudgeRunOutput{
		PerAgent: result.PerAgent,
		Warnings: warnings,
	}, nil
}

// filterNWiseJudges returns the subset of judges whose Mode is
// n_wise. Used by executeJudgeRun to pass only n_wise judges into the
// evaluator and to short-circuit the fast path when the spec declares
// none. Per-agent Evaluate does the equivalent filter internally for
// the non-nwise modes.
func filterNWiseJudges(judges []scoring.LLMJudgeDeclaration) []scoring.LLMJudgeDeclaration {
	out := make([]scoring.LLMJudgeDeclaration, 0, len(judges))
	for _, j := range judges {
		if j.Mode == scoring.JudgeMethodNWise {
			out = append(out, j)
		}
	}
	return out
}

// collectAndResolveJudgeRefs gathers the union of every judge's
// ContextFrom and ReferenceFrom references, then calls
// scoring.ResolveContextReferences to map them to concrete string
// values. Used by executeJudgeRunAgent (#148 phase 5) to populate
// judge.Input.ResolvedReferences before the evaluator runs.
//
// Deduplication matters: a pack with 5 judges all referencing
// "challenge_input" should only resolve it once. The returned map
// is the same one the judge evaluator reads — the prompt builders
// look up each judge's needed refs by string key.
//
// Returns nil when there are no references to resolve, matching the
// ResolveContextReferences contract.
func collectAndResolveJudgeRefs(
	judges []scoring.LLMJudgeDeclaration,
	challengeInputs []scoring.EvidenceInput,
	events []scoring.Event,
) map[string]string {
	refSet := make(map[string]struct{})
	for _, j := range judges {
		for _, ref := range j.ContextFrom {
			if ref != "" {
				refSet[ref] = struct{}{}
			}
		}
		if j.ReferenceFrom != "" {
			refSet[j.ReferenceFrom] = struct{}{}
		}
	}
	if len(refSet) == 0 {
		return nil
	}
	refs := make([]string, 0, len(refSet))
	for ref := range refSet {
		refs = append(refs, ref)
	}
	return scoring.ResolveContextReferences(challengeInputs, events, refs)
}

// recordJudgeEvents emits one scoring.judge.recorded event per judge
// result so the replay timeline shows judge progress alongside
// deterministic scoring events. Mirrors the per-metric event loop in
// recordScoringEvents but with judge-specific payload fields.
func recordJudgeEvents(
	ctx context.Context,
	repo RunRepository,
	runID uuid.UUID,
	runAgentID uuid.UUID,
	evaluationSpecID uuid.UUID,
	judgeResults []scoring.JudgeResult,
) error {
	for _, jr := range judgeResults {
		payload := map[string]any{
			"evaluation_spec_id": evaluationSpecID,
			"judge_key":          jr.Key,
			"mode":               jr.Mode,
			"state":              jr.State,
			"sample_count":       jr.SampleCount,
			"model_count":        jr.ModelCount,
		}
		if jr.NormalizedScore != nil {
			payload["normalized_score"] = *jr.NormalizedScore
		}
		if jr.Confidence != "" {
			payload["confidence"] = jr.Confidence
		}
		if jr.Reason != "" {
			payload["reason"] = jr.Reason
		}

		if _, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
			Event: runevents.Envelope{
				EventID:       fmt.Sprintf("scoring:%s:%s:judge:%s", runAgentID, evaluationSpecID, jr.Key),
				SchemaVersion: runevents.SchemaVersionV1,
				RunID:         runID,
				RunAgentID:    runAgentID,
				EventType:     runevents.EventTypeScoringJudgeRecorded,
				Source:        runevents.SourceWorkerScoring,
				OccurredAt:    time.Now().UTC(),
				Payload:       mustMarshalJSON(payload),
				Summary: runevents.SummaryMetadata{
					Status:        string(jr.State),
					EvidenceLevel: runevents.EvidenceLevelDerivedSummary,
				},
			},
		}); err != nil {
			return err
		}
	}
	return nil
}
