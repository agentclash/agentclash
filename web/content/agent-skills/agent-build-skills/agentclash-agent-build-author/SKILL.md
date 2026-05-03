---
name: agentclash-agent-build-author
description: Use when creating, editing, validating, or readying AgentClash agent builds and build versions, including agent identity, spec JSON, prompts, model/runtime expectations, tool bindings, and version readiness.
metadata:
  agentclash.role: agent-builds
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Agent Build Author

## Purpose
Create or update an AgentClash agent build and draft build version so it has a source-backed spec, a validation-clean prompt policy, and a ready build version ID that deployment setup can consume.

## Use When
- The user needs a new AgentClash agent build or a new version of an existing build.
- A prompt, model expectation, output schema, tool binding, memory, workflow, reasoning, guardrail, or publication note should be captured in build-version JSON.
- A deployment is blocked because the user has no ready `build_version_id`.
- A coding agent needs to turn product behavior into an AgentClash build spec before deployment setup.

## Do Not Use When
- The CLI is not authenticated or no workspace is selected; use `agentclash-cli-setup` first.
- Provider accounts, model aliases, runtime profiles, workspace tools, or secrets are missing; use `agentclash-runtime-resources-setup` first.
- The build version is already ready and the user only needs a deployment; use `agentclash-agent-deployment-setup`.
- The user is authoring challenge pack YAML; use the challenge-pack skills.

## Inputs Needed
- Workspace ID and confirmation that `agentclash doctor` can reach it.
- Build name and optional description.
- Whether to create a new build or add a version to an existing build ID.
- Agent kind: `llm_agent`, `workflow_agent`, `programmatic_agent`, `multi_agent_system`, or `hosted_external`.
- Prompt or policy instructions. Validation requires `policy_spec.instructions`.
- Interface, reasoning, memory, workflow, guardrail, model, output schema, trace, and publication requirements.
- Optional workspace tool IDs and knowledge source IDs to bind to the version.
- Runtime assumptions handed off from `agentclash-runtime-resources-setup`, such as model alias, provider account, shell/network needs, and timeouts.

## Environment
Use hosted production by default:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash doctor
```

Build commands need a resolved workspace for list/create. Workspace resolution follows the normal CLI setup path, so prefer `agentclash workspace use <workspace-id>` or `AGENTCLASH_WORKSPACE` over hard-coding IDs into reusable notes.

## Procedure
1. Verify CLI and workspace readiness with `agentclash doctor`.
2. Run `agentclash build list` and decide whether to create a new build or version an existing build.
3. Create the build when needed with `agentclash build create --name ... --description ...`.
4. Draft `version-spec.json` from the exact build-version fields below. Keep unknown or not-yet-used spec sections as JSON objects, not prose.
5. Create a draft build version with `agentclash build version create <BUILD_ID> --spec-file version-spec.json`.
6. Inspect the draft with `agentclash build version get <VERSION_ID> --json`.
7. Validate with `agentclash build version validate <VERSION_ID> --json` so a coding agent can inspect `valid` and `errors`. Fix any errors by editing the spec and running `agentclash build version update <VERSION_ID> --spec-file version-spec.json`.
8. Mark ready only after validation passes and the user confirms the draft is deployable: `agentclash build version ready <VERSION_ID>`.
9. Report the `agent_build_id`, `build_version_id`, `version_status`, agent kind, and deployment prerequisites.

## Spec Fields
Agent build creation accepts:

```json
{
  "name": "Support Triage Agent",
  "description": "Answers support benchmark cases with cited evidence."
}
```

Build version creation and draft update accept:

```json
{
  "agent_kind": "llm_agent",
  "interface_spec": {
    "input": "challenge case prompt plus attached artifacts",
    "output": "JSON answer matching output_schema"
  },
  "policy_spec": {
    "instructions": "Read the full case, inspect provided artifacts, cite evidence, and return only JSON."
  },
  "reasoning_spec": {
    "strategy": "inspect evidence before answering"
  },
  "memory_spec": {},
  "workflow_spec": {
    "steps": ["read input", "inspect artifacts", "answer"]
  },
  "guardrail_spec": {
    "refuse_if": ["missing required evidence"]
  },
  "model_spec": {
    "preferred_model_alias": "primary-chat",
    "temperature": 0.1
  },
  "output_schema": {
    "type": "object",
    "required": ["answer"],
    "properties": {
      "answer": { "type": "string" },
      "evidence": {
        "type": "array",
        "items": { "type": "string" }
      }
    }
  },
  "trace_contract": {
    "must_record": ["tool_calls", "final_answer"]
  },
  "publication_spec": {
    "version_notes": "Initial support triage build."
  },
  "tools": [
    {
      "tool_id": "<WORKSPACE_TOOL_UUID>",
      "binding_role": "evidence_lookup",
      "binding_config": {}
    }
  ],
  "knowledge_sources": [
    {
      "knowledge_source_id": "<KNOWLEDGE_SOURCE_UUID>",
      "binding_role": "reference",
      "binding_config": {}
    }
  ]
}
```

Required readiness invariant: `policy_spec` must contain an `instructions` field. The current validation also checks that `agent_kind` is one of the supported enum values. Omitted JSON spec objects default to `{}`; omitted tool and knowledge-source bindings default to empty lists.

## Commands
Verify setup and inspect existing builds:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash doctor
agentclash build list
agentclash build get <BUILD_ID>
```

