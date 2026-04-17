# codex/issue-299-behavioral-analysis — Test Contract

## Functional Behavior
- Evaluation specs may opt into a `behavioral` config that declares Phase 1 signal keys, weights, optional gates, and pass thresholds without changing packs that omit the config.
- A scorecard dimension whose source is `behavioral` evaluates the configured signals from the run's tool-call event stream and returns a weighted composite score on `[0,1]`.
- `buildEvidence()` captures a stable tool-call trace from `tool.call.completed` and `tool.call.failed` events, including tool name, JSON arguments, whether the call failed, and failure-origin metadata when present.
- `recovery_behavior` measures how often the first call after a failure adapts instead of retrying the same tool with identical arguments.
- `exploration_efficiency` measures duplicate tool calls and returns `1 - (duplicate_calls / total_calls)`.
- `error_cascade` returns `1 / max_consecutive_failures`, using the longest adjacent failure streak in the trace.
- `scope_adherence` measures out-of-scope calls from failure-origin metadata and returns `1 - (out_of_scope_calls / total_calls)`.
- The behavioral composite honors per-signal weights and gate thresholds, and dimension results surface unavailable reasons when the pack is misconfigured or evidence is missing.
- Individual metric collectors expose each Phase 1 signal so packs can score them directly without using the composite dimension.
- Run-agent scorecards persist the `behavioral_score` alongside the existing built-in dimension scores, and run-scorecard summaries can surface the behavioral dimension when present.
- Existing specs, evaluation flows, and persistence behavior remain backward compatible when no behavioral config or behavioral dimension is declared.

## Unit Tests
- `TestBehavioralSignalRecoveryBehavior` — adaptive retry patterns score higher than identical retries, and traces with no recovery attempts stay unavailable or neutral per the implementation contract.
- `TestBehavioralSignalExplorationEfficiency` — duplicate tool+argument pairs reduce the score while unique calls stay at `1.0`.
- `TestBehavioralSignalErrorCascade` — longer consecutive failure streaks reduce the score and a no-failure trace stays at `1.0`.
- `TestBehavioralSignalScopeAdherence` — out-of-scope failures derived from failure-origin metadata reduce the score correctly.
- `TestBehavioralCompositeDimension` — configured weights and gate thresholds produce the expected composite score and pass/fail behavior.
- `TestBuildEvidenceCapturesToolCallTrace` — completed and failed tool events populate the extracted trace in order with normalized arguments and failure metadata.
- `TestValidateEvaluationSpecBehavioralConfig` — unknown keys, duplicate declarations, missing thresholds for gated signals, and missing config for a behavioral dimension are rejected.

## Integration / Functional Tests
- `EvaluateRunAgent()` with a spec declaring a behavioral dimension returns a populated behavioral dimension score plus the existing dimensions.
- `PersistScoringResult()` stores `behavioral_score` in `run_agent_scorecards` and keeps the JSON scorecard document consistent with the evaluation output.
- Loading a spec with behavioral signal metric collectors evaluates those metrics from the same tool-call evidence used by the dimension.

## Smoke Tests
- `cd backend && go test ./internal/scoring/...`
- `cd backend && go test ./internal/repository/...`

## E2E Tests
- N/A — this change is scoring-engine and persistence focused; no UI or external API journey is required for acceptance.

## Manual / cURL Tests
- N/A — verification is code-level through Go tests and repository persistence checks rather than HTTP endpoints.
