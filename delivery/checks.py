#!/usr/bin/env python3
"""Run repeatable checks; child output stays in private temporary storage."""

import argparse
import os
from pathlib import Path
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "deploy/aws/scripts"))
from common import require
from tools import install


def check(group, evidence):
    evidence = Path(evidence).resolve()
    require(not evidence.is_relative_to(ROOT), "Check evidence must be outside Git")
    evidence.mkdir(mode=0o700, parents=True, exist_ok=True)
    env = {
        key: value
        for key, value in os.environ.items()
        if key
        in (
            "PATH",
            "HOME",
            "USER",
            "TMPDIR",
            "GOPATH",
            "GOCACHE",
            "GOMODCACHE",
            "DOCKER_HOST",
            "DOCKER_CONTEXT",
            "CI",
        )
    }
    env["TMPDIR"] = str(evidence)

    def run(argv, cwd=ROOT, timeout=2400):
        with (evidence / "checks.log").open("ab") as log:
            result = subprocess.run(
                argv,
                cwd=cwd,
                env=env,
                stdout=log,
                stderr=subprocess.STDOUT,
                timeout=timeout,
            )
        require(result.returncode == 0, "A required check failed")

    if group in ("backend", "runtime", "cli"):
        for command in (
            ["go", "build", "./..."],
            ["go", "vet", "./..."],
            ["go", "test", "-short", "-race", "-count=1", "./..."],
        ):
            run(command, ROOT / group)
        if group == "backend":
            run([sys.executable, "scripts/db/test-migrator.py"])
    elif group == "terminal":
        for command in (
            ["bun", "install", "--frozen-lockfile"],
            ["bun", "run", "typecheck"],
            ["bun", "run", "test"],
        ):
            run(command, ROOT / "services/try-cli")
    elif group == "frontend":
        for command in (
            ["npm", "ci"],
            ["npm", "test"],
            ["npx", "tsc", "--noEmit"],
            ["npm", "run", "lint"],
            ["npm", "run", "build"],
        ):
            run(command, ROOT / "web")
    elif group == "platform":
        actionlint = evidence / "actionlint"
        install("actionlint", actionlint)
        gitleaks = evidence / "gitleaks"
        install("gitleaks", gitleaks)
        run(
            [
                sys.executable,
                "delivery/secret_policy.py",
                "--gitleaks",
                str(gitleaks),
                "--evidence-dir",
                str(evidence),
            ]
        )
        run(
            [
                str(actionlint),
                "-shellcheck=",
                *map(str, sorted((ROOT / ".github/workflows").glob("aws-*.yml"))),
            ]
        )
        run(
            [
                "cfn-lint",
                *map(str, sorted((ROOT / "deploy/aws/cloudformation").glob("*.yaml"))),
            ]
        )
        for tests in ("delivery/tests", "deploy/aws/tests"):
            run([sys.executable, "-m", "unittest", "discover", "-s", tests, "-v"])
        run(
            [
                "ruff",
                "check",
                "--select",
                "F",
                "delivery",
                "deploy/aws/scripts",
                "deploy/aws/tests",
            ]
        )
        for path in (ROOT / "deploy/aws").rglob("*.sh"):
            run(["bash", "-n", str(path)])
    elif group == "images":
        from build import build_images

        build_images(run)
        run(
            [
                sys.executable,
                "deploy/aws/tests/rehearse.py",
                "--evidence-dir",
                str(evidence),
            ],
            timeout=1800,
        )
        run(
            [
                sys.executable,
                "deploy/aws/tests/rehearse-edge.py",
                "--evidence-dir",
                str(evidence),
            ]
        )
    else:
        raise ValueError("Unknown check group")


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "group",
        choices=[
            "backend",
            "runtime",
            "cli",
            "terminal",
            "frontend",
            "platform",
            "images",
        ],
    )
    parser.add_argument("--evidence-dir")
    args = parser.parse_args()
    if args.evidence_dir:
        check(args.group, args.evidence_dir)
    else:
        with tempfile.TemporaryDirectory(prefix="agentclash-ci-") as directory:
            check(args.group, directory)
    print("Required check group passed")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        sys.exit(
            "Required check group failed; reproduce with --evidence-dir in private storage"
        )
