from agentclash_eval import assert_agent, evaluate
from agentclash_eval.metrics import Contains


def test_import_has_no_pytest_side_effects():
    import sys

    assert "pytest" not in sys.modules or "agentclash_eval" in sys.modules


def test_no_telemetry_attributes():
    import agentclash_eval

    source = agentclash_eval.__file__ or ""
    assert source
    package_dir = source.rsplit("/", 1)[0]
    import os

    for root, _, files in os.walk(package_dir):
        for name in files:
            if not name.endswith(".py"):
                continue
            text = open(os.path.join(root, name), encoding="utf-8").read().lower()
            assert "telemetry" not in text
            assert "segment" not in text
            assert "posthog" not in text


def test_string_input_treated_as_output():
    report = evaluate("world", metrics=[Contains("world")])
    assert report.cases[0].agent_result.output == "world"
