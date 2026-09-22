#!/usr/bin/env python3
"""Run repeatable checks; child output stays in private temporary storage."""

import argparse
import ast
import os
from pathlib import Path
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "deploy/aws/scripts"))
from common import Refused, require
from tools import install


def command_label(argv):
    """Only fixed tool names may enter public failure messages, never arguments."""
    name = Path(argv[0]).name
    return (
        name
        if name in ("docker", "trivy", "bun", "go", "npm", "npx", "cfn-lint", "ruff")
        else "validation-tool"
    )


def refusal_detail(error):
    """Publish only invariant text already present literally in public source."""
    if not isinstance(error, Refused):
        return None
    for name in ("checks.py", "build.py", "maintained.py", "image_scan.py", "tools.py"):
        for node in ast.walk(ast.parse((ROOT / "delivery" / name).read_text())):
            if (
                isinstance(node, ast.Call)
                and isinstance(node.func, ast.Name)
                and node.func.id == "require"
                and len(node.args) > 1
                and isinstance(node.args[1], ast.Constant)
                and isinstance(node.args[1].value, str)
                and node.args[1].value == str(error)
            ):
                return node.args[1].value
    return None


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
        # Bun resolves the linked core package through its owning workspace in
        # a full checkout. A fresh runner also needs that workspace's locked
        # dependencies; a developer's existing node_modules can hide this gap.
        run(["bun", "install", "--frozen-lockfile"], ROOT / "try-cli")
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
        from build import build_graph

        install("trivy", evidence / "trivy")
        print("Image scanner checksum verified", flush=True)
        env.update(
            TRIVY_CONFIG=os.devnull,
            TRIVY_IGNORE_FILE=os.devnull,
            TRIVY_CACHE_DIR=str(evidence / "trivy-cache"),
        )

        # Build/scanner callers need captured JSON as well as private logs.
        def image_run(argv, timeout=2400):
            result = subprocess.run(
                argv, cwd=ROOT, env=env, capture_output=True, timeout=timeout
            )
            with (evidence / "checks.log").open("ab") as log:
                log.write(result.stdout + result.stderr)
            if result.returncode:
                print(
                    "Image check subprocess failed: " + command_label(argv), flush=True
                )
            require(result.returncode == 0, "Required image build or scan failed")
            return result.stdout

        build_graph(ROOT, evidence, image_run)
        selection = str(evidence / "image-selection.json")
        run(
            [
                sys.executable,
                "deploy/aws/tests/rehearse-compose.py",
                "--evidence-dir",
                str(evidence),
                "--images",
                selection,
            ]
        )
        run(
            [
                sys.executable,
                "deploy/aws/tests/rehearse.py",
                "--evidence-dir",
                str(evidence),
                "--images",
                selection,
            ],
            timeout=1800,
        )
        run(
            [
                sys.executable,
                "deploy/aws/tests/rehearse-edge.py",
                "--evidence-dir",
                str(evidence),
                "--images",
                selection,
            ]
        )
        run(
            [
                sys.executable,
                "deploy/aws/tests/rehearse-terminal.py",
                "--evidence-dir",
                str(evidence),
                "--images",
                selection,
            ],
            timeout=600,
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
    except Exception as error:
        # Class names are bounded local diagnostics; exception messages and
        # child output can contain private paths or values and stay private.
        categories = {
            "HTTPError",
            "URLError",
            "TimeoutExpired",
            "FileNotFoundError",
            "Refused",
            "KeyError",
            "ValueError",
        }
        category = type(error).__name__
        detail = refusal_detail(error)
        if detail:
            print("Check invariant: " + detail, file=sys.stderr)
        print(
            "Check failure category: "
            + (category if category in categories else "internal-error"),
            file=sys.stderr,
        )
        sys.exit(
            "Required check group failed; reproduce with --evidence-dir in private storage"
        )
