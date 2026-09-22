#!/usr/bin/env python3
"""Install reviewed public tool bytes outside the checkout, verifying SHA256."""

import argparse
import io
import json
from pathlib import Path
import platform
import sys
import tarfile
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "deploy/aws/scripts"))
from common import require, verify_blob


def install(name, destination, target=None):
    target = target or (
        platform.system().lower()
        + "_"
        + {"x86_64": "amd64", "arm64": "arm64", "aarch64": "arm64"}[platform.machine()]
    )
    entry = json.loads((ROOT / "delivery/tools.lock.json").read_text())["tools"][name][
        "platforms"
    ][target]
    path = Path(destination).resolve()
    require(not path.is_relative_to(ROOT), "Tools must be outside the checkout")
    require(not path.exists(), "Tool destination already exists")
    require(entry["url"].startswith("https://github.com/"), "Unapproved tool origin")
    with urllib.request.urlopen(entry["url"], timeout=90) as response:
        data = response.read(200 * 1024 * 1024)
    verify_blob(data, entry["sha256"])
    if entry["member"]:
        with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as archive:
            member = archive.getmember(entry["member"])
            require(
                member.isfile() and member.size < 200 * 1024 * 1024,
                "Invalid tool archive member",
            )
            data = archive.extractfile(member).read()
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    with path.open("xb") as stream:
        stream.write(data)
    path.chmod(0o700)
    return entry


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("name", choices=["compose", "gitleaks", "trivy", "actionlint"])
    parser.add_argument("destination")
    parser.add_argument("--platform")
    args = parser.parse_args()
    install(args.name, args.destination, args.platform)
    print("Reviewed tool checksum verified")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        sys.exit("Tool installation failed; no unverified binary executed")
