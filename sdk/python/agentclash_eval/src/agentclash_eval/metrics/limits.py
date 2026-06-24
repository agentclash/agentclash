from __future__ import annotations

from agentclash_eval.metrics.base import Metric
from agentclash_eval.models import AgentEvalResult, MetricResult


class LatencyLimit(Metric):
    """Pass when run latency is at or below the limit."""

    key = "latency_limit"
    name = "LatencyLimit"

    def __init__(self, max_ms: float, *, key: str | None = None) -> None:
        self.max_ms = max_ms
        if key is not None:
            self.key = key

    def evaluate(self, result: AgentEvalResult) -> MetricResult:
        latency = result.metadata.latency_ms if result.metadata else None
        if latency is None:
            return MetricResult(
                key=self.key,
                name=self.name,
                passed=False,
                reason="latency_ms metadata missing",
                evidence={"max_ms": self.max_ms},
            )
        passed = latency <= self.max_ms
        return MetricResult(
            key=self.key,
            name=self.name,
            passed=passed,
            threshold=self.max_ms,
            score=latency,
            reason=(
                f"latency {latency}ms within limit {self.max_ms}ms"
                if passed
                else f"latency {latency}ms exceeds limit {self.max_ms}ms"
            ),
            evidence={"latency_ms": latency, "max_ms": self.max_ms},
        )


class CostLimit(Metric):
    """Pass when run cost is at or below the limit."""

    key = "cost_limit"
    name = "CostLimit"

    def __init__(self, max_usd: float, *, key: str | None = None) -> None:
        self.max_usd = max_usd
        if key is not None:
            self.key = key

    def evaluate(self, result: AgentEvalResult) -> MetricResult:
        cost = result.metadata.cost_usd if result.metadata else None
        if cost is None:
            return MetricResult(
                key=self.key,
                name=self.name,
                passed=False,
                reason="cost_usd metadata missing",
                evidence={"max_usd": self.max_usd},
            )
        passed = cost <= self.max_usd
        return MetricResult(
            key=self.key,
            name=self.name,
            passed=passed,
            threshold=self.max_usd,
            score=cost,
            reason=(
                f"cost ${cost} within limit ${self.max_usd}"
                if passed
                else f"cost ${cost} exceeds limit ${self.max_usd}"
            ),
            evidence={"cost_usd": cost, "max_usd": self.max_usd},
        )
