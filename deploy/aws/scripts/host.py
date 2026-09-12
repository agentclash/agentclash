#!/usr/bin/env python3
"""Serialized host operations; output is deliberately limited to status.

Read /etc/agentclash/host.json (0600). All application delivery is installed and
reviewed separately; SSM supplies only the content digest, never shell syntax.
"""

import argparse
import fcntl
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import sys
import subprocess
import time

from common import (
    Refused,
    account_guard,
    aws,
    digest,
    image_ref,
    private_json,
    require,
    run,
    verify_blob,
    write_private,
)
from secret_store import load
from render import render
from database import DATABASES, initialize_sql, grants_sql

ROOT = Path(__file__).resolve().parents[1]
STATE = Path("/var/lib/agentclash")
RUNTIME = Path("/run/agentclash")
SERVICES = ("api", "worker", "terminal")


def manifest_validate(manifest, settings):
    require(
        manifest["account_id"] == settings["account_id"]
        and manifest["region"] == settings["region"],
        "Release destination mismatch",
    )
    require(manifest["platform"] == "linux/amd64", "Wrong release architecture")
    require(
        set(manifest["images"]) == {"api", "worker", "terminal", "app-schema"},
        "Incomplete application images",
    )
    for image in manifest["images"].values():
        require(
            image_ref(image).split("@")[0] == settings["repository"],
            "Image outside approved repository",
        )
    digest(manifest["platform_lock_sha256"])
    require(
        manifest["platform_lock_sha256"]
        == hashlib.sha256((ROOT / "images.lock.json").read_bytes()).hexdigest(),
        "Platform version change requires a separate platform release",
    )


def settings_fingerprint(settings):
    fields = (
        "account_id",
        "region",
        "instance_id",
        "repository",
        "database_endpoint",
        "cache_volume_id",
        "cache_uuid",
        "secrets",
        "delivery",
    )
    return hashlib.sha256(
        json.dumps({key: settings.get(key) for key in fields}, sort_keys=True).encode()
    ).hexdigest()


def runtime_guard(settings):
    require(
        (STATE / "runtime-config-sha256").read_text().strip()
        == settings_fingerprint(settings),
        "Private settings changed; prepare a matching runtime generation",
    )
    generation = Path((STATE / "current-generation").read_text().strip())
    require(
        generation.is_relative_to(RUNTIME) and generation.is_dir(),
        "Runtime secrets are absent after restart",
    )


def compose(*args, data=None, timeout=300):
    require((STATE / "compose.env").is_file(), "Runtime has not been prepared")
    return run(
        [
            "docker",
            "compose",
            "--project-directory",
            str(ROOT),
            "--env-file",
            str(STATE / "compose.env"),
            "-f",
            str(ROOT / "compose.yaml"),
            *args,
        ],
        data=data,
        timeout=timeout,
    )


def fence(closed=True):
    edge = RUNTIME / "edge"
    edge.mkdir(mode=0o755, exist_ok=True)
    target = edge / "fence.caddy"
    temp = edge / "fence.next"
    temp.write_bytes(
        (
            ROOT / "config" / ("fence.closed.caddy" if closed else "fence.open.caddy")
        ).read_bytes()
    )
    temp.chmod(0o644)
    os.replace(temp, target)
    # Updating the file alone is not a write fence. A failed reload must fail.
    ids = compose("ps", "--status", "running", "-q", "caddy").strip()
    if ids:
        compose(
            "exec",
            "-T",
            "caddy",
            "/bin/sh",
            "/opt/secret-exec.sh",
            "caddy",
            "reload",
            "--config",
            "/etc/caddy/Caddyfile",
            "--adapter",
            "caddyfile",
        )


def stopped():
    require(
        not compose("ps", "--status", "running", "-q", *SERVICES).strip(),
        "Application writers must be stopped",
    )


