# Test Contract: Issue #583 Harness Scorecards

## Intent

Agent Harness scoring must use the existing AgentClash scorecard and LLM judge pipeline instead of a separate harness-only scorer.

## Expectations

- Command validators continue to execute in the harness sandbox.
- Harness command validator results are converted into standard scoring validator results.
- Harness `llm_judges` execute through the existing workflow judge evaluator instead of `llm_judges.skipped`.
- Harness executions persist run-agent scorecards through `StoreRunAgentEvaluationResults`.
- Scorecards include dimensions, pass/fail verdict, reasons, validator results, and LLM judge results.
- Agent Harness API/UI can surface scorecard status using the canonical run-agent scorecard endpoint via `run_agent_id`.

## Verification

- `cd backend && go test ./internal/scoring ./internal/workflow ./internal/repository ./internal/api`
- Focused Agent Harness UI tests if UI rendering changes.
