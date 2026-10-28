from contextlib import redirect_stderr, redirect_stdout
import io
import json
import os
from pathlib import Path
import runpy
import subprocess
import sys
import tarfile
import tempfile
import time
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
from common import Refused


class BuildFailureDiagnosticsTests(unittest.TestCase):
    def run_failed_build(self, error=None):
        previous_umask = os.umask(0o077)
        self.addCleanup(os.umask, previous_umask)
        with tempfile.TemporaryDirectory() as directory:
            config = {
                "enabled": True,
                "kind": "build",
                "account_id": "111111111111",
                "region": "ap-southeast-1",
                "artifact_bucket": "fixture-artifacts",
                "repository": "111111111111.dkr.ecr.ap-southeast-1.amazonaws.com/app",
                "profile": "fixture-sso",
                "operator": "fixture-operator",
                "publication_approval": {
                    "operator": "fixture-operator",
                    "source_revision": "a" * 40,
                    "account_id": "111111111111",
                    "region": "ap-southeast-1",
                    "expires_at": int(time.time()) + 1800,
                    "local_required_checks_passed": True,
                },
            }
            config_path = Path(directory) / "config.json"
            config_path.write_text(json.dumps(config))
            config_path.chmod(0o600)

            def subprocess_result(command, **kwargs):
                if command[:2] == ["git", "archive"]:
                    with tarfile.open(fileobj=kwargs["stdout"], mode="w"):
                        pass
                    return subprocess.CompletedProcess(command, 0)
                if Path(command[0]).name == "gitleaks":
                    return subprocess.CompletedProcess(command, 0, stdout=b"")
                if Path(command[0]).name != "trivy":
                    raise AssertionError("Unexpected external build command")
                if error is not None:
                    raise error
                return subprocess.CompletedProcess(
                    command, 1, stdout=b"PRIVATE_STDOUT", stderr=b"PRIVATE_STDERR"
                )

            def image_graph(source, evidence, run, revision):
                run(["/PRIVATE_PATH/trivy", "image", "PRIVATE_ARGUMENT"])

            stdout, stderr = io.StringIO(), io.StringIO()
            with (
                patch.object(
                    sys,
                    "argv",
                    [
                        "ci.py", "build", "--kind", "build",
                        "--config", str(config_path), "--source", "a" * 40,
                    ],
                ),
                patch.dict(os.environ, {"DOCKER_CONFIG": "PRIVATE_ENVIRONMENT"}),
                patch("build.subprocess.check_output", side_effect=[b"a" * 40, b""]),
                patch("build.subprocess.run", side_effect=subprocess_result),
                patch("build.install"),
                patch("build.build_graph", side_effect=image_graph),
                patch("cloud.Cloud") as cloud,
                redirect_stdout(stdout),
                redirect_stderr(stderr),
            ):
                with self.assertRaises(SystemExit) as stopped:
                    runpy.run_path(str(ROOT / "delivery/ci.py"), run_name="__main__")
                cloud.assert_not_called()
                self.assertIsInstance(stopped.exception.__context__, Refused)
                if error is None:
                    self.assertEqual(
                        str(stopped.exception.__context__),
                        "Build or scanner gate failed; nothing has been published",
                    )
            return stdout.getvalue(), stderr.getvalue(), str(stopped.exception)

    def assert_private_data_absent(self, output):
        for canary in (
            "PRIVATE_PATH", "PRIVATE_ARGUMENT", "PRIVATE_ENVIRONMENT",
            "PRIVATE_STDOUT", "PRIVATE_STDERR", "PRIVATE_EXCEPTION",
        ):
            self.assertNotIn(canary, output)
        self.assertNotIn(str(ROOT), output)

    def test_scanner_failure_reports_safe_progress_and_public_refusal_location(self):
        stdout, stderr, exit_message = self.run_failed_build()
        self.assertIn("Build subprocess: trivy", stdout)
        self.assertIn("Build subprocess failed: trivy", stdout)
        self.assertRegex(stderr, r"Delivery source location: delivery/build\.py:[0-9]+")
        self.assertIn(
            "Build or scanner gate failed; nothing has been published", stderr
        )
        self.assertEqual(
            exit_message,
            "Delivery refused or failed; inspect private state before retrying",
        )
        self.assert_private_data_absent(stdout + stderr + exit_message)

    def test_nonpublic_exception_value_is_omitted_from_cli_diagnostics(self):
        stdout, stderr, exit_message = self.run_failed_build(Refused("PRIVATE_EXCEPTION"))
        self.assertIn("Build subprocess: trivy", stdout)
        self.assertRegex(stderr, r"Delivery source location: delivery/build\.py:[0-9]+")
        self.assertNotIn("Delivery invariant:", stderr)
        self.assert_private_data_absent(stdout + stderr + exit_message)


if __name__ == "__main__":
    unittest.main()
