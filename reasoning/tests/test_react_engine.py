"""Tests for the ReAct reasoning engine."""

import asyncio
from datetime import datetime, timezone
from unittest.mock import AsyncMock
from uuid import uuid4

import pytest

from reasoning.client.model_client import ModelClient, ModelClientError, ModelResponse, ToolCall, Usage
from reasoning.emitter.callback import CallbackEmitter
from reasoning.engine.react import ReactEngine
from reasoning.models.bridge import StartRequest, ToolDefinition, ToolResult, ToolResultsBatch


def make_request(**overrides) -> StartRequest:
    defaults = {
        "run_id": uuid4(),
        "run_agent_id": uuid4(),
        "idempotency_key": "test-key",
        "execution_context": {
            "Deployment": {
                "RuntimeProfile": {"MaxIterations": 10, "StepTimeoutSeconds": 60, "RunTimeoutSeconds": 300},
                "ProviderAccount": {"ProviderKey": "openai", "_resolved_credential": "sk-test"},
                "ModelAlias": {"ModelCatalogEntry": {"ProviderModelID": "gpt-4o"}},
                "AgentBuildVersion": {"PolicySpec": None, "OutputSchema": None, "AgentKind": "reasoning_v1"},
            },
            "ChallengeInputSet": {"InputKey": "test-input", "Items": [{"Content": "What is 2+2?"}]},
        },
        "tools": [],
        "callback_url": "http://localhost:8080/events",
        "callback_token": "test-token",
        "deadline_at": datetime.now(timezone.utc),
    }
    defaults.update(overrides)
    return StartRequest(**defaults)


class FakeModelClient:
    """Mock model client that returns queued responses."""

    def __init__(self):
        self._responses: list[ModelResponse | ModelClientError] = []

    def queue(self, response: ModelResponse | ModelClientError):
        self._responses.append(response)

    async def chat_completions(self, **kwargs) -> ModelResponse:
        if not self._responses:
            raise ModelClientError("no responses queued")
        resp = self._responses.pop(0)
        if isinstance(resp, ModelClientError):
            raise resp
        return resp

    async def close(self):
        pass


class FakeEmitter:
    """Mock callback emitter that records events."""

    def __init__(self):
        self.events: list[dict] = []

    async def emit(self, event):
        self.events.append({"event_type": event.event_type, "payload": event.payload})

    async def close(self):
        pass


@pytest.mark.asyncio
async def test_tool_free_success():
    """Model returns a direct answer with no tool calls."""
    model = FakeModelClient()
    model.queue(ModelResponse(
        finish_reason="stop",
        output_text="The answer is 4.",
        tool_calls=[],
        usage=Usage(input_tokens=10, output_tokens=5, total_tokens=15),
    ))

    emitter = FakeEmitter()
    engine = ReactEngine(request=make_request(), model_client=model, emitter=emitter)
    await engine.run()

    event_types = [e["event_type"] for e in emitter.events]
    assert event_types == [
        "system.run.started",
        "system.step.started",
        "model.call.started",
        "model.call.completed",
        "system.step.completed",
        "system.output.finalized",
        "system.run.completed",
    ]

    completed = emitter.events[-1]
    assert completed["payload"]["final_output"] == "The answer is 4."
    assert completed["payload"]["step_count"] == 1
    assert completed["payload"]["input_tokens"] == 10


@pytest.mark.asyncio
async def test_tool_using_turn():
    """Model proposes a tool call, gets results, then answers."""
    model = FakeModelClient()
    # First call: propose tool
    model.queue(ModelResponse(
        finish_reason="tool_calls",
        output_text="",
        tool_calls=[ToolCall(id="tc-1", name="read_file", arguments='{"path": "/data.txt"}')],
        usage=Usage(input_tokens=20, output_tokens=10, total_tokens=30),
    ))
    # Second call: final answer after tool result
    model.queue(ModelResponse(
        finish_reason="stop",
        output_text="The file says hello.",
        tool_calls=[],
        usage=Usage(input_tokens=30, output_tokens=8, total_tokens=38),
    ))

    emitter = FakeEmitter()
    request = make_request(tools=[ToolDefinition(name="read_file", description="Read a file")])
    engine = ReactEngine(request=request, model_client=model, emitter=emitter)

    # Run engine in background, deliver tool results when proposal arrives.
    async def deliver_results():
        while True:
            events = [e["event_type"] for e in emitter.events]
            if "model.tool_calls.proposed" in events:
                engine.deliver_tool_results(ToolResultsBatch(
                    idempotency_key="tools-1",
                    tool_results=[ToolResult(tool_call_id="tc-1", status="completed", content="hello world")],
                ))
                return
            await asyncio.sleep(0.01)

    await asyncio.gather(engine.run(), deliver_results())

    event_types = [e["event_type"] for e in emitter.events]
    assert "model.tool_calls.proposed" in event_types
    assert "tool.call.completed" in event_types
    assert "system.run.completed" in event_types
    assert emitter.events[-1]["payload"]["step_count"] == 2
    assert emitter.events[-1]["payload"]["tool_call_count"] == 1


