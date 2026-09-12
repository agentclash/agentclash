#!/usr/bin/env python3
"""Run the pinned Caddy image locally: full fence, reload and request-log redaction."""

import argparse
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
sys.path[:0] = [str(ROOT.parents[1] / "delivery"), str(ROOT / "scripts")]
from maintained import selection


def main():
    os.umask(0o077)
    p = argparse.ArgumentParser()
    p.add_argument("--evidence-dir", required=True)
    p.add_argument("--images", required=True)
    args = p.parse_args()
    evidence = Path(args.evidence_dir).resolve()
    if evidence.is_relative_to(ROOT.parents[1]):
        raise RuntimeError("Evidence must remain outside Git")
    image = selection(args.images)["caddy"]
    name = "agentclash-edge-test-" + secrets.token_hex(5)

    def run(*command):
        result = subprocess.run(command, capture_output=True)
        if result.returncode:
            raise RuntimeError("Caddy test command failed")
        return result.stdout

    with tempfile.TemporaryDirectory(dir=evidence, prefix="edge-") as tmp:
        edge = Path(tmp)
        edge.chmod(0o755)
        fence = edge / "fence.caddy"
        fence.write_bytes((ROOT / "config/fence.closed.caddy").read_bytes())
        fence.chmod(0o644)
        try:
            run(
                "docker",
                "run",
                "-d",
                "--name",
                name,
                "--platform",
                "linux/amd64",
                "--user",
                "10001:10001",
                "--read-only",
                "--tmpfs",
                "/data:rw,uid=10001,gid=10001,mode=0700",
                "--tmpfs",
                "/config:rw,uid=10001,gid=10001,mode=0700",
                "-e",
                "API_DOMAIN=http://api.localhost",
                "-e",
                "TERMINAL_DOMAIN=http://terminal.localhost",
                "-e",
                "ACME_EMAIL=ops@example.com",
                "-v",
                str(ROOT / "config/Caddyfile") + ":/etc/caddy/Caddyfile:ro",
                "-v",
                str(edge) + ":/run/edge:ro",
                "-p",
                "127.0.0.1::80",
                image,
                "caddy",
                "run",
                "--config",
                "/etc/caddy/Caddyfile",
                "--adapter",
                "caddyfile",
            )
            port = (
                run(
                    "docker",
                    "inspect",
                    "--format",
                    '{{(index (index .NetworkSettings.Ports "80/tcp") 0).HostPort}}',
                    name,
                )
                .decode()
                .strip()
            )
            token = secrets.token_hex(16)

            def request(method="GET"):
                req = urllib.request.Request(
                    "http://127.0.0.1:" + port + "/?sessionId=" + token,
                    method=method,
                    headers={
                        "Host": "api.localhost",
                        "Authorization": "Bearer " + token,
                        "X-Trycli-Proxy-Secret": token,
                    },
                )
                try:
                    return urllib.request.urlopen(req, timeout=5).status
                except urllib.error.HTTPError as error:
                    return error.code

            for _ in range(30):
                try:
                    if request() == 503:
                        break
                except OSError:
                    pass
                time.sleep(0.2)
            else:
                raise RuntimeError("Caddy fence did not start")
            for method in ("GET", "HEAD", "OPTIONS", "POST"):
                if request(method) != 503:
                    raise RuntimeError("Full maintenance fence did not cover method")
            fence.write_bytes((ROOT / "config/fence.open.caddy").read_bytes())
            run(
                "docker",
                "exec",
                name,
                "caddy",
                "reload",
                "--config",
                "/etc/caddy/Caddyfile",
                "--adapter",
                "caddyfile",
            )
            if request() != 502:
                raise RuntimeError(
                    "Open fence did not route to unavailable test upstream"
                )
            logs = subprocess.run(["docker", "logs", name], capture_output=True)
            if token.encode() in logs.stdout + logs.stderr:
                raise RuntimeError("Caddy leaked request metadata")
            (evidence / "edge-result.json").write_text(
                json.dumps(
                    {
                        "status": "passed",
                        "checks": [
                            "rootless pinned Caddy",
                            "all-method fence",
                            "reload",
                            "request metadata absent from error log",
                        ],
                    }
                )
                + "\n"
            )
            print("Caddy fence, reload and error-log privacy passed")
        finally:
            subprocess.run(["docker", "rm", "-f", name], capture_output=True)


if __name__ == "__main__":
    main()
