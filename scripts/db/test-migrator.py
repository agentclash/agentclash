#!/usr/bin/env python3
"""Run migration integration/image tests on an isolated, disposable PostgreSQL.

Requires Docker, Go and Python 3. No existing database or provider credentials
are used. Synthetic passwords live only in child environments and are redacted
from command output. The temporary container and image are removed on exit.
"""

import argparse
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import tempfile
import time


ROOT = Path(__file__).resolve().parents[2]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--skip-image", action="store_true", help="run only the PostgreSQL integration suite")
    parser.add_argument("--prior-repository-test-binary", type=Path, help="also verify a previously compiled repository test binary")
    args = parser.parse_args()
    password = secrets.token_hex(24)
    env = {k: v for k, v in os.environ.items() if not k.startswith(("PG", "MIGRATION", "MIGRATOR_TEST")) and k != "DATABASE_URL"}
    env["POSTGRES_PASSWORD"] = password
    image = "agentclash-migrator-test:" + secrets.token_hex(8)
    container = "agentclash-migrator-test-" + secrets.token_hex(8)
    signal_container = container + "-signal"

    def run(command, *, child_env=None, expected=0, contains=None, quiet=False, cwd=ROOT, timeout=600):
        result = subprocess.run(command, env=child_env or env, cwd=cwd, text=True, capture_output=True, timeout=timeout)
        output = (result.stdout + result.stderr).replace(password, "[REDACTED]")
        if not quiet or result.returncode != expected:
            print(output, end="", flush=True)
        if result.returncode != expected:
            raise RuntimeError(f"{command[0]} exited {result.returncode}, expected {expected}")
        if contains is not None and contains not in output:
            raise RuntimeError("test command did not report the expected outcome")
        return result.stdout.strip()

    try:
        run(["docker", "info"], quiet=True, timeout=30)
        print("[migrator-test] Starting disposable PostgreSQL 18", flush=True)
        run([
            "docker", "run", "--name", container, "--detach", "--rm", "--publish", "127.0.0.1::5432",
            "--tmpfs", "/var/lib/postgresql", "-e", "POSTGRES_PASSWORD",
            "-e", "POSTGRES_DB=migrator_test_control", "postgres:18-alpine",
        ], quiet=True)
        for _ in range(60):
            ready = subprocess.run(["docker", "exec", container, "pg_isready", "-U", "postgres", "-d", "migrator_test_control"], env=env, capture_output=True, timeout=10)
            if ready.returncode == 0:
                break
            time.sleep(0.5)
        else:
            raise RuntimeError("disposable PostgreSQL did not become ready")
        port = run(["docker", "inspect", "--format", '{{(index (index .NetworkSettings.Ports "5432/tcp") 0).HostPort}}', container], quiet=True)
        test_url = f"postgres://postgres:{password}@127.0.0.1:{port}/migrator_test_control?sslmode=disable"
        test_env = dict(env, MIGRATOR_TEST_DATABASE_URL=test_url)
        run(["go", "test", "-race", "-count=1", "-tags=migrationintegration", "-v", "./internal/dbmigrate"], child_env=test_env, cwd=ROOT / "backend")
        if args.skip_image:
            return

        print("[migrator-test] Building and exercising the linux/amd64 migrator image", flush=True)
        run(["docker", "build", "--platform", "linux/amd64", "--tag", image, "--file", "backend/Dockerfile.migrator", "."])
        metadata = json.loads(run(["docker", "image", "inspect", "--format", "{{json .Config}}", image], quiet=True))
        if metadata["User"] != "65532:65532" or any(item.startswith("DATABASE_URL=") for item in metadata.get("Env", [])):
            raise RuntimeError("migrator image user/environment contract failed")
        image_env = dict(env, DATABASE_URL=f"postgres://postgres:{password}@127.0.0.1:5432/migrator_test_control?sslmode=disable")
        docker_run = ["docker", "run", "--rm", "--platform", "linux/amd64", "--network", "container:" + container, "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "-e", "DATABASE_URL"]

        def query(sql):
            return run(["docker", "exec", container, "psql", "-X", "-U", "postgres", "-d", "migrator_test_control", "-Atqc", sql], quiet=True)

        run(docker_run + [image], child_env=image_env)
        ledger_query = "SELECT md5(string_agg(version || applied_at::text, ',' ORDER BY version)) FROM schema_migrations"
        before = query(ledger_query)
        run(docker_run + [image], child_env=image_env)
        if query(ledger_query) != before:
            raise RuntimeError("repeat image run changed the ledger")
        with tempfile.TemporaryDirectory(prefix="agentclash-migrator-failure-") as scratch:
            migrations = Path(scratch) / "migrations"
            migrations.mkdir(mode=0o755)
            # CI keeps evidence private with umask 077. This directory contains
            # only synthetic SQL and is bind-mounted for the non-root migrator.
            migrations.chmod(0o755)
            fixture = migrations / "99999_rollback_probe.sql"
            fixture.write_text("-- +goose Up\nCREATE TABLE rollback_probe(id integer); SELECT 1 / 0;\n-- +goose Down\nDROP TABLE rollback_probe;\n")
            fixture.chmod(0o644)
            run(docker_run + ["--mount", f"type=bind,source={migrations},target=/migrations,readonly", image], child_env=image_env, expected=1, contains="SQLSTATE 22012")
            if query("SELECT to_regclass('public.rollback_probe') IS NULL AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version='99999_rollback_probe')") != "t":
                raise RuntimeError("failed image run left schema or ledger changes")

            fixture.write_text("-- +goose Up\nCREATE TABLE rollback_probe(id integer); SELECT pg_sleep(30);\n-- +goose Down\nDROP TABLE rollback_probe;\n")
            process = subprocess.Popen(docker_run + ["--name", signal_container, "--mount", f"type=bind,source={migrations},target=/migrations,readonly", image], env=image_env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            try:
                for _ in range(100):
                    if query("SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND application_name='agentclash-migrator' AND wait_event='PgSleep')") == "t":
                        break
                    if process.poll() is not None:
                        raise RuntimeError("signal-test migrator exited before the slow query")
                    time.sleep(0.1)
                else:
                    raise RuntimeError("signal-test migrator did not reach the slow query")
                run(["docker", "kill", "--signal=TERM", signal_container], quiet=True)
                stdout, stderr = process.communicate(timeout=15)
                print((stdout + stderr).replace(password, "[REDACTED]"), end="", flush=True)
                if process.returncode != 1:
                    raise RuntimeError("SIGTERM did not produce a failed migration exit")
            finally:
                if process.poll() is None:
                    subprocess.run(["docker", "rm", "--force", signal_container], env=env, capture_output=True, timeout=30)
                    process.communicate(timeout=15)
            deadline_env = dict(image_env, MIGRATION_TIMEOUT="1s")
            run(docker_run + ["-e", "MIGRATION_TIMEOUT", "--mount", f"type=bind,source={migrations},target=/migrations,readonly", image], child_env=deadline_env, expected=1, contains="apply 99999_rollback_probe: timed out")
            if query("SELECT to_regclass('public.rollback_probe') IS NULL AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version='99999_rollback_probe')") != "t":
                raise RuntimeError("interrupted image run left schema or ledger changes")

        local_env = dict(env, DATABASE_URL=test_url)
        run(["make", "--no-print-directory", "db-migrate"], child_env=local_env)
        if query(ledger_query) != before:
            raise RuntimeError("local wrapper changed the existing ledger")
        tests = "^TestRepository(GetRunByID|CreateToolRestoresArchivedSlug|ListVisibleChallengePacksRespectsPublicPacksOptIn)$"
        if args.prior_repository_test_binary:
            run([str(args.prior_repository_test_binary.resolve()), "-test.v", "-test.run", tests], child_env=local_env)
        else:
            run(["go", "test", "-count=1", "-v", "-run", tests, "./internal/repository"], child_env=local_env, cwd=ROOT / "backend")
        print("[migrator-test] Database, image, rollback, wrapper and application compatibility checks passed", flush=True)
    finally:
        subprocess.run(["docker", "rm", "--force", signal_container, container], env=env, capture_output=True, timeout=30)
        subprocess.run(["docker", "image", "rm", image], env=env, capture_output=True, timeout=30)


if __name__ == "__main__":
    try:
        main()
    except subprocess.TimeoutExpired:
        sys.exit("[migrator-test] A test command exceeded its deadline")
    except RuntimeError as error:
        sys.exit("[migrator-test] " + str(error))
