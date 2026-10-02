#!/usr/bin/env python3
"""Exercise the production Vibe worker factory with disposable local services."""

import json
import os
from pathlib import Path
import secrets
import subprocess
import time

ROOT = Path(__file__).resolve().parents[3]


def main():
    env = {key: value for key, value in os.environ.items() if key in (
        "PATH", "HOME", "USER", "TMPDIR", "GOTMPDIR", "GOPATH", "GOCACHE", "DOCKER_HOST", "DOCKER_CONTEXT",
    )}
    password = secrets.token_hex(24)
    env["POSTGRES_PASSWORD"] = password
    prefix = "agentclash-vibe-worker-" + secrets.token_hex(6)
    postgres, temporal = prefix + "-pg", prefix + "-temporal"
    images = json.loads((ROOT / "deploy/aws/images.lock.json").read_text())["images"]

    def run(command, *, child_env=None, timeout=300):
        result = subprocess.run(command, env=child_env or env, capture_output=True, timeout=timeout)
        if result.returncode:
            # Only synthetic fixture credentials exist; preserve diagnostics
            # while preventing their accidental appearance in CI output.
            print((result.stdout + result.stderr).decode().replace(password, "[REDACTED]"), flush=True)
            raise RuntimeError("Local Vibe worker rehearsal failed")
        return result.stdout

    def ready(container, command):
        for _ in range(90):
            result = subprocess.run(["docker", "exec", container, *command], env=env, capture_output=True, timeout=10)
            if not result.returncode:
                return
            time.sleep(.5)
        raise RuntimeError("Local Vibe service readiness deadline exceeded")

    def port(container, number):
        return json.loads(run(["docker", "inspect", container]))[0]["NetworkSettings"]["Ports"][str(number) + "/tcp"][0]["HostPort"]

    try:
        run(["docker", "run", "--rm", "-d", "--name", temporal, "-p", "127.0.0.1::7233", "--entrypoint", "temporal", images["temporal-admin"]["image"], "server", "start-dev", "--ip", "0.0.0.0"])
        run(["docker", "run", "--rm", "-d", "--name", postgres, "-p", "127.0.0.1::5432", "--tmpfs", "/var/lib/postgresql", "-e", "POSTGRES_PASSWORD", "-e", "POSTGRES_DB=vibe_test_control", images["postgres"]["image"]])
        ready(temporal, ["temporal", "operator", "cluster", "health", "--address", "localhost:7233"])
        ready(postgres, ["pg_isready", "-U", "postgres", "-d", "vibe_test_control"])
        child = dict(env,
            VIBE_WORKER_TEST_TEMPORAL_ADDRESS="127.0.0.1:" + port(temporal, 7233),
            VIBE_WORKER_TEST_DATABASE_URL="postgres://postgres:" + password + "@127.0.0.1:" + port(postgres, 5432) + "/vibe_test_control?sslmode=disable",
        )
        # Selecting this test without its required services fails; it cannot
        # silently skip and turn absent integration coverage into a CI pass.
        output = run(["go", "test", "-race", "-count=1", "-timeout=100s", "-tags=maintenanceintegration", "-v", "-run", "^TestVibeProductionWorkerLifecycle$", "./internal/worker"], child_env=child, timeout=180)
        print(output.decode().replace(password, "[REDACTED]"), end="", flush=True)
    finally:
        subprocess.run(["docker", "rm", "--force", temporal, postgres], env=env, capture_output=True, timeout=30)


if __name__ == "__main__":
    os.chdir(ROOT / "backend")
    main()
