# Issue #586 Expectations

## Scope

Agent Harnesses should support privacy and credibility controls without duplicating the existing scoring, LLM judge, validator, replay, or event pipelines.

## Expected behavior

- Harness `evaluation_config.validators[]` can mark command validators as hidden/private. Hidden validators still execute and score through the existing validator result path, but persisted harness events, canonical run events, and scorecard raw output must not expose the command, stdout, stderr, or expected artifacts.
- Harness `evaluation_config` can classify result provenance (`public_benchmark`, `private_task_bank`, `live_replay`, `ad_hoc`) and include benchmark contamination metadata. The persisted scorecard event should surface that metadata so reports can distinguish result types.
- Harness `evaluation_config.privacy` can declare replay/artifact redaction, retention days, audit logging, and provider data-use policy. The worker should record these controls as structured audit/policy events and apply redaction to replay payloads.
- Harness LLM judges should continue using the existing judge evaluator and must retain model/version/schema/rationale call metadata already stored in judge payloads; this issue may add bias-mitigation metadata, but must not fork judge execution.

## Validation

- Unit tests cover hidden validator redaction, privacy/metadata events, and preservation of scoring results.
- Existing Agent Harness execution, scoring, repository, API, and focused UI tests remain green.
