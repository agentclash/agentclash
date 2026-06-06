# codex-agent-tryout-template-runtimes - Test Contract

## Functional Behavior
- Agent tryout templates expose executable runtime metadata, not just static catalog JSON: availability, unavailable reason, input schema, tool allowlist, sandbox/network policy, expected artifacts, validation strategy, max cost, and max duration.
- Creating anonymous or workspace tryouts rejects malformed template input before persistence or execution with a stable `invalid_input` response.
- Creating tryouts rejects unavailable templates before persistence or execution with a stable `template_unavailable` response and a user-readable reason.
- Server-side runtime policy is authoritative: client input cannot add arbitrary tools, commands, network access, artifact paths, model policy, cost limit, or duration.
- The harness execution payload includes the resolved runtime definition so workers/templates can enforce task-specific tools, sandbox limits, expected artifacts, and validators.
- At least the first production template, `meeting-minutes`, is marked executable and includes concrete schema, tool policy, expected artifact metadata, and validation metadata.
- Network access is disabled by default for executable templates unless explicitly enabled by the server-owned runtime definition.

## Unit Tests
- `TestAgentTryoutTemplatesExposeRuntimeMetadata` - list templates includes availability, runtime/tool/sandbox/artifact/validation metadata, and `meeting-minutes` is executable.
- `TestAgentTryoutManagerRejectsInvalidTemplateInputBeforeCreate` - malformed input for a template returns `ErrInvalidAgentTryoutInput` and does not create a tryout.
- `TestAgentTryoutManagerRejectsUnavailableTemplateBeforeCreate` - unavailable templates return `ErrAgentTryoutTemplateUnavailable` and do not create/dispatch.
- `TestAgentTryoutManagerIgnoresClientRuntimePolicyFields` - extra input fields attempting to set tools/network/commands/model/cost are rejected or ignored and cannot alter snapshots.
- `TestBuildAgentTryoutHarnessPayloadIncludesRuntimePolicy` - harness snapshot and execution config include the resolved runtime policy, expected artifacts, validators, sandbox policy, and template slug.

## Integration / Functional Tests
- Existing agent tryout repository tests continue to round-trip template, tool, model, and evaluation snapshots.
- Existing create/dispatch manager tests continue to prove exactly one execution is linked for executable templates.
- API handler tests cover `template_unavailable` and malformed input responses.

## Smoke Tests
- `go test -short -count=1 ./internal/api -run 'AgentTryout|TemplateRuntime'`
- `go test -short -count=1 ./internal/repository -run AgentTryout`
- `go test -short -count=1 ./...`
- `go vet ./...`

## E2E Tests
- N/A - full E2B/provider execution requires external credentials. This change must still curl local API routes with dev/local configuration to verify template metadata and validation behavior.

## Manual / cURL Tests
- `GET /v1/agent-tryout-templates` returns runtime metadata for templates, including `meeting-minutes.available=true`, expected artifacts, sandbox policy, and validators.
- `POST /v1/agent-tryouts` with valid `meeting-minutes` input returns `201` and snapshots server-owned runtime/tool/evaluation policy.
- `POST /v1/agent-tryouts` with malformed `meeting-minutes` input returns `400 invalid_input` and creates no tryout.
- `POST /v1/agent-tryouts` for an unavailable template returns `409 template_unavailable` or equivalent stable product-facing error without dispatching work.
- `POST /v1/agent-tryouts` with extra client-supplied runtime/tool/network fields cannot enable arbitrary tools or network access.
