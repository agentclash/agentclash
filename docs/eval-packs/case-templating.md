# Case templating

Eval pack cases can parameterize strings with `{{placeholder}}` syntax. Placeholders resolve from each case's `payload` object and from structured `inputs[]` entries (inputs override payload keys).

## Syntax

- Top-level: `{{order_id}}`
- Nested payload paths: `{{customer.id}}`
- Placeholder names match `[A-Za-z_][A-Za-z0-9_.]*`

## Supported fields (v1)

| Field | When validated | When rendered |
| --- | --- | --- |
| `code_execution.config.test_command` | Bundle load (`ValidateBundle`) | Run post-execution verification |
| `prompt_eval` challenge `instructions` | N/A (lenient at runtime) | Prompt eval executor |

Future multi-turn `user_simulator` turn messages will use the same renderer (#841).

## Example

```yaml
validators:
  - key: tests
    type: code_execution
    target: file:generated_code
    config:
      test_command: pytest tests/test_{{order_id}}.py -q

input_sets:
  - key: default
    cases:
      - challenge_key: fix-order
        case_key: case-1
        payload:
          order_id: "12345"
```

At bundle validation, `test_command` must resolve for every case. At run time, the first case in the run agent's input set supplies values.

## Related

- Tool argument templating uses `${...}` via `templateutil` (different syntax).
- Stored case payloads may use the canonical `StoredCaseDocument` envelope; nested values under `payload` are available to placeholders.
