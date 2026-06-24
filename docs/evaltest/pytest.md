# Opt-in pytest integration

The core package does **not** register a pytest plugin on import.

## Without plugin

```python
from agentclash_eval import assert_agent
from agentclash_eval.metrics import Contains

def test_agent():
    assert_agent("hello world", metrics=[Contains("world")])
```

Run normally:

```bash
pytest tests/
```

Failures include metric name, reason, and evidence via `EvalAssertionError`.

## With plugin (opt-in)

Collect eval reports to JSON:

```bash
AGENTCLASH_EVAL_REPORT=agentclash-results/results.json \
  pytest -p agentclash_eval.pytest_plugin tests/
```

The plugin improves failure capture and writes the last report to the path in
`AGENTCLASH_EVAL_REPORT`.