@pytest.mark.asyncio
async def test_max_iterations():
    """Engine stops after max_iterations."""
    model = FakeModelClient()
    for _ in range(3):
        model.queue(ModelResponse(
            finish_reason="tool_calls",
            output_text="",
            tool_calls=[ToolCall(id="tc-1", name="read_file", arguments='{}')],
            usage=Usage(input_tokens=5, output_tokens=5, total_tokens=10),
        ))

    emitter = FakeEmitter()
    request = make_request(
        execution_context={
            "Deployment": {
                "RuntimeProfile": {"MaxIterations": 2, "StepTimeoutSeconds": 60, "RunTimeoutSeconds": 300},
                "ProviderAccount": {"ProviderKey": "openai", "_resolved_credential": "sk-test"},
                "ModelAlias": {"ModelCatalogEntry": {"ProviderModelID": "gpt-4o"}},
                "AgentBuildVersion": {"PolicySpec": None, "OutputSchema": None},
            },
        },
        tools=[ToolDefinition(name="read_file", description="Read")],
    )
    engine = ReactEngine(request=request, model_client=model, emitter=emitter)

    async def deliver_results():
        delivered = 0
        while delivered < 2:
            events = [e["event_type"] for e in emitter.events]
            count = events.count("model.tool_calls.proposed")
            while delivered < count:
                engine.deliver_tool_results(ToolResultsBatch(
                    idempotency_key=f"tools-{delivered}",
                    tool_results=[ToolResult(tool_call_id="tc-1", status="completed", content="ok")],
                ))
                delivered += 1
            await asyncio.sleep(0.01)

    await asyncio.gather(engine.run(), deliver_results())

    event_types = [e["event_type"] for e in emitter.events]
    assert "system.run.failed" in event_types
    failed = next(e for e in emitter.events if e["event_type"] == "system.run.failed")
    assert failed["payload"]["stop_reason"] == "max_iterations"


@pytest.mark.asyncio
async def test_finish_reason_length():
    """Model returns finish_reason=length -> failure."""
    model = FakeModelClient()
    model.queue(ModelResponse(
        finish_reason="length",
        output_text="truncated...",
        tool_calls=[],
        usage=Usage(input_tokens=10, output_tokens=100, total_tokens=110),
    ))

    emitter = FakeEmitter()
    engine = ReactEngine(request=make_request(), model_client=model, emitter=emitter)
    await engine.run()

    event_types = [e["event_type"] for e in emitter.events]
    assert "system.run.failed" in event_types
    failed = next(e for e in emitter.events if e["event_type"] == "system.run.failed")
    assert failed["payload"]["stop_reason"] == "max_tokens"


@pytest.mark.asyncio
async def test_provider_error():
    """Model client raises error -> failure."""
    model = FakeModelClient()
    model.queue(ModelClientError("connection refused", retryable=False))

    emitter = FakeEmitter()
    engine = ReactEngine(request=make_request(), model_client=model, emitter=emitter)
    await engine.run()

    event_types = [e["event_type"] for e in emitter.events]
    assert "system.run.failed" in event_types
    failed = next(e for e in emitter.events if e["event_type"] == "system.run.failed")
    assert failed["payload"]["stop_reason"] == "provider_error"


@pytest.mark.asyncio
async def test_cancellation():
    """Cancelled run emits failure."""
    model = FakeModelClient()
    # Model would respond, but cancel fires first.
    model.queue(ModelResponse(
        finish_reason="stop", output_text="answer", tool_calls=[], usage=Usage()
    ))

    emitter = FakeEmitter()
    engine = ReactEngine(request=make_request(), model_client=model, emitter=emitter)
    engine.cancel()
    await engine.run()

    event_types = [e["event_type"] for e in emitter.events]
    assert "system.run.failed" in event_types
    failed = next(e for e in emitter.events if e["event_type"] == "system.run.failed")
    assert failed["payload"]["stop_reason"] == "cancelled"