def prepare(settings, release_digest):
    # Never replace a generation under running containers, including platform services.
    if (STATE / "compose.env").exists():
        require(
            not compose("ps", "--status", "running", "-q").strip(),
            "Stop existing services before platform preparation",
        )
    release_digest = digest(release_digest)
    raw = aws(
        settings,
        "s3",
        "cp",
        "s3://" + settings["artifact_bucket"] + "/releases/" + release_digest + ".json",
        "-",
        "--only-show-errors",
    )
    verify_blob(raw, release_digest)
    manifest = json.loads(raw)
    manifest_validate(manifest, settings)
    require((STATE / "cache").is_mount(), "Retained cache filesystem is not mounted")
    require(
        (STATE / "cache/.volume-identity").read_text().strip()
        == settings["cache_volume_id"],
        "Retained cache volume not verified",
    )
    generation, payloads = load(settings)
    try:
        render(generation, payloads)
    except Exception:
        shutil.rmtree(generation)
        raise
    images = {
        name: value["image"]
        for name, value in json.loads((ROOT / "images.lock.json").read_text())[
            "images"
        ].items()
    }
    images.update(manifest["images"])
    content = (
        "SECRETS_ROOT="
        + str(generation)
        + "\n"
        + "".join(
            "IMAGE_" + key.upper().replace("-", "_") + "=" + image_ref(value) + "\n"
            for key, value in sorted(images.items())
        )
    )
    pending = STATE / "compose.pending.env"
    write_private(pending, content)
    os.replace(pending, STATE / "compose.env")
    (STATE / "current-release.json").write_bytes(raw)
    (STATE / "current-release.json").chmod(0o600)
    (STATE / "current-generation").write_text(str(generation))
    (STATE / "runtime-config-sha256").write_text(settings_fingerprint(settings))
    fence()
    compose("config", "--quiet")


def stop_applications():
    # Call admission drain first; the operator records workflow/sandbox reconciliation
    # before this final full fence. Never remove containers before inspecting exit state.
    fence()
    ids = compose("ps", "--all", "-q", *SERVICES).decode().split()
    if not ids:
        stopped()
        return
    marker = STATE / "cleanup-unresolved"
    marker.touch(mode=0o600)
    compose("stop", *SERVICES, timeout=330)
    states = json.loads(run(["docker", "inspect", *ids]))
    require(
        all(
            not c["State"]["Running"]
            and c["State"]["ExitCode"] == 0
            and not c["State"].get("OOMKilled")
            for c in states
        ),
        "Unclean application stop: replacement blocked pending reconciliation",
    )
    marker.unlink()


