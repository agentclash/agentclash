# Roadmap #693 / Issue #706 Test Contract

## Feature

Challenge packs can declare a deterministic `tool_call_assertion` validator that scores recorded tool-call evidence.

## Manifest Contract

The validator is config-only:

```yaml
validators:
  - key: used_submit
    type: tool_call_assertion
    target: tool_calls
    config:
      tool_name: submit
      must_call: true
      arguments_contain:
        answer: "42"
```

Supported config fields:

- `tool_name`: optional tool-name filter for presence, absence, count, and argument-fragment checks.
- `must_call`: optional boolean; defaults to `true` when `tool_name` is set. `false` asserts absence.
- `count`: optional exact count for calls matching `tool_name` and `arguments_contain`.
- `min_count` / `max_count`: optional count bounds for matching calls.
- `arguments_contain`: optional JSON object fragment that must be contained in at least one matching tool call's arguments.
- `ordered_tools`: optional ordered list of tool names.
- `order_mode`: optional `subsequence` (default) or `exact`.

`expected_from` is not required. `target` must be `tool_calls`.

## Required Test Cases

1. Presence passes when a matching tool call is recorded.
2. Absence passes when the forbidden tool is not recorded and fails when it is.
3. Exact/min/max count checks pass and fail against synthetic tool-call events.
4. Ordered tool checks support subsequence order and exact full order.
5. Argument fragments match nested JSON objects without requiring exact argument equality.
6. Missing required config or invalid config values fail spec validation.
7. Validator output includes safe summary evidence (counts, tool names, matched indices) and does not echo raw tool arguments.

## Validation Commands

```bash
cd backend
go test ./internal/scoring ./internal/challengepack ./internal/repository -run 'TestToolCallAssertion|TestOrderedEvaluationEventsHandlesMixedSequences|TestLoadEvaluationSpecToolCallAssertion|TestParseYAMLAcceptsToolCallAssertionValidator|TestBuildRunAgentScorecardDocumentIncludesToolCallAssertionEvidence' -count=1
go test ./...
```
