from __future__ import annotations

from typing import Any, Protocol

from agentclash_eval.models import AgentEvalResult, MetricResult


class Metric(Protocol):
    key: str
    name: str

    def evaluate(self, result: AgentEvalResult) -> MetricResult: ...


def metric_key_for(metric: Metric) -> str:
    return getattr(metric, "key", metric.__class__.__name__)


def metric_name_for(metric: Metric) -> str:
    return getattr(metric, "name", metric.__class__.__name__)
