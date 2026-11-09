#!/usr/bin/env python3
"""Run repeatable checks; child output stays in private temporary storage."""

import argparse
import ast
import concurrent.futures
import errno
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time

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
    for name in (
        "checks.py",
        "build.py",
        "maintained.py",
        "image_scan.py",
        "tools.py",
        "activation.py",
        "cloud.py",
    ):
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


def failure_location(error):
    """Return a public source location, without exception values or locals."""
    location = None
    frame = error.__traceback__
    while frame:
        path = Path(frame.tb_frame.f_code.co_filename).resolve()
        if path.parent == ROOT / "delivery" and path.suffix == ".py":
            location = "delivery/" + path.name + ":" + str(frame.tb_lineno)
        frame = frame.tb_next
    return location


# A group can run as parallel parts: aws-checks.yml gives each part its own
# runner. Without a part, the group runs every part in sequence.
PARTS = {
    "backend": ("compile", "test", "migrator"),
    "runtime": ("compile", "test"),
    "frontend": ("test", "static", "build"),
}


def check(group, evidence, part=None):
    require(part is None or part in PARTS.get(group, ()), "Unknown check part")

    def selected(name):
        return part in (None, name)

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

    def run(argv, cwd=ROOT, timeout=2400, label=None, log_name="checks.log"):
        # A caller label must be a fixed public name from this file.
        label = label or command_label(argv)
        if label == "go" and len(argv) > 1 and argv[1] in ("build", "vet", "test"):
            label += "-" + argv[1]
        print("Required check subprocess: " + label, flush=True)
        start = time.monotonic()
        with (evidence / log_name).open("ab") as log:
            result = subprocess.run(
                argv,
                cwd=cwd,
                env=env,
                stdout=log,
                stderr=subprocess.STDOUT,
                timeout=timeout,
            )
        seconds = round(time.monotonic() - start)
        print(
            "Required check subprocess took: " + label + " " + str(seconds) + "s",
            flush=True,
        )
        if result.returncode:
            print("Required check subprocess failed: " + label, flush=True)
        require(result.returncode == 0, "A required check failed")

    if group in ("backend", "runtime", "cli"):
        if selected("compile"):
            run(["go", "build", "./..."], ROOT / group)
            run(["go", "vet", "./..."], ROOT / group)
        if selected("test"):
            run(["go", "test", "-short", "-race", "-count=1", "./..."], ROOT / group)
        if group == "backend" and selected("migrator"):
            run([sys.executable, "scripts/db/test-migrator.py"])
            run([sys.executable, "deploy/aws/tests/rehearse-vibe-worker.py"], timeout=240)
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
        run(["npm", "ci"], ROOT / "web")
        if selected("test"):
            run(["npm", "test"], ROOT / "web")
        if selected("static"):
            run(["npx", "tsc", "--noEmit"], ROOT / "web")
            run(["npm", "run", "lint"], ROOT / "web")
        if selected("build"):
            run(["npm", "run", "build"], ROOT / "web")
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
                *map(str, sorted((ROOT / ".github/workflows").glob("*.yml"))),
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
        from build import build_graph, prepare_bootstrap

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

        # rehearse.py runs two host `go test -race` checks. Compile them while
        # the images build, so the rehearsals find a warm Go build cache.
        warm = evidence / "go-test-binaries"
        warm.mkdir(mode=0o700)
        with concurrent.futures.ThreadPoolExecutor(1) as compiler:
            # A low CPU priority gives the CPU to the image loads, scans and
            # builds first. The compile has time until the rehearsals start.
            compiled = compiler.submit(
                run,
                [
                    "nice",
                    "-n",
                    "10",
                    "go",
                    "test",
                    "-race",
                    "-tags=awsplatform",
                    "-c",
                    "-o",
                    str(warm),
                    "./internal/temporalutil",
                    "./internal/pubsub",
                ],
                ROOT / "backend",
                label="go-test-compile",
                log_name="go-test-compile.log",
            )
            graph = build_graph(ROOT, evidence, image_run)
            prepare_bootstrap(ROOT, evidence, image_run, graph["platform_images"]["go"])
        compiled.result()
        selection = str(evidence / "image-selection.json")
        # Each rehearsal uses random container and network names, free host
        # ports and its own work directory, so they run at the same time. Each
        # writes its own private log. rehearse.py runs as two processes: the
        # database and Temporal part, and the cache part.
        rehearsals = {
            "rehearse-compose": (["rehearse-compose.py"], 2400),
            "rehearse-platform": (["rehearse.py", "--platform-only"], 1800),
            "rehearse-cache": (["rehearse.py", "--cache-only"], 1800),
            "rehearse-edge": (["rehearse-edge.py"], 2400),
            "rehearse-terminal": (["rehearse-terminal.py"], 600),
        }
        with concurrent.futures.ThreadPoolExecutor(len(rehearsals)) as pool:
            jobs = [
                pool.submit(
                    run,
                    [
                        sys.executable,
                        "deploy/aws/tests/" + script,
                        *options,
                        "--evidence-dir",
                        str(evidence),
                        "--images",
                        selection,
                    ],
                    timeout=timeout,
                    label=name,
                    log_name=name + ".log",
                )
                for name, ((script, *options), timeout) in rehearsals.items()
            ]
        for job in jobs:
            job.result()
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
    parser.add_argument(
        "--part", choices=sorted({name for parts in PARTS.values() for name in parts})
    )
    parser.add_argument("--evidence-dir")
    args = parser.parse_args()
    if args.evidence_dir:
        check(args.group, args.evidence_dir, args.part)
    else:
        with tempfile.TemporaryDirectory(prefix="agentclash-ci-") as directory:
            check(args.group, directory, args.part)
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
            "RemoteDisconnected",
            "IncompleteRead",
            "TimeoutError",
            "ConnectionResetError",
            "ConnectionAbortedError",
            "ConnectionRefusedError",
            "TimeoutExpired",
            "FileNotFoundError",
            "Refused",
            "KeyError",
            "ValueError",
            "OSError",
            "PermissionError",
            "FileExistsError",
            "TypeError",
            "AttributeError",
            "JSONDecodeError",
        }
        category = type(error).__name__
        location = failure_location(error)
        if location:
            print("Check source location: " + location, file=sys.stderr)
        if isinstance(error, OSError) and error.errno in errno.errorcode:
            print(
                "Check system error: " + errno.errorcode[error.errno], file=sys.stderr
            )
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
