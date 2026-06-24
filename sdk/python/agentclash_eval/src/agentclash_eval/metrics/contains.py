from __future__ import annotations

from agentclash_eval.metrics.base import Metric
from agentclash_eval.models import AgentEvalResult, MetricResult


class Contains(Metric):
    """Pass when agent output contains the expected substring (case-sensitive)."""

    key = "contains"
    name = "Contains"

    def __init__(self, expected: str, *, key: str | None = None) -> None:
        self.expected = expected
        if key is not None:
            self.key = key

    def evaluate(self, result: AgentEvalResult) -> MetricResult:
        output = result.output or ""
        passed = self.expected in output
        return MetricResult(
            key=self.key,
            name=self.name,
            passed=passed,
            reason=(
                f"output contains {self.expected!r}"
                if passed
                else f"output missing expected substring {self.expected!r}"
            ),
            evidence={"expected": self.expected, "output_excerpt": output[:200]},
        )
