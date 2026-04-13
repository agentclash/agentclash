package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/challengepack"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
)

// PromptEvalExecutor runs a single provider call without any sandbox, tools,
// or agent loop. It produces the same Result/event shape as NativeExecutor so
// the scoring pipeline can consume it unchanged.
type PromptEvalExecutor struct {
	client   provider.Client
	observer Observer
}

func NewPromptEvalExecutor(client provider.Client, observer Observer) PromptEvalExecutor {
	if observer == nil {
		observer = NoopObserver{}
	}
	return PromptEvalExecutor{
		client:   client,
		observer: observer,
	}
}

func (e PromptEvalExecutor) Execute(ctx context.Context, executionContext repository.RunAgentExecutionContext) (result Result, err error) {
	defer func() {
		if err != nil {
			if observerErr := e.observer.OnRunFailure(ctx, err); observerErr != nil {
				err = errors.Join(err, NewFailure(StopReasonObserverError, "record prompt_eval terminal failure event", observerErr))
			}
			return
		}
		if observerErr := e.observer.OnRunComplete(ctx, result); observerErr != nil {
			result = Result{}
			err = NewFailure(StopReasonObserverError, "record prompt_eval terminal completion event", observerErr)
		}
	}()

	if executionContext.Deployment.ProviderAccount == nil {
		return Result{}, provider.NewFailure(
			"",
			provider.FailureCodeInvalidRequest,
			"prompt_eval deployment is missing provider account in execution context",
			false,
			nil,
		)
	}
	if executionContext.Deployment.ModelAlias == nil {
		return Result{}, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"prompt_eval deployment is missing model alias in execution context",
			false,
			nil,
		)
	}

	runCtx := ctx
	cancel := func() {}
	if timeout := runTimeout(executionContext); timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	messages, err := buildPromptEvalMessages(executionContext)
	if err != nil {
		return Result{}, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"build prompt_eval messages",
			false,
			err,
		)
	}

	metadata, err := buildProviderMetadata(executionContext)
	if err != nil {
		return Result{}, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"marshal prompt_eval provider metadata",
			false,
			err,
		)
	}

	if observerErr := e.observer.OnStepStart(runCtx, 1); observerErr != nil {
		return Result{}, NewFailure(StopReasonObserverError, "record prompt_eval step start event", observerErr)
	}

	request := provider.Request{
		ProviderKey:         executionContext.Deployment.ProviderAccount.ProviderKey,
		ProviderAccountID:   executionContext.Deployment.ProviderAccount.ID.String(),
		CredentialReference: executionContext.Deployment.ProviderAccount.CredentialReference,
		Model:               executionContext.Deployment.ModelAlias.ModelCatalogEntry.ProviderModelID,
		TraceMode:           executionContext.Deployment.RuntimeProfile.TraceMode,
		StepTimeout:         stepTimeout(executionContext),
		Messages:            messages,
		Tools:               nil,
		Metadata:            metadata,
	}

	if observerErr := e.observer.OnProviderCall(runCtx, request); observerErr != nil {
		return Result{}, NewFailure(StopReasonObserverError, "record prompt_eval provider call event", observerErr)
	}

	response, invokeErr := e.client.InvokeModel(runCtx, request)
	if invokeErr != nil {
		if errors.Is(invokeErr, context.Canceled) {
			return Result{}, invokeErr
		}
		if errors.Is(runCtx.Err(), context.Canceled) {
			return Result{}, runCtx.Err()
		}
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return Result{}, NewFailure(StopReasonTimeout, "prompt_eval execution exceeded runtime budget", runCtx.Err())
		}
		if _, ok := provider.AsFailure(invokeErr); ok {
			return Result{}, invokeErr
		}
		if failure, ok := AsFailure(invokeErr); ok {
			return Result{}, failure
		}
		return Result{}, NewFailure(StopReasonProviderError, "prompt_eval provider call failed", invokeErr)
	}

	if observerErr := e.observer.OnProviderResponse(runCtx, response); observerErr != nil {
		return Result{}, NewFailure(StopReasonObserverError, "record prompt_eval provider response event", observerErr)
	}
	if observerErr := e.observer.OnStepEnd(runCtx, 1); observerErr != nil {
		return Result{}, NewFailure(StopReasonObserverError, "record prompt_eval step completion event", observerErr)
	}

	return Result{
		FinalOutput:   response.OutputText,
		StopReason:    StopReasonCompleted,
		StepCount:     1,
		ToolCallCount: 0,
		Usage:         response.Usage,
	}, nil
}

