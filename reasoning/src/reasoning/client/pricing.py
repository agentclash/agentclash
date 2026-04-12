"""Static pricing table for cost estimation.

Prices are per million tokens. Add new models as needed.
This is a best-effort estimate -- actual costs depend on provider billing.
"""

from __future__ import annotations

# Prices in USD per million tokens.
PRICING: dict[str, dict[str, float]] = {
    # OpenAI
    "gpt-4o": {"input_per_m": 2.50, "output_per_m": 10.00},
    "gpt-4o-mini": {"input_per_m": 0.15, "output_per_m": 0.60},
    "gpt-4.1": {"input_per_m": 2.00, "output_per_m": 8.00},
    "gpt-4.1-mini": {"input_per_m": 0.40, "output_per_m": 1.60},
    "gpt-4.1-nano": {"input_per_m": 0.10, "output_per_m": 0.40},
    "o3-mini": {"input_per_m": 1.10, "output_per_m": 4.40},
    # Anthropic
    "claude-sonnet-4-20250514": {"input_per_m": 3.00, "output_per_m": 15.00},
    "claude-opus-4-20250514": {"input_per_m": 15.00, "output_per_m": 75.00},
    "claude-haiku-3-5-20241022": {"input_per_m": 0.80, "output_per_m": 4.00},
    # Google
    "gemini-2.0-flash": {"input_per_m": 0.10, "output_per_m": 0.40},
    "gemini-2.5-pro": {"input_per_m": 1.25, "output_per_m": 10.00},
    "gemini-2.5-flash": {"input_per_m": 0.15, "output_per_m": 0.60},
    # Together / open-source
    "meta-llama/Llama-3.3-70B-Instruct-Turbo": {"input_per_m": 0.88, "output_per_m": 0.88},
}


def estimate_cost(model_id: str, input_tokens: int, output_tokens: int) -> float | None:
    """Estimate cost in USD for a model call. Returns None if model not in pricing table."""
    prices = PRICING.get(model_id)
    if not prices:
        return None
    return (input_tokens * prices["input_per_m"] + output_tokens * prices["output_per_m"]) / 1_000_000
