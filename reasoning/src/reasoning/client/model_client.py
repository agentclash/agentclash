"""Model-agnostic LLM client with provider-aware response parsing.

Supports OpenAI, Anthropic, and Gemini response formats. For providers that
are OpenAI-compatible (Together, Groq, Fireworks), use provider_key="openai".

To add a new provider:
1. Add finish_reason mappings to FINISH_REASON_MAP
2. Add a _parse_{provider}_response method
3. Add request formatting if the provider is not OpenAI-compatible
"""

from __future__ import annotations

import asyncio
import json
import logging
from dataclasses import dataclass, field
from typing import Any

import httpx

logger = logging.getLogger(__name__)

TRANSIENT_STATUS_CODES = {429, 500, 502, 503, 504}
MAX_RETRIES = 3
INITIAL_BACKOFF = 1.0

# Canonical finish reason normalization across providers.
FINISH_REASON_MAP: dict[str, str] = {
    # OpenAI (canonical)
    "stop": "stop",
    "tool_calls": "tool_calls",
    "length": "length",
    "content_filter": "content_filter",
    # Anthropic
    "end_turn": "stop",
    "tool_use": "tool_calls",
    "max_tokens": "length",
    # Google Gemini
    "STOP": "stop",
    "MAX_TOKENS": "length",
    "SAFETY": "content_filter",
    "RECITATION": "content_filter",
}


def normalize_finish_reason(raw: str) -> str:
    """Map provider-specific finish reasons to canonical values."""
    return FINISH_REASON_MAP.get(raw, raw)


@dataclass
class Usage:
    input_tokens: int = 0
    output_tokens: int = 0
    total_tokens: int = 0


@dataclass
class ToolCall:
    id: str
    name: str
    arguments: str  # raw JSON string


@dataclass
class ModelResponse:
    finish_reason: str
    output_text: str
    tool_calls: list[ToolCall] = field(default_factory=list)
    usage: Usage = field(default_factory=Usage)
    raw_response: dict[str, Any] = field(default_factory=dict)


class ModelClientError(Exception):
    def __init__(self, message: str, *, retryable: bool = False, status_code: int | None = None):
        super().__init__(message)
        self.retryable = retryable
        self.status_code = status_code


