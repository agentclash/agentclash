"""Tests for cost estimation."""

from reasoning.client.pricing import estimate_cost


def test_estimate_cost_known_model():
    # gpt-4o: input=2.50/M, output=10.00/M
    cost = estimate_cost("gpt-4o", input_tokens=1000, output_tokens=500)
    assert cost is not None
    expected = (1000 * 2.50 + 500 * 10.00) / 1_000_000
    assert abs(cost - expected) < 1e-10


def test_estimate_cost_unknown_model():
    cost = estimate_cost("some-unknown-model", input_tokens=1000, output_tokens=500)
    assert cost is None


def test_estimate_cost_zero_tokens():
    cost = estimate_cost("gpt-4o", input_tokens=0, output_tokens=0)
    assert cost == 0.0


def test_estimate_cost_anthropic_model():
    cost = estimate_cost("claude-sonnet-4-20250514", input_tokens=1000, output_tokens=500)
    assert cost is not None
    expected = (1000 * 3.00 + 500 * 15.00) / 1_000_000
    assert abs(cost - expected) < 1e-10


def test_estimate_cost_gemini_model():
    cost = estimate_cost("gemini-2.0-flash", input_tokens=10000, output_tokens=5000)
    assert cost is not None
    expected = (10000 * 0.10 + 5000 * 0.40) / 1_000_000
    assert abs(cost - expected) < 1e-10
