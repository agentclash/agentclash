#!/usr/bin/env python3
"""One delivery interface for protected CI and the explicit human SSO fallback."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "deploy/aws/scripts"))
from common import private_json, require
from release_contract import validate_images, validate_release
from cloud import Cloud, validate_config, github_guard


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "operation", choices=["build", "deploy", "promote", "inspect", "validate"]
    )
    parser.add_argument(
        "--kind", choices=["build", "staging", "production"], required=True
    )
    parser.add_argument(
        "--config", help="0600 private file for the SSO/validation path"
    )
    parser.add_argument(
        "--source", help="Public source commit SHA; private manifests stay in S3"
    )
    parser.add_argument("--output", help="Private output file, only for SSO inspect")
    args = parser.parse_args()
    if args.config:
        require(
            not Path(args.config).resolve().is_relative_to(ROOT),
            "Private configuration must be outside Git",
        )
    config = (
        private_json(args.config)
        if args.config
        else json.loads(os.environ["AWS_DELIVERY_CONFIG"])
    )
    validate_config(config, args.kind)
    require(
        args.operation != "build" or args.kind == "build",
        "Build configuration required",
    )
    if args.operation == "validate":
        print(
            "Private delivery configuration structure passed; no live protection check performed"
        )
        return
    require(
        args.source and re.fullmatch(r"[a-f0-9]{40}", args.source),
        "Exact source revision required",
    )
    mode = "sso" if args.config else "oidc"
    if mode == "oidc":
        workflow = "aws-release.yml" if args.operation == "build" else "aws-deploy.yml"
        github_guard(config, workflow)
        require(
            args.operation in ("build", "deploy", "promote"), "Unsupported CI operation"
        )
        if args.operation == "build":
            require(
                args.source == os.environ["GITHUB_SHA"],
                "Build source is not current main",
            )
    else:
        workflow = "aws-deploy.yml"
        if args.operation == "build":
            approval = config["publication_approval"]
            require(
                approval["operator"] == config["operator"]
                and approval["source_revision"] == args.source
                and approval["account_id"] == config["account_id"]
                and approval["region"] == config["region"],
                "SSO publication approval mismatch",
            )
            require(
                time.time() < approval["expires_at"] <= time.time() + 3600
                and approval["local_required_checks_passed"] is True,
                "SSO bootstrap publication approval is absent or expired",
            )
    with tempfile.TemporaryDirectory(prefix="agentclash-delivery-") as directory:
        if args.operation == "build":
            from build import prepare, publish

            prepared = prepare(
                directory,
                args.source,
                [
                    config.get(key)
                    for key in (
                        "account_id",
                        "artifact_bucket",
                        "repository",
                        "role_arn",
                        "instance_id",
                    )
                ],
            )  # No AWS credentials during build/scans.
            if mode == "sso":
                require(
                    time.time() < config["publication_approval"]["expires_at"],
                    "SSO publication approval expired during build; review before retrying",
                )
            cloud = Cloud(config, directory, mode=mode, workflow=workflow)
            publish(cloud, directory, prepared)
        else:
            require(
                args.kind in ("staging", "production"), "Deployment target required"
            )
            require(
                subprocess.run(
                    ["git", "merge-base", "--is-ancestor", args.source, "HEAD"],
                    cwd=ROOT,
                    capture_output=True,
                ).returncode
                == 0,
                "Source is not in the reviewed checkout history",
            )
            cloud = Cloud(config, directory, mode=mode, workflow=workflow)
            sha, manifest = cloud.release(args.source)
            validate_images(
                manifest,
                config["repository"],
                json.loads((ROOT / "deploy/aws/images.lock.json").read_text()),
            )
            validate_release(manifest, ROOT)
            if args.operation == "inspect":
                path = Path(args.output).resolve()
                require(
                    mode == "sso"
                    and not path.is_relative_to(ROOT)
                    and not path.exists(),
                    "Inspect requires a new private file outside Git",
                )
                with path.open("x") as output:
                    json.dump(
                        {"release_sha256": sha, "manifest": manifest}, output, indent=2
                    )
            else:
                cloud.send(args.operation, sha)
    print("Delivery operation passed; private references and output were not published")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        sys.exit("Delivery refused or failed; inspect private state before retrying")