class ModelClient:
    """Provider-aware LLM client.

    Args:
        api_key: Provider API key.
        base_url: Provider API base URL.
        provider_key: One of "openai", "anthropic", "gemini". OpenAI-compatible
            providers (Together, Groq, Fireworks) should use "openai".
    """

    def __init__(self, api_key: str, base_url: str = "https://api.openai.com/v1", provider_key: str = "openai"):
        self._provider_key = provider_key
        self._api_key = api_key
        self._base_url = base_url

        headers: dict[str, str] = {"Content-Type": "application/json"}
        if provider_key == "anthropic":
            headers["x-api-key"] = api_key
            headers["anthropic-version"] = "2023-06-01"
        else:
            headers["Authorization"] = f"Bearer {api_key}"

        self._client = httpx.AsyncClient(
            base_url=base_url,
            headers=headers,
            timeout=httpx.Timeout(120.0, connect=10.0),
        )

    async def close(self) -> None:
        await self._client.aclose()

    async def chat_completions(
        self,
        model: str,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]] | None = None,
        temperature: float = 0.0,
        max_tokens: int | None = None,
    ) -> ModelResponse:
        endpoint, body = self._build_request(model, messages, tools, temperature, max_tokens)

        last_error: Exception | None = None
        for attempt in range(MAX_RETRIES):
            try:
                resp = await self._client.post(endpoint, json=body)
            except httpx.TransportError as exc:
                last_error = ModelClientError(f"transport error: {exc}", retryable=True)
                await self._backoff(attempt)
                continue

            if resp.status_code in TRANSIENT_STATUS_CODES:
                last_error = ModelClientError(
                    f"provider returned {resp.status_code}", retryable=True, status_code=resp.status_code
                )
                logger.warning("transient provider error (attempt %d/%d): %s", attempt + 1, MAX_RETRIES, last_error)
                await self._backoff(attempt)
                continue

            if resp.status_code >= 400:
                raise ModelClientError(
                    f"provider returned {resp.status_code}: {resp.text}", retryable=False, status_code=resp.status_code
                )

            return self._parse(resp.json())

        raise last_error or ModelClientError("all retries exhausted")

    def _build_request(
        self,
        model: str,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]] | None,
        temperature: float,
        max_tokens: int | None,
    ) -> tuple[str, dict[str, Any]]:
        if self._provider_key == "anthropic":
            return self._build_anthropic_request(model, messages, tools, temperature, max_tokens)
        return self._build_openai_request(model, messages, tools, temperature, max_tokens)

    @staticmethod
    def _build_openai_request(
        model: str,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]] | None,
        temperature: float,
        max_tokens: int | None,
    ) -> tuple[str, dict[str, Any]]:
        body: dict[str, Any] = {"model": model, "messages": messages, "temperature": temperature}
        if tools:
            body["tools"] = [{"type": "function", "function": t} for t in tools]
        if max_tokens is not None:
            body["max_tokens"] = max_tokens
        return "/chat/completions", body

    @staticmethod
    def _build_anthropic_request(
        model: str,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]] | None,
        temperature: float,
        max_tokens: int | None,
    ) -> tuple[str, dict[str, Any]]:
        # Extract system message if present.
        system_text = ""
        user_messages = []
        for msg in messages:
            if msg.get("role") == "system":
                system_text = msg.get("content", "")
            else:
                user_messages.append(msg)

        body: dict[str, Any] = {
            "model": model,
            "messages": user_messages,
            "temperature": temperature,
            "max_tokens": max_tokens or 4096,
        }
        if system_text:
            body["system"] = system_text
        if tools:
            body["tools"] = [{"name": t["name"], "description": t.get("description", ""), "input_schema": t.get("parameters", {})} for t in tools]
        return "/v1/messages", body

    def _parse(self, data: dict[str, Any]) -> ModelResponse:
        if self._provider_key == "anthropic":
            return self._parse_anthropic_response(data)
        return self._parse_openai_response(data)

    @staticmethod
    def _parse_openai_response(data: dict[str, Any]) -> ModelResponse:
        choice = data.get("choices", [{}])[0]
        message = choice.get("message", {})
        raw_finish_reason = choice.get("finish_reason", "")

        tool_calls: list[ToolCall] = []
        for tc in message.get("tool_calls", []):
            func = tc.get("function", {})
            tool_calls.append(ToolCall(id=tc["id"], name=func.get("name", ""), arguments=func.get("arguments", "{}")))

        usage_data = data.get("usage", {})
        usage = Usage(
            input_tokens=usage_data.get("prompt_tokens", 0),
            output_tokens=usage_data.get("completion_tokens", 0),
            total_tokens=usage_data.get("total_tokens", 0),
        )

        return ModelResponse(
            finish_reason=normalize_finish_reason(raw_finish_reason),
            output_text=message.get("content", "") or "",
            tool_calls=tool_calls,
            usage=usage,
            raw_response=data,
        )

    @staticmethod
    def _parse_anthropic_response(data: dict[str, Any]) -> ModelResponse:
        raw_stop_reason = data.get("stop_reason", "")
        content_blocks = data.get("content", [])

        output_text = ""
        tool_calls: list[ToolCall] = []
        for block in content_blocks:
            block_type = block.get("type", "")
            if block_type == "text":
                output_text += block.get("text", "")
            elif block_type == "tool_use":
                tool_calls.append(ToolCall(
                    id=block.get("id", ""),
                    name=block.get("name", ""),
                    arguments=json.dumps(block.get("input", {})),
                ))

        usage_data = data.get("usage", {})
        usage = Usage(
            input_tokens=usage_data.get("input_tokens", 0),
            output_tokens=usage_data.get("output_tokens", 0),
            total_tokens=usage_data.get("input_tokens", 0) + usage_data.get("output_tokens", 0),
        )

        return ModelResponse(
            finish_reason=normalize_finish_reason(raw_stop_reason),
            output_text=output_text,
            tool_calls=tool_calls,
            usage=usage,
            raw_response=data,
        )

    @staticmethod
    async def _backoff(attempt: int) -> None:
        delay = INITIAL_BACKOFF * (2**attempt)
        await asyncio.sleep(delay)
