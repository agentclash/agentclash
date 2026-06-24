from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol


@dataclass
class JudgeRequest:
    prompt: str
    prompt_template_id: str
    prompt_template_version: str = "1.0.0"


@dataclass
class JudgeResponse:
    raw_output: str
    retry_count: int = 0


class JudgeProvider(Protocol):
    def complete(self, request: JudgeRequest) -> JudgeResponse: ...


class JudgeProviderError(Exception):
    def __init__(self, code: str, message: str, *, retryable: bool = False) -> None:
        super().__init__(message)
        self.code = code
        self.message = message
        self.retryable = retryable


class FakeJudgeProvider:
    """Deterministic in-memory judge for unit tests."""

    def __init__(self, responses: dict[str, str] | None = None) -> None:
        self.responses = responses or {}
        self.calls: list[JudgeRequest] = []

    def complete(self, request: JudgeRequest) -> JudgeResponse:
        self.calls.append(request)
        if request.prompt_template_id in self.responses:
            return JudgeResponse(raw_output=self.responses[request.prompt_template_id])
        return JudgeResponse(raw_output='{"passed": true, "score": 1.0, "reason": "ok"}')
