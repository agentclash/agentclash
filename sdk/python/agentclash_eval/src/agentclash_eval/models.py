from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass
class Message:
    role: str
    content: str
    name: str | None = None
    tool_call_id: str | None = None


@dataclass
class ToolCall:
    name: str
    arguments: dict[str, Any] | None = None
    arguments_json: str | None = None
    turn_index: int | None = None
    result: str | None = None


@dataclass
class RetrievalContext:
    content: str
    source: str | None = None
    score: float | None = None
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass
class RunMetadata:
    model: str | None = None
    provider: str | None = None
    latency_ms: float | None = None
    cost_usd: float | None = None
    step_count: int | None = None
    token_count: int | None = None
    tags: list[str] = field(default_factory=list)
    extra: dict[str, Any] = field(default_factory=dict)


@dataclass
class AgentEvalResult:
    input: str
    output: str | None = None
    messages: list[Message] = field(default_factory=list)
    tool_calls: list[ToolCall] = field(default_factory=list)
    retrieval_context: list[RetrievalContext] = field(default_factory=list)
    metadata: RunMetadata | None = None


@dataclass
class JudgeEvidence:
    prompt_template_id: str
    verdict: str
    prompt_template_version: str | None = None
    raw_output: str | None = None
    raw_output_redacted: str | None = None
    parsed_verdict: dict[str, Any] | None = None
    retry_count: int = 0


@dataclass
class MetricResult:
    key: str
    name: str
    passed: bool
    score: float | None = None
    threshold: float | None = None
    reason: str | None = None
    evidence: dict[str, Any] = field(default_factory=dict)
    judge_evidence: JudgeEvidence | None = None
    error: str | None = None


@dataclass
class EvalCase:
    case_id: str
    name: str
    file: str | None = None
    line: int | None = None
    tags: list[str] = field(default_factory=list)
    input: str | None = None
    expected: str | None = None


@dataclass
class EvalCaseResult:
    case: EvalCase
    status: str
    metrics: list[MetricResult]
    agent_result: AgentEvalResult | None = None
    duration_ms: float | None = None
    cost_usd: float | None = None
    error: str | None = None


@dataclass
class EvalFailure:
    case_id: str
    metric_key: str
    reason: str
    case_name: str | None = None
    metric_name: str | None = None
    evidence: dict[str, Any] = field(default_factory=dict)
    input: str | None = None
    output: str | None = None


@dataclass
class EvalSummary:
    total: int
    passed: int
    failed: int
    skipped: int
    errored: int
    metric_failures: int = 0
    duration_ms: float | None = None
    total_cost_usd: float | None = None


@dataclass
class EvalReport:
    schema_version: int
    report_id: str
    generated_at: str
    runner: dict[str, str]
    summary: EvalSummary
    cases: list[EvalCaseResult]
    exit_code: int
    failures: list[EvalFailure] = field(default_factory=list)
    duration_ms: float | None = None
    total_cost_usd: float | None = None
