#!/usr/bin/env python3
"""Exercise exact Linux Caddy/Bun bytes with the existing terminal WebSocket test.

Uses synthetic providers, a disposable TLS Valkey and an isolated shared network
namespace. Test containers get no Docker socket, cloud credentials or host port.
"""

import argparse
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[3]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
from common import require
from maintained import selection, local_base, inspect


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--evidence-dir", required=True)
    parser.add_argument("--images", required=True)
    args = parser.parse_args()
    evidence = Path(args.evidence_dir).resolve()
    require(not evidence.is_relative_to(ROOT), "Evidence must stay outside Git")
    evidence.mkdir(mode=0o700, parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="terminal-images-", dir=evidence))
    images = selection(args.images)
    names = []
    logs = None
    test_image = "agentclash-terminal-rehearsal:" + secrets.token_hex(6)
    env = {
        k: v
        for k, v in os.environ.items()
        if k in ("PATH", "HOME", "USER", "DOCKER_HOST", "DOCKER_CONTEXT")
    }
    step = 0

    def run(command, timeout=300):
        nonlocal step
        step += 1
        result = subprocess.run(command, env=env, capture_output=True, timeout=timeout)
        (work / (str(step) + ".log")).write_bytes(result.stdout + result.stderr)
        require(
            result.returncode == 0,
            "Terminal image rehearsal failed; inspect private evidence",
        )
        return result.stdout

    try:
        context = work / "context"
        context.mkdir(mode=0o700)
        shutil.copytree(
            ROOT / "services/try-cli/tests",
            context / "tests",
            ignore=shutil.ignore_patterns("node_modules", ".env*", "__pycache__"),
        )
        shutil.copyfile(ROOT / "web/src/lib/try-cli/proxy.ts", context / "proxy.ts")
        (context / "Dockerfile").write_text("""ARG CADDY_IMAGE
ARG TERMINAL_IMAGE
FROM ${CADDY_IMAGE} AS edge
FROM ${TERMINAL_IMAGE}
COPY --from=edge /usr/bin/caddy /usr/bin/caddy
COPY --chown=10001:10001 tests /app/services/try-cli/tests
COPY --chown=10001:10001 proxy.ts /app/web/src/lib/try-cli/proxy.ts
ENTRYPOINT ["bun", "--no-install", "--no-env-file", "--preserve-symlinks", "test", "tests/integration/proxy.test.ts", "--timeout", "110000"]
""")
        run(
            [
                "docker",
                "build",
                "--platform",
                "linux/amd64",
                "--build-arg",
                "CADDY_IMAGE=" + local_base(run, images["caddy"]),
                "--build-arg",
                "TERMINAL_IMAGE=" + local_base(run, images["terminal"]),
                "-t",
                test_image,
                str(context),
            ]
        )
        test_id = inspect(run, test_image)["Id"]
        cert = work / "certificates"
        cert.mkdir(mode=0o700)
        run(
            [
                "openssl",
                "req",
                "-x509",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-days",
                "1",
                "-subj",
                "/CN=Terminal image rehearsal",
                "-keyout",
                str(cert / "ca.key"),
                "-out",
                str(cert / "ca.pem"),
            ]
        )
        run(
            [
                "openssl",
                "req",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-subj",
                "/CN=localhost",
                "-keyout",
                str(cert / "server.key"),
                "-out",
                str(cert / "server.csr"),
            ]
        )
        (cert / "server.ext").write_text(
            "subjectAltName=DNS:localhost\nextendedKeyUsage=serverAuth\n"
        )
        run(
            [
                "openssl",
                "x509",
                "-req",
                "-days",
                "1",
                "-in",
                str(cert / "server.csr"),
                "-CA",
                str(cert / "ca.pem"),
                "-CAkey",
                str(cert / "ca.key"),
                "-CAcreateserial",
                "-extfile",
                str(cert / "server.ext"),
                "-out",
                str(cert / "server.pem"),
            ]
        )
        password = secrets.token_urlsafe(32)
        (cert / "valkey.conf").write_text(
            "\n".join(
                [
                    "port 0",
                    "tls-port 6379",
                    "bind 127.0.0.1",
                    "tls-auth-clients no",
                    "tls-cert-file /test/server.pem",
                    "tls-key-file /test/server.key",
                    "tls-ca-cert-file /test/ca.pem",
                    "requirepass " + password,
                    'save ""',
                    "appendonly no",
                    "",
                ]
            )
        )
        cache = "agentclash-terminal-cache-" + secrets.token_hex(6)
        names.append(cache)
        run(
            [
                "docker",
                "run",
                "-d",
                "--name",
                cache,
                "--platform",
                "linux/amd64",
                "--user",
                "0:0",
                "--entrypoint",
                "valkey-server",
                "-v",
                str(cert) + ":/test:ro",
                images["valkey"],
                "/test/valkey.conf",
            ]
        )
        # Writable child of a private parent, only mounted in this test container.
        logs = work / "test-output"
        logs.mkdir(mode=0o700)
        logs.chmod(0o777)
        ca = work / "public-ca.pem"
        shutil.copyfile(cert / "ca.pem", ca)
        ca.chmod(0o644)
        settings = work / "test.env"
        settings.write_text(
            "\n".join(
                [
                    "NODE_ENV=test",
                    "TRY_CLI_TEST_REDIS_URL=rediss://:" + password + "@localhost:6379",
                    "TRY_CLI_TEST_CA_FILE=/test-ca.pem",
                    "TRY_CLI_TEST_PRIVATE_DIR=/evidence",
                    "",
                ]
            )
        )
        terminal = "agentclash-terminal-client-" + secrets.token_hex(6)
        names.append(terminal)
        run(
            [
                "docker",
                "run",
                "--name",
                terminal,
                "--platform",
                "linux/amd64",
                "--network",
                "container:" + cache,
                "--read-only",
                "--cap-drop",
                "ALL",
                "--security-opt",
                "no-new-privileges:true",
                "--tmpfs",
                "/tmp:rw,mode=1777",
                "--env-file",
                str(settings),
                "-v",
                str(logs) + ":/evidence",
                "-v",
                str(ca) + ":/test-ca.pem:ro",
                test_id,
            ],
            timeout=150,
        )
        (work / "result.json").write_text(
            json.dumps(
                {
                    "status": "passed",
                    "images": images,
                    "checks": [
                        "exact rebuilt Linux Caddy",
                        "patched Bun application bytes",
                        "TLS Valkey",
                        "identity headers",
                        "WebSocket reconnect and 65-second idle",
                        "callbacks during drain",
                        "synthetic sandbox cleanup",
                    ],
                },
                indent=2,
            )
            + "\n"
        )
        print("Terminal HTTP/WebSocket/drain rehearsal passed with exact Linux images")
    finally:
        for name in reversed(names):
            subprocess.run(["docker", "rm", "-fv", name], env=env, capture_output=True)
        if logs is not None and logs.exists():
            # Linux preserves the container UID on bind-mounted Caddy data.
            # Return only this disposable test-output tree to the runner so
            # private temporary-directory cleanup can traverse its 0700 dirs.
            run(
                [
                    "docker",
                    "run",
                    "--rm",
                    "--network",
                    "none",
                    "--read-only",
                    "--user",
                    "0:0",
                    "--cap-drop",
                    "ALL",
                    "--cap-add",
                    "CHOWN",
                    "--cap-add",
                    "DAC_OVERRIDE",
                    "--security-opt",
                    "no-new-privileges:true",
                    "--entrypoint",
                    "chown",
                    "--mount",
                    "type=bind,source=" + str(logs) + ",target=/evidence",
                    images["alpine"],
                    "-R",
                    str(os.getuid()) + ":" + str(os.getgid()),
                    "/evidence",
                ]
            )
        subprocess.run(
            ["docker", "image", "rm", test_image], env=env, capture_output=True
        )


if __name__ == "__main__":
    main()