// buildPromptEvalMessages assembles the single-shot prompt:
//   - system message: policy_spec instructions (if any)
//   - user message: rendered challenge.instructions with {{var}} substitutions
//     drawn from the first case in the run agent's input set.
func buildPromptEvalMessages(executionContext repository.RunAgentExecutionContext) ([]provider.Message, error) {
	challenge, err := selectPromptEvalChallenge(executionContext)
	if err != nil {
		return nil, err
	}
	instructions, err := extractChallengeInstructions(challenge.Definition)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(instructions) == "" {
		return nil, fmt.Errorf("challenge %q is missing instructions required for prompt_eval", challenge.ChallengeKey)
	}

	vars := promptEvalVariables(executionContext)
	rendered := renderPromptTemplate(instructions, vars)
	if leftovers := promptEvalTemplatePattern.FindAllString(rendered, -1); len(leftovers) > 0 {
		slog.Default().Warn(
			"prompt_eval executor rendered prompt with unresolved template tokens",
			"run_agent_id", executionContext.RunAgent.ID.String(),
			"unresolved_tokens", leftovers,
		)
	}

	messages := make([]provider.Message, 0, 2)
	if system := strings.TrimSpace(extractPolicyInstructions(executionContext.Deployment.AgentBuildVersion.PolicySpec)); system != "" {
		messages = append(messages, provider.Message{Role: "system", Content: system})
	}
	messages = append(messages, provider.Message{Role: "user", Content: rendered})
	return messages, nil
}

func selectPromptEvalChallenge(executionContext repository.RunAgentExecutionContext) (repository.ChallengeDefinitionExecutionContext, error) {
	if len(executionContext.ChallengePackVersion.Challenges) == 0 {
		return repository.ChallengeDefinitionExecutionContext{}, errors.New("prompt_eval run is missing challenge definitions")
	}
	if executionContext.ChallengeInputSet == nil || len(executionContext.ChallengeInputSet.Cases) == 0 {
		return executionContext.ChallengePackVersion.Challenges[0], nil
	}
	firstCaseKey := executionContext.ChallengeInputSet.Cases[0].ChallengeKey
	for _, challenge := range executionContext.ChallengePackVersion.Challenges {
		if challenge.ChallengeKey == firstCaseKey {
			return challenge, nil
		}
	}
	return executionContext.ChallengePackVersion.Challenges[0], nil
}

func extractChallengeInstructions(definition json.RawMessage) (string, error) {
	if len(definition) == 0 {
		return "", nil
	}
	var decoded struct {
		Instructions string `json:"instructions"`
	}
	if err := json.Unmarshal(definition, &decoded); err != nil {
		return "", fmt.Errorf("decode challenge definition: %w", err)
	}
	return decoded.Instructions, nil
}

func promptEvalVariables(executionContext repository.RunAgentExecutionContext) map[string]string {
	vars := map[string]string{}
	if executionContext.ChallengeInputSet == nil || len(executionContext.ChallengeInputSet.Cases) == 0 {
		return vars
	}
	if n := len(executionContext.ChallengeInputSet.Cases); n > 1 {
		slog.Default().Warn(
			"prompt_eval executor using first case only; additional cases ignored",
			"run_agent_id", executionContext.RunAgent.ID.String(),
			"case_count", n,
		)
	}
	first := executionContext.ChallengeInputSet.Cases[0]
	for _, input := range first.Inputs {
		if input.Key == "" {
			continue
		}
		if rendered, ok := promptEvalRenderInputValue(input); ok {
			vars[input.Key] = rendered
		}
	}
	return vars
}

func promptEvalRenderInputValue(input challengepack.CaseInput) (string, bool) {
	if input.Value == nil {
		return "", false
	}
	switch typed := input.Value.(type) {
	case string:
		return typed, true
	case bool:
		if typed {
			return "true", true
		}
		return "false", true
	case int:
		return fmt.Sprintf("%d", typed), true
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", typed), "0"), "."), true
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return "", false
		}
		return string(encoded), true
	}
}

var promptEvalTemplatePattern = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

func renderPromptTemplate(template string, vars map[string]string) string {
	return promptEvalTemplatePattern.ReplaceAllStringFunc(template, func(match string) string {
		groups := promptEvalTemplatePattern.FindStringSubmatch(match)
		if len(groups) != 2 {
			return match
		}
		if value, ok := vars[groups[1]]; ok {
			return value
		}
		return match
	})
}

func RenderPromptTemplate(template string, vars map[string]string) string {
	return renderPromptTemplate(template, vars)
}
