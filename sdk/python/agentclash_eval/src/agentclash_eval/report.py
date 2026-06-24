from __future__ import annotations

import uuid
from datetime import datetime, timezone
from typing import Any

from agentclash_eval.models import (
    AgentEvalResult,
    EvalCase,
    EvalCaseResult,
    EvalFailure,
    EvalReport,
    EvalSummary,
    JudgeEvidence,
    Message,
    MetricResult,
    RetrievalContext,
    RunMetadata,
    ToolCall,
)


def to_report_dict(report: EvalReport) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "schema_version": report.schema_version,
        "report_id": report.report_id,
        "generated_at": report.generated_at,
        "runner": report.runner,
        "summary": {
            "total": report.summary.total,
            "passed": report.summary.passed,
            "failed": report.summary.failed,
            "skipped": report.summary.skipped,
            "errored": report.summary.errored,
            "metric_failures": report.summary.metric_failures,
        },
        "cases": [_case_result_to_dict(case) for case in report.cases],
        "failures": [_failure_to_dict(failure) for failure in report.failures],
        "exit_code": report.exit_code,
    }
    if report.summary.duration_ms is not None:
        payload["summary"]["duration_ms"] = report.summary.duration_ms
    if report.summary.total_cost_usd is not None:
        payload["summary"]["total_cost_usd"] = report.summary.total_cost_usd
    if report.duration_ms is not None:
        payload["duration_ms"] = report.duration_ms
    if report.total_cost_usd is not None:
        payload["total_cost_usd"] = report.total_cost_usd
    return payload


def _case_result_to_dict(case: EvalCaseResult) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "case": {
            "case_id": case.case.case_id,
            "name": case.case.name,
        },
        "status": case.status,
        "metrics": [_metric_to_dict(metric) for metric in case.metrics],
    }
    if case.case.file:
        payload["case"]["file"] = case.case.file
    if case.case.line:
        payload["case"]["line"] = case.case.line
    if case.case.input:
        payload["case"]["input"] = case.case.input
    if case.case.expected:
        payload["case"]["expected"] = case.case.expected
    if case.agent_result is not None:
        payload["agent_result"] = _agent_result_to_dict(case.agent_result)
    if case.duration_ms is not None:
        payload["duration_ms"] = case.duration_ms
    if case.cost_usd is not None:
        payload["cost_usd"] = case.cost_usd
    if case.error:
        payload["error"] = case.error
    return payload


def _agent_result_to_dict(result: AgentEvalResult) -> dict[str, Any]:
    payload: dict[str, Any] = {"input": result.input}
    if result.output is not None:
        payload["output"] = result.output
    if result.messages:
        payload["messages"] = [
            {
                "role": msg.role,
                "content": msg.content,
                **({"name": msg.name} if msg.name else {}),
                **({"tool_call_id": msg.tool_call_id} if msg.tool_call_id else {}),
            }
            for msg in result.messages
        ]
    if result.tool_calls:
        payload["tool_calls"] = [
            {
                "name": call.name,
                **({"arguments": call.arguments} if call.arguments is not None else {}),
                **({"arguments_json": call.arguments_json} if call.arguments_json else {}),
                **({"turn_index": call.turn_index} if call.turn_index is not None else {}),
                **({"result": call.result} if call.result else {}),
            }
            for call in result.tool_calls
        ]
    if result.retrieval_context:
        payload["retrieval_context"] = [
            {
                "content": ctx.content,
                **({"source": ctx.source} if ctx.source else {}),
                **({"score": ctx.score} if ctx.score is not None else {}),
                **({"metadata": ctx.metadata} if ctx.metadata else {}),
            }
            for ctx in result.retrieval_context
        ]
    if result.metadata is not None:
        payload["metadata"] = _metadata_to_dict(result.metadata)
    return payload


def _metadata_to_dict(metadata: RunMetadata) -> dict[str, Any]:
    payload: dict[str, Any] = {}
    for field in ("model", "provider", "latency_ms", "cost_usd", "step_count", "token_count"):
        value = getattr(metadata, field)
        if value is not None:
            payload[field] = value
    if metadata.tags:
        payload["tags"] = metadata.tags
    if metadata.extra:
        payload["extra"] = metadata.extra
    return payload


def _metric_to_dict(metric: MetricResult) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "key": metric.key,
        "name": metric.name,
        "passed": metric.passed,
    }
    for field in ("score", "threshold", "reason", "error"):
        value = getattr(metric, field)
        if value is not None:
            payload[field] = value
    if metric.evidence:
        payload["evidence"] = metric.evidence
    if metric.judge_evidence is not None:
        payload["judge_evidence"] = _judge_evidence_to_dict(metric.judge_evidence)
    return payload


def _judge_evidence_to_dict(evidence: JudgeEvidence) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "prompt_template_id": evidence.prompt_template_id,
        "verdict": evidence.verdict,
    }
    for field in (
        "prompt_template_version",
        "raw_output",
        "raw_output_redacted",
        "parsed_verdict",
        "retry_count",
    ):
        value = getattr(evidence, field)
        if value is not None and value != 0:
            payload[field] = value
    return payload


def _failure_to_dict(failure: EvalFailure) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "case_id": failure.case_id,
        "metric_key": failure.metric_key,
        "reason": failure.reason,
    }
    for field in ("case_name", "metric_name", "input", "output"):
        value = getattr(failure, field)
        if value is not None:
            payload[field] = value
    if failure.evidence:
        payload["evidence"] = failure.evidence
    return payload


def new_report_id() -> str:
    return f"rpt-{uuid.uuid4().hex[:12]}"
