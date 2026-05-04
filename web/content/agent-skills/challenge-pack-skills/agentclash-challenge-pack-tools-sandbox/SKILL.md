---
name: agentclash-challenge-pack-tools-sandbox
description: Use when defining AgentClash challenge pack tool access, sandbox runtime needs, filesystem expectations, network policy, command execution, and secret references.
metadata:
  agentclash.role: challenge-pack-tools
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Challenge Pack Tools And Sandbox

## Purpose
Define the native execution surface a challenge pack needs: pack-defined custom tools, broad tool policy, sandbox network/package/env settings, and safe secret references.

Use this skill only when a pack truly needs native files, tools, network, packages, or sandbox behavior. Keep the runtime surface narrow enough that failures are attributable to the agent, not to an over-broad environment.

## Use When
- A challenge pack needs top-level `tools.custom`.
- The pack needs `version.tool_policy.allowed_tool_kinds`.
- The pack needs `version.sandbox` for network access, CIDR allowlists, environment variables, apt packages, or a sandbox template.
- A coding agent needs exact source-backed YAML shapes without reading the AgentClash source repo.
- A reviewer needs to check that no raw secrets or unsupported tool kinds are being introduced.

## Do Not Use When
- The pack is `prompt_eval` and only needs prompt/final-output evaluation.
- The task is workspace infrastructure setup with `agentclash infra tool ...`; use `agentclash-runtime-resources-setup`.
- The task is artifact declaration, scoring validators, LLM judges, validation/publish, or eval running; use the focused downstream skills.

## Environment
Use hosted production for CLI examples unless the user intentionally targets a local or self-hosted backend.

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

## Validation Commands
Validate after adding or changing tools, tool policy, or sandbox settings.

```bash
agentclash challenge-pack validate path/to/pack.yaml
agentclash challenge-pack validate path/to/pack.yaml --json
```

Human output prints `Challenge pack is valid` or `Challenge pack has errors`. Use `--json` for structured `valid` and `errors` fields.

## Execution Mode Rules
`prompt_eval` packs cannot use challenge-pack tools or sandbox settings.

Do not include these in `prompt_eval`:
- top-level `tools`
- `version.tool_policy`
- `version.sandbox`

Use `native` when the task needs files, tool calls, network policy, extra packages, sandbox templates, file validators, directory checks, or code execution.

```yaml
version:
  number: 1
  execution_mode: native
```

## Tool Policy
`version.tool_policy.allowed_tool_kinds` accepts only these broad kinds:

```yaml
version:
  tool_policy:
    allowed_tool_kinds:
      - browser
      - build
      - data
      - file
      - network
```

Supported values are exactly `browser`, `build`, `data`, `file`, and `network`. Do not use `shell`; the current validator rejects it.

Use the narrowest set possible:
- `file` for reading/writing workspace files.
- `build` for build/test style operations.
- `network` for outbound HTTP or API access.
- `browser` for browser interaction.
- `data` for structured data access tools.

## Pack-Defined Custom Tools
Challenge-pack custom tools live at top-level `tools.custom`, not under `version`.

```yaml
tools:
  custom:
    - name: check_inventory
      description: Check inventory for a SKU.
      parameters:
        type: object
        properties:
          sku:
            type: string
        required:
          - sku
      implementation:
        primitive: http_request
        args:
          method: GET
          url: "https://api.example.com/inventory/${sku}"
          headers:
            Authorization: "Bearer ${secrets.INVENTORY_API_KEY}"
```

Source-backed fields:
- `tools.custom[]` entries are the supported pack-defined tool shape.
- `name` should be stable and unique in the pack.
- `parameters` must be valid JSON Schema when provided. If omitted, validation defaults to an empty object schema, but authoring explicit parameters is clearer.
- `implementation` is required.
- Non-`mock` implementations require `implementation.primitive`.
- Non-`mock` implementations require `implementation.args`, and `args` must be a JSON/YAML object.
- `implementation.primitive` cannot equal the tool's own `name`.
- Tool delegation cycles are rejected, and delegation depth greater than 8 is rejected.

Mock tools are the only exception to primitive/args validation:

```yaml
tools:
  custom:
    - name: fake_lookup
      parameters:
        type: object
      implementation:
        type: mock
```

## Template Placeholders And Secrets
Template placeholders are validated inside `implementation.args`.

Allowed placeholder forms:
- `${sku}` or `${sku.id}` when `sku` is declared in `parameters.properties`.
- `${parameters}` for the full parameters object.
- `${secrets.INVENTORY_API_KEY}` for a runtime secret reference.

Rejected placeholder forms:
- `${missing}` when `missing` is not declared in `parameters.properties`.
- `${}` empty placeholders.
- unclosed placeholders such as `${sku`.

