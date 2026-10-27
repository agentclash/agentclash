#!/usr/bin/env python3
"""Own the API fixture process and reconcile only its recorded child databases."""
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import uuid

from fixture import ROOT, deterministic_env, parent_env, record, reconcile, run_owned


def main():
    dsn = os.environ.get("VIBE_TEST_DATABASE_URL", "")
    parent_env(dsn)
    log = os.environ["VIBE_BROWSER_OWNERSHIP_LOG"]
    directory = Path(tempfile.gettempdir()) / ("agentclash-vibe-api-" + uuid.uuid4().hex)
    record("fixture_directory", str(directory), phase="intent")
    directory.mkdir(mode=0o700)
    record("fixture_directory", str(directory), phase="created")
    child = None
    stopping = None

    def stop(signum, _frame):
        nonlocal stopping
        if stopping is None:
            stopping = time.monotonic()
            record("process", "browser-api", phase="signal", signal=signum)
        if child is not None and child.poll() is None:
            child.send_signal(signum)

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    result = 1
    try:
        binary = str(directory / "api.test")
        build = run_owned(["go", "test", "-c", "-o", binary, "./internal/api"], name="browser-api-build", cwd=ROOT / "backend", env=deterministic_env())
        if build.returncode or stopping is not None:
            return build.returncode or 130
        record("process", "browser-api", phase="intent")
        child = subprocess.Popen([binary, "-test.run=^TestVibeBrowserStack$", "-test.count=1", "-test.timeout=12m", "-test.v"], cwd=ROOT / "backend/internal/api", env=deterministic_env(), start_new_session=True)
        record("process", "browser-api", phase="started", child_pid=child.pid, process_group=child.pid)
        while child.poll() is None:
            if stopping is not None and time.monotonic() - stopping > 10:
                record("process", "browser-api", phase="escalated", child_pid=child.pid)
                child.kill()
                break
            time.sleep(0.1)
        result = child.wait()
        record("process", "browser-api", phase="exited", child_pid=child.pid, exit_code=result)
    finally:
        if child is not None:
            if child.poll() is None:
                child.kill()
                child.wait()
            # Descendants of this fresh session include only the fixture's
            # optional Temporal dev server. Shared Temporal is never a child.
            try:
                os.killpg(child.pid, signal.SIGTERM)
                deadline = time.monotonic() + 5
                while True:
                    if time.monotonic() >= deadline:
                        os.killpg(child.pid, signal.SIGKILL)
                        record("process_group", str(child.pid), phase="forced", signal=9)
                        result = result or 1
                        # A killed descendant may remain a zombie until init reaps it.
                        # Do not claim group absence without observation.
                        break
                    time.sleep(0.1)
                    os.killpg(child.pid, 0)
            except ProcessLookupError:
                record("process_group", str(child.pid), phase="absent")
            reconcile(dsn, log, child.pid)
        shutil.rmtree(directory)
        record("fixture_directory", str(directory), phase="absent")
    return result if result >= 0 else 128 - result


if __name__ == "__main__":
    sys.exit(main())
