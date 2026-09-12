#!/usr/bin/env python3
"""Private host checks -> one bounded CloudWatch health metric, no payload logs."""

from datetime import datetime, timezone
import json
import os
from pathlib import Path
import shutil
import sys

from common import account_guard, aws, private_json, run
from host import runtime_guard, STATE, ROOT, compose


def main():
    os.umask(0o077)
    settings = private_json("/etc/agentclash/host.json")
    account_guard(settings)
    runtime_guard(settings)
    healthy = 1
    try:
        run([sys.executable, str(ROOT / "scripts/host.py"), "health"], timeout=120)
        for marker in (
            "cleanup-unresolved",
            "restore-unverified",
            "recovery-unverified",
        ):
            if (STATE / marker).exists():
                healthy = 0
        backup = private_json(STATE / "last-cache-backup.json")
        if (
            datetime.now(timezone.utc) - datetime.fromisoformat(backup["completed_at"])
        ).total_seconds() > 90000:
            healthy = 0
        generation = Path((STATE / "current-generation").read_text())
        for cert in generation.glob("*/*.crt"):
            run(["openssl", "x509", "-in", str(cert), "-checkend", "2592000", "-noout"])
        if shutil.disk_usage(STATE / "cache").free < 2 * 1024**3:
            healthy = 0
        mem = dict(
            line.split(":", 1)
            for line in Path("/proc/meminfo").read_text().splitlines()
        )
        if int(mem["MemAvailable"].split()[0]) < 1024**2:
            healthy = 0
        raw = compose("ps", "--all", "--format", "json").decode().strip()
        states = (
            json.loads(raw)
            if raw.startswith("[")
            else [json.loads(line) for line in raw.splitlines()]
        )
        expected = {"api", "worker", "terminal", "temporal", "valkey", "caddy"}
        if {x.get("Service") for x in states} & expected != expected:
            healthy = 0
        if any(
            x.get("State") != "running" or x.get("Health") == "unhealthy"
            for x in states
            if x.get("Service")
            in ("api", "worker", "terminal", "temporal", "valkey", "caddy")
        ):
            healthy = 0
    except Exception:
        healthy = 0
    aws(
        settings,
        "cloudwatch",
        "put-metric-data",
        "--namespace",
        "AgentClash/Platform",
        "--metric-data",
        json.dumps(
            [
                {
                    "MetricName": "Healthy",
                    "Dimensions": [
                        {"Name": "InstanceId", "Value": settings["instance_id"]}
                    ],
                    "Value": healthy,
                    "Unit": "Count",
                }
            ]
        ),
    )
    print("Platform health metric submitted")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        sys.exit("Platform check failed; missing metrics are alarmed")
