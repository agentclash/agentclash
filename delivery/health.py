"""Quiet live host checks, also used by the release state-machine adapter."""

import json
import time
import urllib.request
import urllib.parse
from pathlib import Path

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
        "list",
        "--query",
        'ExecutionStatus="Running"',
        "--limit",
        "1",
        "--output",
        "json",
    )
    # The pinned CLI omits zero-valued count fields. An empty workflow list
    # proves the same drain condition without depending on that encoding.
    require(json.loads(raw) == [], "Open Temporal work remains or response is invalid")
    inventory = json.loads(database_query(host, (Path(__file__).parent / "vibe-drain.sql").read_text()))
    require(
        isinstance(inventory, dict)
        and set(inventory) == {"operations", "outbox", "continuations", "attempts", "holds", "input_work", "cleanup", "enquiries"}
        and all(type(count) is int and count == 0 for count in inventory.values()),
        "Vibe database work or unreconciled holds remain",
    )
    require(
        settings["delivery"]["namespace"] == "agentclash-prod",
        "Use an isolated cluster for rehearsal",
    )


def database_query(host, query):
    return host.compose(
        "run", "--rm", "-T", "--no-deps", "database-admin",
        "psql", "-X", "-At", "-d", "agentclash", "-c", query,
        timeout=30,
    )


def vibe_runtime(host):
    # The candidate worker reads its own pinned secret generation and validates
    # its installed parser. No inference, workflow or database work is started.
    value = json.loads(host.compose(
        "run", "--rm", "-T", "--no-deps", "worker", "/app", "--check-vibe-runtime",
        timeout=30,
    ))
    require(
        isinstance(value, dict)
        and set(value) == {"enabled", "pdf_required", "pdf_available"}
        and all(type(flag) is bool for flag in value.values()),
        "Invalid Vibe runtime receipt",
    )
    require(not value["pdf_required"] or value["pdf_available"], "Required PDF reader is unavailable")
    return value


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


def poller_times(value):
    # CLI 1.7.3 legacy mode uses Go's JSON encoding of protobuf timestamps,
    # not protojson's RFC3339 camelCase fields. Verified by the real rehearsal.
    return [
        float(p["last_access_time"]["seconds"])
        + float(p["last_access_time"].get("nanos", 0)) / 1_000_000_000
        for p in value.get("pollers", [])
    ]


def ready(host, manifest, started_at, timeout=240):
    expected = vibe_runtime(host)
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
            queues = ["execution", "scoring", "background"]
            if expected["enabled"]:
                queues.append("vibe-evals")
            for queue in queues:
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
                    # A promotion may happen hours after deployment. A poller
                    # seen only at startup is not evidence that it still polls.
                    fresh_after = max(started_at, time.time() - 120)
                    require(
                        any(
                            accessed >= fresh_after for accessed in poller_times(value)
                        ),
                        "Missing fresh worker pollers",
                    )
            if expected["pdf_required"]:
                # Heartbeats carry a 90-second lease. Requiring its inferred
                # last update after this deployment excludes old workers.
                fresh_after = max(started_at, time.time() - 90)
                raw = database_query(host, "SELECT EXISTS(SELECT 1 FROM vibe_input_workers WHERE expires_at>now() AND expires_at-interval '90 seconds'>=to_timestamp(" + str(fresh_after) + "));" )
                require(raw.strip() == b"t", "Missing fresh PDF reader heartbeat")
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
