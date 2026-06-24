# feat/eval-sdk-1108 — Test Contract

## Functional Behavior

- All 10 deterministic metrics implemented with typed constructors.
- Each returns MetricResult with key, name, passed, score/threshold when applicable, reason, evidence.
- Metrics work without network credentials.
- Usable in assert_agent and evaluate.

## Unit Tests

- `tests/test_metrics.py` covers pass/fail for RegexMatch, JSONPath, ToolCalled, ToolSequence, ToolArgumentEquals, NoForbiddenTool, LatencyLimit, CostLimit, multi-turn tool indices.

## Smoke Tests

- `cd sdk/python/agentclash_eval && PYTHONPATH=src python3 -m pytest -q`

## E2E Tests

- N/A

## Manual Tests

- N/A
