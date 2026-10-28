"""The maintenance boundary owns state preservation and failed-transition fencing."""

import copy
import io
import json
from pathlib import Path
import sys
import tarfile
import tempfile
import time
import types
import unittest
import urllib.error
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
from common import Refused, verify_blob
from release_contract import (
    APPLICATIONS,
    DERIVED,
    canonical,
    platform_hash,
    recipe_hash,
    sha256,
)
from platform_update import Host, execute


class PlatformUpdateTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name)
        self.root = self.base / "installed"
        self.state = self.base / "state"
        self.state.mkdir()
        self.settings_path = self.base / "host.json"
        files = {
            "delivery/release.py": "unchanged driver",
            "delivery/health.py": "unchanged health",
            "delivery/tools.lock.json": "{}",
            "delivery/maintained.py": "old maintained packages",
            "deploy/aws/compose.yaml": "unchanged compose",
            "deploy/aws/images.lock.json": json.dumps(
                {"images": {"valkey": {"image": "public/valkey@sha256:" + "f" * 64}}}
            ),
            "deploy/aws/platform/inputs.lock.json": "old input pins",
            "deploy/aws/scripts/host.py": "unchanged host",
            "tools/docker-compose": "unchanged executable",
            "tools/SHA256SUMS": "unchanged checksum",
        }
        for name, value in files.items():
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(value)
        (self.root / "tools/platform-source.sha256").write_text(
            platform_hash(self.root) + "\n"
        )
        self.settings = {
            "account_id": "1" * 12,
            "region": "ap-southeast-1",
            "instance_id": "i-" + "a" * 17,
            "operator": "maintenance-owner",
            "repository": "registry.example/app",
            "artifact_bucket": "private-artifacts",
            "database_endpoint": "retained-rds",
            "cache_volume_id": "retained-ebs",
            "cache_uuid": "retained-uuid",
            "secrets": {"api": {"version_id": "pinned-secret"}},
            "delivery_driver_sha256": sha256(files["delivery/release.py"].encode()),
            "delivery": {
                "enabled": True,
                "environment": "production",
                "namespace": "agentclash-prod",
            },
        }
        self.old = self.manifest("a", self.root, "b")
        self.settings["platform_images_sha256"] = sha256(
            canonical(self.old["platform_images"])
        )
        self.settings_path.write_bytes(canonical(self.settings))
        self.settings_path.chmod(0o600)
        self.old_sha = sha256(canonical(self.old))
        self.deployed = canonical(
            {
                "release_sha256": self.old_sha,
                "previous_release_sha256": None,
                "backup_sha256": "1" * 64,
            }
        )
        self.original_controls = {
            "deployed-release.json": self.deployed,
            "current-release.json": canonical(self.old),
            "current-generation": b"/run/agentclash/generation-retained",
            "runtime-config-sha256": b"original-fingerprint",
            "compose.env": b"SECRETS_ROOT=/run/agentclash/generation-retained\n",
            "last-offline-backup.json": canonical({"sha256": "2" * 64}),
        }
        for name, data in self.original_controls.items():
            (self.state / name).write_bytes(data)
        self.candidate_root = self.base / "candidate"
        import shutil

        shutil.copytree(self.root, self.candidate_root)
        (self.candidate_root / "delivery/maintained.py").write_text(
            "new maintained packages"
        )
        (self.candidate_root / "deploy/aws/platform/inputs.lock.json").write_text(
            "new input pins"
        )
        (self.candidate_root / "tools/platform-source.sha256").write_text(
            platform_hash(self.candidate_root) + "\n"
        )
        self.candidate = self.manifest("c", self.candidate_root, "d")
        self.objects = {}
        self.refresh_bundle()
        self.objects["releases", self.old_sha, ".json"] = canonical(self.old)
        now = time.time()
        self.approval = {
            **{
                k: self.settings[k]
                for k in ("account_id", "region", "instance_id", "operator")
            },
            "format": 1,
            "operation": "platform-update",
            "environment": "production",
            "previous_release_sha256": self.old_sha,
            "release_sha256": self.candidate_sha,
            "old_platform_source_sha256": self.old["platform_source_sha256"],
            "platform_source_sha256": self.candidate["platform_source_sha256"],
            "old_platform_images_sha256": self.settings["platform_images_sha256"],
            "platform_images_sha256": sha256(
                canonical(self.candidate["platform_images"])
            ),
            "bootstrap_sha256": self.candidate["bootstrap_sha256"],
            "issued_at": now,
            "expires_at": now + 600,
            "acknowledgements": {
                k: True
                for k in (
                    "drain_verified",
                    "callbacks_reconciled",
                    "no_automatic_schema_rollback",
                    "destination_writes_acknowledged",
                    "isolated_rehearsal_verified",
                )
            },
            "evidence": {},
        }
        self.bind_candidate()
        self.h = FixtureHost(self)

    def bind_candidate(self):
        self.approval.update(
            release_sha256=self.candidate_sha,
            bootstrap_sha256=self.candidate["bootstrap_sha256"],
            platform_source_sha256=self.candidate["platform_source_sha256"],
        )
        for kind in ("drain", "recovery", "rehearsal"):
            value = {
                "kind": kind,
                "release_sha256": self.candidate_sha,
                "checks_passed": True,
                "environment": "staging" if kind == "rehearsal" else "production",
                "instance_id": self.settings["instance_id"],
                "isolated": True,
            }
            digest = sha256(canonical(value))
            self.approval["evidence"][kind] = digest
            self.objects["evidence/operator", digest, ".json"] = canonical(value)

    def manifest(self, source, root, image):
        return {
            "format": 2,
            "source_revision": source * 40,
            "platform": "linux/amd64",
            "account_id": self.settings["account_id"],
            "region": self.settings["region"],
            "migration_set_sha256": "e" * 64,
            "build_evidence_sha256": "e" * 64,
            "bootstrap_sha256": "e" * 64,
            "platform_source_sha256": platform_hash(root),
            "platform_recipe_sha256": recipe_hash(root),
            "platform_lock_sha256": sha256(
                (root / "deploy/aws/images.lock.json").read_bytes()
            ),
            "images": {
                name: "registry.example/app@sha256:" + image * 64
                for name in APPLICATIONS
            },
            "platform_images": {
                **{
                    name: "registry.example/app@sha256:" + image * 64
                    for name in DERIVED
                },
                "valkey": "public/valkey@sha256:" + "f" * 64,
            },
        }

    def refresh_bundle(self, extra=None):
        output = io.BytesIO()
        with tarfile.open(fileobj=output, mode="w:gz") as archive:
            for path in sorted(self.candidate_root.rglob("*")):
                if path.is_file():
                    data = path.read_bytes()
                    member = tarfile.TarInfo(str(path.relative_to(self.candidate_root)))
                    member.size = len(data)
                    archive.addfile(member, io.BytesIO(data))
            if extra:
                archive.addfile(extra, io.BytesIO(b"x" * extra.size))
        raw = output.getvalue()
        self.candidate["bootstrap_sha256"] = sha256(raw)
        build = {
            k: self.candidate[k]
            for k in (
                "source_revision",
                "images",
                "platform_images",
                "platform_recipe_sha256",
                "platform_lock_sha256",
            )
        }
        build["scanners_passed"] = True
        digest = sha256(canonical(build))
        self.candidate["build_evidence_sha256"] = digest
        self.objects["build-evidence", digest, ".json"] = canonical(build)
        self.candidate_sha = sha256(canonical(self.candidate))
        self.objects["releases", self.candidate_sha, ".json"] = canonical(
            self.candidate
        )
        self.objects["bundles", self.candidate["bootstrap_sha256"], ".tar.gz"] = raw

    def update(self):
        return execute(self.h, self.approval, self.candidate_sha)

    def test_update_prepares_new_platform_without_changing_secrets_or_predecessor(self):
        self.update()
        self.assertEqual(
            (self.state / "deployed-release.json").read_bytes(), self.deployed
        )
        updated = json.loads(self.settings_path.read_bytes())
        self.assertEqual(
            {
                **updated,
                "platform_images_sha256": self.settings["platform_images_sha256"],
            },
            self.settings,
        )
        self.assertEqual(
            platform_hash(self.root), self.candidate["platform_source_sha256"]
        )
        self.assertTrue(self.h.fenced)
        self.assertFalse(self.h.applications_running)
        self.assertTrue(self.h.platform_running)
        self.assertFalse((self.state / "recovery-unverified").exists())
        record = json.loads(
            (self.state / "platform-update-completed.json").read_bytes()
        )
        self.assertEqual(record["previous_offline_backup_sha256"], "2" * 64)
        self.assertEqual(record["backup_sha256"], "3" * 64)
        snapshots = Path(record["snapshot_directory"])
        for name, data in self.original_controls.items():
            self.assertEqual((snapshots / "state" / name).read_bytes(), data)

    def test_pending_state_is_refused_without_host_changes(self):
        for name in (
            "candidate-release.json",
            "cleanup-unresolved",
            "restore-unverified",
            "recovery-unverified",
            "deployment-unresolved.json",
            "platform-update-unresolved.json",
        ):
            with self.subTest(name=name):
                marker = self.state / name
                marker.write_text("existing")
                with self.assertRaises(Refused):
                    self.update()
                self.assertEqual(marker.read_text(), "existing")
                self.assertEqual(self.h.events, [])
                marker.unlink()

    def test_approval_mismatch_or_expiry_never_stops_current_runtime(self):
        for key, value in (
            ("instance_id", "i-wrong"),
            ("previous_release_sha256", "0" * 64),
            ("bootstrap_sha256", "0" * 64),
            ("platform_images_sha256", "0" * 64),
            ("expires_at", 0),
        ):
            original = self.approval[key]
            self.approval[key] = value
            with self.subTest(key=key), self.assertRaises(Refused):
                self.update()
            self.assertEqual(self.h.events, [])
            self.approval[key] = original

    def test_changed_scripts_are_refused_even_with_matching_new_hashes(self):
        (self.candidate_root / "deploy/aws/scripts/host.py").write_text(
            "unapproved new driver"
        )
        self.candidate["platform_source_sha256"] = platform_hash(self.candidate_root)
        (self.candidate_root / "tools/platform-source.sha256").write_text(
            self.candidate["platform_source_sha256"] + "\n"
        )
        self.refresh_bundle()
        self.bind_candidate()
        with self.assertRaisesRegex(Refused, "only the reviewed two-file"):
            self.update()
        self.assertEqual(self.h.events, [])

    def test_unsafe_bundle_members_are_refused_before_fencing(self):
        for name, kind in (
            ("../escape", tarfile.REGTYPE),
            ("unexpected", tarfile.REGTYPE),
            ("linked", tarfile.SYMTYPE),
            ("delivery/health.py", tarfile.REGTYPE),
            ("hardlink", tarfile.LNKTYPE),
        ):
            with self.subTest(name=name):
                member = tarfile.TarInfo(name)
                member.type, member.size = kind, 1 if kind == tarfile.REGTYPE else 0
                member.linkname = "outside"
                self.refresh_bundle(member)
                self.bind_candidate()
                with self.assertRaisesRegex(Refused, "unsafe bootstrap member"):
                    self.update()
                self.assertEqual(self.h.events, [])

    def assert_failed_transition(self, failure):
        self.h.failure = failure
        with self.assertRaisesRegex(Refused, "injected host failure"):
            self.update()
        self.assertTrue(self.h.fenced)
        self.assertTrue((self.state / "recovery-unverified").exists())
        transaction = json.loads(
            (self.state / "platform-update-unresolved.json").read_bytes()
        )
        self.assertEqual(transaction["release_sha256"], self.candidate_sha)
        self.assertEqual(
            (self.state / "deployed-release.json").read_bytes(), self.deployed
        )
        self.assertFalse((self.state / "platform-update-completed.json").exists())

    def test_drain_failure_leaves_recognized_recovery_block(self):
        self.assert_failed_transition("drain")

    def test_backup_failure_keeps_old_platform_and_predecessor(self):
        self.assert_failed_transition("backup")
        self.assertEqual(platform_hash(self.root), self.old["platform_source_sha256"])

    def test_failed_caddy_stop_does_not_install_platform_files(self):
        self.assert_failed_transition("stop-all")
        self.assertEqual(platform_hash(self.root), self.old["platform_source_sha256"])

    def test_post_install_failure_does_not_claim_success_or_remove_owned_marker(self):
        self.assert_failed_transition("prepare")
        self.assertEqual(
            platform_hash(self.root), self.candidate["platform_source_sha256"]
        )

    def test_platform_start_failure_keeps_applications_stopped_and_intake_closed(self):
        self.assert_failed_transition("start-platform")
        self.assertFalse(self.h.applications_running)

    def test_failed_caddy_readiness_keeps_recognized_recovery_marker(self):
        self.assert_failed_transition("caddy-readiness")
        self.assertFalse(self.h.applications_running)

    def test_listener_requires_running_exact_image_and_real_closed_http_responses(self):
        live = Host.__new__(Host)
        live.host = types.SimpleNamespace(
            compose=lambda *args: b"caddy-id", fence=lambda: None
        )
        live.health = types.SimpleNamespace(
            NoRedirect=__import__(
                "urllib.request", fromlist=["HTTPRedirectHandler"]
            ).HTTPRedirectHandler
        )
        settings = {
            "delivery": {
                "public_readiness_urls": [
                    "https://api.example/healthz/ready",
                    "https://terminal.example/health/ready",
                ]
            }
        }
        state = {
            "Id": "caddy-id",
            "Config": {"Image": self.candidate["platform_images"]["caddy"]},
            "State": {"Running": True, "OOMKilled": False},
        }

        def closed(request, timeout):
            raise urllib.error.HTTPError(
                request.full_url, 503, "maintenance", {}, io.BytesIO(b"Maintenance")
            )

        opener = types.SimpleNamespace(open=closed)
        with (
            patch("platform_update.run", return_value=canonical([state])),
            patch("platform_update.urllib.request.build_opener", return_value=opener),
        ):
            live.verify_listener(settings, self.candidate, timeout=0)
            for change in ({"Running": False}, {"OOMKilled": True}):
                with self.subTest(state=change):
                    bad = {**state, "State": {**state["State"], **change}}
                    with patch("platform_update.run", return_value=canonical([bad])):
                        with self.assertRaises(Refused):
                            live.verify_listener(settings, self.candidate, timeout=0)
            bad = {**state, "Config": {"Image": "unapproved-image"}}
            with patch("platform_update.run", return_value=canonical([bad])):
                with self.assertRaises(Refused):
                    live.verify_listener(settings, self.candidate, timeout=0)
            for status, body in (
                (200, b"Maintenance"),
                (503, b"upstream unavailable"),
                (502, b"Maintenance"),
            ):

                def wrong(request, timeout):
                    raise urllib.error.HTTPError(
                        request.full_url, status, "wrong response", {}, io.BytesIO(body)
                    )

                with self.subTest(status=status, body=body):
                    opener.open = wrong
                    with self.assertRaises(Refused):
                        live.verify_listener(settings, self.candidate, timeout=0)

    def test_first_journal_write_failure_still_fences_after_creating_recovery_marker(
        self,
    ):
        import platform_update

        original = platform_update.os.replace

        def failed_replace(source, destination):
            if destination == self.state / "platform-update-unresolved.json":
                raise OSError("journal filesystem failure")
            return original(source, destination)

        with patch("platform_update.os.replace", side_effect=failed_replace):
            with self.assertRaisesRegex(OSError, "journal filesystem failure"):
                self.update()
        self.assertTrue((self.state / "recovery-unverified").exists())
        self.assertTrue(self.h.fenced)
        self.assertEqual(
            (self.state / "deployed-release.json").read_bytes(), self.deployed
        )

    def test_settings_write_failure_retains_old_settings_and_recovery_snapshot(self):
        import platform_update

        original = platform_update.os.replace

        def failed_replace(source, destination):
            if destination == self.settings_path:
                raise OSError("settings filesystem failure")
            return original(source, destination)

        with patch("platform_update.os.replace", side_effect=failed_replace):
            with self.assertRaisesRegex(OSError, "settings filesystem failure"):
                self.update()
        self.assertEqual(json.loads(self.settings_path.read_bytes()), self.settings)
        self.assertTrue((self.state / "recovery-unverified").exists())
        self.assertTrue(self.h.fenced)
        self.assertFalse(self.h.applications_running)


