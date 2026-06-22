package simulator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
)

const defaultSimulatorTimeout = 60 * time.Second

// TranscriptTurn is one conversational turn for simulator context.
type TranscriptTurn struct {
	Actor   string
	Content string
	PhaseID string
}

// Input drives LLM user message generation.
type Input struct {
	Persona          string
	Transcript       []TranscriptTurn
	CasePayload      map[string]any
	PhaseID          string
	MaxOutputTokens  int32
	ProviderKey      string
	ProviderAccountID string
	CredentialRef    string
	Model            string
}

// Metadata describes the simulator provider call.
type Metadata struct {
	ProviderKey     string `json:"provider_key"`
	ProviderModelID string `json:"provider_model_id"`
	PhaseID         string `json:"phase_id"`
}

// Generator produces the next simulated user message.
type Generator struct {
	client provider.Client
}

func NewGenerator(client provider.Client) Generator {
	return Generator{client: client}
}

func (g Generator) GenerateUserMessage(ctx context.Context, input Input) (message string, metadata Metadata, err error) {
	if g.client == nil {
		return "", Metadata{}, provider.NewFailure("", provider.FailureCodeInvalidRequest, "simulator provider client is not configured", false, nil)
	}
	if strings.TrimSpace(input.Persona) == "" {
		return "", Metadata{}, provider.NewFailure(input.ProviderKey, provider.FailureCodeInvalidRequest, "llm simulator persona is required", false, nil)
	}
	if strings.TrimSpace(input.Model) == "" {
		return "", Metadata{}, provider.NewFailure(input.ProviderKey, provider.FailureCodeInvalidRequest, "llm simulator model is required", false, nil)
	}

	prompt := buildSimulatorPrompt(input)
	request := provider.Request{
		ProviderKey:         input.ProviderKey,
		ProviderAccountID:   input.ProviderAccountID,
		CredentialReference: input.CredentialRef,
		Model:               input.Model,
		StepTimeout:         defaultSimulatorTimeout,
		Messages: []provider.Message{
			{Role: "user", Content: prompt},
		},
	}

	callCtx := ctx
	cancel := func() {}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		callCtx, cancel = context.WithTimeout(ctx, defaultSimulatorTimeout)
	}
	defer cancel()

	response, invokeErr := g.client.InvokeModel(callCtx, request)
	if invokeErr != nil {
		return "", Metadata{}, invokeErr
	}

	message = strings.TrimSpace(response.OutputText)
	if message == "" {
		return "", Metadata{}, provider.NewFailure(response.ProviderKey, provider.FailureCodeInvalidRequest, "llm simulator returned empty message", false, nil)
	}

	return message, Metadata{
		ProviderKey:     response.ProviderKey,
		ProviderModelID: response.ProviderModelID,
		PhaseID:         input.PhaseID,
	}, nil
}

func buildSimulatorPrompt(input Input) string {
	var b strings.Builder
	b.WriteString("You are simulating an end user in a multi-turn evaluation.\n\n")
	b.WriteString("Persona:\n")
	b.WriteString(strings.TrimSpace(input.Persona))
	b.WriteString("\n\nConversation so far:\n")
	if len(input.Transcript) == 0 {
		b.WriteString("(no prior turns)\n")
	} else {
		for _, turn := range input.Transcript {
			actor := strings.TrimSpace(turn.Actor)
			if actor == "" {
				actor = "user"
			}
			b.WriteString(fmt.Sprintf("%s: %s\n", actor, strings.TrimSpace(turn.Content)))
		}
	}
	if len(input.CasePayload) > 0 {
		encoded, _ := json.Marshal(input.CasePayload)
		b.WriteString("\nCase context JSON:\n")
		b.Write(encoded)
		b.WriteByte('\n')
	}
	b.WriteString("\nWrite the next user message only. Do not include role prefixes or explanations.")
	return b.String()
}

// ResolveTarget picks provider credentials from the run agent execution context.
// Falls back to the deployment's provider account when simulator-specific config is absent.
func ResolveTarget(executionContext repository.RunAgentExecutionContext) (providerKey, providerAccountID, credentialRef, model string, err error) {
	if executionContext.Deployment.ProviderAccount == nil || executionContext.Deployment.ModelID == "" {
		return "", "", "", "", provider.NewFailure("", provider.FailureCodeInvalidRequest, "multi_turn llm simulator requires deployment provider account and model", false, nil)
	}
	return executionContext.Deployment.ProviderAccount.ProviderKey,
		executionContext.Deployment.ProviderAccount.ID.String(),
		executionContext.Deployment.ProviderAccount.CredentialReference,
		executionContext.Deployment.ModelID,
		nil
}

// TranscriptFromTurns converts engine transcript turns to simulator input.
func TranscriptFromTurns(turns []TranscriptTurn) []TranscriptTurn {
	return append([]TranscriptTurn(nil), turns...)
}

// ActorForEvent maps run event actors to transcript labels.
func ActorForEvent(actor string) string {
	switch strings.TrimSpace(actor) {
	case runevents.ConversationActorScripted, runevents.ConversationActorLLM, runevents.ConversationActorHuman:
		return "user"
	default:
		return strings.TrimSpace(actor)
	}
}
