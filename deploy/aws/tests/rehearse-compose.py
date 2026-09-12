#!/usr/bin/env python3
"""Create a production Compose admin container using only synthetic settings."""

import argparse
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[3]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
from common import require
from maintained import selection


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--evidence-dir", required=True)
    parser.add_argument("--images", required=True)
    args = parser.parse_args()
    evidence = Path(args.evidence_dir).resolve()
    require(not evidence.is_relative_to(ROOT), "Evidence must stay outside Git")
    evidence.mkdir(mode=0o700, parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="compose-smoke-", dir=evidence))
    images = selection(args.images)
    project = "agentclash-compose-test-" + secrets.token_hex(6)
    env = {
        k: v
        for k, v in os.environ.items()
        if k in ("PATH", "HOME", "USER", "DOCKER_HOST", "DOCKER_CONTEXT")
    }
    recipient = work / "secrets/database-admin"
    recipient.mkdir(mode=0o755, parents=True)
    recipient.chmod(0o755)
    (recipient / "env.sh").write_text("# Synthetic fixture; no credentials.\n")
    (recipient / "env.sh").chmod(0o644)
    env_file = work / "compose.env"
    env_file.write_text(
        "SECRETS_ROOT="
        + str(work / "secrets")
        + "\n"
        + "".join(
            "IMAGE_" + name.upper().replace("-", "_") + "=" + ref + "\n"
            for name, ref in images.items()
        )
    )
    compose = [
        "docker",
        "compose",
        "--project-name",
        project,
        "--env-file",
        str(env_file),
        "--file",
        str(ROOT / "deploy/aws/compose.yaml"),
        "--profile",
        "admin",
    ]
    step = 0

    def run(command):
        nonlocal step
        step += 1
        result = subprocess.run(command, env=env, capture_output=True, timeout=120)
        (work / (str(step) + ".log")).write_bytes(result.stdout + result.stderr)
        require(
            result.returncode == 0, "Compose smoke failed; inspect private evidence"
        )
        return result.stdout

    try:
        model = json.loads(run(compose + ["config", "--format", "json"]))
        expected = {"rw", "noexec", "nosuid", "size=64m", "mode=1777"}
        for service in model["services"].values():
            require(len(service["tmpfs"]) == 1, "Split tmpfs declaration")
            target, options = service["tmpfs"][0].split(":", 1)
            require(
                target == "/tmp" and set(options.split(",")) == expected,
                "Unexpected temporary filesystem configuration",
            )
        require(
            not model["services"]["database-admin"].get("ports"), "Admin port exposed"
        )
        run(
            compose
            + [
                "run",
                "--detach",
                "--no-deps",
                "--name",
                project,
                "database-admin",
                "/bin/sh",
                "-ec",
                "sleep 300",
            ]
        )
        container = json.loads(run(["docker", "inspect", project]))[0]
        mounts = container["HostConfig"]["Tmpfs"]
        require(set(mounts) == {"/tmp"}, "Unexpected Docker tmpfs targets")
        require(
            set(mounts["/tmp"].split(",")) == expected, "Docker tmpfs options changed"
        )
        require(container["HostConfig"]["ReadonlyRootfs"], "Writable container root")
        require(not container["HostConfig"]["PortBindings"], "Unexpected host port")
        run(
            [
                "docker",
                "exec",
                project,
                "/bin/sh",
                "-ec",
                """
test -w /tmp
test "$(stat -c %a /tmp)" = 1777
awk '$2 == "/tmp" && $3 == "tmpfs" { found=1 } END { exit !found }' /proc/mounts
printf fixture > /tmp/compose-smoke
test "$(cat /tmp/compose-smoke)" = fixture
""",
            ]
        )
        (work / "result.json").write_text(
            json.dumps(
                {
                    "production_compose": True,
                    "all_service_mounts_valid": True,
                    "docker_create_and_tmpfs_write": True,
                    "credentials_used": False,
                    "host_ports": False,
                },
                indent=2,
            )
            + "\n"
        )
        print("Production Compose model, Docker creation and writable tmpfs passed.")
    finally:
        subprocess.run(
            ["docker", "rm", "--force", project],
            env=env,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            timeout=60,
        )
        run(compose + ["down", "--remove-orphans"])


if __name__ == "__main__":
    main()
