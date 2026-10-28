#!/usr/bin/env python3
"""Prepare the reviewed dependency platform update on an existing host, through human SSO.

Run a hash-verified copy outside /opt/agentclash as root through the approved SSM
operator channel. This command never migrates databases or starts applications.
"""

import argparse
import copy
import fcntl
import importlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import stat
import sys
import tarfile
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

INSTALL = Path("/opt/agentclash")
# Standalone copies on the host must import its installed contracts, not modules
# from an unpacked candidate. The local path only makes --help usable in Git.
LIBRARY = INSTALL if INSTALL.is_dir() else Path(__file__).resolve().parents[1]
sys.path[:0] = [str(LIBRARY / "deploy/aws/scripts"), str(LIBRARY / "delivery")]
from common import (
    Refused,
    account_guard,
    aws,
    digest,
    private_json,
    require,
    run,
    verify_blob,
)
from release_contract import (
    canonical,
    platform_files,
    platform_hash,
    sha256,
    validate_approval,
    validate_images,
    validate_release,
)

CHANGED = (
    "delivery/maintained.py",
    "deploy/aws/platform/inputs.lock.json",
    "deploy/aws/platform/caddy/go.mod",
    "deploy/aws/platform/caddy/go.sum",
    "deploy/aws/platform/temporal/go.mod",
    "deploy/aws/platform/temporal/go.sum",
    "deploy/aws/platform/temporal/cli/go.mod",
    "deploy/aws/platform/temporal/cli/go.sum",
)
ADDED = (
    "deploy/aws/platform/Dockerfile.caddy",
    "deploy/aws/platform/caddy/apply-jose-patch.sh",
    "deploy/aws/platform/caddy/JOSE-MIGRATION.md",
    "deploy/aws/platform/caddy/jose_migration_test.go",
)
METADATA = "tools/platform-source.sha256"
CONTROLS = (
    "compose.env",
    "current-release.json",
    "current-generation",
    "runtime-config-sha256",
    "deployed-release.json",
    "last-offline-backup.json",
)
BLOCKERS = (
    "candidate-release.json",
    "cleanup-unresolved",
    "restore-unverified",
    "recovery-unverified",
    "deployment-unresolved.json",
    "platform-update-unresolved.json",
)


def regular(path):
    require(
        path.is_file() and not path.is_symlink(), "Expected a regular installed file"
    )
    return path.read_bytes()


def sync_directory(path):
    directory = os.open(path, os.O_RDONLY)
    try:
        os.fsync(directory)
    finally:
        os.close(directory)


def replace(path, data, mode=0o600):
    """A pending write is evidence of interruption, never an implicit retry."""
    require(not path.is_symlink(), "Symlink destination refused")
    pending = path.with_name(path.name + ".platform-update-pending")
    fd = os.open(pending, os.O_WRONLY | os.O_CREAT | os.O_EXCL, mode)
    with os.fdopen(fd, "wb") as output:
        output.write(data)
        output.flush()
        os.fsync(output.fileno())
    os.replace(pending, path)
    sync_directory(path.parent)


def unpack(raw, root, destination, manifest):
    """An exact installed inventory and explicit source delta bound maintenance."""
    verify_blob(raw, manifest["bootstrap_sha256"])
    require(
        all(
            not (root / name).exists() and not (root / name).is_symlink()
            for name in ADDED
        ),
        "Reviewed additions must not already exist on the predecessor",
    )
    names = (
        set(platform_files(root))
        | set(ADDED)
        | {
            "tools/docker-compose",
            "tools/SHA256SUMS",
            METADATA,
        }
    )
    notices = root / "tools/licenses-compose"
    if notices.exists():
        require(not notices.is_symlink(), "Notice directory must not be a link")
        for path in notices.rglob("*"):
            require(not path.is_symlink(), "Notice links refused")
            if path.is_file():
                names.add(str(path.relative_to(root)))
    seen, total = set(), 0
    with tarfile.open(fileobj=io.BytesIO(raw), mode="r:gz") as archive:
        for member in archive:
            name = member.name
            path = PurePosixPath(name)
            require(
                member.isfile()
                and name in names
                and name not in seen
                and not path.is_absolute()
                and ".." not in path.parts
                and str(path) == name
                and member.size <= 256 * 1024**2,
                "Unexpected or unsafe bootstrap member",
            )
            total += member.size
            require(total <= 512 * 1024**2, "Bootstrap exceeds the bounded size")
            seen.add(name)
            target = destination / name
            target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            with archive.extractfile(member) as source, target.open("xb") as output:
                output.write(source.read())
            target.chmod(0o600)
    require(seen == names, "Incomplete bootstrap inventory")
    changed = {
        name
        for name in names
        if name in ADDED or regular(root / name) != regular(destination / name)
    }
    require(
        changed == {*CHANGED, *ADDED, METADATA},
        "This command permits only the reviewed dependency update",
    )
    validate_release(manifest, destination)
    require(
        regular(destination / METADATA) == (platform_hash(destination) + "\n").encode(),
        "Bootstrap source metadata mismatch",
    )


