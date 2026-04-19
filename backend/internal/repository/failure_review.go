package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/failurereview"
	"github.com/google/uuid"
)

func (r *Repository) ListRunFailureReviewItems(ctx context.Context, runID uuid.UUID, agentID *uuid.UUID) ([]failurereview.Item, error) {
	runAgents, err := r.ListRunAgentsByRunID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("list run agents for failure review: %w", err)
	}

	items := make([]failurereview.Item, 0)
	challengePackStatusByVersion := make(map[uuid.UUID]string)
	// TODO(#330): batch the per-run-agent reads in this method once we have either
	// consolidated read queries or a materialized failure review read model.
	for _, runAgent := range runAgents {
		if agentID != nil && runAgent.ID != *agentID {
			continue
		}

		executionContext, err := r.GetRunAgentExecutionContextByID(ctx, runAgent.ID)
		if err != nil {
			return nil, fmt.Errorf("load run-agent execution context %s: %w", runAgent.ID, err)
		}

		scorecard, err := r.GetRunAgentScorecardByRunAgentID(ctx, runAgent.ID)
		if err != nil {
			if err == ErrRunAgentScorecardNotFound {
				continue
			}
			return nil, fmt.Errorf("load run-agent scorecard %s: %w", runAgent.ID, err)
		}

		judgeResults, err := r.ListJudgeResultsByRunAgentAndEvaluationSpec(ctx, runAgent.ID, scorecard.EvaluationSpecID)
		if err != nil {
			return nil, fmt.Errorf("list judge results %s: %w", runAgent.ID, err)
		}
		metricResults, err := r.ListMetricResultsByRunAgentAndEvaluationSpec(ctx, runAgent.ID, scorecard.EvaluationSpecID)
		if err != nil {
			return nil, fmt.Errorf("list metric results %s: %w", runAgent.ID, err)
		}
		llmJudgeResults, err := r.ListLLMJudgeResultsByRunAgentAndEvaluationSpec(ctx, runAgent.ID, scorecard.EvaluationSpecID)
		if err != nil {
			return nil, fmt.Errorf("list llm judge results %s: %w", runAgent.ID, err)
		}
		runEvents, err := r.ListRunEventsByRunAgentID(ctx, runAgent.ID)
		if err != nil {
			return nil, fmt.Errorf("list run events %s: %w", runAgent.ID, err)
		}
		challengePackStatus, err := challengePackLifecycleStatus(ctx, r, executionContext.Run.ChallengePackVersionID, challengePackStatusByVersion)
		if err != nil {
			return nil, fmt.Errorf("load challenge pack lifecycle %s: %w", executionContext.Run.ChallengePackVersionID, err)
		}

		runAgentItems, err := failurereview.BuildRunAgentItems(failurereview.RunAgentInput{
			RunID:                executionContext.Run.ID,
			RunStatus:            executionContext.Run.Status,
			RunAgentID:           runAgent.ID,
			RunAgentLabel:        runAgent.Label,
			DeploymentType:       executionContext.Deployment.DeploymentType,
			ChallengePackStatus:  challengePackStatus,
			HasChallengeInputSet: executionContext.ChallengeInputSet != nil,
			ToolPolicy:           executionContext.ChallengePackVersion.Manifest,
			Cases:                mapFailureReviewCases(executionContext.ChallengeInputSet),
			Scorecard:            scorecard.Scorecard,
			JudgeResults:         mapFailureReviewJudgeResults(judgeResults),
			MetricResults:        mapFailureReviewMetricResults(metricResults),
			LLMJudgeResults:      mapFailureReviewLLMJudgeResults(llmJudgeResults),
			Events:               mapFailureReviewEvents(runEvents),
		})
		if err != nil {
			return nil, fmt.Errorf("build failure review items for %s: %w", runAgent.ID, err)
		}

		items = append(items, runAgentItems...)
	}

	return items, nil
}

func mapFailureReviewCases(inputSet *ChallengeInputSetExecutionContext) []failurereview.CaseContext {
	if inputSet == nil {
		return nil
	}
	items := make([]failurereview.CaseContext, 0, len(inputSet.Cases))
	for _, item := range inputSet.Cases {
		artifacts := make([]failurereview.ArtifactContext, 0, len(item.Artifacts)+len(item.Assets))
		for _, artifact := range item.Artifacts {
			artifacts = append(artifacts, failurereview.ArtifactContext{
				Key: artifact.Key,
			})
		}
		for _, asset := range item.Assets {
			artifacts = append(artifacts, failurereview.ArtifactContext{
				Key:       asset.Key,
				Kind:      asset.Kind,
				Path:      asset.Path,
				MediaType: asset.MediaType,
			})
		}
		items = append(items, failurereview.CaseContext{
			ChallengeIdentityID: item.ChallengeIdentityID,
			ChallengeKey:        item.ChallengeKey,
			CaseKey:             item.CaseKey,
			ItemKey:             item.ItemKey,
			Payload:             cloneJSON(item.Payload),
			Artifacts:           artifacts,
		})
	}
	return items
}