class FixtureHost:
    def __init__(self, test):
        self.root, self.state, self.settings_path = (
            test.root,
            test.state,
            test.settings_path,
        )
        self.settings = copy.deepcopy(test.settings)
        self.objects = test.objects
        self.events, self.failure = [], None
        self.fenced, self.applications_running, self.platform_running = (
            False,
            True,
            True,
        )

    def get(self, prefix, digest, suffix=".json"):
        data = self.objects[prefix, digest, suffix]
        verify_blob(data, digest)
        return data

    def event(self, name):
        self.events.append(name)
        if name == self.failure:
            raise Refused("injected host failure")

    def fence(self):
        self.fenced = True
        self.event("fence")

    def drain(self):
        self.event("drain")

    def stop(self):
        self.event("stop")
        self.applications_running = False
        self.platform_running = False

    def backup(self):
        self.event("backup")
        (self.state / "last-offline-backup.json").write_bytes(
            canonical({"sha256": "3" * 64})
        )
        return "3" * 64

    def stop_all(self):
        self.event("stop-all")
        self.platform_running = False

    def prepare(self, settings, release_sha):
        self.event("prepare")
        self.settings = settings
        (self.state / "current-release.json").write_bytes(
            self.get("releases", release_sha)
        )
        (self.state / "current-generation").write_text("/run/agentclash/generation-new")
        (self.state / "compose.env").write_text(
            "SECRETS_ROOT=/run/agentclash/generation-new\n"
        )
        (self.state / "runtime-config-sha256").write_text("new-fingerprint")

    def start_platform(self):
        self.event("start-platform")
        self.platform_running = True
        self.event("caddy-readiness")

    def verify_prepared(self, settings, manifest):
        self.event("verify-prepared")
        if self.applications_running or not self.fenced:
            raise Refused("unsafe prepared runtime")

    def emergency_fence(self):
        self.fenced = True


if __name__ == "__main__":
    unittest.main()
