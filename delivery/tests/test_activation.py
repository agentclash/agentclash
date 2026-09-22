import base64
import copy
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import MagicMock, patch

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
import activation
from common import Refused
from test_ci import config as delivery_config


def fixture(kind="build", phase="claims"):
    value = {
        "kind": kind,
        "phase": phase,
        "expires_at": 1600,
        "nonce": "a" * 32,
        "source_revision": "b" * 40,
        "repository": "fixture/project",
        "closed_host_verified": True,
    }
    value["receipt"] = {
        "bucket": "fixture-artifacts",
        "region": "ap-southeast-1",
        "url": "https://fixture-artifacts.s3.ap-southeast-1.amazonaws.com/build-evidence/activation/"
        + value["nonce"]
        + "/"
        + kind
        + "/"
        + phase
        + ".json?X-Amz-Signature=fixture",
    }
    delivery = delivery_config(kind)
    delivery["role_arn"] = (
        "arn:aws:iam::" + delivery["account_id"] + ":role/fixture-" + kind
    )
    delivery["github"] = {
        "repository": value["repository"],
        "environment": "aws-" + kind,
    }
    value["delivery"] = delivery
    value["probe"] = {
        "instance_id": delivery["instance_id"],
        "documents": copy.deepcopy(delivery["documents"]),
        "release_key": "releases/" + "c" * 64 + ".json",
        "runtime_secret_arn": "arn:aws:secretsmanager:ap-southeast-1:"
        + delivery["account_id"]
        + ":secret:fixture",
        "other_role_arn": "arn:aws:iam::"
        + delivery["account_id"]
        + ":role/fixture-peer",
    }
    env = {
        "GITHUB_ACTIONS": "true",
        "GITHUB_EVENT_NAME": "workflow_dispatch",
        "GITHUB_REF": "refs/heads/main",
        "GITHUB_REPOSITORY": value["repository"],
        "GITHUB_WORKFLOW_REF": value["repository"]
        + "/.github/workflows/aws-verify.yml@refs/heads/main",
        "GITHUB_SHA": value["source_revision"],
        "ACTIVATION_KIND": kind,
    }
    return value, env


def encoded_claims(value, **changes):
    claims = {
        "iss": "https://token.actions.githubusercontent.com",
        "aud": "sts.amazonaws.com",
        "repository": value["repository"],
        "ref": "refs/heads/main",
        "sha": value["source_revision"],
        "environment": "aws-" + value["kind"],
        "workflow_ref": value["repository"]
        + "/.github/workflows/aws-verify.yml@refs/heads/main",
        "exp": 2000,
        "sub": "repo:fixture/project:environment:aws-" + value["kind"],
        "private_unselected_claim": "PRIVATE_FIXTURE_CANARY",
        **changes,
    }
    return (
        "fixture."
        + base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip("=")
        + ".signature"
    )


