#!/usr/bin/env python3
"""Build an allowlisted, reproducible bootstrap archive in private storage."""

import argparse
import gzip
import io
import json
from pathlib import Path
import sys
import tarfile

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "deploy/aws/scripts"))
from common import require, verify_blob
from release_contract import platform_files, platform_hash, sha256


def bundle(root, compose, destination):
    root, compose, destination = Path(root), Path(compose), Path(destination).resolve()
    require(
        not destination.is_relative_to(root.resolve()),
        "Bundle destination must be private and outside Git",
    )
    expected = json.loads((root / "delivery/tools.lock.json").read_text())["tools"][
        "compose"
    ]["platforms"]["linux_amd64"]["sha256"]
    binary = compose.read_bytes()
    verify_blob(binary, expected)
    files = {name: (root / name).read_bytes() for name in platform_files(root)}
    files["tools/docker-compose"] = binary
    files["tools/SHA256SUMS"] = (expected + "  tools/docker-compose\n").encode()
    files["tools/platform-source.sha256"] = (platform_hash(root) + "\n").encode()
    with destination.open("xb") as output:
        with gzip.GzipFile(fileobj=output, filename="", mode="wb", mtime=0) as zipped:
            with tarfile.open(fileobj=zipped, mode="w|") as archive:
                for name, data in sorted(files.items()):
                    member = tarfile.TarInfo(name)
                    member.size, member.mode, member.mtime = len(data), 0o644, 0
                    archive.addfile(member, io.BytesIO(data))
    destination.chmod(0o600)
    return {
        "sha256": sha256(destination.read_bytes()),
        "platform_source_sha256": platform_hash(root),
        "files": sorted(files),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--compose", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    bundle(ROOT, args.compose, args.output)
    print("Bootstrap bundle verified and written privately")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        sys.exit("Bundle preparation refused or failed")
