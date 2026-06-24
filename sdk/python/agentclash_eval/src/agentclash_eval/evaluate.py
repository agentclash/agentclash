from __future__ import annotations

import time
from typing import Any, Sequence

from agentclash_eval._version import __version__
from agentclash_eval.metrics.base import Metric
from agentclash_eval.models import (
    AgentEvalResult,
    EvalCase,
    EvalCaseResult,
    EvalFailure,
    EvalReport,
    EvalSummary,
)
from agentclash_eval.normalize import normalize_result
from agentclash_eval.report import new_report_id, to_report_dict


class EvalAssertionError(AssertionError):
    """Raised by assert_agent when one or more metrics fail."""

    def __init__(self, report: EvalReport) -> None:
        self.report = report
        lines = ["agent eval failed:"]
        for failure in report.failures:
            lines.append(f"  - {failure.metric_name or failure.metric_key}: {failure.reason}")
        super().__init__("\n".join(lines))


def evaluate(
    result: str | dict[str, Any] | AgentEvalResult,
    metrics: Sequence[Metric],
    *,
    case_id: str = "eval_case",
    case_name: str | None = None,
) -> EvalReport:
    started = time.perf_counter()
    agent_result = normalize_result(result)
    metric_results = [_run_metric(metric, agent_result) for metric in metrics]

    failures = [
        EvalFailure(
            case_id=case_id,
            case_name=case_name or case_id,
            metric_key=metric.key,
            metric_name=metric.name,
            reason=metric.reason or "metric failed",
            evidence=dict(metric.evidence),
            input=agent_result.input,
            output=agent_result.output,
        )
        for metric in metric_results
        if not metric.passed
    ]

    failed = 1 if failures else 0
    passed = 1 if not failures else 0
    status = "passed" if not failures else "failed"
    exit_code = 0 if not failures else 1
    duration_ms = (time.perf_counter() - started) * 1000
    cost_usd = agent_result.metadata.cost_usd if agent_result.metadata else None

    case = EvalCase(case_id=case_id, name=case_name or case_id, input=agent_result.input)
    case_result = EvalCaseResult(
        case=case,
        status=status,
        agent_result=agent_result,
        metrics=metric_results,
        duration_ms=duration_ms,
        cost_usd=cost_usd,
    )

    return EvalReport(
        schema_version=1,
        report_id=new_report_id(),
        generated_at=_utc_now(),
        runner={
            "name": "agentclash-evals",
            "version": __version__,
            "language": "python",
        },
        summary=EvalSummary(
            total=1,
            passed=passed,
            failed=failed,
            skipped=0,
            errored=0,
            metric_failures=len(failures),
            duration_ms=duration_ms,
            total_cost_usd=cost_usd,
        ),
        cases=[case_result],
        failures=failures,
        exit_code=exit_code,
        duration_ms=duration_ms,
        total_cost_usd=cost_usd,
    )


def assert_agent(
    result: str | dict[str, Any] | AgentEvalResult,
    metrics: Sequence[Metric],
    *,
    case_id: str = "eval_case",
    case_name: str | None = None,
) -> EvalReport:
    report = evaluate(result, metrics, case_id=case_id, case_name=case_name)
    if report.failures:
        raise EvalAssertionError(report)
    return report


def _run_metric(metric: Metric, agent_result: AgentEvalResult):
    return metric.evaluate(agent_result)


def _utc_now() -> str:
    from datetime import datetime, timezone

    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


__all__ = ["assert_agent", "evaluate", "EvalAssertionError", "to_report_dict"]
