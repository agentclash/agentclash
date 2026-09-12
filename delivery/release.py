"""Protected host transaction. Called only by host.py while its lock is held.

No subprocess invokes another locking host entrypoint. Runtime configuration and
platform binaries are supplied by the separately approved immutable bootstrap.
"""

import json
import os
from pathlib import Path
import sys
import time

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "deploy/aws/scripts"))
sys.path.insert(0, str(Path(__file__).resolve().parent))
from common import aws, digest, require, verify_blob, run
from release_contract import canonical, sha256, validate_approval, validate_release
import health


class Host:
    def __init__(self, settings):
        import host

        self.host = host
        self.settings = settings
        self.state = host.STATE

    def read(self, name):
        path = self.state / name
        return json.loads(path.read_bytes()) if path.exists() else None

    def write(self, name, value):
        pending = self.state / (name + ".pending")
        require(not pending.exists(), "Previous write was interrupted")
        with pending.open("xb") as stream:
            stream.write(canonical(value))
            stream.flush()
            os.fsync(stream.fileno())
        pending.chmod(0o600)
        os.replace(pending, self.state / name)
        fd = os.open(self.state, os.O_RDONLY)
        try:
            os.fsync(fd)
        finally:
            os.close(fd)

    def remove(self, name):
        (self.state / name).unlink(missing_ok=True)

    def object(self, prefix, sha, checked=True):
        raw = aws(
            self.settings,
            "s3",
            "cp",
            "s3://"
            + self.settings["artifact_bucket"]
            + "/"
            + prefix
            + "/"
            + digest(sha)
            + ".json",
            "-",
            "--only-show-errors",
        )
        if checked:
            verify_blob(raw, sha)
        return json.loads(raw), sha256(raw)

    def validate(self, manifest):
        self.host.manifest_validate(manifest, self.settings)
        validate_release(manifest, self.host.ROOT.parents[1])

    def fence(self, closed=True):
        self.host.fence(closed)

    def drain(self):
        # Detailed SQL/session/provider drain is reviewed before this final fence.
        # This signal also blocks any terminal admission bypassing the edge.
        if self.host.compose("ps", "--status", "running", "-q", "terminal").strip():
            self.host.compose("kill", "-s", "SIGUSR2", "terminal")
        health.drained(self.host, self.settings)

    def stop(self):
        self.host.stop_applications()
        health.clean_stop(self.host, ("temporal", "valkey"))
        health.no_database_writers(self.host)

    def backup(self):
        from backup import backup

        backup(self.settings, "offline")
        return self.read("last-offline-backup.json")["sha256"]

    def select(self, manifest):
        # Preserve the pinned secret generation and platform images. Only the four
        # candidate application references can change in an application release.
        path = self.state / "compose.env"
        content = path.read_text().splitlines()
        updates = {
            "IMAGE_" + name.upper().replace("-", "_"): image
            for name, image in manifest["images"].items()
        }
        require(
            set(updates) <= {line.split("=", 1)[0] for line in content},
            "Incomplete runtime image environment",
        )
        text = (
            "\n".join(
                key + "=" + updates.get(key, value)
                for key, value in (line.split("=", 1) for line in content)
            )
            + "\n"
        )
        pending = self.state / "compose.delivery.env"
        require(not pending.exists(), "Unresolved candidate environment")
        pending.write_text(text)
        pending.chmod(0o600)
        os.replace(pending, path)
        self.host.compose("config", "--quiet")
        # Authentication is confined to the root-owned host Docker config; runtime
        # containers never receive it. Password is sent over stdin, not argv.
        password = aws(self.settings, "ecr", "get-login-password")
        registry = self.settings["repository"].split("/")[0]
        run(
            ["docker", "login", "--username", "AWS", "--password-stdin", registry],
            data=password,
        )
        self.host.compose(
            "pull", "api", "worker", "terminal", "app-schema", timeout=600
        )

    def migrate(self):
        from database import grants_sql

        self.host.compose("run", "--rm", "-T", "--no-deps", "app-schema", timeout=2100)
        self.host.compose(
            "run",
            "--rm",
            "-T",
            "--no-deps",
            "database-admin",
            "psql",
            "-X",
            "-v",
            "ON_ERROR_STOP=1",
            data=grants_sql("agentclash").encode(),
        )

    def start(self):
        self.host.compose("up", "-d", "--no-deps", "valkey", "temporal", timeout=300)
        self.host.wait_for_platform()
        self.host.compose(
            "up", "-d", "--no-deps", "worker", "api", "terminal", timeout=300
        )

    def ready(self, manifest, started_at):
        self.host.wait_for_platform()
        health.ready(self.host, manifest, started_at)

    def public_ready(self):
        health.public_ready(self.settings)


