package engine

import (
	"log/slog"

	"github.com/agentclash/agentclash/backend/internal/evalpack"
	"github.com/agentclash/agentclash/backend/internal/repository"
)

func caseTemplateContextForExecution(executionContext repository.RunAgentExecutionContext) evalpack.CaseTemplateContext {
	if executionContext.ChallengeInputSet == nil || len(executionContext.ChallengeInputSet.Cases) == 0 {
		return evalpack.CaseTemplateContext{}
	}
	if n := len(executionContext.ChallengeInputSet.Cases); n > 1 {
		slog.Default().Warn(
			"case template rendering using first case only; additional cases ignored",
			"run_agent_id", executionContext.RunAgent.ID.String(),
			"case_count", n,
		)
	}
	first := executionContext.ChallengeInputSet.Cases[0]
	ctx, err := evalpack.BuildCaseTemplateContextFromPayload(first.Payload, first.Inputs)
	if err != nil {
		slog.Default().Warn(
			"case template context decode failed; rendering with empty context",
			"run_agent_id", executionContext.RunAgent.ID.String(),
			"error", err,
		)
		return evalpack.CaseTemplateContext{}
	}
	return ctx
}

func renderCaseTemplateCommand(template string, executionContext repository.RunAgentExecutionContext) string {
	ctx := caseTemplateContextForExecution(executionContext)
	rendered, err := evalpack.RenderCaseTemplate(template, ctx)
	if err != nil {
		slog.Default().Warn(
			"code execution test_command rendered with unresolved placeholders",
			"run_agent_id", executionContext.RunAgent.ID.String(),
			"error", err,
		)
		return evalpack.RenderCaseTemplateLenient(template, ctx)
	}
	return rendered
}
