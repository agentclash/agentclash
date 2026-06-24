from __future__ import annotations

from typing import Any

from agentclash_eval.models import AgentEvalResult, Message, RunMetadata, ToolCall


def from_openai_response(response: dict[str, Any], *, input_text: str) -> AgentEvalResult:
    """Map an OpenAI-style chat completion response dict into AgentEvalResult."""
    choices = response.get("choices") or []
    if not choices:
        raise ValueError("OpenAI response missing choices")
    message = choices[0].get("message") or {}
    output = message.get("content")
    tool_calls = []
    for item in message.get("tool_calls") or []:
        function = item.get("function") or {}
        tool_calls.append(
            ToolCall(
                name=str(function.get("name", "")),
                arguments_json=function.get("arguments"),
            )
        )
    metadata = RunMetadata(
        model=str(response.get("model", "")),
        provider="openai",
        token_count=_usage_tokens(response.get("usage")),
    )
    return AgentEvalResult(
        input=input_text,
        output=output,
        messages=[Message(role="assistant", content=output or "")],
        tool_calls=tool_calls,
        metadata=metadata,
    )


def from_langchain_messages(messages: list[Any], *, input_text: str) -> AgentEvalResult:
    """Map LangChain-like message objects or dicts into AgentEvalResult."""
    normalized: list[Message] = []
    tool_calls: list[ToolCall] = []
    output = None
    for item in messages:
        if isinstance(item, dict):
            role = str(item.get("type") or item.get("role") or "assistant")
            content = item.get("content")
            if role in {"human", "user"}:
                role = "user"
            elif role == "ai":
                role = "assistant"
            normalized.append(Message(role=role, content=str(content or "")))
            if role == "assistant":
                output = str(content or "")
            for call in item.get("tool_calls") or []:
                tool_calls.append(
                    ToolCall(
                        name=str(call.get("name", "")),
                        arguments=call.get("args") or call.get("arguments"),
                    )
                )
            continue
        role = getattr(item, "type", None) or getattr(item, "role", "assistant")
        content = getattr(item, "content", "")
        if role in {"human", "user"}:
            role = "user"
        elif role == "ai":
            role = "assistant"
        normalized.append(Message(role=str(role), content=str(content or "")))
        if role == "assistant":
            output = str(content or "")
    return AgentEvalResult(
        input=input_text,
        output=output,
        messages=normalized,
        tool_calls=tool_calls,
    )


def _usage_tokens(usage: Any) -> int | None:
    if not isinstance(usage, dict):
        return None
    total = usage.get("total_tokens")
    return int(total) if total is not None else None
