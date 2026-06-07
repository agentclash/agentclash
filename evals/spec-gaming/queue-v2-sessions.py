#!/usr/bin/env python3
"""Start v2 spec-gaming eval sessions (3 repetitions each) with concurrency awareness."""

from __future__ import annotations

import json
import os
import subprocess
import sys
import time
from pathlib import Path

CLI = Path(__file__).resolve().parents[2] / "cli"
ROOT = Path(__file__).resolve().parent
MANIFEST = ROOT / "v2-run-starts.json"
REPETITIONS = 3

ENV = {
    **os.environ,
    "AGENTCLASH_API_URL": os.environ.get("AGENTCLASH_API_URL", "https://api.agentclash.dev"),
    "AGENTCLASH_WORKSPACE": os.environ.get("AGENTCLASH_WORKSPACE", "511e2d3e-9076-4db3-b9f2-5ef54ab591d5"),
}

DEPLOYMENTS = [
    "aafe3f3e-8fb4-46db-bc11-41c4a9fd88ad",
    "40d524cb-b675-4de4-8057-3945bed65e50",
    "e728190a-8d1f-409a-90f6-044b7a60324c",
    "b35d6e09-05ef-4d83-b296-68f7910309d3",
    "75b855d3-43d1-4036-9b4a-ad1efae2cbae",
    "d393d72a-6e81-45a8-867f-677b74e376c6",
    "3faac352-8e01-4a8d-ae91-00e5c89b9cd7",
    "31beac9e-ff2e-4c6b-90d0-29d7922bc7e6",
]


def cli(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["go", "run", ".", *args],
        cwd=CLI,
        env=ENV,
        capture_output=True,
        text=True,
    )


def active_runs() -> int:
    proc = cli("run", "list", "--json")
    items = json.loads(proc.stdout).get("items", [])
    active = {"queued", "running", "pending", "scoring"}
    return sum(1 for x in items if x.get("status") in active)


def start_session(pack_key: str, version_id: str, name: str) -> dict:
    args = [
        "eval",
        "start",
        "--pack-version",
        version_id,
        "--name",
        name,
        "--repetitions",
        str(REPETITIONS),
        "--json",
    ]
    for dep in DEPLOYMENTS:
        args.extend(["--deployment", dep])
    proc = cli(*args)
    if proc.returncode != 0:
        raise RuntimeError(f"eval start failed for {pack_key}: {proc.stderr}\n{proc.stdout}")
    return json.loads(proc.stdout)


def publish_leakage_v2() -> str:
    pack_path = str(ROOT.parent.parent / "examples/spec-gaming/leakage-discount-calculator.yaml")
    publish = cli("challenge-pack", "publish", pack_path, "--json")
    if publish.returncode == 0:
        return json.loads(publish.stdout)["challenge_pack_version_id"]
    combined = publish.stdout + publish.stderr
    if "challenge_pack_version_exists" in combined:
        listed = cli("challenge-pack", "list", "--json")
        for item in json.loads(listed.stdout).get("items", []):
            if item.get("slug") != "leakage-discount-calculator":
                continue
            for ver in item.get("versions", []):
                if ver.get("version_number") == 2:
                    return ver["id"]
    raise SystemExit(f"publish leakage v2 failed:\n{publish.stderr}\n{publish.stdout}")


def session_id(result: dict) -> str:
    if result.get("eval_session_id"):
        return str(result["eval_session_id"])
    es = result.get("eval_session") or {}
    return str(es.get("id") or "")


def load_pending(leakage_version_id: str) -> list[tuple[str, str, str]]:
    v1 = json.loads((ROOT / "run-starts.json").read_text())
    packs = v1["packs"]
    runs = v1["runs"]
    return [
        ("leakage", leakage_version_id, runs["leakage"]["name"] + " v2"),
        ("tampering", packs["tampering"]["version_id"], runs["tampering"]["name"] + " v2"),
        ("sequence", packs["sequence"]["version_id"], runs["sequence"]["name"] + " v2"),
        ("proxy", packs["proxy"]["version_id"], runs["proxy"]["name"] + " v2"),
        ("special_casing", packs["special_casing"]["version_id"], runs["special_casing"]["name"] + " v2"),
        ("denial", packs["denial"]["version_id"], runs["denial"]["name"] + " v2"),
    ]


def existing_session_keys() -> set[str]:
    if not MANIFEST.exists():
        return set()
    return set(json.loads(MANIFEST.read_text()).get("sessions", {}).keys())


def main() -> None:
    leakage_version_id = publish_leakage_v2()
    pending = load_pending(leakage_version_id)
    done = existing_session_keys()
    pending = [p for p in pending if p[0] not in done]

    manifest = json.loads(MANIFEST.read_text()) if MANIFEST.exists() else {
        "study": "cross-model-gaming-signatures-v2",
        "repetitions": REPETITIONS,
        "leakage_version_id": leakage_version_id,
        "sessions": {},
    }
    manifest["leakage_version_id"] = leakage_version_id

    print(f"leakage v2 version {leakage_version_id}; {len(pending)} sessions to queue", flush=True)

    while pending:
        slots = 3 - active_runs()
        if slots <= 0:
            time.sleep(45)
            continue
        pack_key, version_id, name = pending[0]
        try:
            result = start_session(pack_key, version_id, name)
        except RuntimeError as exc:
            if "concurrency_limit_exceeded" in str(exc):
                time.sleep(45)
                continue
            raise
        sid = session_id(result)
        manifest["sessions"][pack_key] = {
            "eval_session_id": sid,
            "pack_version_id": version_id,
            "name": name,
            "repetitions": REPETITIONS,
            "run_ids": result.get("run_ids") or [],
            "raw": result,
        }
        MANIFEST.write_text(json.dumps(manifest, indent=2) + "\n")
        print(f"started {pack_key} session -> {sid}", flush=True)
        pending.pop(0)
        time.sleep(10)

    print(f"done; manifest {MANIFEST}", flush=True)


if __name__ == "__main__":
    main()