def approval_check(approval, settings, old, candidate, release_sha, previous):
    require(
        approval.get("format") == 1 and approval.get("operation") == "platform-update",
        "Platform update approval required",
    )
    expected = {
        "old_platform_source_sha256": old["platform_source_sha256"],
        "platform_source_sha256": candidate["platform_source_sha256"],
        "old_platform_images_sha256": settings["platform_images_sha256"],
        "platform_images_sha256": sha256(canonical(candidate["platform_images"])),
        "bootstrap_sha256": candidate["bootstrap_sha256"],
    }
    require(
        all(approval.get(key) == digest(value) for key, value in expected.items()),
        "Approved platform transition mismatch",
    )
    require(
        approval["acknowledgements"].get("isolated_rehearsal_verified") is True,
        "Reviewed isolated rehearsal is required",
    )
    # Retain the existing exact-target, predecessor, operator, evidence and <=1h
    # acceptance contract; platform maintenance adds its own stronger bindings.
    return validate_approval(
        {**approval, "operation": "deploy"}, settings, release_sha, previous, "deploy"
    )


def execute(h, approval, release_sha):
    """Caller holds the real deployment lock; host/cloud I/O stays on the host."""
    release_sha = digest(release_sha)
    require(
        not any((h.state / name).exists() for name in BLOCKERS),
        "Existing candidate or unresolved recovery requires reconciliation",
    )
    require(
        not any(h.state.glob("*.pending"))
        and not any(h.state.glob("*.platform-update-pending"))
        and not any(
            (h.state / name).exists()
            for name in ("compose.pending.env", "compose.delivery.env")
        ),
        "An interrupted state write requires reconciliation",
    )
    targets = [
        h.settings_path,
        *(h.root / name for name in (*CHANGED, *ADDED, METADATA)),
    ]
    require(
        not any(
            path.with_name(path.name + ".platform-update-pending").exists()
            for path in targets
        ),
        "An interrupted installed-file write requires reconciliation",
    )
    settings = copy.deepcopy(h.settings)
    controls = {name: regular(h.state / name) for name in CONTROLS}
    settings_raw = regular(h.settings_path)
    require(
        json.loads(settings_raw) == settings,
        "Private settings changed during preflight",
    )
    deployed = json.loads(controls["deployed-release.json"])
    previous_backup = digest(json.loads(controls["last-offline-backup.json"])["sha256"])
    previous = digest(deployed["release_sha256"])
    require(
        sha256(controls["current-release.json"]) == previous,
        "Prepared release differs from deployed predecessor",
    )
    old = json.loads(h.get("releases", previous))
    candidate = json.loads(h.get("releases", release_sha))
    for value in (old, candidate):
        require(
            value["account_id"] == settings["account_id"]
            and value["region"] == settings["region"]
            and value["platform"] == "linux/amd64",
            "Release destination mismatch",
        )
        validate_images(
            value,
            settings["repository"],
            json.loads(regular(h.root / "deploy/aws/images.lock.json")),
        )
    validate_release(old, h.root)
    require(
        old["platform_lock_sha256"]
        == candidate["platform_lock_sha256"]
        == sha256(regular(h.root / "deploy/aws/images.lock.json")),
        "Upstream lock changes require a separate platform release",
    )
    require(
        sha256(canonical(old["platform_images"])) == settings["platform_images_sha256"],
        "Installed platform map does not match the predecessor",
    )
    require(
        sha256(regular(h.root / "delivery/release.py"))
        == settings["delivery_driver_sha256"],
        "Installed delivery driver mismatch",
    )
    evidence_names = approval_check(
        approval, settings, old, candidate, release_sha, previous
    )
    build = json.loads(h.get("build-evidence", candidate["build_evidence_sha256"]))
    require(
        build.get("scanners_passed") is True
        and all(
            build[key] == candidate[key]
            for key in (
                "source_revision",
                "images",
                "platform_images",
                "platform_recipe_sha256",
                "platform_lock_sha256",
            )
        ),
        "Build evidence does not bind the exact candidate",
    )
    for name in evidence_names:
        evidence = json.loads(h.get("evidence/operator", approval["evidence"][name]))
        require(
            evidence["release_sha256"] == release_sha
            and evidence["kind"] == name
            and evidence["checks_passed"] is True,
            "Evidence does not approve this candidate",
        )
        if name == "rehearsal":
            require(
                evidence["environment"] == "staging" and evidence["isolated"] is True,
                "Rehearsal must be isolated",
            )
        else:
            require(
                evidence["environment"] == settings["delivery"]["environment"]
                and evidence["instance_id"] == settings["instance_id"],
                "Evidence target mismatch",
            )
    with tempfile.TemporaryDirectory(
        prefix="platform-stage-", dir=h.state
    ) as temporary:
        stage = Path(temporary)
        unpack(
            h.get("bundles", candidate["bootstrap_sha256"], ".tar.gz"),
            h.root,
            stage,
            candidate,
        )
        approval_check(approval, settings, old, candidate, release_sha, previous)
        approval_sha = sha256(canonical(approval))
        snapshot = h.state / ("platform-update-" + approval_sha)
        snapshot.mkdir(mode=0o700)
        for name in (*CHANGED, METADATA):
            path = snapshot / "installed" / name
            path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            replace(path, regular(h.root / name))
        (snapshot / "state").mkdir(mode=0o700)
        for name, data in controls.items():
            path = snapshot / "state" / name
            replace(path, data)
        replace(snapshot / "host.json", settings_raw)
        for directory in sorted(
            (p for p in snapshot.rglob("*") if p.is_dir()),
            key=lambda p: len(p.parts),
            reverse=True,
        ):
            sync_directory(directory)
        sync_directory(snapshot)
        sync_directory(h.state)
        owner = canonical(
            {"operation": "platform-update", "approval_sha256": approval_sha}
        )
        marker = h.state / "recovery-unverified"
        transaction = {
            "format": 1,
            "release_sha256": release_sha,
            "previous_release_sha256": previous,
            "approval_sha256": approval_sha,
            "snapshot_directory": str(snapshot),
            "previous_offline_backup_sha256": previous_backup,
            "added_source_paths": list(ADDED),
            "stage": "fence",
            "started_at": time.time(),
        }
        record = h.state / "platform-update-unresolved.json"
        try:
            with marker.open("xb") as output:
                marker.chmod(0o600)
                output.write(owner)
                output.flush()
                os.fsync(output.fileno())
            sync_directory(h.state)
            replace(record, canonical(transaction))
            h.fence()
            for name in ("drain", "stop", "backup", "stop_all"):
                transaction["stage"] = name
                replace(record, canonical(transaction))
                result = getattr(h, name)()
                if name == "backup":
                    transaction["backup_sha256"] = digest(result)
                    replace(record, canonical(transaction))
            approval_check(approval, settings, old, candidate, release_sha, previous)
            require(
                regular(h.settings_path) == settings_raw
                and regular(h.state / "deployed-release.json")
                == controls["deployed-release.json"],
                "Predecessor or settings changed during maintenance",
            )
            transaction["stage"] = "install"
            replace(record, canonical(transaction))
            for name in (*CHANGED, *ADDED, METADATA):
                target = h.root / name
                mode = 0o600 if name in ADDED else stat.S_IMODE(target.stat().st_mode)
                replace(target, regular(stage / name), mode)
            updated = {
                **settings,
                "platform_images_sha256": approval["platform_images_sha256"],
            }
            replace(h.settings_path, canonical(updated))
            transaction["stage"] = "prepare"
            replace(record, canonical(transaction))
            h.prepare(updated, release_sha)
            validate_release(candidate, h.root)
            h.verify_prepared(updated, candidate)
            transaction["stage"] = "start-platform"
            replace(record, canonical(transaction))
            h.start_platform()
            h.verify_prepared(updated, candidate)
            # prepare() predates the transaction journal and writes these files
            # directly. Flush its complete generation before unblocking delivery.
            for name in CONTROLS:
                with (h.state / name).open("rb") as output:
                    os.fsync(output.fileno())
            sync_directory(h.state)
            backup_receipt = regular(h.state / "last-offline-backup.json")
            require(
                json.loads(backup_receipt)["sha256"] == transaction["backup_sha256"],
                "Maintenance backup receipt changed",
            )
            replace(snapshot / "maintenance-backup.json", backup_receipt)
            require(
                regular(h.settings_path) == canonical(updated)
                and regular(h.state / "deployed-release.json")
                == controls["deployed-release.json"],
                "Settings or predecessor changed during preparation",
            )
            require(regular(marker) == owner, "Recovery marker ownership changed")
            transaction.update(stage="complete", completed_at=time.time())
            replace(snapshot / "operation.json", canonical(transaction))
            replace(h.state / "platform-update-completed.json", canonical(transaction))
            record.unlink()
            marker.unlink()
            sync_directory(h.state)
        except BaseException:
            h.emergency_fence()
            raise


