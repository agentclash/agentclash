package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
)

func (a *Activities) executeRunAgentEvaluation(ctx context.Context, runAgentID uuid.UUID) (scoring.RunAgentEvaluation, error) {
	executionContext, err := a.repo.GetRunAgentExecutionContextByID(ctx, runAgentID)
	if err != nil {
		return scoring.RunAgentEvaluation{}, err
	}

	manifestSpec, err := scoring.LoadEvaluationSpec(executionContext.ChallengePackVersion.Manifest)
	if err != nil {
		emitErr := recordScoringFailedEvent(ctx, a.repo, executionContext.Run.ID, runAgentID, fmt.Sprintf("load evaluation spec from manifest: %v", err))
		if emitErr != nil {
			return scoring.RunAgentEvaluation{}, fmt.Errorf("load evaluation spec from manifest: %w; additionally failed to record scoring failure: %v", err, emitErr)
		}
		return scoring.RunAgentEvaluation{}, fmt.Errorf("load evaluation spec from manifest: %w", err)
	}

	specRecord, err := ensurePersistedEvaluationSpec(ctx, a.repo, executionContext.ChallengePackVersion.ID, manifestSpec)
	if err != nil {
		emitErr := recordScoringFailedEvent(ctx, a.repo, executionContext.Run.ID, runAgentID, fmt.Sprintf("load persisted evaluation spec: %v", err))
		if emitErr != nil {
			return scoring.RunAgentEvaluation{}, fmt.Errorf("load persisted evaluation spec: %w; additionally failed to record scoring failure: %v", err, emitErr)
		}
		return scoring.RunAgentEvaluation{}, fmt.Errorf("load persisted evaluation spec: %w", err)
	}

	persistedSpec, err := scoring.DecodeDefinition(specRecord.Definition)
	if err != nil {
		emitErr := recordScoringFailedEvent(ctx, a.repo, executionContext.Run.ID, runAgentID, fmt.Sprintf("decode persisted evaluation spec: %v", err))
		if emitErr != nil {
			return scoring.RunAgentEvaluation{}, fmt.Errorf("decode persisted evaluation spec: %w; additionally failed to record scoring failure: %v", err, emitErr)
		}
		return scoring.RunAgentEvaluation{}, fmt.Errorf("decode persisted evaluation spec: %w", err)
	}

	events, err := a.repo.ListRunEventsByRunAgentID(ctx, runAgentID)
	if err != nil {
		emitErr := recordScoringFailedEvent(ctx, a.repo, executionContext.Run.ID, runAgentID, fmt.Sprintf("list run events: %v", err))
		if emitErr != nil {
			return scoring.RunAgentEvaluation{}, fmt.Errorf("list run events: %w; additionally failed to record scoring failure: %v", err, emitErr)
		}
		return scoring.RunAgentEvaluation{}, fmt.Errorf("list run events: %w", err)
	}

	challengeInputs, err := mapChallengeInputs(executionContext.ChallengePackVersion.Manifest, executionContext.ChallengeInputSet)
	if err != nil {
		emitErr := recordScoringFailedEvent(ctx, a.repo, executionContext.Run.ID, runAgentID, fmt.Sprintf("map challenge inputs: %v", err))
		if emitErr != nil {
			return scoring.RunAgentEvaluation{}, fmt.Errorf("map challenge inputs: %w; additionally failed to record scoring failure: %v", err, emitErr)
		}
		return scoring.RunAgentEvaluation{}, fmt.Errorf("map challenge inputs: %w", err)
	}

	evaluationInput := scoring.EvaluationInput{
		RunAgentID:       runAgentID,
		EvaluationSpecID: specRecord.ID,
		ChallengeInputs:  challengeInputs,
		Events:           mapRunEvents(events),
	}

	var evaluation scoring.RunAgentEvaluation
	switch persistedSpec.JudgeMode {
	case scoring.JudgeModeDeterministic:
		evaluation, err = scoring.EvaluateRunAgent(evaluationInput, persistedSpec)
	default:
		judgeResults, judgeWarnings := evaluateLLMJudges(ctx, a.judgeClient, a.repo, executionContext, evaluationInput, persistedSpec)
		evaluation, err = scoring.EvaluateRunAgentWithLLMJudgeResults(evaluationInput, persistedSpec, judgeResults)
		if err == nil && len(judgeWarnings) > 0 {
			evaluation.Warnings = append(evaluation.Warnings, judgeWarnings...)
		}
	}
	if err != nil {
		emitErr := recordScoringFailedEvent(ctx, a.repo, executionContext.Run.ID, runAgentID, fmt.Sprintf("evaluate run agent: %v", err))
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

	if err := a.repo.StoreRunAgentEvaluationResults(ctx, evaluation); err != nil {
		emitErr := recordScoringFailedEvent(ctx, a.repo, executionContext.Run.ID, runAgentID, fmt.Sprintf("persist evaluation results: %v", err))
		if emitErr != nil {
			return scoring.RunAgentEvaluation{}, fmt.Errorf("persist evaluation results: %w; additionally failed to record scoring failure: %v", err, emitErr)
		}
		return scoring.RunAgentEvaluation{}, fmt.Errorf("persist evaluation results: %w", err)
	}

	if err := recordScoringEvents(ctx, a.repo, executionContext.Run.ID, evaluation); err != nil {
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
			Type:           string(event.EventType),
			Source:         string(event.Source),
			SequenceNumber: event.SequenceNumber,
			OccurredAt:     event.OccurredAt.UTC(),
			Payload:        cloneJSON(event.Payload),
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

	llmJudgeResults := make([]map[string]any, 0, len(evaluation.LLMJudgeResults))
	for _, result := range evaluation.LLMJudgeResults {
		payload := map[string]any{
			"judge_key":     result.JudgeKey,
			"mode":          result.Mode,
			"sample_count":  result.SampleCount,
			"model_count":   result.ModelCount,
			"reason":        result.Reason,
		}
		if result.NormalizedScore != nil {
			payload["normalized_score"] = *result.NormalizedScore
		}
		if result.Confidence != nil {
			payload["confidence"] = *result.Confidence
		}
		if result.Variance != nil {
			payload["variance"] = *result.Variance
		}
		llmJudgeResults = append(llmJudgeResults, payload)
	}

	return map[string]any{
		"evaluation_spec_id": evaluation.EvaluationSpecID,
		"status":             evaluation.Status,
		"dimension_scores":   dimensionScores,
		"llm_judge_results":  llmJudgeResults,
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