Never paste raw secret values into YAML, chat, commits, or examples. Use secret names only. If a secret value is not already configured, ask the user to set it through the workspace secret flow without revealing the value in chat.

## Sandbox Settings
`version.sandbox` is valid only for `native` packs.

```yaml
version:
  execution_mode: native
  sandbox:
    network_access: true
    network_allowlist:
      - 203.0.113.0/24
    env_vars:
      DATASET_MODE: fixture
      API_BASE_URL: https://api.example.com
    additional_packages:
      - jq
      - python3-venv
    sandbox_template_id: codex
```

Source-backed sandbox fields:
- `network_access`: boolean.
- `network_allowlist`: list of CIDR ranges. Hostnames such as `api.example.com` are not valid allowlist entries.
- `env_vars`: string map. Keys must match `[A-Za-z_][A-Za-z0-9_]*`.
- `additional_packages`: apt-style package names.
- `sandbox_template_id`: optional template identifier string.

Keep `network_access: false` or omit sandbox network settings unless the case truly needs outbound network. If network is needed, use the smallest CIDR allowlist available.

## Filesystem Expectations
`version.filesystem` exists as a raw map on the bundle model, but the current challenge-pack validator does not define a source-backed schema for it. Do not invent `version.filesystem` subfields in a skill-authored pack. Prefer explicit assets, case inputs, sandbox package/env settings, and scoring file evidence until the user or product docs provide an exact filesystem contract.

Use file-related behavior through:
- `version.assets` and case `inputs[].artifact_key`.
- `version.tool_policy.allowed_tool_kinds: [file]`.
- scoring file validators that target `file:<post_execution_check_key>`.
- `version.evaluation_spec.post_execution_checks` for file or directory capture.

## Compatibility Checklist
Before validating:

- Execution mode is `native` if `tools`, `tool_policy`, or `sandbox` are present.
- `allowed_tool_kinds` contains only `browser`, `build`, `data`, `file`, and `network`.
- No `shell` tool kind is present.
- Every custom tool has a stable `name`, parameter schema, `implementation.primitive`, and object `implementation.args`, unless it is a deliberate `type: mock` tool.
- Every `${...}` placeholder in tool args is declared as a parameter, is `${parameters}`, or starts with `${secrets.}`.
- No raw secret values are present.
- `network_allowlist` uses CIDR ranges.
- `env_vars` keys are valid environment variable names.
- `additional_packages` names are valid apt package names.
- Native settings are backed by a smoke case that proves the environment actually works.

## Common Validation Failures
- A `prompt_eval` pack includes `tools`, `version.tool_policy`, or `version.sandbox`.
- `version.tool_policy.allowed_tool_kinds` includes `shell`, `code`, or provider-specific tool names.
- `allowed_tool_kinds` is not an array of strings.
- A non-mock custom tool omits `implementation.primitive` or `implementation.args`.
- `implementation.args` is a string/list instead of an object.
- Tool args use unknown placeholders such as `${order_id}` without declaring `order_id` in `parameters.properties`.
- A tool delegates to itself or creates a delegation cycle.
- `network_allowlist` contains a hostname instead of CIDR.
- `env_vars` contains a key like `api-key` that is not a valid environment variable name.
- `additional_packages` includes an invalid apt package name.

## Authoring Procedure
1. Confirm whether `prompt_eval` is enough. If yes, omit tools and sandbox.
2. If native behavior is required, set `version.execution_mode: native`.
3. Add only the needed `allowed_tool_kinds`.
4. Define `tools.custom` only for pack-defined tools; use workspace infra skills for reusable workspace tools.
5. Write explicit JSON Schema parameters for each custom tool.
6. Use `${parameter}` and `${secrets.KEY}` placeholders in `implementation.args`; never raw secrets.
7. Add `version.sandbox` only for real network/env/package/template requirements.
8. Add a smoke case that proves the tool or sandbox dependency is reachable.
9. Run `agentclash challenge-pack validate ... --json` and fix every returned field error.
10. Hand off to artifacts, scoring, or validation/publish skills.

## Report Back Format
```text
Execution mode:
Tool policy:
Custom tools:
- name:
  primitive:
  parameters:
  secret references:
Sandbox:
Network:
Packages:
Filesystem/artifact dependencies:
Smoke case:
Validation command:
Validation result:
Ready for scoring/publish: <yes/no>
Open issues:
```

## Related Skills
- `agentclash-runtime-resources-setup`
- `agentclash-challenge-pack-yaml-author`
- `agentclash-challenge-pack-input-sets`
- `agentclash-challenge-pack-artifacts`
- `agentclash-challenge-pack-scoring-validators`
- `agentclash-challenge-pack-validation-publish`
