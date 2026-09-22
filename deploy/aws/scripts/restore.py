#!/usr/bin/env python3
"""Restore an offline backup only into an empty, fenced destination.

Requires a private /etc/agentclash/restore-approval.json tied to its exact digest,
account and retained target volume. Never drops a database or overwrites cache.
"""

import fcntl
import json
import os
import shutil
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile

from common import account_guard, aws, digest, private_json, require, verify_blob
from backup import checksum
from host import runtime_guard, STATE, ROOT, compose, fence, stopped
from database import DATABASES, grants_sql


def safe_extract(archive_path, destination):
    with tarfile.open(archive_path) as archive:
        for member in archive.getmembers():
            parts = Path(member.name).parts
            require(
                parts
                and parts[0] in ("dump.rdb", "appendonlydir")
                and ".." not in parts
                and not member.name.startswith("/")
                and (member.isdir() or member.isfile()),
                "Unsafe backup archive",
            )
        archive.extractall(destination, filter="data")


def main():
    os.umask(0o077)
    require(len(sys.argv) == 2, "Exact backup digest required")
    sha = digest(sys.argv[1])
    settings = private_json("/etc/agentclash/host.json")
    account_guard(settings)
    runtime_guard(settings)
    approval = private_json("/etc/agentclash/restore-approval.json")
    require(
        approval
        == {
            "backup_sha256": sha,
            "account_id": settings["account_id"],
            "cache_volume_id": settings["cache_volume_id"],
            "keep_intake_closed": True,
        },
        "Restore approval does not match target",
    )
    with open(STATE / "deployment.lock", "a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        fence()
        stopped()
        require(
            not compose(
                "ps", "--status", "running", "-q", "temporal", "valkey"
            ).strip(),
            "Stop platform data writers",
        )
        raw = aws(
            settings,
            "s3",
            "cp",
            "s3://"
            + settings["artifact_bucket"]
            + "/backups/manifests/"
            + sha
            + ".json",
            "-",
            "--only-show-errors",
        )
        verify_blob(raw, sha)
        manifest = json.loads(raw)
        require(
            manifest["format"] == 1
            and manifest["kind"] == "offline"
            and set(manifest["objects"]) == {*DATABASES, "cache"},
            "Full offline backup required",
        )
        require(
            manifest["images_lock_sha256"] == checksum(ROOT / "images.lock.json"),
            "Restore with the original platform version first",
        )
        require(
            {p.name for p in (STATE / "cache").iterdir()}
            <= {".volume-identity", "lost+found"},
            "Cache destination is not empty",
        )
        for db in DATABASES:
            count = compose(
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
                "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND c.relkind IN ('r','p','m','v','f','S');",
            )
            require(count.strip() == b"0", "Database destination is not empty")
        require(
            all(
                isinstance(obj["bytes"], int) and obj["bytes"] >= 0
                for obj in manifest["objects"].values()
            ),
            "Invalid backup sizes",
        )
        require(
            shutil.disk_usage(STATE).free
            > sum(obj["bytes"] for obj in manifest["objects"].values()) + 2 * 1024**3,
            "Insufficient disk for verified restore staging",
        )
        # A failure leaves the full write fence and marker intact; never auto-start.
        (STATE / "restore-unverified").touch(mode=0o600)
        with tempfile.TemporaryDirectory(prefix="restore-", dir=STATE) as temporary:
            files = {}
            for name, obj in manifest["objects"].items():
                file = Path(temporary) / name
                aws(
                    settings,
                    "s3",
                    "cp",
                    "s3://"
                    + settings["artifact_bucket"]
                    + "/backups/data/"
                    + digest(obj["sha256"]),
                    str(file),
                    "--only-show-errors",
                )
                require(
                    checksum(file) == obj["sha256"]
                    and file.stat().st_size == obj["bytes"],
                    "Backup object integrity failure",
                )
                files[name] = file
            for db, (owner, _) in DATABASES.items():
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
                    "pg_restore",
                    "--exit-on-error",
                    "--no-owner",
                    "--no-acl",
                    "--role",
                    owner,
                    "--dbname",
                    db,
                ]
                with files[db].open("rb") as stream:
                    result = subprocess.run(
                        command,
                        stdin=stream,
                        stdout=subprocess.DEVNULL,
                        stderr=subprocess.DEVNULL,
                        timeout=1800,
                    )
                require(
                    result.returncode == 0,
                    "Database restore failed; do not retry over partial state",
                )
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
                    data=(grants_sql(db) + "ANALYZE;\n").encode(),
                )
            safe_extract(files["cache"], STATE / "cache")
            for path in (STATE / "cache").rglob("*"):
                if path.name != ".volume-identity":
                    os.chown(path, 999, 999)
    print(
        "Restore copied; intake remains fenced pending data, expiry and recovery verification"
    )


if __name__ == "__main__":
    try:
        main()
    except Exception:
        sys.exit(
            "Restore refused or failed; destination remains closed for operator review"
        )
