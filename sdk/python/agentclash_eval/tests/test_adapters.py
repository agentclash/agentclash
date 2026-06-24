from agentclash_eval import assert_agent, evaluate
from agentclash_eval.adapters import from_langchain_messages, from_openai_response
from agentclash_eval.metrics import ToolCalled


def test_openai_adapter_maps_tool_calls():
    response = {
        "model": "gpt-4.1-mini",
        "choices": [
            {
                "message": {
                    "content": "refund issued",
                    "tool_calls": [
                        {
                            "function": {
                                "name": "issue_refund",
                                "arguments": "{\"order_id\": \"1\"}",
                            }
                        }
                    ],
                }
            }
        ],
        "usage": {"total_tokens": 42},
    }
    result = from_openai_response(response, input_text="refund please")
    report = evaluate(result, metrics=[ToolCalled(["issue_refund"])])
    assert report.exit_code == 0
    assert result.metadata is not None
    assert result.metadata.token_count == 42


def test_langchain_adapter_maps_messages():
    messages = [
        {"type": "human", "content": "hello"},
        {
            "type": "ai",
            "content": "hi there",
            "tool_calls": [{"name": "lookup_order", "args": {"order_id": "9"}}],
        },
    ]
    result = from_langchain_messages(messages, input_text="hello")
    assert result.output == "hi there"
    assert result.tool_calls[0].name == "lookup_order"
