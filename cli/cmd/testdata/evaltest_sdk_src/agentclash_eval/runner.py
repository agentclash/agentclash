from __future__ import annotations

import argparse
import json
from pathlib import Path


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--out", required=True)
    parser.add_argument("--mode", default="smoke")
    args = parser.parse_args()

    out_dir = Path(args.out)
    out_dir.mkdir(parents=True, exist_ok=True)
    report = {
        "schema_version": 1,
        "report_id": "rpt-test",
        "generated_at": "2026-06-24T12:00:00Z",
        "runner": {"name": "agentclash-evaltest", "version": "0.0.0-test", "language": "python"},
        "summary": {
            "total": 1,
            "passed": 1,
            "failed": 0,
            "skipped": 0,
            "errored": 0,
            "metric_failures": 0,
        },
        "cases": [
            {
                "case": {"case_id": "smoke", "name": "smoke", "input": "hello"},
                "status": "passed",
                "metrics": [
                    {
                        "key": "contains",
                        "name": "Contains",
                        "passed": True,
                        "reason": "output contains expected text",
                    }
                ],
                "agent_result": {"input": "hello", "output": "hello world"},
                "duration_ms": 1.0,
            }
        ],
        "failures": [],
        "exit_code": 0,
    }
    (out_dir / "results.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
