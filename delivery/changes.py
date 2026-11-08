#!/usr/bin/env python3
"""Dependency-aware CI selection and an aggregate that cannot pass missing jobs."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess

GROUPS = ("backend", "runtime", "cli", "terminal", "frontend", "platform", "images")
# Release Please writes only the CLI version here. A release PR changes this
# file and cli/CHANGELOG.md, so it needs the CLI checks, not every group.
CLI_RELEASE_MANIFEST = ".github/.release-please-manifest.json"
# The images group takes about 20 minutes. On a pull request, a change to
# application code only (these suffixes) runs the fast group that compiles
# and tests that code, not the images group. Dockerfiles, dependency locks,
# migrations, assets and delivery code still select the images group. A push
# to main still selects it for all application changes, and aws-release.yml
# (workflow_call) selects every group before a publication.
GO_CODE = (".go",)
TS_CODE = (".ts", ".tsx")


def select(paths, force=False, pull_request=False):
    selected = {key: force for key in GROUPS}

    def images_unless_code(path, code):
        if not (pull_request and path.endswith(code)):
            selected["images"] = True

    for path in paths:
        if path == CLI_RELEASE_MANIFEST:
            selected["cli"] = True
            continue
        # Delivery tests and docs, and workflows that the AWS checks do not
        # use, need only the platform group: it runs the delivery tests and
        # lints every workflow.
        if (
            path.startswith("delivery/tests/")
            or (path.startswith("delivery/") and path.endswith(".md"))
            or (
                path.startswith(".github/")
                and not path.startswith(".github/workflows/aws-")
            )
        ):
            selected["platform"] = True
            continue
        if path.startswith((".github/", "delivery/")) or path in (
            "Makefile",
            "go.work",
            "go.work.sum",
            ".dockerignore",
            ".tool-versions",
        ):
            return dict.fromkeys(GROUPS, True)
        if path == ".gitleaks.toml":
            selected["platform"] = True
        if path.startswith("runtime/"):
            for key in ("backend", "runtime", "cli"):
                selected[key] = True
            images_unless_code(path, GO_CODE)
        if path.startswith("backend/"):
            selected["backend"] = True
            images_unless_code(path, GO_CODE)
        if path.startswith(
            ("scripts/db/", "scripts/deployment/", "scripts/dev/bootstrap")
        ):
            selected["backend"] = selected["images"] = True
        if path.startswith("cli/"):
            selected["cli"] = True
        if path.startswith(("services/try-cli/", "try-cli/packages/core/")):
            selected["terminal"] = True
            images_unless_code(path, TS_CODE)
        if path.startswith("try-cli/demos/"):
            selected["terminal"] = selected["images"] = True
        if path in ("try-cli/package.json", "try-cli/bun.lock"):
            selected["terminal"] = True
        if path.startswith("web/"):
            selected["frontend"] = True
        if path.startswith("deploy/aws/"):
            selected["platform"] = True
            if not path.endswith(".md"):
                selected["images"] = True
    return selected


def aggregate(plan, results):
    if results.get("changes", {}).get("result") != "success":
        return False
    return all(
        results.get(group, {}).get("result") == ("success" if required else "skipped")
        for group, required in plan.items()
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=["select", "aggregate"])
    args = parser.parse_args()
    if args.operation == "aggregate":
        plan = json.loads(os.environ["CHECK_PLAN"])
        results = json.loads(os.environ["CHECK_RESULTS"])
        if set(plan) != set(GROUPS) or not aggregate(plan, results):
            raise SystemExit("Required release checks did not all pass")
        print("Required release checks passed")
        return
    event_name = os.environ["GITHUB_EVENT_NAME"]
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    force = event_name not in ("push", "pull_request")
    paths = []
    if not force:
        base = (
            event["pull_request"]["base"]["sha"]
            if event_name == "pull_request"
            else event["before"]
        )
        head = os.environ["GITHUB_SHA"]
        if base == "0" * 40:
            force = True
        else:
            assert re.fullmatch(r"[a-f0-9]{40}", base) and re.fullmatch(
                r"[a-f0-9]{40}", head
            )
            paths = (
                subprocess.check_output(
                    ["git", "diff", "--name-only", "--no-renames", "-z", base, head]
                )
                .decode()
                .split("\0")
            )
    plan = select(paths, force, pull_request=event_name == "pull_request")
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write("plan=" + json.dumps(plan, separators=(",", ":")) + "\n")
        for key, required in plan.items():
            output.write(key + "=" + str(required).lower() + "\n")


if __name__ == "__main__":
    main()