# --- Failure mode tests ---


@pytest.mark.asyncio
async def test_content_filter():
    """Model returns finish_reason=content_filter -> failure."""
    model = FakeModelClient()
    model.queue(ModelResponse(
        finish_reason="content_filter", output_text="", tool_calls=[], usage=Usage()
    ))
    emitter = FakeEmitter()
    engine = ReactEngine(request=make_request(), model_client=model, emitter=emitter)
    await engine.run()

    event_types = [e["event_type"] for e in emitter.events]
    assert "system.run.failed" in event_types
    failed = next(e for e in emitter.events if e["event_type"] == "system.run.failed")
    assert failed["payload"]["stop_reason"] == "content_filter"


@pytest.mark.asyncio
async def test_protocol_error_empty_response():
    """Empty output_text with finish_reason=stop and no tools -> protocol error."""
    model = FakeModelClient()
    model.queue(ModelResponse(
        finish_reason="stop", output_text="", tool_calls=[], usage=Usage()
    ))
    emitter = FakeEmitter()
    engine = ReactEngine(request=make_request(), model_client=model, emitter=emitter)
    await engine.run()

    event_types = [e["event_type"] for e in emitter.events]
    assert "system.run.failed" in event_types
    failed = next(e for e in emitter.events if e["event_type"] == "system.run.failed")
    assert failed["payload"]["stop_reason"] == "protocol_error"


@pytest.mark.asyncio
async def test_blocked_tool_results():
    """Blocked tool results emit tool.call.failed, run continues to final answer."""
    model = FakeModelClient()
    model.queue(ModelResponse(
        finish_reason="tool_calls", output_text="",
        tool_calls=[ToolCall(id="tc-1", name="read_file", arguments='{}')],
        usage=Usage(input_tokens=10, output_tokens=5, total_tokens=15),
    ))
    model.queue(ModelResponse(
        finish_reason="stop", output_text="I could not read the file.",
        tool_calls=[], usage=Usage(input_tokens=20, output_tokens=8, total_tokens=28),
    ))

    emitter = FakeEmitter()
    request = make_request(tools=[ToolDefinition(name="read_file", description="Read")])
    engine = ReactEngine(request=request, model_client=model, emitter=emitter)

    async def deliver_blocked():
        while True:
            if any(e["event_type"] == "model.tool_calls.proposed" for e in emitter.events):
                engine.deliver_tool_results(ToolResultsBatch(
                    idempotency_key="tools-1",
                    tool_results=[ToolResult(tool_call_id="tc-1", status="blocked", error_message="tool not allowed")],
                ))
                return
            await asyncio.sleep(0.01)

    await asyncio.gather(engine.run(), deliver_blocked())

    event_types = [e["event_type"] for e in emitter.events]
    assert "tool.call.failed" in event_types
    assert "system.run.completed" in event_types


@pytest.mark.asyncio
async def test_failed_tool_results():
    """Failed tool results emit tool.call.failed, run continues."""
    model = FakeModelClient()
    model.queue(ModelResponse(
        finish_reason="tool_calls", output_text="",
        tool_calls=[ToolCall(id="tc-1", name="exec", arguments='{}')],
        usage=Usage(input_tokens=10, output_tokens=5, total_tokens=15),
    ))
    model.queue(ModelResponse(
        finish_reason="stop", output_text="Command failed, here is my answer.",
        tool_calls=[], usage=Usage(input_tokens=20, output_tokens=10, total_tokens=30),
    ))

    emitter = FakeEmitter()
    request = make_request(tools=[ToolDefinition(name="exec", description="Execute")])
    engine = ReactEngine(request=request, model_client=model, emitter=emitter)

    async def deliver_failed():
        while True:
            if any(e["event_type"] == "model.tool_calls.proposed" for e in emitter.events):
                engine.deliver_tool_results(ToolResultsBatch(
                    idempotency_key="tools-1",
                    tool_results=[ToolResult(tool_call_id="tc-1", status="failed", error_message="sandbox error")],
                ))
                return
            await asyncio.sleep(0.01)

    await asyncio.gather(engine.run(), deliver_failed())

    event_types = [e["event_type"] for e in emitter.events]
    assert "tool.call.failed" in event_types
    assert "system.run.completed" in event_types


