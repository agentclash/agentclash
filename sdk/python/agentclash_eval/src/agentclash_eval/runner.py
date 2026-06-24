from __future__ import annotations

import argparse
import json
import sys
import uuid
from datetime import datetime, timezone
from pathlib import Path

from agentclash_eval import AgentEvalResult, evaluate
from agentclash_eval._version import __version__
from agentclash_eval.metrics import Contains, ToolCalled
from agentclash_eval.models import ToolCall
from agentclash_eval.report import to_report_dict


def _smoke_cases() -> list[tuple[str, AgentEvalResult, list]]:
    return [
        (
            "smoke_contains",
            AgentEvalResult(input="hello", output="hello world"),
            [Contains("world")],
        ),
        (
            "smoke_tools",
            AgentEvalResult(
                input="refund",
                output="refund issued",
                tool_calls=[
                    ToolCall(name="lookup_order"),
                    ToolCall(name="issue_refund"),
                ],
            ),
            [ToolCalled(["lookup_order", "issue_refund"])],
        ),
    ]


def run_smoke() -> dict:
    cases: list[dict] = []
    failures: list[dict] = []
    passed = 0
    failed = 0
    for case_id, result, metrics in _smoke_cases():
        report = evaluate(result, metrics, case_id=case_id, case_name=case_id)
        payload = to_report_dict(report)
        cases.extend(payload["cases"])
        failures.extend(payload.get("failures", []))
        if report.exit_code == 0:
            passed += 1
        else:
            failed += 1

    exit_code = 1 if failed else 0
    return {
        "schema_version": 1,
        "report_id": f"rpt-{uuid.uuid4().hex[:12]}",
        "generated_at": datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z"),
        "runner": {"name": "agentclash-evals", "version": __version__, "language": "python"},
        "summary": {
            "total": passed + failed,
            "passed": passed,
            "failed": failed,
            "skipped": 0,
            "errored": 0,
            "metric_failures": len(failures),
        },
        "cases": cases,
        "failures": failures,
        "exit_code": exit_code,
    }


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Run local agent eval smoke tests")
    parser.add_argument("--out", required=True, help="Output directory for results.json")
    parser.add_argument("--mode", default="smoke", choices=["smoke"])
    args = parser.parse_args(argv)

    if args.mode != "smoke":
        print(f"unsupported mode: {args.mode}", file=sys.stderr)
        return 2

    payload = run_smoke()
    out_dir = Path(args.out)
    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / "results.json").write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
    return int(payload["exit_code"])


if __name__ == "__main__":
    raise SystemExit(main())
