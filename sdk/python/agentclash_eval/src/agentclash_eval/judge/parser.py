from __future__ import annotations

import json
import re
from dataclasses import dataclass
from typing import Any


class JudgeParseError(Exception):
    def __init__(self, message: str, *, raw_output: str | None = None) -> None:
        super().__init__(message)
        self.raw_output = raw_output


@dataclass
class JudgeVerdict:
    passed: bool
    score: float | None = None
    reason: str | None = None
    raw: dict[str, Any] | None = None


_FENCE_RE = re.compile(r"```(?:json)?\s*(.*?)```", re.DOTALL | re.IGNORECASE)


def parse_judge_output(raw_output: str) -> JudgeVerdict:
    candidates = _candidate_json_strings(raw_output)
    last_error: Exception | None = None
    for candidate in candidates:
        try:
            payload = json.loads(candidate)
        except json.JSONDecodeError as exc:
            last_error = exc
            repaired = _repair_json(candidate)
            if repaired is None:
                continue
            try:
                payload = json.loads(repaired)
            except json.JSONDecodeError as exc2:
                last_error = exc2
                continue
        return _payload_to_verdict(payload)

    raise JudgeParseError(
        f"unable to parse judge output as JSON: {last_error}",
        raw_output=raw_output,
    )


def _candidate_json_strings(raw_output: str) -> list[str]:
    stripped = raw_output.strip()
    candidates = [stripped]
    for match in _FENCE_RE.finditer(raw_output):
        block = match.group(1).strip()
        if block:
            candidates.append(block)
    brace = _extract_balanced_object(stripped)
    if brace:
        candidates.append(brace)
    deduped: list[str] = []
    for item in candidates:
        if item not in deduped:
            deduped.append(item)
    return deduped


def _extract_balanced_object(text: str) -> str | None:
    start = text.find("{")
    if start < 0:
        return None
    depth = 0
    for index in range(start, len(text)):
        char = text[index]
        if char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                return text[start : index + 1]
    return None


def _repair_json(text: str) -> str | None:
    trimmed = text.strip()
    if not trimmed:
        return None
    if trimmed.endswith(","):
        trimmed = trimmed[:-1]
    if not trimmed.endswith("}"):
        trimmed = trimmed + "}"
    try:
        json.loads(trimmed)
    except json.JSONDecodeError:
        return None
    return trimmed


def _payload_to_verdict(payload: Any) -> JudgeVerdict:
    if not isinstance(payload, dict):
        raise JudgeParseError("judge output must be a JSON object")
    passed = bool(payload.get("passed"))
    score = payload.get("score")
    reason = payload.get("reason")
    return JudgeVerdict(
        passed=passed,
        score=float(score) if score is not None else None,
        reason=str(reason) if reason is not None else None,
        raw=payload,
    )
