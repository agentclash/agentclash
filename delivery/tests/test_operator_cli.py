import copy
from contextlib import redirect_stdout
import io
import json
import os
from pathlib import Path
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
import operator_cli
from common import Refused


class ApprovalStore:
    def __init__(self):
        self.uploads = []

    def release(self, source):
        return "c" * 64, {}

    def call(self, *args):
        if args[:2] != ("s3api", "put-object"):
            raise AssertionError("Unexpected cloud operation")
        self.uploads.append(
            {
                "key": args[args.index("--key") + 1],
                "body": json.loads(Path(args[args.index("--body") + 1]).read_text()),
            }
        )


class OperatorApprovalTests(unittest.TestCase):
    def setUp(self):
        previous_umask = os.umask(0o077)
        self.addCleanup(os.umask, previous_umask)
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.config_path = Path(self.directory.name) / "config.json"
        self.approval_path = Path(self.directory.name) / "approval.json"
        self.store = ApprovalStore()
        self.now = int(time.time())
        self.config = {
            "enabled": True,
            "kind": "staging",
            "account_id": "111111111111",
            "region": "ap-southeast-1",
            "artifact_bucket": "fixture-artifacts",
            "repository": "111111111111.dkr.ecr.ap-southeast-1.amazonaws.com/app",
            "instance_id": "i-aaaaaaaaaaaaaaaaa",
            "operator": "fixture-operator",
            "expires_at": self.now + 3600,
            "documents": {
                op: {"name": "fixture-" + op, "version": "1", "sha256": "b" * 64}
                for op in ("deploy", "promote")
            },
        }
        self.approval = {
            "account_id": "111111111111",
            "region": "ap-southeast-1",
            "instance_id": "i-aaaaaaaaaaaaaaaaa",
            "environment": "staging",
            "operation": "deploy",
            "operator": "fixture-operator",
            "release_sha256": "c" * 64,
            "previous_release_sha256": "d" * 64,
            "issued_at": self.now,
            "expires_at": self.now + 900,
            "acknowledgements": {
                "isolated_rehearsal_verified": True,
                "drain_verified": True,
                "callbacks_reconciled": True,
                "no_automatic_schema_rollback": True,
                "destination_writes_acknowledged": True,
            },
            "evidence": {"drain": "e" * 64, "recovery": "f" * 64},
        }

    def publish(self):
        for path, value in (
            (self.config_path, self.config),
            (self.approval_path, self.approval),
        ):
            path.write_text(json.dumps(value))
            path.chmod(0o600)
        with (
            patch.object(
                sys,
                "argv",
                [
                    "operator_cli.py",
                    "approval",
                    "--config",
                    str(self.config_path),
                    "--file",
                    str(self.approval_path),
                    "--source",
                    "a" * 40,
                ],
            ),
            patch.object(operator_cli, "Cloud", return_value=self.store),
            redirect_stdout(io.StringIO()),
        ):
            operator_cli.main()

    def test_publishes_valid_isolated_staging_approval(self):
        try:
            self.publish()
        except Refused as error:
            self.fail(f"Valid isolated staging approval was refused: {error}")
        self.assertEqual(len(self.store.uploads), 1)
        uploaded = self.store.uploads[0]
        self.assertEqual(uploaded["key"], "approvals/staging/deploy/" + "c" * 64 + ".json")
        self.assertEqual(uploaded["body"]["release_sha256"], "c" * 64)
        self.assertEqual(uploaded["body"]["expires_at"], self.now + 900)

    def test_publishes_production_approval_without_staging_expiry(self):
        self.config["kind"] = "production"
        self.config.pop("expires_at")
        self.approval["environment"] = "production"
        self.approval["evidence"]["rehearsal"] = "1" * 64
        self.publish()
        self.assertEqual(len(self.store.uploads), 1)
        self.assertEqual(
            self.store.uploads[0]["key"],
            "approvals/production/deploy/" + "c" * 64 + ".json",
        )

    def test_expiry_or_unverified_isolation_cannot_upload_approval(self):
        config, approval = copy.deepcopy(self.config), copy.deepcopy(self.approval)
        for name in (
            "missing-target", "expired-target", "unbounded-target", "expired-approval", "isolation"
        ):
            with self.subTest(name=name):
                self.config, self.approval = copy.deepcopy(config), copy.deepcopy(approval)
                if name == "missing-target":
                    self.config.pop("expires_at")
                    reason = "Temporary staging approval"
                elif name == "expired-target":
                    self.config["expires_at"] = self.now - 60
                    reason = "Temporary staging approval"
                elif name == "unbounded-target":
                    self.config["expires_at"] = self.now + 86460
                    reason = "Temporary staging approval"
                elif name == "expired-approval":
                    self.approval["expires_at"] = self.now - 60
                    reason = "Approval expired"
                else:
                    self.approval["acknowledgements"]["isolated_rehearsal_verified"] = False
                    reason = "Temporary rehearsal"
                with self.assertRaisesRegex(Refused, reason):
                    self.publish()
                self.assertEqual(self.store.uploads, [])


if __name__ == "__main__":
    unittest.main()
