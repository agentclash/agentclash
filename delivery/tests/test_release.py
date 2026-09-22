from pathlib import Path
import sys
import tempfile
import json
import time
import unittest
from unittest.mock import Mock, patch

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
from common import Refused
from release_contract import (
    canonical,
    sha256,
    validate_release,
    platform_hash,
    recipe_hash,
    validate_images,
    DERIVED,
    UPSTREAM,
)
from release import Host, execute
import health


class FixtureHost(Host):
    def __init__(self, directory, settings, failure=None):
        self.state = Path(directory)
        self.settings = settings
        self.failure = failure
        self.calls = []
        self.objects = {}
        self.closed = True

    def event(self, stage):
        self.calls.append(stage)
        if stage == self.failure:
            raise Refused("injected failure")

    def object(self, prefix, sha, checked=True):
        value = self.objects[prefix, sha]
        if checked:
            assert sha256(canonical(value)) == sha
        return value, sha256(canonical(value))

    def validate(self, manifest):
        validate_release(manifest, ROOT)
        validate_images(
            manifest,
            self.settings["repository"],
            json.loads((ROOT / "deploy/aws/images.lock.json").read_text()),
        )

    def fence(self, closed=True):
        self.closed = closed
        self.event("close" if closed else "open")

    def emergency_fence(self):
        self.fence()

    def drain(self):
        self.event("drain")

    def stop(self):
        self.event("stop")

    def select(self, manifest):
        self.event("select")

    def migrate(self):
        self.event("migrate")

    def start(self):
        self.event("start")

    def ready(self, *args):
        self.event("ready")

    def public_ready(self):
        self.event("public_ready")

    def record(self, value):
        self.event("record")
        return sha256(canonical(value))

    def backup(self):
        self.event("backup")
        return "b" * 64


