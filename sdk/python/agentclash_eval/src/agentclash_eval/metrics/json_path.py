from __future__ import annotations

import json
from typing import Any

from agentclash_eval.metrics.base import Metric
from agentclash_eval.models import AgentEvalResult, MetricResult


def _resolve_json_path(document: Any, path: str) -> Any:
    current = document
    if path.startswith("$."):
        path = path[2:]
    elif path == "$":
        return document
    for part in path.split("."):
        if part == "":
            continue
        if isinstance(current, dict):
            if part not in current:
                raise KeyError(part)
            current = current[part]
            continue
        if isinstance(current, list):
            index = int(part)
            current = current[index]
            continue
        raise KeyError(part)
    return current


class JSONPath(Metric):
    """Pass when a JSONPath expression resolves to an expected value in parsed output."""

    key = "json_path"
    name = "JSONPath"

    def __init__(
        self,
        path: str,
        expected: Any | None = None,
        *,
        key: str | None = None,
    ) -> None:
        self.path = path
        self.expected = expected
        if key is not None:
            self.key = key

    def evaluate(self, result: AgentEvalResult) -> MetricResult:
        output = result.output or ""
        try:
            parsed = json.loads(output)
        except json.JSONDecodeError as exc:
            return MetricResult(
                key=self.key,
                name=self.name,
                passed=False,
                reason=f"output is not valid JSON: {exc.msg}",
                evidence={"path": self.path},
            )
        try:
            actual = _resolve_json_path(parsed, self.path)
        except (KeyError, IndexError, ValueError) as exc:
            return MetricResult(
                key=self.key,
                name=self.name,
                passed=False,
                reason=f"JSONPath {self.path!r} not found: {exc}",
                evidence={"path": self.path},
            )

        if self.expected is None:
            return MetricResult(
                key=self.key,
                name=self.name,
                passed=True,
                reason=f"JSONPath {self.path!r} resolved",
                evidence={"path": self.path, "actual": actual},
            )

        passed = actual == self.expected
        return MetricResult(
            key=self.key,
            name=self.name,
            passed=passed,
            reason=(
                f"JSONPath {self.path!r} equals expected value"
                if passed
                else f"JSONPath {self.path!r} expected {self.expected!r}, got {actual!r}"
            ),
            evidence={"path": self.path, "expected": self.expected, "actual": actual},
        )
