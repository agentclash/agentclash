from __future__ import annotations

from typing import Any

from agentclash_eval.models import (
    AgentEvalResult,
    Message,
    RetrievalContext,
    RunMetadata,
    ToolCall,
)


def normalize_result(value: str | dict[str, Any] | AgentEvalResult) -> AgentEvalResult:
    if isinstance(value, AgentEvalResult):
        return value
    if isinstance(value, str):
        return AgentEvalResult(input=value, output=value)
    if isinstance(value, dict):
        return _from_dict(value)
    raise TypeError(
        f"expected str, dict, or AgentEvalResult, got {type(value).__name__}"
    )


def _from_dict(data: dict[str, Any]) -> AgentEvalResult:
    messages = [
        Message(
            role=str(item.get("role", "")),
            content=str(item.get("content", "")),
            name=item.get("name"),
            tool_call_id=item.get("tool_call_id"),
        )
        for item in data.get("messages", [])
    ]
    tool_calls = [
        ToolCall(
            name=str(item.get("name", "")),
            arguments=item.get("arguments"),
            arguments_json=item.get("arguments_json"),
            turn_index=item.get("turn_index"),
            result=item.get("result"),
        )
        for item in data.get("tool_calls", [])
    ]
    retrieval_context = [
        RetrievalContext(
            content=str(item.get("content", "")),
            source=item.get("source"),
            score=item.get("score"),
            metadata=dict(item.get("metadata") or {}),
        )
        for item in data.get("retrieval_context", [])
    ]
    metadata_raw = data.get("metadata")
    metadata = None
    if isinstance(metadata_raw, dict):
        metadata = RunMetadata(
            model=metadata_raw.get("model"),
            provider=metadata_raw.get("provider"),
            latency_ms=metadata_raw.get("latency_ms"),
            cost_usd=metadata_raw.get("cost_usd"),
            step_count=metadata_raw.get("step_count"),
            token_count=metadata_raw.get("token_count"),
            tags=list(metadata_raw.get("tags") or []),
            extra=dict(metadata_raw.get("extra") or {}),
        )

    input_value = data.get("input")
    if input_value is None:
        raise ValueError("dict result must include 'input'")

    return AgentEvalResult(
        input=str(input_value),
        output=data.get("output"),
        messages=messages,
        tool_calls=tool_calls,
        retrieval_context=retrieval_context,
        metadata=metadata,
    )
