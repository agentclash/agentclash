from agentclash_eval import AgentEvalResult, assert_agent, evaluate
from agentclash_eval.metrics import Contains, ToolCalled
from agentclash_eval.models import ToolCall


def refund_agent(prompt: str) -> AgentEvalResult:
    return AgentEvalResult(
        input=prompt,
        output="Refund issued for order 12345.",
        tool_calls=[
            ToolCall(name="lookup_order", arguments={"order_id": "12345"}),
            ToolCall(name="issue_refund", arguments={"order_id": "12345"}),
        ],
    )


def test_refund_agent_plain_function():
    result = refund_agent("Customer was double charged.")
    assert_agent(result, metrics=[Contains("Refund"), ToolCalled(["lookup_order", "issue_refund"])])
