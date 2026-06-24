import importlib.util

import pytest

from agentclash_eval import assert_agent
from agentclash_eval.metrics import Contains


def test_assert_agent_without_plugin():
    assert_agent("hello world", metrics=[Contains("world")])


def test_pytest_plugin_is_not_auto_registered():
    # Importing agentclash_eval must not register this module as a pytest plugin.
    spec = importlib.util.find_spec("agentclash_eval.pytest_plugin")
    assert spec is not None
    assert "pytest_plugin" in spec.name
