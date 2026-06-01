# User simulator manifest

Per-case `user_simulator` blocks define hybrid multi-turn user actors for packs with `version.execution_mode: multi_turn`. This issue validates schema only; execution lands in later sub-issues.

## Case field

```yaml
input_sets:
  - key: default
    cases:
      - challenge_key: fix-order
        case_key: case-1
        payload:
          order_id: "12345"
        user_simulator:
          schema_version: 1
          kind: hybrid
          max_turns: 20
          phases:
            - id: open
              actor: scripted
              turns:
                - message: "Refund order {{order_id}}"
            - id: pushback
              actor: scripted
              trigger: on_assistant_mismatch
              turns:
                - message: "That is the wrong order."
            - id: dynamic
              actor: llm
              trigger: on_assistant_mismatch
              persona: "Frustrated customer"
              max_turns: 6
            - id: human
              actor: human
              trigger: manual
              timeout_ms: 1800000
```

## Actors

| Actor | Required fields | Optional fields |
| --- | --- | --- |
| `scripted` | `turns[].message` | — |
| `llm` | `persona` | `model` (override the inherited simulator model — see below) |
| `human` | — | `timeout_ms` |

### LLM phase `model` override

`actor: llm` phases inherit provider, credentials, and **model** from the
agent deployment by default. If the deployment runs on a model that only
supports `/v1/responses` (e.g. `o3`, `o4-mini`, `o4-mini-deep-research`),
inheritance fails because the simulator's provider client uses
`/v1/chat/completions`. Pin a chat-compatible model id with `model:` to
avoid the mismatch:

```yaml
phases:
  - id: dynamic
    actor: llm
    trigger: on_assistant_mismatch
    persona: "Frustrated customer"
    model: "gpt-4o-mini"   # overrides the inherited reasoning-model id
```

The override only applies to `actor: llm` phases — setting `model:` on
`scripted` or `human` phases is rejected at pack validation time. Provider
and credentials are still inherited from the deployment, so the override
must name a model the deployment's provider serves.

## Triggers

`always`, `on_assistant_mismatch`, `on_validator_fail`, `on_judge_below`, `on_agent_loop`, `on_max_llm_turns`, `manual`, `never`

The first phase must use an empty trigger or `always`. Later phases must declare a trigger explicitly.

## Validation rules

- `multi_turn` packs require `user_simulator` on every case.
- Other execution modes reject `user_simulator`.
- Scripted `message` strings support `{{placeholder}}` templating (see [case-templating.md](./case-templating.md)).

## Related

- Epic: [#839](https://github.com/agentclash/agentclash/issues/839)
- Sub-issue: [#841](https://github.com/agentclash/agentclash/issues/841)