class ActivationTests(unittest.TestCase):
    def test_context_and_bounded_approval(self):
        value, env = fixture()
        activation.validate(value, env, now=1000)
        for key in env:
            with self.subTest(key=key), self.assertRaises(Refused):
                activation.validate(value, {**env, key: "unexpected"}, now=1000)
        for expiry in (0, 1000, 4601):
            with self.assertRaises(Refused):
                activation.validate({**value, "expires_at": expiry}, env, now=1000)
        with patch.dict(os.environ, {}, clear=True), self.assertRaises(KeyError):
            activation.main()

    def test_private_destination_cannot_redirect_or_escape_prefix(self):
        value, env = fixture()
        original = value["receipt"]["url"]
        for url in (
            original.replace("https:", "http:"),
            original.replace("amazonaws.com", "example.com"),
            original.replace("/build-evidence/", "/approvals/"),
            original.split("?")[0],
            original.replace("https://", "https://user@"),
            original + "#fragment",
        ):
            changed = copy.deepcopy(value)
            changed["receipt"]["url"] = url
            with self.subTest(url=url), self.assertRaises(Refused):
                activation.validate(changed, env, now=1000)
        with self.assertRaises(RuntimeError):
            activation.NoRedirect().redirect_request(
                None, None, 302, None, None, "https://example.com"
            )

    def test_permissions_require_closed_host_and_pinned_matching_targets(self):
        value, env = fixture("production", "permissions")
        activation.validate(value, env, now=1000)
        for change in (
            lambda x: x.update(closed_host_verified=False),
            lambda x: x["probe"].update(instance_id="i-" + "b" * 17),
            lambda x: x["probe"]["documents"]["deploy"].update(version="$LATEST"),
            lambda x: x["probe"].update(other_role_arn=x["delivery"]["role_arn"]),
        ):
            changed = copy.deepcopy(value)
            change(changed)
            with self.assertRaises(Refused):
                activation.validate(changed, env, now=1000)

    def test_claim_context_and_selection(self):
        value, _ = fixture()
        with patch("activation.time.time", return_value=1000):
            selected = activation.claims(encoded_claims(value), value)
            self.assertNotIn("PRIVATE_FIXTURE_CANARY", json.dumps(selected))
            for key, bad in (
                ("aud", "wrong"),
                ("iss", "wrong"),
                ("ref", "branch"),
                ("environment", "other"),
                ("sha", "c" * 40),
                ("sub", "repo:*"),
                ("exp", 999),
            ):
                with self.subTest(key=key), self.assertRaises(Refused):
                    activation.claims(encoded_claims(value, **{key: bad}), value)
            with self.assertRaises(Refused):
                activation.claims("malformed", value)

    def test_receipt_is_conditional_encrypted_private_put(self):
        value, _ = fixture()
        response = MagicMock()
        response.__enter__.return_value.status = 200
        with patch("activation.urllib.request.build_opener") as opener:
            opener.return_value.open.return_value = response
            activation.upload(value, {"passed": True})
            req = opener.return_value.open.call_args.args[0]
            self.assertEqual(req.get_method(), "PUT")
            headers = {k.lower(): v for k, v in req.header_items()}
            self.assertEqual(headers["if-none-match"], "*")
            self.assertEqual(headers["x-amz-server-side-encryption"], "AES256")
            self.assertNotIn("authorization", headers)

    def test_only_authorization_errors_count(self):
        for code, message, expected in (
            (1, "(AccessDenied)", True),
            (0, "(AccessDenied)", False),
            (1, "(ValidationException)", False),
            (1, "Could not connect", False),
            (1, "(ResourceNotFoundException)", False),
        ):
            self.assertEqual(
                activation.authorization_denied(
                    subprocess.CompletedProcess([], code, b"", message.encode())
                ),
                expected,
            )

    def test_build_probes_use_no_real_secret_version_or_release(self):
        value, _ = fixture("build", "permissions")
        cloud = MagicMock()
        cloud.raw.return_value = subprocess.CompletedProcess(
            [], 1, b"", b"(AccessDenied)"
        )
        objects = {value["probe"]["release_key"]: b"{}"}
        cloud.get.side_effect = objects.__getitem__
        cloud.put.side_effect = lambda key, path: objects.update(
            {key: path.read_bytes()}
        )
        with (
            tempfile.TemporaryDirectory() as directory,
            patch("activation.Cloud", return_value=cloud),
            patch("activation.token", return_value="TRANSIENT_FIXTURE"),
            patch("sys.stdout", new_callable=io.StringIO) as output,
        ):
            result = activation.verify_permissions(value, directory)
            self.assertEqual(output.getvalue(), "")
        calls = [call.args[0] for call in cloud.raw.call_args_list]
        secret = next(args for args in calls if "get-secret-value" in args)
        self.assertEqual(secret[secret.index("--version-id") + 1], "0" * 32)
        ssm = [args for args in calls if "send-command" in args]
        self.assertEqual(len(ssm), 2)
        self.assertTrue(
            any("ReleaseSha256" in str(args) and "0" * 64 in str(args) for args in ssm)
        )
        self.assertNotIn("TRANSIENT_FIXTURE", json.dumps(result))

    def test_unexpected_allow_aborts_before_publication(self):
        value, _ = fixture("build", "permissions")
        cloud = MagicMock()
        cloud.raw.return_value = subprocess.CompletedProcess(
            [], 0, b"PRIVATE_FIXTURE_CANARY", b""
        )
        with (
            tempfile.TemporaryDirectory() as directory,
            patch("activation.Cloud", return_value=cloud),
            self.assertRaises(Refused),
        ):
            activation.verify_permissions(value, directory)
        cloud.put.assert_not_called()

    def test_production_invokes_only_pinned_sentinel_documents(self):
        value, _ = fixture("production", "permissions")
        cloud = MagicMock()
        cloud.get.return_value = b"{}"
        cloud.call.return_value = b'{"Command":{"CommandId":"fixture-command"}}'

        def result(args, **kwargs):
            if "get-command-invocation" in args:
                return subprocess.CompletedProcess(
                    args, 0, b'{"Status":"Failed","ResponseCode":1}', b""
                )
            return subprocess.CompletedProcess(args, 1, b"", b"(AccessDenied)")

        cloud.raw.side_effect = result
        with (
            tempfile.TemporaryDirectory() as directory,
            patch("activation.Cloud", return_value=cloud),
            patch("activation.token", return_value="TRANSIENT_FIXTURE"),
            patch("activation.time.sleep"),
        ):
            report = activation.verify_permissions(value, directory)
        commands = [
            call.args
            for call in cloud.call.call_args_list
            if "send-command" in call.args
        ]
        self.assertEqual(len(commands), 2)
        for args, operation in zip(commands, ("deploy", "promote")):
            doc = value["delivery"]["documents"][operation]
            for flag, expected in (
                ("--document-name", doc["name"]),
                ("--document-version", doc["version"]),
                ("--document-hash", doc["sha256"]),
                ("--instance-ids", value["delivery"]["instance_id"]),
            ):
                self.assertEqual(args[args.index(flag) + 1], expected)
            self.assertEqual(
                json.loads(args[args.index("--parameters") + 1]),
                {"ReleaseSha256": ["0" * 64]},
            )
        self.assertEqual(sum(bool(x.get("host_refused")) for x in report), 2)
        cloud.put.assert_not_called()

    def test_entrypoint_failure_does_not_leak_private_config(self):
        result = subprocess.run(
            [sys.executable, str(ROOT / "delivery/activation.py")],
            capture_output=True,
            text=True,
            env={**os.environ, "AWS_ACTIVATION_CONFIG": "PRIVATE_FIXTURE_CANARY"},
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("PRIVATE_FIXTURE_CANARY", result.stdout + result.stderr)
        self.assertNotIn("Traceback", result.stderr)


if __name__ == "__main__":
    unittest.main()
