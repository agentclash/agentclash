#!/usr/bin/env python3
"""Dependency-aware CI selection and an aggregate that cannot pass missing jobs."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess

GROUPS = ("backend", "runtime", "cli", "terminal", "frontend", "platform", "images")


def select(paths, force=False):
    selected = {key: force for key in GROUPS}
    for path in paths:
        if path.startswith((".github/", "delivery/")) or path in (
            "Makefile",
            "go.work",
            "go.work.sum",
            ".dockerignore",
        ):
            return dict.fromkeys(GROUPS, True)
        if path.startswith("runtime/"):
            for key in ("backend", "runtime", "cli", "images"):
                selected[key] = True
        if path.startswith(
            ("backend/", "scripts/db/", "scripts/deployment/", "scripts/dev/bootstrap")
        ):
            selected["backend"] = selected["images"] = True
        if path.startswith("cli/"):
            selected["cli"] = True
        if path.startswith(
            ("services/try-cli/", "try-cli/packages/core/", "try-cli/demos/")
        ):
            selected["terminal"] = selected["images"] = True
        if path.startswith("web/"):
            selected["frontend"] = True
        if path.startswith("deploy/aws/"):
            selected["platform"] = selected["images"] = True
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
    plan = select(paths, force)
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write("plan=" + json.dumps(plan, separators=(",", ":")) + "\n")
        for key, required in plan.items():
            output.write(key + "=" + str(required).lower() + "\n")


if __name__ == "__main__":
    main()
