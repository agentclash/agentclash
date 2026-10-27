#!/usr/bin/env python3
"""Record a private GitHub protection snapshot for later activation review."""

import argparse
import json
import os
from pathlib import Path
import sys
import urllib.parse

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "deploy/aws/scripts"))
from common import private_json, require
from cloud import get_json, protection_snapshot


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True)
    parser.add_argument(
        "--token-file",
        required=True,
        help="0600 private JSON containing a short-lived personal GitHub token",
    )
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    for value in (args.config, args.token_file, args.output):
        require(
            not Path(value).resolve().is_relative_to(ROOT),
            "Private material must remain outside Git",
        )
    github = private_json(args.config)["github"]
    token = private_json(args.token_file)["token"]
    require(
        get_json("https://api.github.com/user", token)["login"].casefold()
        == github["operator_login"].casefold(),
        "Personal GitHub identity mismatch",
    )
    base = "https://api.github.com/repos/" + github["repository"]
    url = base + "/environments/" + urllib.parse.quote(github["environment"], safe="")
    environment = get_json(url, token)
    branches = get_json(url + "/deployment-branch-policies?per_page=100", token)
    rules = get_json(base + "/rules/branches/main", token)
    snapshot = protection_snapshot(environment, branches, rules)
    with Path(args.output).open("x") as output:
        json.dump(
            {
                "protection_sha256": snapshot,
                "environment": environment,
                "branches": branches,
                "rules": rules,
                "manual_admin_bypass_and_denial_tests_still_required": True,
            },
            output,
            indent=2,
        )
    print("Protection snapshot stored privately; no settings were changed")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        sys.exit("Protection snapshot refused or failed")
