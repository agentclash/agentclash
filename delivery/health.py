"""Quiet live host checks, also used by the release state-machine adapter."""

from datetime import datetime
import json
import time
import urllib.request
import urllib.parse

from common import Refused, require, run


def clean_stop(host, services):
    ids = host.compose("ps", "--all", "-q", *services).decode().split()
    host.compose("stop", *services, timeout=360)
    if ids:
        states = json.loads(run(["docker", "inspect", *ids]))
        require(
            all(
                not c["State"]["Running"]
                and c["State"]["ExitCode"] == 0
                and not c["State"].get("OOMKilled")
                for c in states
            ),
            "Platform stop was unclean",
        )


def drained(host, settings):
    # The private operator receipt covers SQL status/session/sandbox inventory and
    # paused external starts/schedules. Recheck all open Temporal work after fencing.
    raw = host.compose(
        "run",
        "--rm",
        "-T",
        "--no-deps",
        "temporal-admin",
        "temporal",
        "workflow",
        "count",
        "--query",
        'ExecutionStatus="Running"',
        "--output",
        "json",
    )
    require(json.loads(raw).get("count") in (0, "0"), "Open Temporal work remains")
    require(
        settings["delivery"]["namespace"] == "agentclash-prod",
        "Use an isolated cluster for rehearsal",
    )


def no_database_writers(host):
    query = "SELECT count(*) FROM pg_stat_activity WHERE datname IN ('agentclash','temporal','temporal_visibility') AND pid <> pg_backend_pid() AND backend_type='client backend';"
    raw = host.compose(
        "run",
        "--rm",
        "-T",
        "--no-deps",
        "database-admin",
        "psql",
        "-X",
        "-At",
        "-c",
        query,
    )
    require(raw.strip() == b"0", "Database writers remain after stop")


def ready(host, manifest, started_at, timeout=240):
    deadline = time.monotonic() + timeout
    while True:
        try:
            ids = (
                host.compose("ps", "--all", "-q", "api", "worker", "terminal")
                .decode()
                .split()
            )
            require(len(ids) == 3, "Expected exactly three application containers")
            states = json.loads(run(["docker", "inspect", *ids]))
            require(
                {c["Config"]["Labels"]["com.docker.compose.service"] for c in states}
                == {"api", "worker", "terminal"},
                "Unexpected application container set",
            )
            for c in states:
                name = c["Config"]["Labels"]["com.docker.compose.service"]
                require(
                    c["Config"]["Image"] == manifest["images"][name],
                    "Running image mismatch",
                )
                require(
                    c["State"]["Running"] and not c["State"].get("OOMKilled"),
                    "Application exited",
                )
                if name != "worker":
                    require(
                        c["State"].get("Health", {}).get("Status") == "healthy",
                        "Application not ready",
                    )
            for queue in ("execution", "scoring", "background"):
                for kind in ("workflow", "activity"):
                    value = json.loads(
                        host.compose(
                            "run",
                            "--rm",
                            "-T",
                            "--no-deps",
                            "temporal-admin",
                            "temporal",
                            "task-queue",
                            "describe",
                            "--task-queue",
                            queue,
                            "--legacy-mode",
                            "--task-queue-type-legacy",
                            kind,
                            "--output",
                            "json",
                            timeout=20,
                        )
                    )
                    require(
                        any(
                            datetime.fromisoformat(
                                p["lastAccessTime"].replace("Z", "+00:00")
                            ).timestamp()
                            >= started_at
                            for p in value.get("pollers", [])
                        ),
                        "Missing fresh worker pollers",
                    )
            return
        except (Refused, KeyError, ValueError):
            require(
                time.monotonic() < deadline, "Application readiness deadline exceeded"
            )
            time.sleep(3)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        return None


def public_ready(settings):
    urls = settings["delivery"]["public_readiness_urls"]
    require(len(urls) == 2, "Both public readiness endpoints are required")
    for url, path in zip(urls, ("/healthz/ready", "/health/ready")):
        parsed = urllib.parse.urlsplit(url)
        require(
            parsed.scheme == "https"
            and parsed.hostname
            and parsed.path == path
            and not (
                parsed.username or parsed.password or parsed.query or parsed.fragment
            ),
            "Invalid public readiness URL",
        )
        with urllib.request.build_opener(NoRedirect).open(url, timeout=20) as response:
            require(response.status == 200, "Public readiness failed")
