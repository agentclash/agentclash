from __future__ import annotations

import pytest

from agentclash_eval import AgentEvalResult, evaluate
from agentclash_eval.metrics import (
    CostLimit,
    JSONPath,
    LatencyLimit,
    NoForbiddenTool,
    RegexMatch,
    ToolArgumentEquals,
    ToolCalled,
    ToolSequence,
)
from agentclash_eval.models import RunMetadata, ToolCall


def _result(**kwargs) -> AgentEvalResult:
    return AgentEvalResult(input="test", **kwargs)


def test_regex_match_pass_and_fail():
    passed = evaluate(_result(output="order 12345"), metrics=[RegexMatch(r"order \d+")])
    assert passed.exit_code == 0
    failed = evaluate(_result(output="no ids here"), metrics=[RegexMatch(r"order \d+")])
    assert failed.exit_code == 1


def test_json_path_expected_value():
    report = evaluate(
        _result(output='{"status":"ok","count":2}'),
        metrics=[JSONPath("$.status", "ok")],
    )
    assert report.exit_code == 0


def test_tool_called_and_sequence():
    result = _result(
        output="done",
        tool_calls=[
            ToolCall(name="lookup_order"),
            ToolCall(name="issue_refund"),
        ],
    )
    assert evaluate(result, metrics=[ToolCalled(["lookup_order", "issue_refund"])]).exit_code == 0
    assert evaluate(result, metrics=[ToolSequence(["lookup_order", "issue_refund"])]).exit_code == 0


def test_tool_argument_equals():
    result = _result(
        tool_calls=[ToolCall(name="issue_refund", arguments={"order_id": "1", "amount_usd": 10})],
    )
    report = evaluate(
        result,
        metrics=[ToolArgumentEquals("issue_refund", {"order_id": "1"})],
    )
    assert report.exit_code == 0


def test_no_forbidden_tool():
    result = _result(tool_calls=[ToolCall(name="safe_tool")])
    assert evaluate(result, metrics=[NoForbiddenTool(["delete_all"])]).exit_code == 0
    bad = _result(tool_calls=[ToolCall(name="delete_all")])
    assert evaluate(bad, metrics=[NoForbiddenTool(["delete_all"])]).exit_code == 1


def test_latency_and_cost_limits():
    ok = _result(
        output="ok",
        metadata=RunMetadata(latency_ms=100, cost_usd=0.01),
    )
    assert evaluate(ok, metrics=[LatencyLimit(200), CostLimit(0.02)]).exit_code == 0
    slow = _result(output="ok", metadata=RunMetadata(latency_ms=500))
    assert evaluate(slow, metrics=[LatencyLimit(200)]).exit_code == 1


def test_multi_turn_tool_indices_preserved():
    result = _result(
        output="refunded",
        tool_calls=[
            ToolCall(name="lookup_order", turn_index=0),
            ToolCall(name="issue_refund", turn_index=1),
        ],
    )
    report = evaluate(result, metrics=[ToolSequence(["lookup_order", "issue_refund"])])
    assert report.cases[0].agent_result.tool_calls[1].turn_index == 1
