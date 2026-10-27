"""Bounded, exact-owned local database fixtures. Never clean by prefix or scan."""
import argparse
from contextlib import contextmanager
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time
import uuid
from urllib.parse import urlsplit, urlunsplit

from db_env import connection_env

ROOT = Path(__file__).resolve().parents[2]


def record(kind, name, **details):
    path = os.environ.get("VIBE_BROWSER_OWNERSHIP_LOG")
    if not path:
        raise ValueError("VIBE_BROWSER_OWNERSHIP_LOG is required for owned fixtures")
    with open(path, "a", opener=lambda p, flags: os.open(p, flags, 0o600)) as out:
        json.dump(dict(kind=kind, name=name, pid=os.getpid(), time=datetime.now(timezone.utc).isoformat(), **details), out)
        out.write("\n")
        out.flush()
        os.fsync(out.fileno())


def parent_env(dsn):
    env = connection_env(dsn)
    if not re.fullmatch(r"vibe_test(?:_[a-zA-Z0-9_]+)?", env["PGDATABASE"]):
        raise ValueError("explicit local vibe_test fixture database required")
    return env


def sql(env, statement):
    result = subprocess.run(["psql", "-X", "-A", "-t", "-v", "ON_ERROR_STOP=1"],
                            input=statement, text=True, env=env, capture_output=True, timeout=20)
    if result.returncode:
        # psql diagnostics can contain connection credentials; never forward them.
        raise RuntimeError("local fixture database command failed (exit %s)" % result.returncode)
    return result.stdout.strip()


def identity(env, name):
    if not re.fullmatch(r"vibe_test_[a-zA-Z0-9_]+", name):
        raise ValueError("invalid owned child name")
    return sql(env, "SELECT oid::text FROM pg_database WHERE datname='%s';" % name)


def drop_owned(env, name, oid):
    if name == env["PGDATABASE"] or not oid or not str(oid).isdigit():
        raise ValueError("missing child ownership")
    current = identity(env, name)
    if current and current != str(oid):
        raise ValueError("child database identity changed; retained")
    if current:
        try:
            sql(env, 'DROP DATABASE "%s" WITH (FORCE);' % name)
        except RuntimeError:
            # The Go fixture can finish its already-started DROP after its
            # process exits. Only exact absence can resolve that race; any
            # surviving/replaced identity preserves the original error.
            if identity(env, name):
                raise
    if identity(env, name):
        raise RuntimeError("owned database remains")
    record("database", name, phase="absent", oid=str(oid))


@contextmanager
def database(dsn, label="suite"):
    env = parent_env(dsn)
    name = "vibe_test_%s_%s" % (re.sub(r"[^a-z0-9]", "", label)[:12], uuid.uuid4().hex)
    record("database", name, phase="intent")
    if identity(env, name):
        raise ValueError("owned child name already exists")
    sql(env, 'CREATE DATABASE "%s";' % name)
    oid = identity(env, name)
    record("database", name, phase="created", oid=oid)
    child_env = dict(env, PGDATABASE=name)
    parsed = urlsplit(dsn)
    child_dsn = urlunsplit(parsed._replace(path="/" + name))
    try:
        migrations = sorted((ROOT / "backend/db/migrations").glob("*.sql"))
        if not migrations:
            raise RuntimeError("fixture migrations unavailable")
        for path in migrations:
            sql(child_env, path.read_text().split("-- +goose Down")[0])
        yield child_dsn
    finally:
        drop_owned(env, name, oid)


def deterministic_env():
    env = dict(os.environ)
    for key in list(env):
        if key.startswith("VIBE_LIVE_") or key in {"DATABASE_URL", "TEST_DATABASE_URL", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "OPENROUTER_API_KEY"}:
            env.pop(key)
    return env


def run_owned(command, *, name, **kwargs):
    """Wait/reap the exact process group we start, including on interruption."""
    record("process", name, phase="intent")
    child = None
    interrupted = None
    handlers = {}

    def stop(signum, _frame):
        nonlocal interrupted
        if interrupted is None:
            interrupted = (signum, time.monotonic())
            record("process", name, phase="signal", signal=signum)
        if child is not None and child.poll() is None:
            os.killpg(child.pid, signum)

    try:
        for sig in (signal.SIGINT, signal.SIGTERM):
            handlers[sig] = signal.signal(sig, stop)
        child = subprocess.Popen(command, start_new_session=True, **kwargs)
        record("process", name, phase="started", child_pid=child.pid)
        while True:
            try:
                stdout, stderr = child.communicate(timeout=0.2)
                break
            except subprocess.TimeoutExpired:
                if interrupted and time.monotonic() - interrupted[1] > 10:
                    os.killpg(child.pid, signal.SIGKILL)
        code = child.returncode
        if interrupted and code == 0:
            code = 128 + interrupted[0]
        record("process", name, phase="exited", child_pid=child.pid, exit_code=code)
        return subprocess.CompletedProcess(command, code, stdout, stderr)
    finally:
        if child is not None and child.poll() is None:
            os.killpg(child.pid, signal.SIGKILL)
            child.wait()
        for sig, handler in handlers.items():
            signal.signal(sig, handler)


def reconcile(dsn, log, owner_pid):
    """Only successful creation receipts from this exact child process qualify."""
    env = parent_env(dsn)
    records = [json.loads(line) for line in Path(log).read_text().splitlines() if line.strip()]
    for row in records:
        if row.get("pid") == owner_pid and row.get("kind") == "database" and row.get("phase") == "created":
            drop_owned(env, row["name"], row["oid"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--database-env", choices=["VIBE_TEST_DATABASE_URL", "DATABASE_URL"], default="VIBE_TEST_DATABASE_URL")
    parser.add_argument("--label", default="suite", help="Owned child name label; a fresh UUID is always appended")
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command[1:] if args.command[:1] == ["--"] else args.command
    if not command:
        parser.error("command required")
    with database(os.environ.get("VIBE_TEST_DATABASE_URL", ""), label=args.label) as dsn:
        env = deterministic_env()
        env[args.database_env] = dsn
        result = run_owned(command, name="isolated-suite", env=env)
    return result.returncode


if __name__ == "__main__":
    sys.exit(main())