def wait_for_platform(timeout=180):
    deadline = time.monotonic() + timeout
    while True:
        try:
            compose("run", "--rm", "-T", "--no-deps", "temporal-admin", timeout=15)
            require(
                compose(
                    "run", "--rm", "-T", "--no-deps", "cache-admin", timeout=15
                ).strip()
                == b"PONG",
                "Cache authentication/health failed",
            )
            break
        except (Refused, subprocess.TimeoutExpired):
            require(
                time.monotonic() < deadline,
                "Platform readiness deadline exceeded",
            )
            time.sleep(2)


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "command",
        choices=[
            "prepare",
            "fence",
            "terminal-drain",
            "stop-apps",
            "stop-platform",
            "start-platform",
            "db-init",
            "db-grants",
            "app-schema",
            "temporal-schema",
            "namespace-init",
            "health",
            "deploy",
            "promote",
        ],
    )
    parser.add_argument("argument", nargs="?")
    args = parser.parse_args()
    require(
        os.geteuid() == 0, "Host operations require the platform operator via SSM/root"
    )
    settings = private_json("/etc/agentclash/host.json")
    account_guard(settings)
    STATE.mkdir(mode=0o700, exist_ok=True)
    if args.command != "prepare":
        runtime_guard(settings)
    with open(STATE / "deployment.lock", "a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if args.command == "prepare":
            prepare(settings, args.argument)
        elif args.command == "fence":
            fence()
        elif args.command == "terminal-drain":
            compose("kill", "-s", "SIGUSR2", "terminal")
        elif args.command == "stop-apps":
            stop_applications()
        elif args.command == "stop-platform":
            stopped()
            compose("stop", "caddy", "temporal", "valkey", timeout=360)
        elif args.command == "start-platform":
            require(
                not (STATE / "cleanup-unresolved").exists(), "Unresolved prior cleanup"
            )
            fence()
            compose("up", "-d", "--no-deps", "valkey", "temporal", "caddy", timeout=300)
            wait_for_platform()

        elif args.command in ("db-init", "db-grants"):
            stopped()
            # Administrative secret is re-read under its pinned version and never stored on disk.
            ref = settings["secrets"]["database-admin"]
            payload = json.loads(
                json.loads(
                    aws(
                        settings,
                        "secretsmanager",
                        "get-secret-value",
                        "--secret-id",
                        ref["arn"],
                        "--version-id",
                        ref["version_id"],
                        "--output",
                        "json",
                    )
                )["SecretString"]
            )
            if args.command == "db-init":
                passwords = {
                    role: payload["env"][role.upper() + "_PASSWORD"]
                    for pair in DATABASES.values()
                    for role in pair
                }
                sql = initialize_sql(passwords)
            else:
                require(args.argument in DATABASES, "Choose one database for grants")
                sql = grants_sql(args.argument)
            compose(
                "run",
                "--rm",
                "-T",
                "--no-deps",
                "database-admin",
                "psql",
                "-X",
                "-v",
                "ON_ERROR_STOP=1",
                data=sql.encode(),
            )
        elif args.command == "app-schema":
            stopped()
            compose("run", "--rm", "-T", "--no-deps", "app-schema", timeout=2100)
            compose(
                "run",
                "--rm",
                "-T",
                "--no-deps",
                "database-admin",
                "psql",
                "-X",
                "-v",
                "ON_ERROR_STOP=1",
                data=grants_sql("agentclash").encode(),
            )
        elif args.command == "temporal-schema":
            stopped()
            require(
                not compose("ps", "--status", "running", "-q", "temporal").strip(),
                "Stop Temporal for the initial platform schema job",
            )
            require(
                args.argument in ("init", "upgrade"),
                "Choose explicit schema initialization or upgrade",
            )
            for db in ("temporal", "temporal_visibility"):
                compose(
                    "run",
                    "--rm",
                    "-T",
                    "--no-deps",
                    "temporal-schema",
                    "/bin/sh",
                    "/opt/temporal-schema.sh",
                    db,
                    args.argument,
                    timeout=900,
                )
        elif args.command == "namespace-init":
            stopped()
            compose(
                "run",
                "--rm",
                "-T",
                "--no-deps",
                "temporal-admin",
                "temporal",
                "operator",
                "namespace",
                "create",
                "--namespace",
                "agentclash-prod",
                "--retention",
                "30d",
            )
        elif args.command == "health":
            compose("run", "--rm", "-T", "--no-deps", "temporal-admin")
            require(
                compose("run", "--rm", "-T", "--no-deps", "cache-admin").strip()
                == b"PONG",
                "Cache authentication/health failed",
            )
        elif args.command in ("deploy", "promote"):
            release = digest(args.argument)
            # The fixed driver is imported under this lock. Never launch a child
            # locking host operation or accept a remote script location.
            driver = Path("/opt/agentclash/delivery/release.py")
            require(
                driver.is_file() and "delivery_driver_sha256" in settings,
                "Application delivery has not been activated",
            )
            verify_blob(driver.read_bytes(), settings["delivery_driver_sha256"])
            require(
                not any(
                    (STATE / name).exists()
                    for name in (
                        "cleanup-unresolved",
                        "restore-unverified",
                        "recovery-unverified",
                        "deployment-unresolved.json",
                    )
                ),
                "Unresolved cleanup or restore verification",
            )
            spec = importlib.util.spec_from_file_location("agentclash_delivery", driver)
            module = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(module)
            module.execute(settings, release, args.command)
        print("Host operation passed")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        # Full-fence state/failed-cleanup markers intentionally survive failures.
        sys.exit(
            "Host operation refused or failed; public output contains no configuration"
        )
