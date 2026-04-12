"""FastAPI application for the reasoning service."""

from __future__ import annotations

import asyncio
import logging
import uuid
from contextlib import asynccontextmanager
from typing import Any

from fastapi import FastAPI, HTTPException

from reasoning.client.model_client import ModelClient
from reasoning.emitter.callback import CallbackEmitter
from reasoning.engine.react import ReactEngine
from reasoning.models.bridge import CancelRequest, StartRequest, StartResponse, ToolResultsBatch

logger = logging.getLogger(__name__)

# Registry of active reasoning runs keyed by reasoning_run_id.
_runs: dict[str, ReactEngine] = {}
# Idempotency cache for start requests keyed by (run_agent_id, idempotency_key).
_start_cache: dict[tuple[str, str], StartResponse] = {}


@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info("reasoning service starting")
    yield
    # Cleanup: cancel all active runs.
    for engine in _runs.values():
        engine.cancel()
    _runs.clear()
    _start_cache.clear()
    logger.info("reasoning service stopped")


app = FastAPI(title="agentclash-reasoning", version="0.1.0", lifespan=lifespan)


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/reasoning/runs")
async def start_run(request: StartRequest) -> StartResponse:
    cache_key = (str(request.run_agent_id), request.idempotency_key)
    if cache_key in _start_cache:
        return _start_cache[cache_key]

    reasoning_run_id = f"rr-{uuid.uuid4().hex[:16]}"

    # Resolve model credentials from execution context.
    credential = (
        request.execution_context
        .get("Deployment", {})
        .get("ProviderAccount", {})
        .get("_resolved_credential", "")
    )
    provider_key = (
        request.execution_context
        .get("Deployment", {})
        .get("ProviderAccount", {})
        .get("ProviderKey", "")
    )
    base_url = _provider_base_url(provider_key)

    if not credential:
        response = StartResponse(accepted=False, reasoning_run_id="", error="no provider credential in execution context")
        return response

    # Map provider keys to the model client's provider abstraction.
    # OpenAI-compatible providers (together, groq, fireworks) use "openai".
    client_provider = provider_key if provider_key in ("openai", "anthropic", "gemini") else "openai"
    model_client = ModelClient(api_key=credential, base_url=base_url, provider_key=client_provider)
    emitter = CallbackEmitter(callback_url=request.callback_url, callback_token=request.callback_token)

    engine = ReactEngine(request=request, model_client=model_client, emitter=emitter)
    _runs[reasoning_run_id] = engine

    response = StartResponse(accepted=True, reasoning_run_id=reasoning_run_id)
    _start_cache[cache_key] = response

    # Launch engine in background task.
    asyncio.create_task(_run_engine(reasoning_run_id, engine, model_client, emitter))

    return response


@app.post("/reasoning/runs/{reasoning_run_id}/tool-results")
async def submit_tool_results(reasoning_run_id: str, batch: ToolResultsBatch) -> dict[str, bool]:
    engine = _runs.get(reasoning_run_id)
    if engine is None:
        raise HTTPException(status_code=404, detail="reasoning run not found")

    engine.deliver_tool_results(batch)
    return {"accepted": True}


@app.post("/reasoning/runs/{reasoning_run_id}/cancel")
async def cancel_run(reasoning_run_id: str, request: CancelRequest) -> dict[str, bool]:
    engine = _runs.get(reasoning_run_id)
    if engine is None:
        raise HTTPException(status_code=404, detail="reasoning run not found")

    engine.cancel()
    return {"acknowledged": True}


async def _run_engine(reasoning_run_id: str, engine: ReactEngine, model_client: ModelClient, emitter: CallbackEmitter) -> None:
    try:
        await engine.run()
    except Exception:
        logger.exception("reasoning run %s failed with unhandled exception", reasoning_run_id)
    finally:
        _runs.pop(reasoning_run_id, None)
        await model_client.close()
        await emitter.close()


def _provider_base_url(provider_key: str) -> str:
    mapping = {
        "openai": "https://api.openai.com/v1",
        "anthropic": "https://api.anthropic.com/v1",
        "together": "https://api.together.xyz/v1",
        "groq": "https://api.groq.com/openai/v1",
        "fireworks": "https://api.fireworks.ai/inference/v1",
    }
    return mapping.get(provider_key, "https://api.openai.com/v1")
