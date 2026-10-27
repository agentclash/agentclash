#!/usr/bin/env python3
"""Explicit new-volume initialization or existing-volume mount; never format on boot."""

import argparse
import fcntl
import json
import os
from pathlib import Path
import sys

from common import account_guard, private_json, require, run


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=["initialize", "mount"])
    args = parser.parse_args()
    require(os.geteuid() == 0, "Root operator required")
    settings = private_json("/etc/agentclash/host.json")
    account_guard(settings)
    with open("/var/lib/agentclash/deployment.lock", "a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        operate(settings, args.command)


def operate(settings, command):
    volume_id = settings["cache_volume_id"]
    require(
        volume_id.startswith("vol-")
        and all(c in "0123456789abcdef" for c in volume_id[4:]),
        "Invalid cache volume reference",
    )
    disks = json.loads(run(["lsblk", "--json", "--nodeps", "--output", "PATH,SERIAL"]))[
        "blockdevices"
    ]
    matches = [
        disk["path"]
        for disk in disks
        if (disk.get("serial") or "").replace("-", "") == volume_id.replace("-", "")
    ]
    require(
        len(matches) == 1, "Expected retained EBS volume not attached unambiguously"
    )
    device = matches[0]
    signatures = json.loads(run(["wipefs", "--no-act", "--json", device]))["signatures"]
    if command == "initialize":
        require(
            not signatures and not settings.get("cache_uuid"),
            "Refusing to format existing or previously identified state",
        )
        run(["mkfs.ext4", "-m", "0", device])
    else:
        require(
            settings.get("cache_uuid"), "Recorded filesystem UUID required for rebuild"
        )
    uuid = run(["blkid", "-s", "UUID", "-o", "value", device]).decode().strip()
    if command == "mount":
        require(uuid == settings["cache_uuid"], "Filesystem UUID mismatch")
    target = Path("/var/lib/agentclash/cache")
    target.mkdir(mode=0o700, exist_ok=True)
    require(
        not target.is_mount() and not any(target.iterdir()),
        "Mount target is already occupied",
    )
    run(["mount", "-o", "nodev,nosuid,noexec", "UUID=" + uuid, str(target)])
    marker = target / ".volume-identity"
    if command == "initialize":
        marker.write_text(volume_id + "\n")
        marker.chmod(0o600)
        settings["cache_uuid"] = uuid
        pending = Path("/etc/agentclash/host.next.json")
        pending.write_text(json.dumps(settings, indent=2) + "\n")
        pending.chmod(0o600)
        os.replace(pending, "/etc/agentclash/host.json")
    require(marker.read_text().strip() == volume_id, "Retained cache identity mismatch")
    os.chown(target, 999, 999)
    fstab = Path("/etc/fstab")
    line = (
        "UUID=" + uuid + " " + str(target) + " ext4 defaults,nodev,nosuid,noexec 0 2\n"
    )
    existing = fstab.read_text()
    require(
        str(target) not in existing or line in existing, "Conflicting cache mount entry"
    )
    if line not in existing:
        with fstab.open("a") as stream:
            stream.write(line)
    print("Retained cache volume verified and mounted")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        sys.exit(
            "Volume operation refused or failed; inspect privately before retrying"
        )