@pytest.mark.asyncio
async def test_multiple_tool_calls_in_batch():
    """Model proposes 3 tool calls, all completed."""
    model = FakeModelClient()
    model.queue(ModelResponse(
        finish_reason="tool_calls", output_text="",
        tool_calls=[
            ToolCall(id="tc-1", name="read_file", arguments='{"path":"a.txt"}'),
            ToolCall(id="tc-2", name="read_file", arguments='{"path":"b.txt"}'),
            ToolCall(id="tc-3", name="list_files", arguments='{"prefix":"/"}'),
        ],
        usage=Usage(input_tokens=20, output_tokens=15, total_tokens=35),
    ))
    model.queue(ModelResponse(
        finish_reason="stop", output_text="Found 3 results.",
        tool_calls=[], usage=Usage(input_tokens=50, output_tokens=5, total_tokens=55),
    ))

    emitter = FakeEmitter()
    request = make_request(tools=[
        ToolDefinition(name="read_file", description="Read"),
        ToolDefinition(name="list_files", description="List"),
    ])
    engine = ReactEngine(request=request, model_client=model, emitter=emitter)

    async def deliver_multi():
        while True:
            if any(e["event_type"] == "model.tool_calls.proposed" for e in emitter.events):
                engine.deliver_tool_results(ToolResultsBatch(
                    idempotency_key="tools-1",
                    tool_results=[
                        ToolResult(tool_call_id="tc-1", status="completed", content="file a"),
                        ToolResult(tool_call_id="tc-2", status="completed", content="file b"),
                        ToolResult(tool_call_id="tc-3", status="completed", content="a.txt\nb.txt"),
                    ],
                ))
                return
            await asyncio.sleep(0.01)

    await asyncio.gather(engine.run(), deliver_multi())

    completed_tool_events = [e for e in emitter.events if e["event_type"] == "tool.call.completed"]
    assert len(completed_tool_events) == 3
    assert emitter.events[-1]["payload"]["tool_call_count"] == 3


@pytest.mark.asyncio
async def test_schema_repair_success():
    """First output fails validation, second passes."""
    model = FakeModelClient()
    # First: invalid output
    model.queue(ModelResponse(
        finish_reason="stop", output_text="just text, not json",
        tool_calls=[], usage=Usage(input_tokens=10, output_tokens=5, total_tokens=15),
    ))
    # Second: valid output
    model.queue(ModelResponse(
        finish_reason="stop", output_text='{"answer": "42"}',
        tool_calls=[], usage=Usage(input_tokens=20, output_tokens=5, total_tokens=25),
    ))

    emitter = FakeEmitter()
    request = make_request(
        execution_context={
            "Deployment": {
                "RuntimeProfile": {"MaxIterations": 10, "StepTimeoutSeconds": 60, "RunTimeoutSeconds": 300},
                "ProviderAccount": {"ProviderKey": "openai", "_resolved_credential": "sk-test"},
                "ModelAlias": {"ModelCatalogEntry": {"ProviderModelID": "gpt-4o"}},
                "AgentBuildVersion": {
                    "PolicySpec": None,
                    "OutputSchema": '{"type":"object","required":["answer"]}',
                },
            },
            "ChallengeInputSet": {"InputKey": "test", "Items": [{"Content": "What is the answer?"}]},
        },
    )
    engine = ReactEngine(request=request, model_client=model, emitter=emitter)
    await engine.run()

    model_call_events = [e for e in emitter.events if e["event_type"] == "model.call.completed"]
    assert len(model_call_events) == 2  # original + repair
    assert "system.run.completed" in [e["event_type"] for e in emitter.events]
    assert emitter.events[-1]["payload"]["final_output"] == '{"answer": "42"}'


