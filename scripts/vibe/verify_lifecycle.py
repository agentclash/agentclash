#!/usr/bin/env python3
"""Exercise the real browser fixture's failure, child-loss and stop owners."""
import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time
from urllib.request import urlopen

from fixture import ROOT, deterministic_env, identity, parent_env, record, adopt_fixture_descendants, reap_fixture_descendants


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=["failure", "child-loss", "interrupt"])
    args = parser.parse_args()
    env = deterministic_env()
    env["VIBE_BROWSER_STACK"] = "1"
    env.setdefault("VIBE_BROWSER_API_ADDRESS", "127.0.0.1:55441")
    address = env["VIBE_BROWSER_API_ADDRESS"]
    if args.mode == "failure":
        env["VIBE_BROWSER_API_ADDRESS"] = "invalid-fixture-listener"
    # With no external address, the existing SDK fixture owns its dev server.
    # Keep any orphan from a failing supervisor waitable by this proof process.
    adopt_fixture_descendants()
    admin = parent_env(env["VIBE_TEST_DATABASE_URL"])
    ledger = Path(env["VIBE_BROWSER_OWNERSHIP_LOG"])
    record("process", "lifecycle-" + args.mode, phase="intent")
    supervisor = subprocess.Popen([sys.executable, str(ROOT / "scripts/vibe/browser_stack.py")], env=env)
    record("process", "lifecycle-" + args.mode, phase="started", child_pid=supervisor.pid)
    api_pid = None

    def rows():
        return [json.loads(line) for line in ledger.read_text().splitlines() if line.strip()]

    try:
        deadline = time.monotonic() + 180
        while supervisor.poll() is None and time.monotonic() < deadline:
            for row in rows():
                if row.get("pid") == supervisor.pid and row.get("name") == "browser-api" and row.get("phase") == "started":
                    api_pid = row["child_pid"]
            if api_pid and args.mode != "failure":
                try:
                    with urlopen("http://" + address + "/__fixture/ready", timeout=1) as response:
                        if response.status == 200:
                            break
                except OSError:
                    pass
            time.sleep(0.1)
        if not api_pid:
            raise RuntimeError("actual API child did not start")
        if args.mode == "child-loss":
            # Pin the exact live process. The PID alone is not cleanup authority.
            handle = os.pidfd_open(api_pid)
            try:
                status = Path(f"/proc/{api_pid}/status").read_text()
                if f"PPid:\t{supervisor.pid}\n" not in status:
                    raise RuntimeError("API child ownership changed; retained")
                signal.pidfd_send_signal(handle, signal.SIGKILL)
            finally:
                os.close(handle)
        elif args.mode == "interrupt":
            supervisor.send_signal(signal.SIGTERM)
        code = supervisor.wait(timeout=60)
        record("process", "lifecycle-" + args.mode, phase="exited", child_pid=supervisor.pid, exit_code=code)
        if args.mode in {"failure", "child-loss"} and code == 0:
            raise RuntimeError("fixture failure was masked")
        created = [r for r in rows() if r.get("pid") == api_pid and r.get("kind") == "database" and r.get("phase") == "created"]
        if len(created) != 1:
            raise RuntimeError("expected one exact owned database creation")
        for row in created:
            if identity(admin, row["name"]):
                raise RuntimeError("owned child database remains: " + row["name"])
            record("database", row["name"], phase="absence_verified_by_lifecycle_test", oid=row["oid"])
        for row in rows():
            if row.get("pid") == supervisor.pid and row.get("kind") == "fixture_directory" and row.get("phase") == "created" and Path(row["name"]).exists():
                raise RuntimeError("owned fixture directory remains")
        group = [r for r in rows() if r.get("pid") == supervisor.pid and r.get("kind") == "process_group" and r.get("name") == str(api_pid)]
        if not any(r.get("phase") == "absent" for r in group):
            raise RuntimeError("owned API/Temporal process group has no absence receipt")
        print(json.dumps({"mode": args.mode, "fixture_exit": code, "databases_absent": [r["name"] for r in created], "passed": True}))
    finally:
        if supervisor.poll() is None:
            supervisor.terminate()
            supervisor.wait(timeout=60)
        reap_fixture_descendants()
        if api_pid:
            try:
                os.killpg(api_pid, 0)
            except ProcessLookupError:
                record("process_group", str(api_pid), phase="absence_verified_by_lifecycle_test")


if __name__ == "__main__":
    main()
