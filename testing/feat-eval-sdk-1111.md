# feat/eval-sdk-1111 — Test Contract

## Functional Behavior

- Opt-in pytest plugin at `agentclash_eval.pytest_plugin`
- Core import does not auto-register plugin
- Example at `examples/evaltest/python/test_refund_agent.py`
- Docs at `docs/evaltest/pytest.md`

## Unit Tests

- `tests/test_pytest_plugin.py`

## Smoke Tests

- `PYTHONPATH=src pytest -q`

## Manual Tests

- `AGENTCLASH_EVAL_REPORT=/tmp/report.json pytest -p agentclash_eval.pytest_plugin tests/`
