import pytest

from agentclash_eval import AgentEvalResult, evaluate
from agentclash_eval.judge import (
    FakeJudgeProvider,
    TaskCompletion,
    parse_judge_output,
)
from agentclash_eval.judge.parser import JudgeParseError


def test_parse_judge_output_valid_json():
    verdict = parse_judge_output('{"passed": true, "score": 0.9, "reason": "done"}')
    assert verdict.passed is True
    assert verdict.score == 0.9


def test_parse_judge_output_markdown_fence():
    raw = 'Here is the result:\n```json\n{"passed": false, "reason": "missing refund"}\n```'
    verdict = parse_judge_output(raw)
    assert verdict.passed is False


def test_parse_judge_output_repair_trailing_comma():
    verdict = parse_judge_output('{"passed": true, "score": 1.0,')
    assert verdict.passed is True


def test_parse_judge_output_unrepairable():
    with pytest.raises(JudgeParseError):
        parse_judge_output("this is not json at all")


def test_task_completion_with_fake_provider():
    provider = FakeJudgeProvider(
        {"task_completion_v1": '{"passed": true, "score": 0.95, "reason": "completed"}'}
    )
    report = evaluate(
        AgentEvalResult(input="refund", output="refund issued"),
        metrics=[TaskCompletion(provider)],
    )
    assert report.exit_code == 0
    assert provider.calls


def test_task_completion_parser_failure_surfaces_error():
    provider = FakeJudgeProvider({"task_completion_v1": "not-json"})
    report = evaluate(
        AgentEvalResult(input="refund", output="refund issued"),
        metrics=[TaskCompletion(provider, max_retries=0)],
    )
    assert report.exit_code == 1
    assert report.cases[0].metrics[0].error
