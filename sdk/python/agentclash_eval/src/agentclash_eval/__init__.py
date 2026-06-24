"""Local pre-deploy agent eval assertions."""

from agentclash_eval.evaluate import assert_agent, evaluate
from agentclash_eval.models import AgentEvalResult, EvalReport, MetricResult

__all__ = [
    "AgentEvalResult",
    "EvalReport",
    "MetricResult",
    "assert_agent",
    "evaluate",
]

from agentclash_eval._version import __version__