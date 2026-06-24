from __future__ import annotations

import json
from pathlib import Path

import pytest

from agentclash_eval import AgentEvalResult, assert_agent, evaluate
from agentclash_eval.evaluate import EvalAssertionError
from agentclash_eval.metrics import Contains, OutputSchema
from agentclash_eval.report import to_report_dict


def test_assert_agent_passes_with_contains():
    report = assert_agent("hello world", metrics=[Contains("world")])
    assert report.exit_code == 0
    assert report.summary.passed == 1


def test_assert_agent_raises_on_failure():
    with pytest.raises(EvalAssertionError) as excinfo:
        assert_agent("hello", metrics=[Contains("world")])
    assert "Contains" in str(excinfo.value)
    assert excinfo.value.report.exit_code == 1


def test_evaluate_accepts_dict_result():
    result = {
        "input": "refund please",
        "output": "Your refund is processing.",
        "tool_calls": [{"name": "issue_refund", "arguments": {"order_id": "1"}}],
    }
    report = evaluate(result, metrics=[Contains("refund")])
    assert report.exit_code == 0
    assert report.cases[0].agent_result is not None
    assert report.cases[0].agent_result.tool_calls[0].name == "issue_refund"


def test_evaluate_accepts_agent_eval_result():
    result = AgentEvalResult(input="hi", output='{"status": "ok"}')
    report = evaluate(result, metrics=[OutputSchema(dict)])
    assert report.exit_code == 0


def test_output_schema_rejects_invalid_json():
    result = AgentEvalResult(input="hi", output="not-json")
    report = evaluate(result, metrics=[OutputSchema()])
    assert report.exit_code == 1
    assert report.failures[0].metric_name == "OutputSchema"


def test_report_matches_schema_when_jsonschema_installed(jsonschema_available):
    if not jsonschema_available:
        pytest.skip("jsonschema not installed")
    report = evaluate("hello world", metrics=[Contains("world")])
    payload = to_report_dict(report)
    schema_path = (
        Path(__file__).resolve().parents[4]
        / "schemas"
        / "evaltest"
        / "eval-report.schema.json"
    )
    schema = json.loads(schema_path.read_text())
    import jsonschema

    jsonschema.validate(payload, schema)


@pytest.fixture
def jsonschema_available():
    try:
        import jsonschema  # noqa: F401

        return True
    except ImportError:
        return False
