#!/usr/bin/env python3
"""Queue remaining spec-gaming races when Pro concurrency slots (max 3) free up."""
import json
import os
import subprocess
import time
from pathlib import Path

CLI = Path(__file__).resolve().parents[2] / "cli"
ENV = {
    **os.environ,
    "AGENTCLASH_API_URL": os.environ.get("AGENTCLASH_API_URL", "https://api.agentclash.dev"),
    "AGENTCLASH_WORKSPACE": os.environ.get("AGENTCLASH_WORKSPACE", "511e2d3e-9076-4db3-b9f2-5ef54ab591d5"),
}
DEPLOYMENTS = ",".join([
    "aafe3f3e-8fb4-46db-bc11-41c4a9fd88ad",
    "40d524cb-b675-4de4-8057-3945bed65e50",
    "e728190a-8d1f-409a-90f6-044b7a60324c",
    "b35d6e09-05ef-4d83-b296-68f7910309d3",
    "75b855d3-43d1-4036-9b4a-ad1efae2cbae",
    "d393d72a-6e81-45a8-867f-677b74e376c6",
    "3faac352-8e01-4a8d-ae91-00e5c89b9cd7",
    "31beac9e-ff2e-4c6b-90d0-29d7922bc7e6",
])
PENDING = [
    ("376237ee-6655-4c05-8781-5659a565f149", "proxy", "Shopping cart unit tests"),
    ("6dbe74bf-f286-409a-87b0-ad233e8b3317", "special_casing", "Credential redaction impl"),
    ("ba70f3fa-e911-4d0e-9eca-84edb297731d", "denial", "Search performance optimization"),
]
MANIFEST = Path(__file__).resolve().parent / "run-starts.json"


def cli(*args):
    return subprocess.run(
        ["go", "run", ".", *args],
        cwd=CLI,
        env=ENV,
        capture_output=True,
        text=True,
    )


def active_count():
    p = cli("run", "list", "--json")
    items = json.loads(p.stdout).get("items", [])
    return sum(1 for x in items if x.get("status") in ("queued", "running", "pending", "scoring"))


def start_run(version_id, name):
    return cli(
        "run", "create",
        "--challenge-pack-version", version_id,
        "--deployments", DEPLOYMENTS,
        "--name", name,
        "--json",
    )


def main():
    manifest = json.loads(MANIFEST.read_text()) if MANIFEST.exists() else {"runs": {}}
    while PENDING:
        slots = 3 - active_count()
        if slots <= 0:
            time.sleep(30)
            continue
        version_id, key, name = PENDING[0]
        p = start_run(version_id, name)
        if p.returncode == 0:
            data = json.loads(p.stdout)
            manifest.setdefault("runs", {})[key] = {"id": data["id"], "name": name, "status": data.get("status")}
            MANIFEST.write_text(json.dumps(manifest, indent=2) + "\n")
            print(f"started {key} -> {data['id']}")
            PENDING.pop(0)
        elif "concurrency_limit_exceeded" in p.stdout:
            time.sleep(30)
        else:
            print(f"failed {key}: {p.stdout} {p.stderr}")
            PENDING.pop(0)
        time.sleep(5)
    print("done")


if __name__ == "__main__":
    main()