class ReleaseTests(unittest.TestCase):
    def test_real_cli_timestamp_shape_and_no_stale_poller_acceptance(self):
        now = time.time()
        value = {"pollers": [{"last_access_time": {"seconds": int(now), "nanos": 100}}]}
        self.assertGreaterEqual(health.poller_times(value)[0], int(now))
        host = Mock()
        host.compose.side_effect = lambda *args, **kwargs: (
            b"one two three" if args[0] == "ps" else canonical(value)
        )
        states = [
            {
                "Config": {
                    "Image": self.manifest["images"][name],
                    "Labels": {"com.docker.compose.service": name},
                },
                "State": {"Running": True, "Health": {"Status": "healthy"}},
            }
            for name in ("api", "worker", "terminal")
        ]
        with patch("health.run", return_value=canonical(states)):
            health.ready(host, self.manifest, now - 5, timeout=0)
            with self.assertRaises(Refused):
                health.ready(host, self.manifest, now + 60, timeout=0)
            value["pollers"][0]["last_access_time"]["seconds"] = int(now - 600)
            with self.assertRaises(Refused):
                health.ready(host, self.manifest, now - 3600, timeout=0)

    def test_failed_edge_reload_stops_the_public_listener(self):
        h = Host.__new__(Host)
        h.host = Mock()
        h.host.fence.side_effect = Refused("reload refused")
        h.host.compose.return_value = b""
        h.emergency_fence()
        self.assertEqual(h.host.compose.call_args_list[0].args, ("stop", "caddy"))

    def test_current_image_can_be_redeployed_only_with_fresh_predecessor_approval(self):
        self.h.write("deployed-release.json", {"release_sha256": self.sha})
        with self.assertRaises(Refused):
            self.run_release()
        self.approve("deploy", self.sha)
        self.run_release()
        self.assertTrue(self.h.closed)

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix="release-test-")
        self.addCleanup(self.directory.cleanup)
        self.settings = {
            "account_id": str(1) * 12,
            "region": "test-region",
            "instance_id": "i-fixture",
            "operator": "fixture",
            "repository": "registry.example.com/app",
            "delivery": {"enabled": True, "environment": "production"},
        }
        self.h = FixtureHost(self.directory.name, self.settings)
        self.manifest = {
            "format": 2,
            "source_revision": "a" * 40,
            "migration_set_sha256": "b" * 64,
            "build_evidence_sha256": "c" * 64,
            "bootstrap_sha256": "d" * 64,
            "platform_source_sha256": platform_hash(ROOT),
            "platform_recipe_sha256": recipe_hash(ROOT),
            "platform_lock_sha256": sha256(
                (ROOT / "deploy/aws/images.lock.json").read_bytes()
            ),
            "platform_images": {
                **{n: "registry.example.com/app@sha256:" + "b" * 64 for n in DERIVED},
                **{
                    n: json.loads((ROOT / "deploy/aws/images.lock.json").read_text())[
                        "images"
                    ][n]["image"]
                    for n in UPSTREAM
                },
            },
            "images": {
                name: "registry.example.com/app@sha256:" + "a" * 64
                for name in ("api", "worker", "terminal", "app-schema")
            },
        }
        build = {
            "source_revision": self.manifest["source_revision"],
            "images": self.manifest["images"],
            "platform_images": self.manifest["platform_images"],
            "platform_recipe_sha256": self.manifest["platform_recipe_sha256"],
            "platform_lock_sha256": self.manifest["platform_lock_sha256"],
            "scanners_passed": True,
        }
        self.manifest["build_evidence_sha256"] = sha256(canonical(build))
        self.h.objects["build-evidence", self.manifest["build_evidence_sha256"]] = build
        self.sha = sha256(canonical(self.manifest))
        self.h.objects["releases", self.sha] = self.manifest
        self.approve("deploy")

    def approve(self, operation, previous=None):
        now = time.time()
        approval = {
            **{
                k: self.settings[k]
                for k in ("account_id", "region", "instance_id", "operator")
            },
            "environment": "production",
            "operation": operation,
            "release_sha256": self.sha,
            "previous_release_sha256": previous,
            "issued_at": now,
            "expires_at": now + 600,
            "acknowledgements": {
                key: True
                for key in (
                    "drain_verified",
                    "callbacks_reconciled",
                    "no_automatic_schema_rollback",
                    "no_previous_aws_release",
                    "destination_writes_acknowledged",
                )
            },
            "evidence": {},
        }
        for name in ("drain", "recovery", "rehearsal", "smoke"):
            value = {
                "kind": name,
                "release_sha256": self.sha,
                "checks_passed": True,
                "environment": "staging" if name == "rehearsal" else "production",
                "isolated": True,
                "instance_id": self.settings["instance_id"],
            }
            sha = sha256(canonical(value))
            approval["evidence"][name] = sha
            self.h.objects["evidence/operator", sha] = value
        self.h.objects["approvals/production/" + operation, self.sha] = approval
        return approval

    def run_release(self, operation="deploy"):
        execute(self.settings, self.sha, operation, self.h)

    def test_platform_attestation_mismatch_has_no_host_side_effects(self):
        evidence = dict(
            self.h.objects["build-evidence", self.manifest["build_evidence_sha256"]]
        )
        evidence["platform_images"] = {
            **evidence["platform_images"],
            "caddy": "registry.example.com/app@sha256:" + "e" * 64,
        }
        evidence_sha = sha256(canonical(evidence))
        manifest = {**self.manifest, "build_evidence_sha256": evidence_sha}
        release_sha = sha256(canonical(manifest))
        self.h.objects["build-evidence", evidence_sha] = evidence
        self.h.objects["releases", release_sha] = manifest
        with self.assertRaises(Refused):
            execute(self.settings, release_sha, "deploy", self.h)
        self.assertEqual(self.h.calls, [])

    def test_first_release_deploy_remains_closed_then_exact_promotion(self):
        self.run_release()
        self.assertEqual(
            self.h.calls,
            ["close", "drain", "stop", "backup", "select", "migrate", "start", "ready"],
        )
        self.assertTrue(self.h.closed)
        self.assertIsNone(self.h.read("deployed-release.json"))
        self.assertEqual(
            self.h.read("candidate-release.json")["backup_sha256"], "b" * 64
        )
        self.approve("promote")
        self.run_release("promote")
        self.assertFalse(self.h.closed)
        self.assertIsNone(self.h.read("candidate-release.json"))
        self.assertEqual(
            self.h.read("deployed-release.json")["release_sha256"], self.sha
        )
        self.assertIsNone(self.h.read("deployment-unresolved.json"))

    def test_each_failure_preserves_marker_fence_and_prevents_retry(self):
        for stage in ("drain", "stop", "backup", "select", "migrate", "start", "ready"):
            with self.subTest(stage=stage):
                self.h.failure = stage
                with self.assertRaises(Refused):
                    self.run_release()
                self.assertTrue(self.h.closed)
                self.assertEqual(
                    self.h.read("deployment-unresolved.json")["stage"], stage
                )
                before = self.h.calls[:]
                with self.assertRaises(Refused):
                    self.run_release()
                self.assertEqual(before, self.h.calls)
                if stage in ("stop", "backup"):
                    self.assertNotIn("select", self.h.calls)
                self.h.remove("deployment-unresolved.json")
                self.h.calls.clear()

    def test_failed_public_smoke_recloses_without_promotion(self):
        self.run_release()
        self.approve("promote")
        self.h.failure = "public_ready"
        with self.assertRaises(Refused):
            self.run_release("promote")
        self.assertTrue(self.h.closed)
        self.assertIsNone(self.h.read("deployed-release.json"))
        self.assertIsNotNone(self.h.read("deployment-unresolved.json"))

    def test_failed_history_write_recloses_and_keeps_candidate_for_reconciliation(self):
        self.run_release()
        self.approve("promote")
        self.h.failure = "record"
        with self.assertRaises(Refused):
            self.run_release("promote")
        self.assertTrue(self.h.closed)
        self.assertIsNone(self.h.read("deployed-release.json"))
        self.assertIsNotNone(self.h.read("candidate-release.json"))
        self.assertEqual(
            self.h.read("deployment-unresolved.json")["stage"], "record-promotion"
        )

    def test_mismatched_build_attestation_has_no_host_side_effects(self):
        build = self.h.objects["build-evidence", self.manifest["build_evidence_sha256"]]
        build["scanners_passed"] = False
        # Make the blob and manifest hashes internally consistent: the attestation
        # still must explicitly approve the exact candidate before any mutation.
        build_sha = sha256(canonical(build))
        self.h.objects["build-evidence", build_sha] = build
        self.manifest["build_evidence_sha256"] = build_sha
        self.sha = sha256(canonical(self.manifest))
        self.h.objects["releases", self.sha] = self.manifest
        with self.assertRaises(Refused):
            self.run_release()
        self.assertEqual(self.h.calls, [])

    def test_idempotent_deploy_does_not_repeat_writes(self):
        self.run_release()
        self.h.calls.clear()
        self.run_release()
        self.assertEqual(self.h.calls, ["ready"])

    def test_guards_have_no_side_effects(self):
        original = self.approve("deploy")
        variants = [
            {"expires_at": 0},
            {"operator": "other"},
            {"instance_id": "i-other"},
            {"previous_release_sha256": "f" * 64},
            {"environment": "staging"},
            {"release_sha256": "e" * 64},
            {"evidence": {}},
        ]
        for change in variants:
            with self.subTest(change=change):
                self.h.objects["approvals/production/deploy", self.sha] = {
                    **original,
                    **change,
                }
                with self.assertRaises(Refused):
                    self.run_release()
                self.assertEqual(self.h.calls, [])
                self.assertIsNone(self.h.read("deployment-unresolved.json"))

    def test_first_release_acknowledgement_required(self):
        approval = self.approve("deploy")
        approval["acknowledgements"].pop("no_previous_aws_release")
        with self.assertRaises(Refused):
            self.run_release()

    def test_stale_candidate_and_platform_change_refused(self):
        self.h.write(
            "candidate-release.json",
            {"release_sha256": "f" * 64, "previous_release_sha256": None},
        )
        with self.assertRaises(Refused):
            self.run_release("promote")
        changed = {**self.manifest, "platform_source_sha256": "f" * 64}
        with self.assertRaises(Refused):
            validate_release(changed, ROOT)

    def test_prior_cleanup_and_restore_markers_block_all_mutation(self):
        for name in ("cleanup-unresolved", "restore-unverified", "recovery-unverified"):
            (self.h.state / name).touch()
            with self.assertRaises(Refused):
                self.run_release()
            self.h.remove(name)
        self.assertEqual(self.h.calls, [])

    def test_private_state_mode(self):
        self.run_release()
        self.assertEqual(
            (self.h.state / "candidate-release.json").stat().st_mode & 0o777, 0o600
        )

    def test_later_release_requires_exact_previous_manifest(self):
        previous = "f" * 64
        self.h.write("deployed-release.json", {"release_sha256": previous})
        approval = self.approve("deploy", previous)
        approval["acknowledgements"].pop("no_previous_aws_release")
        self.run_release()
        self.assertEqual(
            self.h.read("candidate-release.json")["previous_release_sha256"], previous
        )

    def test_disabled_delivery_never_accesses_cloud(self):
        self.settings["delivery"]["enabled"] = False
        with self.assertRaises(Refused):
            self.run_release()
        self.assertEqual(self.h.calls, [])


if __name__ == "__main__":
    unittest.main()
