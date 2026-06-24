from __future__ import annotations

import re

from agentclash_eval.metrics.base import Metric
from agentclash_eval.models import AgentEvalResult, MetricResult


class RegexMatch(Metric):
    """Pass when agent output matches the given regular expression."""

    key = "regex_match"
    name = "RegexMatch"

    def __init__(self, pattern: str, *, flags: int = 0, key: str | None = None) -> None:
        self.pattern = pattern
        self.flags = flags
        if key is not None:
            self.key = key

    def evaluate(self, result: AgentEvalResult) -> MetricResult:
        output = result.output or ""
        matched = re.search(self.pattern, output, self.flags) is not None
        return MetricResult(
            key=self.key,
            name=self.name,
            passed=matched,
            reason=(
                f"output matches /{self.pattern}/"
                if matched
                else f"output does not match /{self.pattern}/"
            ),
            evidence={"pattern": self.pattern, "output_excerpt": output[:200]},
        )
