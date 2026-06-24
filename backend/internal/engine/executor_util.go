package engine

import (
	"encoding/json"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/challengepack"
	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
)

func cloneMessages(messages []provider.Message) []provider.Message {
	cloned := make([]provider.Message, 0, len(messages))
	for _, message := range messages {
		cloned = append(cloned, provider.Message{
			Role:       message.Role,
			Content:    message.Content,
			ToolCalls:  cloneToolCalls(message.ToolCalls),
			ToolCallID: message.ToolCallID,
			IsError:    message.IsError,
		})
	}
	return cloned
}

func cloneToolDefinitions(definitions []provider.ToolDefinition) []provider.ToolDefinition {
	cloned := make([]provider.ToolDefinition, 0, len(definitions))
	for _, definition := range definitions {
		cloned = append(cloned, provider.ToolDefinition{
			Name:        definition.Name,
			Description: definition.Description,
			Parameters:  cloneJSON(definition.Parameters),
		})
	}
	return cloned
}

func cloneToolCalls(toolCalls []provider.ToolCall) []provider.ToolCall {
	cloned := make([]provider.ToolCall, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		cloned = append(cloned, provider.ToolCall{
			ID:               toolCall.ID,
			Name:             toolCall.Name,
			Arguments:        cloneJSON(toolCall.Arguments),
			ThoughtSignature: toolCall.ThoughtSignature,
		})
	}
	return cloned
}

func addUsage(left provider.Usage, right provider.Usage) provider.Usage {
	return provider.Usage{
		InputTokens:  left.InputTokens + right.InputTokens,
		OutputTokens: left.OutputTokens + right.OutputTokens,
		TotalTokens:  left.TotalTokens + right.TotalTokens,
	}
}

func cloneJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneStringMap(value map[string]string) map[string]string {
	if value == nil {
		return nil
	}
	cloned := make(map[string]string, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}

// sanitizeUserSimulatorForAgent clones a UserSimulatorSpec but strips
// UserSimulatorTurn.Expects from every turn. Expects contains the per-turn
// scoring criteria (ground truth assertions) that the agent must not see —
// if they were present in the workspace JSON the agent could read them and
// trivially satisfy validators without performing the intended task.
//
// The execution engine reads Expects from the database directly (via
// StoredCaseDocument), so stripping them here does not affect scoring.
func sanitizeUserSimulatorForAgent(spec *challengepack.UserSimulatorSpec) *challengepack.UserSimulatorSpec {
	cloned := challengepack.CloneUserSimulatorSpec(spec)
	if cloned == nil {
		return nil
	}
	for i := range cloned.Phases {
		for j := range cloned.Phases[i].Turns {
			cloned.Phases[i].Turns[j].Expects = nil
		}
	}
	return cloned
}

// filterChallengesForInputSet returns only the challenges referenced by the
// active input set's cases and items. This prevents the model from seeing
// challenge definitions that are not part of its current run — which would
// leak the pack's full structure and allow cross-challenge inference.
func filterChallengesForInputSet(
	challenges []repository.ChallengeDefinitionExecutionContext,
	inputSet *repository.ChallengeInputSetExecutionContext,
) []repository.ChallengeDefinitionExecutionContext {
	if inputSet == nil {
		return challenges
	}
	active := make(map[string]struct{})
	for _, c := range inputSet.Cases {
		active[c.ChallengeKey] = struct{}{}
	}
	for _, item := range inputSet.Items {
		active[item.ChallengeKey] = struct{}{}
	}
	filtered := make([]repository.ChallengeDefinitionExecutionContext, 0, len(active))
	for _, ch := range challenges {
		if _, ok := active[ch.ChallengeKey]; ok {
			filtered = append(filtered, ch)
		}
	}
	return filtered
}

func cloneChallengeDefinitions(challenges []repository.ChallengeDefinitionExecutionContext) []repository.ChallengeDefinitionExecutionContext {
	cloned := make([]repository.ChallengeDefinitionExecutionContext, 0, len(challenges))
	for _, challenge := range challenges {
		cloned = append(cloned, repository.ChallengeDefinitionExecutionContext{
			ID:                  challenge.ID,
			ChallengeIdentityID: challenge.ChallengeIdentityID,
			ChallengeKey:        challenge.ChallengeKey,
			ExecutionOrder:      challenge.ExecutionOrder,
			Title:               challenge.Title,
			Category:            challenge.Category,
			Difficulty:          challenge.Difficulty,
			Definition:          cloneJSON(challenge.Definition),
		})
	}
	return cloned
}

func cloneChallengeInputSet(inputSet *repository.ChallengeInputSetExecutionContext) *repository.ChallengeInputSetExecutionContext {
	if inputSet == nil {
		return nil
	}
	cloned := &repository.ChallengeInputSetExecutionContext{
		ID:                     inputSet.ID,
		ChallengePackVersionID: inputSet.ChallengePackVersionID,
		InputKey:               inputSet.InputKey,
		Name:                   inputSet.Name,
		Description:            cloneStringPtr(inputSet.Description),
		InputChecksum:          inputSet.InputChecksum,
		Cases:                  make([]repository.ChallengeCaseExecutionContext, 0, len(inputSet.Cases)),
		Items:                  make([]repository.ChallengeInputItemExecutionContext, 0, len(inputSet.Items)),
	}
	for _, item := range inputSet.Cases {
		cloned.Cases = append(cloned.Cases, repository.ChallengeCaseExecutionContext{
			ID:                  item.ID,
			ChallengeIdentityID: item.ChallengeIdentityID,
			ChallengeKey:        item.ChallengeKey,
			CaseKey:             item.CaseKey,
			ItemKey:             item.ItemKey,
			Payload:             cloneJSON(item.Payload),
			Inputs:              append([]challengepack.CaseInput(nil), item.Inputs...),
			Expectations:        nil,
			UserSimulator:       sanitizeUserSimulatorForAgent(item.UserSimulator),
			Artifacts:           append([]challengepack.ArtifactRef(nil), item.Artifacts...),
			Assets:              append([]challengepack.AssetReference(nil), item.Assets...),
		})
	}
	for _, item := range inputSet.Items {
		cloned.Items = append(cloned.Items, repository.ChallengeInputItemExecutionContext{
			ID:                  item.ID,
			ChallengeIdentityID: item.ChallengeIdentityID,
			ChallengeKey:        item.ChallengeKey,
			ItemKey:             item.ItemKey,
			Payload:             cloneJSON(item.Payload),
		})
	}
	return cloned
}

func normalizeStrings(values []string) []string {
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		cloned = append(cloned, trimmed)
	}
	return cloned
}
