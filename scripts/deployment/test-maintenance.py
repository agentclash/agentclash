#!/usr/bin/env python3
"""Exercise maintenance lifecycle against disposable local Temporal/PostgreSQL.

Requires Go, Docker and Python 3. No existing service, cloud credentials, provider
calls or persistent volumes are used. The generated database password is passed
only in child environments and redacted from output.
"""

import os
from pathlib import Path
import secrets
import subprocess
import sys
import time


ROOT = Path(__file__).resolve().parents[2]


def main():
    env = {k: v for k, v in os.environ.items() if k in (
        "PATH", "HOME", "USER", "TMPDIR", "GOPATH", "GOCACHE", "DOCKER_HOST", "DOCKER_CONTEXT",
    )}
    password = secrets.token_hex(24)
    env["POSTGRES_PASSWORD"] = password
    prefix = "agentclash-maintenance-test-" + secrets.token_hex(8)
    postgres, temporal = prefix + "-pg", prefix + "-temporal"

    def run(command, *, child_env=None, cwd=ROOT, quiet=False, timeout=300):
        result = subprocess.run(command, env=child_env or env, cwd=cwd, text=True, capture_output=True, timeout=timeout)
        if not quiet or result.returncode:
            print((result.stdout + result.stderr).replace(password, "[REDACTED]"), end="", flush=True)
        if result.returncode:
            raise RuntimeError(f"{command[0]} failed ({result.returncode})")
        return result.stdout.strip()

    def ready(command):
        for _ in range(80):
            result = subprocess.run(command, env=env, capture_output=True, timeout=10)
            if result.returncode == 0:
                return
            time.sleep(0.5)
        raise RuntimeError("disposable service did not become ready")

    def port(container, container_port):
        template = '{{(index (index .NetworkSettings.Ports "' + container_port + '/tcp") 0).HostPort}}'
        return run(["docker", "inspect", "--format", template, container], quiet=True)

    try:
        print("[maintenance-test] Starting disposable Temporal and PostgreSQL", flush=True)
        run(["docker", "run", "--name", temporal, "--detach", "--rm", "--publish", "127.0.0.1::7233",
             "temporalio/temporal:1.7.2", "server", "start-dev", "--ip", "0.0.0.0"], quiet=True)
        run(["docker", "run", "--name", postgres, "--detach", "--rm", "--publish", "127.0.0.1::5432",
             "--tmpfs", "/var/lib/postgresql", "-e", "POSTGRES_PASSWORD", "-e", "POSTGRES_DB=maintenance_test",
             "postgres:18-alpine"], quiet=True)
        ready(["docker", "exec", temporal, "temporal", "operator", "cluster", "health", "--address", "localhost:7233"])
        ready(["docker", "exec", postgres, "pg_isready", "-U", "postgres", "-d", "maintenance_test"])
        temporal_env = dict(env, MAINTENANCE_TEST_TEMPORAL_ADDRESS="127.0.0.1:" + port(temporal, "7233"))
        run(["go", "test", "-race", "-count=1", "-timeout=90s", "-tags=maintenanceintegration", "-v",
             "-run", "^TestTemporalMaintenanceLifecycle$", "./internal/worker"], child_env=temporal_env, cwd=ROOT / "backend")
        database_env = dict(env, MIGRATION_DIR=str(ROOT / "backend/db/migrations"), DATABASE_URL=f"postgres://postgres:{password}@127.0.0.1:{port(postgres, '5432')}/maintenance_test?sslmode=disable")
        run(["go", "run", "./cmd/db-migrate"], child_env=database_env, cwd=ROOT / "backend")
        run(["go", "test", "-race", "-count=1", "-v", "-run",
             "^TestRepository(BillingWebhookFailureIsRetryable|BillingWebhookIgnoresStaleSubscriptionState|RunEntitlementGateConsumesQuotaAndBlocksOverage|RunEntitlementGateBlocksConcurrentOverage|SetRunTemporalIDsIsIdempotentForSameValues)$",
             "./internal/repository"], child_env=database_env, cwd=ROOT / "backend")
        audit = (ROOT / "scripts/deployment/drain-audit.sql").read_text()
        result = subprocess.run(["docker", "exec", "-i", postgres, "psql", "-X", "-U", "postgres", "-d", "maintenance_test", "-v", "ON_ERROR_STOP=1"],
                                input=audit, text=True, env=env, capture_output=True, timeout=30)
        if result.returncode:
            raise RuntimeError("drain audit SQL failed against the migrated schema")
        print("[maintenance-test] Lifecycle, cleanup, retry and persisted billing checks passed", flush=True)
    finally:
        subprocess.run(["docker", "rm", "--force", temporal, postgres], env=env, capture_output=True, timeout=30)


if __name__ == "__main__":
    try:
        main()
    except subprocess.TimeoutExpired:
        sys.exit("[maintenance-test] A local test command exceeded its deadline")
    except RuntimeError as error:
        sys.exit("[maintenance-test] " + str(error))
