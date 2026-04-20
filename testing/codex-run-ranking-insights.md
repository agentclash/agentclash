# Codex Run Ranking Insights — Test Contract

## Functional Behavior
- Completed multi-agent runs expose an `Insights` section on the run detail ranking view.
- The section stays hidden for single-agent runs and for runs whose ranking is unavailable.
- Users can generate insights on demand by selecting a workspace provider account and model alias they control.
- Insight generation sends the current run's ranking and scorecard context to an LLM and returns structured advisory output.
- The UI renders the recommended winner, why it won, key tradeoffs, next-step guidance, and confidence notes.
- Insight output is clearly presented as advisory and separate from the deterministic ranking table.
- Failed insight requests surface a readable error without breaking the existing ranking UI.
- The first shipped version is grounded in current-run data only and does not require web search.

## Unit Tests
- `TestRunReadManagerGenerateRankingInsightsRejectsSingleAgentRun` — returns validation error when the run has fewer than 2 agents.
- `TestRunReadManagerGenerateRankingInsightsRejectsUnavailableRanking` — returns validation error when ranking/scorecard is unavailable.
- `TestRunReadManagerGenerateRankingInsightsInvokesSelectedProvider` — uses the chosen provider account + model alias and returns parsed insight payload.
- `TestRunReadManagerGenerateRankingInsightsLoadsWorkspaceSecrets` — resolves `workspace-secret://` credentials successfully for provider invocation.
- `CreateRunInsightsSection` UI test — renders generate controls only when ranking is ready for a multi-agent run.
- `CreateRunInsightsSection` UI test — renders structured insight results and preserves the ranking table.
- `CreateRunInsightsSection` UI test — renders API errors cleanly.

## Integration / Functional Tests
- `POST /v1/runs/{runID}/ranking-insights` accepts a valid provider/model selection and returns structured insight JSON.
- The API enforces run workspace visibility and rejects provider/model selections that are not visible to the run workspace.
- The run detail client fetches workspace provider accounts and model aliases, then successfully calls the ranking insights endpoint.

## Smoke Tests
- Existing `/v1/runs/{runID}/ranking` behavior is unchanged for completed runs.
- The run detail page still loads and shows the raw ranking table even if no insights have been generated.
- Insight generation does not affect run ranking sort controls or row rendering.

## E2E Tests
- N/A — not adding browser E2E coverage in this change.

## Manual / cURL Tests
```bash
curl -X POST "http://localhost:8080/v1/runs/<run-id>/ranking-insights" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "provider_account_id": "<provider-account-id>",
    "model_alias_id": "<model-alias-id>"
  }'
# Expected: 200 with a JSON body containing recommended_winner, why_it_won,
# tradeoffs, recommended_next_step, and confidence_notes.
```

```bash
curl "http://localhost:3000/workspaces/<workspace-id>/runs/<run-id>"
# Expected: completed multi-agent run shows both the raw Ranking table and a
# new Insights card with provider/model selection and on-demand generation.
```