def execute(settings, release_sha, operation="deploy", adapter=None):
    h = adapter or Host(settings)
    release_sha = digest(release_sha)
    require(settings["delivery"].get("enabled") is True, "Delivery is not activated")
    for name in (
        "cleanup-unresolved",
        "restore-unverified",
        "recovery-unverified",
        "deployment-unresolved.json",
    ):
        require(not (h.state / name).exists(), "Unresolved recovery marker")
    manifest, _ = h.object("releases", release_sha)
    h.validate(manifest)
    previous_record = h.read("deployed-release.json")
    previous = previous_record["release_sha256"] if previous_record else None
    candidate = h.read("candidate-release.json")
    if (
        operation == "deploy"
        and candidate
        and candidate["release_sha256"] == release_sha
    ):
        h.ready(manifest, candidate["started_at"])
        return  # Idempotent SSM retry, never repeats the migration or backup.
    if operation == "promote" and previous == release_sha and not candidate:
        h.ready(manifest, previous_record["started_at"])
        h.public_ready()
        return
    require(operation in ("deploy", "promote"), "Unknown operation")
    if operation == "deploy":
        require(
            candidate is None and previous != release_sha,
            "An existing candidate requires reconciliation",
        )
    else:
        require(
            candidate
            and candidate["release_sha256"] == release_sha
            and candidate["previous_release_sha256"] == previous,
            "Promotion candidate mismatch",
        )
    prefix = "approvals/" + settings["delivery"]["environment"] + "/" + operation
    approval, approval_sha = h.object(prefix, release_sha, checked=False)
    evidence_names = validate_approval(
        approval, settings, release_sha, previous, operation
    )
    for name in evidence_names:
        evidence, _ = h.object("evidence/operator", approval["evidence"][name])
        require(
            evidence["release_sha256"] == release_sha
            and evidence["kind"] == name
            and evidence["checks_passed"] is True,
            "Evidence does not approve this candidate",
        )
        # Rehearsal evidence is intentionally from isolated staging; all other
        # evidence is bound to this host/environment to prevent cross-host reuse.
        if name == "rehearsal":
            require(
                evidence["environment"] == "staging" and evidence["isolated"] is True,
                "Rehearsal was not isolated",
            )
        else:
            require(
                evidence["environment"] == settings["delivery"]["environment"]
                and evidence["instance_id"] == settings["instance_id"],
                "Evidence target mismatch",
            )
    transaction = {
        "release_sha256": release_sha,
        "operation": operation,
        "previous_release_sha256": previous,
        "approval_sha256": approval_sha,
        "stage": "fence",
    }
    h.write("deployment-unresolved.json", transaction)
    try:
        h.fence()
        if operation == "deploy":
            for stage in (
                "drain",
                "stop",
                "backup",
                "select",
                "migrate",
                "start",
                "ready",
            ):
                transaction["stage"] = stage
                h.write("deployment-unresolved.json", transaction)
                if stage == "backup":
                    transaction["backup_sha256"] = h.backup()
                elif stage == "select":
                    h.select(manifest)
                elif stage == "start":
                    transaction["started_at"] = time.time()
                    h.start()
                elif stage == "ready":
                    h.ready(manifest, transaction["started_at"])
                else:
                    getattr(h, stage)()
            h.write("current-release.json", manifest)
            h.write("candidate-release.json", transaction)
        else:
            h.ready(manifest, candidate["started_at"])
            h.fence(False)
            h.public_ready()
            h.write(
                "deployed-release.json",
                {
                    **candidate,
                    "promotion_approval_sha256": approval_sha,
                    "promoted_at": time.time(),
                },
            )
            h.remove("candidate-release.json")
        h.remove("deployment-unresolved.json")
    except BaseException:
        # Even cancellation/interrupt leaves a marker. Reclosing is attempted,
        # never interpreted as a rollback or as successful cleanup.
        h.fence()
        raise


if __name__ == "__main__":
    sys.exit("Use the serialized host.py deploy/promote entrypoint")
