"""Tests for the model-agnostic LLM client."""

import pytest
import httpx

from reasoning.client.model_client import ModelClient, ModelClientError, normalize_finish_reason


MOCK_COMPLETION = {
    "id": "chatcmpl-test",
    "object": "chat.completion",
    "choices": [
        {
            "index": 0,
            "message": {"role": "assistant", "content": "Paris"},
            "finish_reason": "stop",
        }
    ],
    "usage": {"prompt_tokens": 10, "completion_tokens": 3, "total_tokens": 13},
}

MOCK_TOOL_COMPLETION = {
    "id": "chatcmpl-test-tool",
    "object": "chat.completion",
    "choices": [
        {
            "index": 0,
            "message": {
                "role": "assistant",
                "content": None,
                "tool_calls": [
                    {
                        "id": "call_abc123",
                        "type": "function",
                        "function": {"name": "read_file", "arguments": '{"path": "/workspace/data.txt"}'},
                    }
                ],
            },
            "finish_reason": "tool_calls",
        }
    ],
    "usage": {"prompt_tokens": 15, "completion_tokens": 8, "total_tokens": 23},
}


@pytest.fixture
def mock_transport():
    """Creates an httpx mock transport for testing."""

    class MockTransport(httpx.AsyncBaseTransport):
        def __init__(self):
            self.requests: list[httpx.Request] = []
            self.responses: list[httpx.Response] = []
            self._response_queue: list[httpx.Response] = []

        def queue_response(self, status_code: int, json_data: dict | None = None, text: str = ""):
            import json

            if json_data is not None:
                content = json.dumps(json_data).encode()
                headers = {"content-type": "application/json"}
            else:
                content = text.encode()
                headers = {"content-type": "text/plain"}
            self._response_queue.append(httpx.Response(status_code, content=content, headers=headers))

        async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
            self.requests.append(request)
            if not self._response_queue:
                return httpx.Response(500, content=b"no response queued")
            resp = self._response_queue.pop(0)
            self.responses.append(resp)
            return resp

    return MockTransport()


@pytest.fixture
def client_with_transport(mock_transport):
    """Creates a ModelClient that uses the mock transport."""
    client = ModelClient(api_key="test-key", base_url="https://api.test.com/v1")
    client._client = httpx.AsyncClient(transport=mock_transport, base_url="https://api.test.com/v1")
    return client, mock_transport


@pytest.mark.asyncio
async def test_successful_completion(client_with_transport):
    client, transport = client_with_transport
    transport.queue_response(200, MOCK_COMPLETION)

    response = await client.chat_completions(
        model="gpt-4o",
        messages=[{"role": "user", "content": "What is the capital of France?"}],
    )

    assert response.finish_reason == "stop"
    assert response.output_text == "Paris"
    assert response.usage.input_tokens == 10
    assert response.usage.output_tokens == 3
    assert len(response.tool_calls) == 0


@pytest.mark.asyncio
async def test_tool_call_completion(client_with_transport):
    client, transport = client_with_transport
    transport.queue_response(200, MOCK_TOOL_COMPLETION)

    response = await client.chat_completions(
        model="gpt-4o",
        messages=[{"role": "user", "content": "Read the data file"}],
        tools=[{"name": "read_file", "description": "Read a file", "parameters": {"type": "object"}}],
    )

    assert response.finish_reason == "tool_calls"
    assert len(response.tool_calls) == 1
    assert response.tool_calls[0].name == "read_file"
    assert response.tool_calls[0].id == "call_abc123"


@pytest.mark.asyncio
async def test_retries_on_429(client_with_transport):
    client, transport = client_with_transport
    transport.queue_response(429, text="rate limited")
    transport.queue_response(429, text="rate limited")
    transport.queue_response(200, MOCK_COMPLETION)

    response = await client.chat_completions(
        model="gpt-4o",
        messages=[{"role": "user", "content": "test"}],
    )

    assert response.output_text == "Paris"
    assert len(transport.requests) == 3


@pytest.mark.asyncio
async def test_no_retry_on_401(client_with_transport):
    client, transport = client_with_transport
    transport.queue_response(401, text="unauthorized")

    with pytest.raises(ModelClientError) as exc_info:
        await client.chat_completions(model="gpt-4o", messages=[{"role": "user", "content": "test"}])

    assert exc_info.value.status_code == 401
    assert not exc_info.value.retryable
    assert len(transport.requests) == 1


