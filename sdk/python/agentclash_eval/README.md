# agentclash-evals

Local pre-deploy agent eval assertions for CI/CD. No AgentClash account required.

## Quick start

```python
from agentclash_eval import assert_agent
from agentclash_eval.metrics import Contains, OutputSchema


def my_agent(prompt: str) -> str:
    return f"Hello! You said: {prompt}"


def test_my_agent():
    result = my_agent("world")
    assert_agent(result, metrics=[Contains("world")])


def test_structured_output():
    result = '{"status": "ok", "answer": 42}'
    assert_agent(result, metrics=[OutputSchema(dict)])
```

Run with pytest or any test runner:

```bash
pip install -e ".[dev]"
pytest
```

## API

- `assert_agent(result, metrics=[...])` — raises `EvalAssertionError` on failure
- `evaluate(result, metrics=[...])` — returns structured `EvalReport`
- Accepts plain strings, dict-like results, or `AgentEvalResult` objects

## Design

- No auth, no telemetry, no network (unless you add judge metrics later)
- Import does not register pytest plugins
- Reports match `schemas/evaltest/eval-report.schema.json`

See [docs/adr/predeploy-eval-sdk-repo-strategy.md](../../../docs/adr/predeploy-eval-sdk-repo-strategy.md).
