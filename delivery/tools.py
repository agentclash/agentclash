#!/usr/bin/env python3
"""Install reviewed public tool bytes outside the checkout, verifying SHA256."""

import argparse
import http.client
import io
import json
from pathlib import Path
import platform
import re
import sys
import tarfile
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "deploy/aws/scripts"))
from common import require, verify_blob

DOWNLOAD_ATTEMPTS = 3
DOWNLOAD_BACKOFF_SECONDS = 5
# Network faults that a later attempt can recover from. HTTPError is a
# URLError, so fetch() checks the status code before it tries again.
TRANSIENT_ERRORS = (
    urllib.error.URLError,
    http.client.HTTPException,
    ConnectionError,
    TimeoutError,
)


GITHUB_API = "https://api.github.com"
RELEASE_DOWNLOAD = re.compile(
    r"https://github\.com/([\w.-]+)/([\w.-]+)/releases/download/([^/?#]+)/([^/?#]+)"
)


def permanent(error):
    """Return True for an HTTP client error that another attempt cannot fix."""
    return (
        isinstance(error, urllib.error.HTTPError)
        and error.code < 500
        and error.code != 429
    )


def fetch(request):
    """Read one address; try again after a transient network fault."""
    for attempt in range(1, DOWNLOAD_ATTEMPTS + 1):
        try:
            with urllib.request.urlopen(request, timeout=90) as response:
                return response.read(200 * 1024 * 1024)
        except TRANSIENT_ERRORS as error:
            if permanent(error) or attempt == DOWNLOAD_ATTEMPTS:
                raise
            time.sleep(DOWNLOAD_BACKOFF_SECONDS * attempt)


def download(url):
    """Read a reviewed URL.

    If the release download path of GitHub stays unavailable, read the same
    release asset through the GitHub API. The caller checks the SHA256 of the
    bytes, so the second path cannot change the installed bytes.
    """
    try:
        return fetch(url)
    except TRANSIENT_ERRORS as error:
        match = RELEASE_DOWNLOAD.fullmatch(url)
        if permanent(error) or match is None:
            raise
    owner, repo, tag, name = match.groups()
    print("Tool download uses the GitHub release API", flush=True)
    release = json.loads(
        fetch(
            urllib.request.Request(
                f"{GITHUB_API}/repos/{owner}/{repo}/releases/tags/"
                + urllib.parse.quote(tag, safe=""),
                headers={"Accept": "application/vnd.github+json"},
            )
        )
    )
    assets = [asset for asset in release.get("assets", []) if asset["name"] == name]
    require(
        len(assets) == 1
        and assets[0]["url"].startswith(f"{GITHUB_API}/repos/{owner}/{repo}/"),
        "Release asset not found",
    )
    return fetch(
        urllib.request.Request(
            assets[0]["url"], headers={"Accept": "application/octet-stream"}
        )
    )


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
    data = download(entry["url"])
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
