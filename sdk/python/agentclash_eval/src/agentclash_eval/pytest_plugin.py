from __future__ import annotations

import json
import os
from pathlib import Path

import pytest

from agentclash_eval.evaluate import EvalAssertionError
from agentclash_eval.report import to_report_dict


@pytest.hookimpl(tryfirst=True, hookwrapper=True)
def pytest_runtest_makereport(item, call):
    outcome = yield
    report = outcome.get_result()
    if report.when != "call":
        return
    exc = call.excinfo
    if exc is None:
        return
    if exc.errisinstance(EvalAssertionError):
        eval_report = exc.value.report
        item.user_properties.append(("agentclash_eval_report", to_report_dict(eval_report)))


def pytest_sessionfinish(session, exitstatus):  # noqa: ARG001
    output = os.environ.get("AGENTCLASH_EVAL_REPORT")
    if not output:
        return
    reports = []
    for item in session.items:
        for key, value in getattr(item, "user_properties", []):
            if key == "agentclash_eval_report":
                reports.append(value)
    if not reports:
        return
    payload = reports[-1] if len(reports) == 1 else {"cases": reports}
    path = Path(output)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