class Host:
    def __init__(self):
        require(
            os.geteuid() == 0 and os.environ.get("GITHUB_ACTIONS") != "true",
            "Human SSO host-root operation required",
        )
        require(
            INSTALL.is_dir() and not Path(__file__).resolve().is_relative_to(INSTALL),
            "Run the verified operator helper outside the installed bootstrap",
        )
        for name in (
            "common",
            "release_contract",
            "secret_store",
            "render",
            "database",
            "host",
            "backup",
            "health",
            "release",
        ):
            module = importlib.import_module(name)
            expected = (
                INSTALL
                / (
                    "delivery"
                    if name in ("health", "release")
                    else "deploy/aws/scripts"
                )
                / (name + ".py")
            )
            require(
                Path(module.__file__).resolve() == expected,
                "Canonical installed module required",
            )
        self.host = importlib.import_module("host")
        self.health = importlib.import_module("health")
        self.release = importlib.import_module("release")
        self.root, self.state = INSTALL, self.host.STATE
        self.settings_path = Path("/etc/agentclash/host.json")
        self.settings = private_json(self.settings_path)
        require(
            stat.S_IMODE(self.settings_path.stat().st_mode) == 0o600,
            "Host settings must have mode 0600",
        )
        account_guard(self.settings)
        request = urllib.request.Request(
            "http://169.254.169.254/latest/api/token",
            method="PUT",
            headers={"X-aws-ec2-metadata-token-ttl-seconds": "60"},
        )
        with urllib.request.urlopen(request, timeout=3) as response:
            token = response.read().decode()
        request = urllib.request.Request(
            "http://169.254.169.254/latest/dynamic/instance-identity/document",
            headers={"X-aws-ec2-metadata-token": token},
        )
        with urllib.request.urlopen(request, timeout=3) as response:
            identity = json.load(response)
        require(
            all(
                identity[key] == self.settings[field]
                for key, field in (
                    ("accountId", "account_id"),
                    ("region", "region"),
                    ("instanceId", "instance_id"),
                )
            ),
            "Host identity mismatch",
        )
        self.host.runtime_guard(self.settings)
        self.adapter = self.release.Host(self.settings)

    def get(self, prefix, value, suffix=".json"):
        raw = aws(
            self.settings,
            "s3",
            "cp",
            "s3://"
            + self.settings["artifact_bucket"]
            + "/"
            + prefix
            + "/"
            + digest(value)
            + suffix,
            "-",
            "--only-show-errors",
        )
        verify_blob(raw, value)
        return raw

    def fence(self):
        self.host.fence()

    def drain(self):
        self.adapter.drain()

    def stop(self):
        self.adapter.stop()

    def backup(self):
        return self.adapter.backup()

    def stop_all(self):
        import health

        health.clean_stop(self.host, ("caddy",))
        require(
            not self.host.compose("ps", "--status", "running", "-q").strip(),
            "All services must stop before platform preparation",
        )

    def prepare(self, settings, release_sha):
        self.host.prepare(settings, release_sha)
        self.settings = settings

    def start_platform(self):
        self.host.fence()
        self.host.compose(
            "up", "-d", "--no-deps", "valkey", "temporal", "caddy", timeout=300
        )
        self.host.wait_for_platform()
        manifest = json.loads(regular(self.state / "current-release.json"))
        self.verify_listener(self.settings, manifest)

    def verify_listener(self, settings, manifest, timeout=60):
        urls = settings["delivery"]["public_readiness_urls"]
        require(len(urls) == 2, "Both public fence probes are required")
        for url, path in zip(urls, ("/healthz/ready", "/health/ready")):
            parsed = urllib.parse.urlsplit(url)
            require(
                parsed.scheme == "https"
                and parsed.hostname
                and parsed.path == path
                and not (
                    parsed.username
                    or parsed.password
                    or parsed.query
                    or parsed.fragment
                ),
                "Invalid public fence probe URL",
            )
        opener = urllib.request.build_opener(self.health.NoRedirect)
        deadline = time.monotonic() + timeout
        while True:
            try:
                ids = self.host.compose("ps", "--all", "-q", "caddy").decode().split()
                require(len(ids) == 1, "Expected exactly one Caddy container")
                for after in (False, True):
                    states = json.loads(run(["docker", "inspect", ids[0]]))
                    require(
                        len(states) == 1
                        and states[0]["Id"] == ids[0]
                        and states[0]["Config"]["Image"]
                        == manifest["platform_images"]["caddy"]
                        and states[0]["State"]["Running"]
                        and not states[0]["State"].get("OOMKilled"),
                        "Approved Caddy listener is not running",
                    )
                    if not after:
                        self.host.fence()  # Running above: this must reload, not skip.
                        for url in urls:
                            request = urllib.request.Request(
                                url, headers={"Cache-Control": "no-cache"}
                            )
                            try:
                                response = opener.open(request, timeout=5)
                            except urllib.error.HTTPError as error:
                                response = error
                            with response:
                                require(
                                    response.status == 503
                                    and response.read(64).strip() == b"Maintenance",
                                    "Public listener did not prove the closed fence",
                                )
                return
            except (Refused, KeyError, ValueError, OSError):
                require(
                    time.monotonic() < deadline,
                    "Closed Caddy readiness deadline exceeded",
                )
                time.sleep(2)

    def verify_prepared(self, settings, manifest):
        self.host.runtime_guard(settings)
        self.host.manifest_validate(manifest, settings)
        self.host.stopped()
        require(
            (self.state / "cache").is_mount()
            and regular(self.state / "cache/.volume-identity").decode().strip()
            == settings["cache_volume_id"],
            "Retained cache identity mismatch",
        )
        values = dict(
            line.split("=", 1)
            for line in regular(self.state / "compose.env").decode().splitlines()
        )
        for name, image in {
            **manifest["images"],
            **manifest["platform_images"],
        }.items():
            require(
                values["IMAGE_" + name.upper().replace("-", "_")] == image,
                "Prepared image map mismatch",
            )
        require(
            regular(self.host.RUNTIME / "edge/fence.caddy")
            == regular(self.root / "deploy/aws/config/fence.closed.caddy"),
            "Prepared fence is not closed",
        )

    def emergency_fence(self):
        self.adapter.emergency_fence()


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--approval",
        required=True,
        help="Root-owned 0600 private platform-update acceptance",
    )
    parser.add_argument(
        "--release",
        required=True,
        help="Exact privately built candidate manifest SHA256",
    )
    args = parser.parse_args()
    h = Host()
    require(
        not Path(args.approval).resolve().is_relative_to(INSTALL),
        "Private approval must be outside the bootstrap",
    )
    approval = private_json(args.approval)
    require(
        stat.S_IMODE(Path(args.approval).stat().st_mode) == 0o600,
        "Approval must have mode 0600",
    )
    with (h.state / "deployment.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        h.host.runtime_guard(h.settings)
        execute(h, approval, args.release)
    print("Platform preparation passed; applications remain stopped and intake closed")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        sys.exit(
            "Platform update refused or failed; inspect private transaction and recovery state before retrying"
        )
