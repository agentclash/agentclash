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

| Actor | Required fields |
| --- | --- |
| `scripted` | `turns[].message` |
| `llm` | `persona` |
| `human` | optional `timeout_ms` |

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