func mapFailureReviewJudgeResults(results []JudgeResultRecord) []failurereview.JudgeResult {
	mapped := make([]failurereview.JudgeResult, 0, len(results))
	for _, result := range results {
		mapped = append(mapped, failurereview.JudgeResult{
			ChallengeIdentityID: cloneUUIDPtr(result.ChallengeIdentityID),
			Key:                 result.JudgeKey,
			Verdict:             cloneStringPtr(result.Verdict),
			NormalizedScore:     cloneFloat64Ptr(result.NormalizedScore),
			Reason:              reasonFromRawOutput(result.RawOutput),
		})
	}
	return mapped
}

func mapFailureReviewMetricResults(results []MetricResultRecord) []failurereview.MetricResult {
	mapped := make([]failurereview.MetricResult, 0, len(results))
	for _, result := range results {
		mapped = append(mapped, failurereview.MetricResult{
			ChallengeIdentityID: cloneUUIDPtr(result.ChallengeIdentityID),
			Key:                 result.MetricKey,
			MetricType:          result.MetricType,
			NumericValue:        cloneFloat64Ptr(result.NumericValue),
			TextValue:           cloneStringPtr(result.TextValue),
			BooleanValue:        cloneBoolPtr(result.BooleanValue),
			Unit:                cloneStringPtr(result.Unit),
		})
	}
	return mapped
}

func mapFailureReviewLLMJudgeResults(results []LLMJudgeResultRecord) []failurereview.LLMJudgeResult {
	mapped := make([]failurereview.LLMJudgeResult, 0, len(results))
	for _, result := range results {
		payload := decodeFailureReviewLLMJudgePayload(result.Payload)
		mapped = append(mapped, failurereview.LLMJudgeResult{
			Key:             result.JudgeKey,
			Mode:            result.Mode,
			NormalizedScore: cloneFloat64Ptr(result.NormalizedScore),
			Reason:          firstNonEmpty(payload.Reason, payload.Error),
			State:           payload.State,
			Verdict:         payload.Verdict,
			Passed:          payload.Pass,
		})
	}
	return mapped
}

func mapFailureReviewEvents(events []RunEvent) []failurereview.Event {
	mapped := make([]failurereview.Event, 0, len(events))
	for _, event := range events {
		mapped = append(mapped, failurereview.Event{
			SequenceNumber: event.SequenceNumber,
			EventType:      string(event.EventType),
			Source:         string(event.Source),
			Payload:        cloneJSON(event.Payload),
		})
	}
	return mapped
}

func reasonFromRawOutput(payload []byte) string {
	var decoded struct {
		Reason string `json:"reason"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return ""
	}
	if decoded.Reason != "" {
		return decoded.Reason
	}
	return decoded.Error
}

func challengePackLifecycleStatus(ctx context.Context, repo *Repository, challengePackVersionID uuid.UUID, cache map[uuid.UUID]string) (string, error) {
	if challengePackVersionID == uuid.Nil {
		return "unknown", nil
	}
	if cached, ok := cache[challengePackVersionID]; ok {
		return cached, nil
	}
	if _, err := repo.GetRunnableChallengePackVersionByID(ctx, challengePackVersionID); err == nil {
		cache[challengePackVersionID] = "runnable"
		return "runnable", nil
	} else if errors.Is(err, ErrChallengePackVersionNotFound) {
		cache[challengePackVersionID] = "archived"
		return "archived", nil
	} else {
		return "", err
	}
}

type failureReviewLLMJudgePayload struct {
	Reason  string `json:"reason"`
	Error   string `json:"error"`
	State   string `json:"state"`
	Verdict string `json:"verdict"`
	Pass    *bool  `json:"pass"`
}

func decodeFailureReviewLLMJudgePayload(payload []byte) failureReviewLLMJudgePayload {
	var decoded failureReviewLLMJudgePayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return failureReviewLLMJudgePayload{}
	}
	return decoded
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