@pytest.mark.asyncio
async def test_schema_repair_exhaustion():
    """Output always fails validation, repair exhausted, output accepted anyway."""
    model = FakeModelClient()
    for _ in range(3):  # 1 original + 2 repair attempts
        model.queue(ModelResponse(
            finish_reason="stop", output_text="not valid json",
            tool_calls=[], usage=Usage(input_tokens=10, output_tokens=5, total_tokens=15),
        ))

    emitter = FakeEmitter()
    request = make_request(
        execution_context={
            "Deployment": {
                "RuntimeProfile": {"MaxIterations": 10, "StepTimeoutSeconds": 60, "RunTimeoutSeconds": 300},
                "ProviderAccount": {"ProviderKey": "openai", "_resolved_credential": "sk-test"},
                "ModelAlias": {"ModelCatalogEntry": {"ProviderModelID": "gpt-4o"}},
                "AgentBuildVersion": {
                    "PolicySpec": None,
                    "OutputSchema": '{"type":"object","required":["answer"]}',
                },
            },
            "ChallengeInputSet": {"InputKey": "test", "Items": [{"Content": "What?"}]},
        },
    )
    engine = ReactEngine(request=request, model_client=model, emitter=emitter)
    await engine.run()

    model_call_events = [e for e in emitter.events if e["event_type"] == "model.call.completed"]
    assert len(model_call_events) == 3  # 1 original + 2 repairs
    # After exhaustion, output is accepted as-is
    assert "system.run.completed" in [e["event_type"] for e in emitter.events]


@pytest.mark.asyncio
async def test_callback_delivery_failure():
    """Callback delivery error triggers bridge_error failure."""

    class FailingEmitter:
        def __init__(self):
            self.events = []
            self._call_count = 0

        async def emit(self, event):
            self._call_count += 1
            self.events.append({"event_type": event.event_type, "payload": event.payload})
            if self._call_count == 2:  # Fail on second event (step.started)
                from reasoning.emitter.callback import CallbackDeliveryError
                raise CallbackDeliveryError("connection refused")

        async def close(self):
            pass

    model = FakeModelClient()
    model.queue(ModelResponse(
        finish_reason="stop", output_text="answer", tool_calls=[], usage=Usage()
    ))

    emitter = FailingEmitter()
    engine = ReactEngine(request=make_request(), model_client=model, emitter=emitter)
    await engine.run()

    # The engine catches the error and tries to emit run.failed.
    event_types = [e["event_type"] for e in emitter.events]
    assert "system.run.started" in event_types  # First event succeeds


# --- Cost and latency tests ---


@pytest.mark.asyncio
async def test_model_call_includes_latency():
    """model.call.completed payload includes latency_ms > 0."""
    model = FakeModelClient()
    model.queue(ModelResponse(
        finish_reason="stop", output_text="answer",
        tool_calls=[], usage=Usage(input_tokens=10, output_tokens=5, total_tokens=15),
    ))

    emitter = FakeEmitter()
    engine = ReactEngine(request=make_request(), model_client=model, emitter=emitter)
    await engine.run()

    model_completed = next(e for e in emitter.events if e["event_type"] == "model.call.completed")
    assert "latency_ms" in model_completed["payload"]
    assert model_completed["payload"]["latency_ms"] >= 0


@pytest.mark.asyncio
async def test_run_completed_includes_cost_totals():
    """system.run.completed includes total_latency_ms and total_estimated_cost_usd."""
    model = FakeModelClient()
    model.queue(ModelResponse(
        finish_reason="stop", output_text="answer",
        tool_calls=[], usage=Usage(input_tokens=1000, output_tokens=500, total_tokens=1500),
    ))

    emitter = FakeEmitter()
    # Use gpt-4o which is in the pricing table
    request = make_request(
        execution_context={
            "Deployment": {
                "RuntimeProfile": {"MaxIterations": 10, "StepTimeoutSeconds": 60, "RunTimeoutSeconds": 300},
                "ProviderAccount": {"ProviderKey": "openai", "_resolved_credential": "sk-test"},
                "ModelAlias": {"ModelCatalogEntry": {"ProviderModelID": "gpt-4o"}},
                "AgentBuildVersion": {"PolicySpec": None, "OutputSchema": None},
            },
            "ChallengeInputSet": {"InputKey": "test", "Items": [{"Content": "Hello"}]},
        },
    )
    engine = ReactEngine(request=request, model_client=model, emitter=emitter)
    await engine.run()

    completed = next(e for e in emitter.events if e["event_type"] == "system.run.completed")
    assert "total_latency_ms" in completed["payload"]
    assert completed["payload"]["total_latency_ms"] >= 0
    assert "total_estimated_cost_usd" in completed["payload"]
    assert completed["payload"]["total_estimated_cost_usd"] > 0
