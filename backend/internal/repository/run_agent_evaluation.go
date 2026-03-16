package repository

import (
	"context"
	"fmt"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
)

type EvaluateRunAgentParams struct {
	RunAgentID       uuid.UUID
	EvaluationSpecID uuid.UUID
}

func (r *Repository) EvaluateRunAgent(ctx context.Context, params EvaluateRunAgentParams) (scoring.RunAgentEvaluation, error) {
	executionContext, err := r.GetRunAgentExecutionContextByID(ctx, params.RunAgentID)
	if err != nil {
		return scoring.RunAgentEvaluation{}, fmt.Errorf("load run-agent execution context: %w", err)
	}

	evaluationSpec, err := r.GetEvaluationSpecByID(ctx, params.EvaluationSpecID)
	if err != nil {
		return scoring.RunAgentEvaluation{}, fmt.Errorf("load evaluation spec: %w", err)
	}
	if evaluationSpec.ChallengePackVersionID != executionContext.ChallengePackVersion.ID {
		return scoring.RunAgentEvaluation{}, fmt.Errorf(
			"evaluation spec %s belongs to challenge pack version %s, not run-agent challenge pack version %s",
			evaluationSpec.ID,
			evaluationSpec.ChallengePackVersionID,
			executionContext.ChallengePackVersion.ID,
		)
	}

	spec, err := scoring.DecodeDefinition(evaluationSpec.Definition)
	if err != nil {
		return scoring.RunAgentEvaluation{}, fmt.Errorf("decode evaluation spec definition: %w", err)
	}

	events, err := r.ListRunEventsByRunAgentID(ctx, params.RunAgentID)
	if err != nil {
		return scoring.RunAgentEvaluation{}, fmt.Errorf("list canonical run events: %w", err)
	}

	evaluation, err := scoring.EvaluateRunAgent(mapEvaluationInput(params.EvaluationSpecID, executionContext, events), spec)
	if err != nil {
		return scoring.RunAgentEvaluation{}, fmt.Errorf("evaluate run-agent: %w", err)
	}
	if err := r.StoreRunAgentEvaluationResults(ctx, evaluation); err != nil {
		return scoring.RunAgentEvaluation{}, fmt.Errorf("store run-agent evaluation results: %w", err)
	}

	return evaluation, nil
}

func mapEvaluationInput(evaluationSpecID uuid.UUID, executionContext RunAgentExecutionContext, events []RunEvent) scoring.EvaluationInput {
	convertedEvents := make([]scoring.Event, 0, len(events))
	for _, event := range events {
		convertedEvents = append(convertedEvents, scoring.Event{
			Type:       string(event.EventType),
			Source:     string(event.Source),
			OccurredAt: event.OccurredAt,
			Payload:    cloneJSON(event.Payload),
		})
	}

	challengeInputs := make([]scoring.EvidenceInput, 0)
	if executionContext.ChallengeInputSet != nil {
		for _, item := range executionContext.ChallengeInputSet.Items {
			challengeInputs = append(challengeInputs, scoring.EvidenceInput{
				ChallengeIdentityID: item.ChallengeIdentityID,
				ChallengeKey:        item.ChallengeKey,
				ItemKey:             item.ItemKey,
				Payload:             cloneJSON(item.Payload),
			})
		}
	}

	return scoring.EvaluationInput{
		RunAgentID:       executionContext.RunAgent.ID,
		EvaluationSpecID: evaluationSpecID,
		ChallengeInputs:  challengeInputs,
		Events:           convertedEvents,
	}
}
