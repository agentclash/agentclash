from __future__ import annotations

from agentclash_eval.metrics.base import Metric
from agentclash_eval.models import AgentEvalResult, MetricResult


class ToolCalled(Metric):
    """Pass when all expected tools were called (order not required)."""

    key = "tool_called"
    name = "ToolCalled"

    def __init__(self, expected: list[str], *, key: str | None = None) -> None:
        self.expected = expected
        if key is not None:
            self.key = key

    def evaluate(self, result: AgentEvalResult) -> MetricResult:
        actual = [call.name for call in result.tool_calls]
        missing = [name for name in self.expected if name not in actual]
        passed = not missing
        return MetricResult(
            key=self.key,
            name=self.name,
            passed=passed,
            reason=(
                "expected tools were called"
                if passed
                else f"missing required tools: {', '.join(missing)}"
            ),
            evidence={"expected": self.expected, "actual": actual},
        )


class ToolSequence(Metric):
    """Pass when tool calls appear in the expected order (extras allowed)."""

    key = "tool_sequence"
    name = "ToolSequence"

    def __init__(self, expected: list[str], *, key: str | None = None) -> None:
        self.expected = expected
        if key is not None:
            self.key = key

    def evaluate(self, result: AgentEvalResult) -> MetricResult:
        actual = [call.name for call in result.tool_calls]
        passed = _contains_subsequence(actual, self.expected)
        return MetricResult(
            key=self.key,
            name=self.name,
            passed=passed,
            reason=(
                "tools called in expected order"
                if passed
                else f"expected sequence {self.expected}, got {actual}"
            ),
            evidence={"expected": self.expected, "actual": actual},
        )


class ToolArgumentEquals(Metric):
    """Pass when a tool was called with exact argument key/value pairs."""

    key = "tool_argument_equals"
    name = "ToolArgumentEquals"

    def __init__(
        self,
        tool: str,
        arguments: dict,
        *,
        key: str | None = None,
    ) -> None:
        self.tool = tool
        self.arguments = arguments
        if key is not None:
            self.key = key

    def evaluate(self, result: AgentEvalResult) -> MetricResult:
        matches = [
            call
            for call in result.tool_calls
            if call.name == self.tool and _arguments_match(call.arguments or {}, self.arguments)
        ]
        passed = bool(matches)
        return MetricResult(
            key=self.key,
            name=self.name,
            passed=passed,
            reason=(
                f"{self.tool} called with expected arguments"
                if passed
                else f"{self.tool} missing call with arguments {self.arguments!r}"
            ),
            evidence={
                "tool": self.tool,
                "expected_arguments": self.arguments,
                "actual_calls": [
                    {"name": call.name, "arguments": call.arguments}
                    for call in result.tool_calls
                    if call.name == self.tool
                ],
            },
        )


class NoForbiddenTool(Metric):
    """Pass when none of the forbidden tools were called."""

    key = "no_forbidden_tool"
    name = "NoForbiddenTool"

    def __init__(self, forbidden: list[str], *, key: str | None = None) -> None:
        self.forbidden = forbidden
        if key is not None:
            self.key = key

    def evaluate(self, result: AgentEvalResult) -> MetricResult:
        actual = [call.name for call in result.tool_calls]
        violations = [name for name in self.forbidden if name in actual]
        passed = not violations
        return MetricResult(
            key=self.key,
            name=self.name,
            passed=passed,
            reason=(
                "no forbidden tools called"
                if passed
                else f"forbidden tools called: {', '.join(violations)}"
            ),
            evidence={"forbidden": self.forbidden, "actual": actual, "violations": violations},
        )


def _contains_subsequence(actual: list[str], expected: list[str]) -> bool:
    if not expected:
        return True
    index = 0
    for name in actual:
        if name == expected[index]:
            index += 1
            if index == len(expected):
                return True
    return False


def _arguments_match(actual: dict, expected: dict) -> bool:
    for key, value in expected.items():
        if key not in actual or actual[key] != value:
            return False
    return True