Create a build:

```bash
agentclash build create \
  --name "Support Triage Agent" \
  --description "Answers support benchmark cases with cited evidence."
```

Create, inspect, validate, update, and ready a build version:

```bash
agentclash build version create <BUILD_ID> --spec-file version-spec.json
agentclash build version get <VERSION_ID> --json
agentclash build version validate <VERSION_ID> --json
agentclash build version update <VERSION_ID> --spec-file version-spec.json
agentclash build version ready <VERSION_ID>
```

When only the agent kind needs to be set or overridden at creation time:

```bash
agentclash build version create <BUILD_ID> --agent-kind llm_agent --spec-file version-spec.json
```

## Expected Output
- `build list` returns workspace build IDs, names, slugs, lifecycle status, and creation timestamps.
- `build get <BUILD_ID>` returns build metadata plus version history.
- `build create` returns a build ID and generated slug.
- `build version create` returns a draft version ID and version number.
- `build version get --json` returns `version_status`, `agent_kind`, all spec objects, tool bindings, knowledge-source bindings, and creation time.
- `build version validate --json` returns `valid: true` when the agent kind is supported and `policy_spec.instructions` exists. Without `--json`, the CLI prints `Version is valid` or `Version has validation errors`.
- `build version ready` changes `version_status` to `ready`; ready versions are the deployable handoff to deployment setup.

## Failure Modes
- `no workspace specified`: run `agentclash link`, pass `--workspace`, set `AGENTCLASH_WORKSPACE`, or add project config with `agentclash init`.
- `name is required`: build creation needs `--name`.
- `request body must be valid JSON`: `--spec-file` must point to valid JSON, not YAML or Markdown.
- Validation fails on `agent_kind`: use one of `llm_agent`, `workflow_agent`, `programmatic_agent`, `multi_agent_system`, or `hosted_external`.
- Validation fails on `policy_spec`: add `policy_spec.instructions` to the version spec.
- Update fails because the version is not draft: ready versions are immutable for normal authoring; create a new build version instead.
- Tool or knowledge-source binding fails validation: IDs must be UUIDs. Workspace tool creation belongs in `agentclash-runtime-resources-setup`.
- Deployment later fails because the version is not ready: run validate, fix errors, then explicitly mark ready.

## Safety Notes
- Do not put provider API keys, tokens, or customer secrets in any build spec field. Use runtime resources and workspace secrets instead.
- Treat `build version ready` as a publish-style action because it makes the version immutable and deployable; get explicit user confirmation first.
- Keep model names and provider-account assumptions in `model_spec` as expectations or notes unless deployment setup has real runtime resource IDs.
- Prefer `build version get --json` before update or ready so the agent does not overwrite a draft blindly.
- Do not invent workspace tool or knowledge-source IDs. List or create the upstream resources first.

## Report Back Format
```text
Workspace: <workspace-id>
Build: <name> (<agent_build_id>)
Version: v<version_number> (<build_version_id>)
Status: <draft | ready>
Agent kind: <agent_kind>
Validation: <valid | errors>
Runtime assumptions: <provider/model/runtime/tool notes>
Next skill: agentclash-runtime-resources-setup | agentclash-agent-deployment-setup
Notes: <version notes, immutable-ready caveats, or blockers>
```

## Related Skills
- `agentclash-cli-setup`
- `agentclash-runtime-resources-setup`
- `agentclash-agent-deployment-setup`

## Related Docs
- `/docs-md/concepts/agents-and-deployments`
- `/docs-md/guides/configure-runtime-resources`
- `/docs-md/reference/cli`
- `/docs-md/reference/config`
