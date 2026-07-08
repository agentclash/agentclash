package engine

import (
	"log/slog"

	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/runner"
)

func caseTemplateContextForExecution(executionContext runner.ExecutionContext) challengepack.CaseTemplateContext {
	if executionContext.ChallengeInputSet == nil || len(executionContext.ChallengeInputSet.Cases) == 0 {
		return challengepack.CaseTemplateContext{}
	}
	if n := len(executionContext.ChallengeInputSet.Cases); n > 1 {
		slog.Default().Warn(
			"case template rendering using first case only; additional cases ignored",
			"run_agent_id", executionContext.RunAgent.ID.String(),
			"case_count", n,
		)
	}
	first := executionContext.ChallengeInputSet.Cases[0]
	ctx, err := challengepack.BuildCaseTemplateContextFromPayload(first.Payload, first.Inputs)
	if err != nil {
		slog.Default().Warn(
			"case template context decode failed; rendering with empty context",
			"run_agent_id", executionContext.RunAgent.ID.String(),
			"error", err,
		)
		return challengepack.CaseTemplateContext{}
	}
	return ctx
}

func renderCaseTemplateCommand(template string, executionContext runner.ExecutionContext) string {
	ctx := caseTemplateContextForExecution(executionContext)
	rendered, err := challengepack.RenderCaseTemplate(template, ctx)
	if err != nil {
		slog.Default().Warn(
			"code execution test_command rendered with unresolved placeholders",
			"run_agent_id", executionContext.RunAgent.ID.String(),
			"error", err,
		)
		return challengepack.RenderCaseTemplateLenient(template, ctx)
	}
	return rendered
}
