#!/usr/bin/env python3
"""Encrypted private content-addressed backups. Online mode backs up cache only.

Offline mode requires all writers stopped and produces three logical DB dumps
plus the exact Valkey AOF set. RDS automated backup/PITR is the online DB backup.
"""

import argparse
from datetime import datetime, timezone
import fcntl
import hashlib
import json
import os
import shutil
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import time

from common import account_guard, aws, private_json, require
from host import runtime_guard, ROOT, STATE, compose, stopped
from database import DATABASES


def checksum(path):
    with Path(path).open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def put_file(settings, path):
    sha = checksum(path)
    aws(
        settings,
        "s3",
        "cp",
        str(path),
        "s3://" + settings["artifact_bucket"] + "/backups/data/" + sha,
        "--sse",
        "AES256",
        "--only-show-errors",
    )
    return {"sha256": sha, "bytes": Path(path).stat().st_size}


def backup(settings, kind):
    require(kind in ("cache", "offline"), "Unknown backup type")
    if kind == "offline":
        stopped()
        require(
            not compose(
                "ps", "--status", "running", "-q", "temporal", "valkey"
            ).strip(),
            "Stop Temporal and Valkey before offline backup",
        )
    else:
        cli = [
            "run",
            "--rm",
            "-T",
            "--no-deps",
            "cache-admin",
            "valkey-cli",
            "--tls",
            "--cacert",
            "/run/secrets/ca.pem",
            "--user",
            "admin",
            "-h",
            "valkey",
            "--raw",
        ]
        before = int(compose(*cli, "LASTSAVE").strip())
        time.sleep(1.1)  # LASTSAVE has second precision.
        require(
            compose(*cli, "BGSAVE").strip() == b"Background saving started",
            "Cache snapshot did not start",
        )
        deadline = time.monotonic() + 120
        while time.monotonic() < deadline:
            if int(compose(*cli, "LASTSAVE").strip()) > before:
                break
            time.sleep(1)
        else:
            raise RuntimeError("Cache snapshot timed out")
    require(
        (STATE / "cache/.volume-identity").read_text().strip()
        == settings["cache_volume_id"],
        "Cache identity mismatch",
    )
    with tempfile.TemporaryDirectory(prefix="backup-", dir=STATE) as temporary:
        directory = Path(temporary)
        manifest = {
            "format": 1,
            "kind": kind,
            "created_at": datetime.now(timezone.utc).isoformat(),
            "images_lock_sha256": checksum(ROOT / "images.lock.json"),
            "objects": {},
            "limits_policy": "Keep paid/trial intake closed until reconciled or the full restriction window has expired",
        }
        cache = directory / "cache.tar"
        with tarfile.open(cache, "w") as archive:
            for name in (
                ["dump.rdb", "appendonlydir"] if kind == "offline" else ["dump.rdb"]
            ):
                archive.add(STATE / "cache" / name, arcname=name)
        manifest["objects"]["cache"] = put_file(settings, cache)
        if kind == "offline":
            for db in DATABASES:
                estimated = int(
                    compose(
                        "run",
                        "--rm",
                        "-T",
                        "--no-deps",
                        "database-admin",
                        "psql",
                        "-X",
                        "-At",
                        "-d",
                        db,
                        "-c",
                        "SELECT pg_database_size(current_database());",
                    ).strip()
                )
                require(
                    shutil.disk_usage(directory).free > estimated * 1.2 + 2 * 1024**3,
                    "Insufficient staging disk for a database backup",
                )
                dump = directory / (db + ".dump")
                with dump.open("wb") as stream:
                    command = [
                        "docker",
                        "compose",
                        "--project-directory",
                        str(ROOT),
                        "--env-file",
                        str(STATE / "compose.env"),
                        "-f",
                        str(ROOT / "compose.yaml"),
                        "run",
                        "--rm",
                        "-T",
                        "--no-deps",
                        "database-admin",
                        "pg_dump",
                        "--format=custom",
                        "--no-owner",
                        "--no-acl",
                        "--dbname",
                        db,
                    ]
                    result = subprocess.run(
                        command, stdout=stream, stderr=subprocess.DEVNULL, timeout=1800
                    )
                require(result.returncode == 0, "Database backup failed")
                manifest["objects"][db] = put_file(settings, dump)
                dump.unlink()
        path = directory / "manifest.json"
        path.write_text(json.dumps(manifest, sort_keys=True) + "\n")
        sha = checksum(path)
        aws(
            settings,
            "s3",
            "cp",
            str(path),
            "s3://"
            + settings["artifact_bucket"]
            + "/backups/manifests/"
            + sha
            + ".json",
            "--sse",
            "AES256",
            "--only-show-errors",
        )
        record = STATE / ("last-" + kind + "-backup.json")
        record.write_text(
            json.dumps(
                {"sha256": sha, "completed_at": datetime.now(timezone.utc).isoformat()}
            )
            + "\n"
        )
        record.chmod(0o600)


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument("kind", choices=["cache", "offline"])
    args = parser.parse_args()
    settings = private_json("/etc/agentclash/host.json")
    account_guard(settings)
    runtime_guard(settings)
    with open(STATE / "deployment.lock", "a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        backup(settings, args.kind)
    print("Private backup verified and recorded")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        sys.exit(
            "Backup refused or failed; last successful backup marker was not advanced"
        )
