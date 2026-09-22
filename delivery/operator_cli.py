#!/usr/bin/env python3
"""Publish reviewed private evidence/approvals through the human SSO path."""

import argparse
import os
from pathlib import Path
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "deploy/aws/scripts"))
from common import private_json, require
from release_contract import canonical, sha256, validate_approval
from cloud import Cloud, validate_config


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("kind", choices=["approval", "evidence"])
    parser.add_argument("--config", required=True)
    parser.add_argument("--file", required=True)
    parser.add_argument("--source", required=True)
    args = parser.parse_args()
    for name in (args.config, args.file):
        require(
            not Path(name).resolve().is_relative_to(ROOT),
            "Private input must be outside Git",
        )
    config, value = private_json(args.config), private_json(args.file)
    validate_config(config, config["kind"])
    require(
        config["kind"] in ("staging", "production"),
        "Operator evidence needs a target environment",
    )
    with tempfile.TemporaryDirectory(prefix="agentclash-approval-") as directory:
        cloud = Cloud(config, directory, mode="sso")
        release_sha, _ = cloud.release(args.source)
        require(value["release_sha256"] == release_sha, "Evidence source mismatch")
        if args.kind == "approval":
            settings = {
                **config,
                "delivery": {"enabled": True, "environment": config["kind"]},
            }
            validate_approval(
                value,
                settings,
                release_sha,
                value["previous_release_sha256"],
                value["operation"],
            )
            key = (
                "approvals/"
                + config["kind"]
                + "/"
                + value["operation"]
                + "/"
                + release_sha
                + ".json"
            )
        else:
            require(
                value["kind"] in ("drain", "recovery", "rehearsal", "smoke")
                and value["checks_passed"] is True,
                "Evidence is incomplete",
            )
            require(
                value["environment"] == config["kind"]
                and value["instance_id"] == config["instance_id"],
                "Evidence destination mismatch",
            )
            if value["kind"] == "rehearsal":
                require(
                    config["kind"] == "staging" and value["isolated"] is True,
                    "Rehearsal evidence requires an isolated target",
                )
            key = "evidence/operator/" + sha256(canonical(value)) + ".json"
        path = Path(directory) / "object.json"
        path.write_bytes(canonical(value))
        if args.kind == "approval":
            # Only the human role may renew an expiring approval. Versioning
            # preserves old approvals; the host records the exact bytes it used.
            cloud.call(
                "s3api",
                "put-object",
                "--bucket",
                config["artifact_bucket"],
                "--key",
                key,
                "--body",
                str(path),
                "--server-side-encryption",
                "AES256",
            )
        else:
            cloud.put(key, path)
    print("Reviewed operator object stored privately")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        sys.exit("Operator publication refused or failed")
