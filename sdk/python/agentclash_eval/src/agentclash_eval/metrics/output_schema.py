from __future__ import annotations

import json
from typing import Any, Callable

from agentclash_eval.metrics.base import Metric
from agentclash_eval.models import AgentEvalResult, MetricResult


class OutputSchema(Metric):
    """Pass when output parses as JSON and satisfies a validator callable or type check."""

    key = "output_schema"
    name = "OutputSchema"

    def __init__(
        self,
        schema: type | Callable[[Any], bool] | None = None,
        *,
        key: str | None = None,
    ) -> None:
        self.schema = schema
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
                evidence={"output_excerpt": output[:200]},
            )

        if self.schema is None:
            return MetricResult(
                key=self.key,
                name=self.name,
                passed=True,
                reason="output is valid JSON",
                evidence={"parsed_type": type(parsed).__name__},
            )

        if isinstance(self.schema, type):
            passed = isinstance(parsed, self.schema)
            reason = (
                f"output JSON matches type {self.schema.__name__}"
                if passed
                else f"expected JSON type {self.schema.__name__}, got {type(parsed).__name__}"
            )
        else:
            try:
                passed = bool(self.schema(parsed))
            except Exception as exc:  # noqa: BLE001 — surface validator errors in CI
                return MetricResult(
                    key=self.key,
                    name=self.name,
                    passed=False,
                    reason=f"schema validator raised: {exc}",
                    evidence={"parsed_type": type(parsed).__name__},
                    error=str(exc),
                )
            reason = "output JSON satisfies schema validator" if passed else "output JSON failed schema validator"

        return MetricResult(
            key=self.key,
            name=self.name,
            passed=passed,
            reason=reason,
            evidence={"parsed_type": type(parsed).__name__},
        )