@pytest.mark.asyncio
async def test_exhausted_retries(client_with_transport):
    client, transport = client_with_transport
    for _ in range(3):
        transport.queue_response(500, text="server error")

    with pytest.raises(ModelClientError) as exc_info:
        await client.chat_completions(model="gpt-4o", messages=[{"role": "user", "content": "test"}])

    assert exc_info.value.retryable
    assert len(transport.requests) == 3


# --- Finish reason normalization ---


def test_normalize_finish_reason_openai():
    assert normalize_finish_reason("stop") == "stop"
    assert normalize_finish_reason("tool_calls") == "tool_calls"
    assert normalize_finish_reason("length") == "length"
    assert normalize_finish_reason("content_filter") == "content_filter"


def test_normalize_finish_reason_anthropic():
    assert normalize_finish_reason("end_turn") == "stop"
    assert normalize_finish_reason("tool_use") == "tool_calls"
    assert normalize_finish_reason("max_tokens") == "length"


def test_normalize_finish_reason_gemini():
    assert normalize_finish_reason("STOP") == "stop"
    assert normalize_finish_reason("MAX_TOKENS") == "length"
    assert normalize_finish_reason("SAFETY") == "content_filter"


def test_normalize_finish_reason_unknown_passthrough():
    assert normalize_finish_reason("some_new_reason") == "some_new_reason"


# --- Anthropic response parsing ---

MOCK_ANTHROPIC_COMPLETION = {
    "id": "msg_test",
    "type": "message",
    "role": "assistant",
    "content": [
        {"type": "text", "text": "The capital of France is Paris."}
    ],
    "stop_reason": "end_turn",
    "usage": {"input_tokens": 20, "output_tokens": 8},
}

MOCK_ANTHROPIC_TOOL_USE = {
    "id": "msg_test_tool",
    "type": "message",
    "role": "assistant",
    "content": [
        {"type": "text", "text": "I'll read that file for you."},
        {"type": "tool_use", "id": "toolu_abc", "name": "read_file", "input": {"path": "/data.txt"}},
    ],
    "stop_reason": "tool_use",
    "usage": {"input_tokens": 30, "output_tokens": 15},
}


def test_parse_anthropic_response():
    client = ModelClient(api_key="test", provider_key="anthropic")
    response = client._parse_anthropic_response(MOCK_ANTHROPIC_COMPLETION)
    assert response.finish_reason == "stop"  # end_turn -> stop
    assert response.output_text == "The capital of France is Paris."
    assert len(response.tool_calls) == 0
    assert response.usage.input_tokens == 20
    assert response.usage.output_tokens == 8


def test_parse_anthropic_tool_use():
    client = ModelClient(api_key="test", provider_key="anthropic")
    response = client._parse_anthropic_response(MOCK_ANTHROPIC_TOOL_USE)
    assert response.finish_reason == "tool_calls"  # tool_use -> tool_calls
    assert response.output_text == "I'll read that file for you."
    assert len(response.tool_calls) == 1
    assert response.tool_calls[0].name == "read_file"
    assert response.tool_calls[0].id == "toolu_abc"
    assert '"path"' in response.tool_calls[0].arguments


@pytest.mark.asyncio
async def test_anthropic_request_uses_messages_endpoint():
    """Verify Anthropic requests go to /v1/messages, not /chat/completions."""
    client = ModelClient(api_key="test-key", base_url="https://api.anthropic.com", provider_key="anthropic")
    endpoint, body = client._build_request(
        model="claude-sonnet-4-20250514",
        messages=[{"role": "system", "content": "You are helpful."}, {"role": "user", "content": "Hi"}],
        tools=[{"name": "read_file", "description": "Read file", "parameters": {"type": "object"}}],
        temperature=0.0,
        max_tokens=4096,
    )
    assert endpoint == "/v1/messages"
    assert body["system"] == "You are helpful."
    assert len(body["messages"]) == 1  # system extracted
    assert body["messages"][0]["role"] == "user"
    assert body["tools"][0]["input_schema"] == {"type": "object"}
    await client.close()
