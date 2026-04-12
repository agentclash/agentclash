"""ReAct (Reason + Act) reasoning strategy engine.

Implements a single-loop strategy that interleaves model reasoning and tool
execution. The engine pauses at tool proposals and waits for Go to execute
tools and return results.
"""

from __future__ import annotations

import asyncio
import json
import logging
import time
from typing import Any
from uuid import UUID

from reasoning.client.model_client import ModelClient, ModelClientError, ModelResponse
from reasoning.client.pricing import estimate_cost
from reasoning.emitter.callback import CallbackDeliveryError, CallbackEmitter
from reasoning.models.bridge import StartRequest, ToolResult, ToolResultsBatch
from reasoning.models.events import Envelope, SummaryMetadata, make_event_id
from reasoning.validation.output import validate_final_output

logger = logging.getLogger(__name__)

MAX_REPAIR_ATTEMPTS = 2


class ReactEngine:
    """Runs a ReAct loop for a single reasoning run."""

    def __init__(
        self,
        request: StartRequest,
        model_client: ModelClient,
        emitter: CallbackEmitter,
        tool_result_timeout: float = 300,
    ):
        self.request = request
        self.run_id = request.run_id
        self.run_agent_id = request.run_agent_id
        self._model_client = model_client
        self._emitter = emitter
        self._tool_result_timeout = tool_result_timeout
        self._sequence = 0
        self._cancelled = False
        self._tool_results_event: asyncio.Event = asyncio.Event()
        self._tool_results: ToolResultsBatch | None = None
        self._total_input_tokens = 0
        self._total_output_tokens = 0
        self._total_model_calls = 0
        self._total_tool_calls = 0
        self._total_latency_ms = 0.0
        self._total_estimated_cost_usd = 0.0
        self._step_count = 0

    def cancel(self) -> None:
        self._cancelled = True

    def deliver_tool_results(self, batch: ToolResultsBatch) -> None:
        self._tool_results = batch
        self._tool_results_event.set()

    async def run(self) -> None:
        runtime_profile = self.request.execution_context.get("Deployment", {}).get("RuntimeProfile", {})
        max_iterations = runtime_profile.get("MaxIterations", 10)
        model_config = self.request.execution_context.get("Deployment", {}).get("ModelAlias", {})
        provider_key = self.request.execution_context.get("Deployment", {}).get("ProviderAccount", {}).get("ProviderKey", "")
        model_id = model_config.get("ModelCatalogEntry", {}).get("ProviderModelID", "")
        credential = self.request.execution_context.get("Deployment", {}).get("ProviderAccount", {}).get("_resolved_credential", "")
        policy_spec = self.request.execution_context.get("Deployment", {}).get("AgentBuildVersion", {}).get("PolicySpec")
        output_schema_raw = self.request.execution_context.get("Deployment", {}).get("AgentBuildVersion", {}).get("OutputSchema")

        system_prompt = ""
        if policy_spec:
            try:
                ps = json.loads(policy_spec) if isinstance(policy_spec, str) else policy_spec
                system_prompt = ps.get("instructions", "")
            except (json.JSONDecodeError, TypeError):
                pass

        output_schema: dict[str, Any] | None = None
        if output_schema_raw:
            try:
                output_schema = json.loads(output_schema_raw) if isinstance(output_schema_raw, str) else output_schema_raw
            except (json.JSONDecodeError, TypeError):
                pass

        tools_for_model = [
            {"name": t.name, "description": t.description, "parameters": t.parameters or {}}
            for t in self.request.tools
        ]

        messages: list[dict[str, Any]] = []
        if system_prompt:
            messages.append({"role": "system", "content": system_prompt})

        # Extract challenge input from execution context.
        challenge_input = self._extract_challenge_input()
        if challenge_input:
            messages.append({"role": "user", "content": challenge_input})

        try:
            await self._emit_run_started(provider_key, model_id)

            repair_attempts = 0

            for step in range(max_iterations):
                if self._cancelled:
                    await self._emit_run_failed("cancelled", "run was cancelled")
                    return

                self._step_count += 1
                await self._emit_step_started(step)
                await self._emit_model_call_started(provider_key, model_id, step)

                call_start = time.monotonic()
                try:
                    response = await self._model_client.chat_completions(
                        model=model_id,
                        messages=messages,
                        tools=tools_for_model if tools_for_model else None,
                    )
                except ModelClientError as exc:
                    await self._emit_model_call_completed_error(provider_key, model_id, step, str(exc))
                    await self._emit_step_completed(step)
                    await self._emit_run_failed("provider_error", str(exc))
                    return

                call_latency_ms = round((time.monotonic() - call_start) * 1000, 1)
                call_cost = estimate_cost(model_id, response.usage.input_tokens, response.usage.output_tokens)
                self._total_input_tokens += response.usage.input_tokens
                self._total_output_tokens += response.usage.output_tokens
                self._total_model_calls += 1
                self._total_latency_ms += call_latency_ms
                if call_cost is not None:
                    self._total_estimated_cost_usd += call_cost

                await self._emit_model_call_completed(provider_key, model_id, step, response, call_latency_ms, call_cost)

                # Branch: tool calls
                if response.tool_calls and response.finish_reason in ("tool_calls", "stop"):
                    self._total_tool_calls += len(response.tool_calls)
                    await self._emit_tool_calls_proposed(step, response)

                    # Wait for Go to execute tools and return results.
                    self._tool_results_event.clear()
                    try:
                        await asyncio.wait_for(self._tool_results_event.wait(), timeout=self._tool_result_timeout)
                    except asyncio.TimeoutError:
                        await self._emit_step_completed(step)
                        await self._emit_run_failed("tool_timeout", "timed out waiting for tool results")
                        return

                    batch = self._tool_results
                    self._tool_results = None
                    if batch is None:
                        await self._emit_step_completed(step)
                        await self._emit_run_failed("protocol_error", "tool results were None")
                        return

                    # Emit tool result events and append to messages.
                    messages.append({
                        "role": "assistant",
                        "content": response.output_text or None,
                        "tool_calls": [
                            {"id": tc.id, "type": "function", "function": {"name": tc.name, "arguments": tc.arguments}}
                            for tc in response.tool_calls
                        ],
                    })
                    for result in batch.tool_results:
                        if result.status == "skipped":
                            continue
                        is_error = result.status in ("failed", "blocked")
                        content = result.error_message if is_error else result.content
                        await self._emit_tool_call_event(step, result)
                        messages.append({
                            "role": "tool",
                            "tool_call_id": result.tool_call_id,
                            "content": content or "",
                        })

                    await self._emit_step_completed(step)
                    continue

                # Branch: final answer (no tool calls, has output)
                if not response.tool_calls and response.output_text and response.finish_reason == "stop":
                    validation = validate_final_output(response.output_text, output_schema)
                    if not validation.valid and repair_attempts < MAX_REPAIR_ATTEMPTS:
                        repair_attempts += 1
                        messages.append({"role": "assistant", "content": response.output_text})
                        messages.append({
                            "role": "user",
                            "content": f"Your output did not match the required schema. Error: {validation.error}. Please provide a corrected output.",
                        })
                        await self._emit_step_completed(step)
                        continue

                    await self._emit_step_completed(step)
                    await self._emit_output_finalized(response.output_text)
                    await self._emit_run_completed(response.output_text, "stop")
                    return

                # Branch: terminal failure
                if response.finish_reason == "length":
                    await self._emit_step_completed(step)
                    await self._emit_run_failed("max_tokens", "model output truncated (finish_reason=length)")
                    return

                if response.finish_reason == "content_filter":
                    await self._emit_step_completed(step)
                    await self._emit_run_failed("content_filter", "output blocked by content filter")
                    return

                # Both empty or unexpected
                await self._emit_step_completed(step)
                await self._emit_run_failed("protocol_error", f"unexpected model response: finish_reason={response.finish_reason}, tool_calls={bool(response.tool_calls)}, output_text={bool(response.output_text)}")
                return

            # Loop exhausted
            await self._emit_run_failed("max_iterations", f"reached max_iterations={max_iterations}")

        except CallbackDeliveryError as exc:
            logger.error("callback delivery failed, run aborting: %s", exc)
            try:
                await self._emit_run_failed("bridge_error", f"callback delivery failed: {exc}")
            except Exception:
                pass

    # --- Event emission helpers ---

    def _next_event(self, event_type: str, payload: dict[str, Any], summary: SummaryMetadata | None = None) -> Envelope:
        self._sequence += 1
        return Envelope(
            event_id=make_event_id(self.run_agent_id, event_type, self._sequence),
            run_id=self.run_id,
            run_agent_id=self.run_agent_id,
            event_type=event_type,
            payload=payload,
            summary=summary or SummaryMetadata(),
        )

    async def _emit(self, event: Envelope) -> None:
        await self._emitter.emit(event)

    async def _emit_run_started(self, provider_key: str, model_id: str) -> None:
        await self._emit(self._next_event("system.run.started", {
            "deployment_type": "native",
            "execution_target": "native",
        }, SummaryMetadata(status="started", provider_key=provider_key, provider_model_id=model_id)))

    async def _emit_step_started(self, step: int) -> None:
        await self._emit(self._next_event("system.step.started", {
            "step_index": step,
        }, SummaryMetadata(step_index=step)))

    async def _emit_step_completed(self, step: int) -> None:
        await self._emit(self._next_event("system.step.completed", {
            "step_index": step,
        }, SummaryMetadata(step_index=step)))

    async def _emit_model_call_started(self, provider_key: str, model_id: str, step: int) -> None:
        await self._emit(self._next_event("model.call.started", {
            "provider_key": provider_key,
            "provider_model_id": model_id,
            "step_index": step,
        }, SummaryMetadata(provider_key=provider_key, provider_model_id=model_id, step_index=step)))

    async def _emit_model_call_completed(self, provider_key: str, model_id: str, step: int, response: ModelResponse, latency_ms: float = 0, estimated_cost_usd: float | None = None) -> None:
        tool_calls_payload = [
            {"id": tc.id, "name": tc.name, "arguments": tc.arguments}
            for tc in response.tool_calls
        ]
        await self._emit(self._next_event("model.call.completed", {
            "provider_key": provider_key,
            "provider_model_id": model_id,
            "finish_reason": response.finish_reason,
            "output_text": response.output_text,
            "tool_calls": tool_calls_payload,
            "usage": {
                "input_tokens": response.usage.input_tokens,
                "output_tokens": response.usage.output_tokens,
                "total_tokens": response.usage.total_tokens,
            },
            "latency_ms": latency_ms,
            "estimated_cost_usd": estimated_cost_usd,
            "raw_response": response.raw_response,
        }, SummaryMetadata(provider_key=provider_key, provider_model_id=model_id, step_index=step)))

    async def _emit_model_call_completed_error(self, provider_key: str, model_id: str, step: int, error: str) -> None:
        await self._emit(self._next_event("model.call.completed", {
            "provider_key": provider_key,
            "provider_model_id": model_id,
            "finish_reason": "error",
            "error": error,
        }, SummaryMetadata(provider_key=provider_key, provider_model_id=model_id, step_index=step)))

    async def _emit_tool_calls_proposed(self, step: int, response: ModelResponse) -> None:
        await self._emit(self._next_event("model.tool_calls.proposed", {
            "tool_calls": [
                {"id": tc.id, "name": tc.name, "arguments": tc.arguments}
                for tc in response.tool_calls
            ],
        }, SummaryMetadata(step_index=step)))

    async def _emit_tool_call_event(self, step: int, result: ToolResult) -> None:
        is_error = result.status in ("failed", "blocked")
        event_type = "tool.call.failed" if is_error else "tool.call.completed"
        await self._emit(self._next_event(event_type, {
            "tool_call_id": result.tool_call_id,
            "tool_name": "",
            "status": result.status,
            "content": result.content if not is_error else "",
            "error_message": result.error_message or "",
        }, SummaryMetadata(step_index=step)))

    async def _emit_output_finalized(self, final_output: str) -> None:
        await self._emit(self._next_event("system.output.finalized", {
            "final_output": final_output,
        }))

    async def _emit_run_completed(self, final_output: str, stop_reason: str) -> None:
        await self._emit(self._next_event("system.run.completed", {
            "final_output": final_output,
            "stop_reason": stop_reason,
            "step_count": self._step_count,
            "tool_call_count": self._total_tool_calls,
            "input_tokens": self._total_input_tokens,
            "output_tokens": self._total_output_tokens,
            "total_tokens": self._total_input_tokens + self._total_output_tokens,
            "total_latency_ms": round(self._total_latency_ms, 1),
            "total_estimated_cost_usd": round(self._total_estimated_cost_usd, 6) if self._total_estimated_cost_usd else None,
        }, SummaryMetadata(status="completed")))

    async def _emit_run_failed(self, stop_reason: str, error_message: str) -> None:
        await self._emit(self._next_event("system.run.failed", {
            "stop_reason": stop_reason,
            "error": error_message,
            "step_count": self._step_count,
            "tool_call_count": self._total_tool_calls,
            "input_tokens": self._total_input_tokens,
            "output_tokens": self._total_output_tokens,
            "total_tokens": self._total_input_tokens + self._total_output_tokens,
            "total_latency_ms": round(self._total_latency_ms, 1),
            "total_estimated_cost_usd": round(self._total_estimated_cost_usd, 6) if self._total_estimated_cost_usd else None,
        }, SummaryMetadata(status="failed")))

    def _extract_challenge_input(self) -> str:
        challenge_input_set = self.request.execution_context.get("ChallengeInputSet")
        if not challenge_input_set:
            return ""
        items = challenge_input_set.get("Items", [])
        if not items:
            return challenge_input_set.get("InputKey", "")
        parts = []
        for item in items:
            content = item.get("Content", "")
            if content:
                parts.append(content)
        return "\n".join(parts) if parts else challenge_input_set.get("InputKey", "")
