import io
import json
from pathlib import Path
import sys
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
from build import scan_bootstrap
from common import Refused


class BootstrapScannerTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.archive = self.directory / "bootstrap.tar.gz"
        with tarfile.open(self.archive, "w:gz") as archive:
            member = tarfile.TarInfo("tools/docker-compose")
            member.size = len(b"fixture")
            member.mode = 0o644
            archive.addfile(member, io.BytesIO(b"fixture"))

    def runner(self, results):
        def run(command, **_):
            installed = Path(command[-1]) / "tools/docker-compose"
            self.assertTrue(installed.stat().st_mode & 0o100)
            self.assertEqual(installed.read_bytes(), b"fixture")
            self.assertIn("rootfs", command)
            self.assertEqual(command[command.index("--exit-code") + 1], "1")
            scanner = command[command.index("--scanners") + 1]
            if scanner == "vuln":
                self.assertEqual(
                    command[command.index("--severity") + 1], "HIGH,CRITICAL"
                )
            report = Path(command[command.index("--output") + 1])
            report.write_text(json.dumps({"Results": results}))

        return run

    def test_clean_binary_requires_actual_compose_coverage(self):
        reports = scan_bootstrap(
            self.directory,
            self.archive,
            self.runner([{"Target": "tools/docker-compose", "Type": "gobinary"}]),
        )
        self.assertEqual(len(reports), 2)

    def test_zero_findings_without_binary_coverage_refuses(self):
        with self.assertRaises(Refused):
            scan_bootstrap(self.directory, self.archive, self.runner([]))

    def test_scanner_failure_is_not_converted_to_success(self):
        def failed(*_, **__):
            raise Refused("scanner failed")

        with self.assertRaises(Refused):
            scan_bootstrap(self.directory, self.archive, failed)

    def test_unrelated_binary_does_not_satisfy_compose_coverage(self):
        with self.assertRaises(Refused):
            scan_bootstrap(
                self.directory,
                self.archive,
                self.runner([{"Target": "tools/unrelated", "Type": "gobinary"}]),
            )


if __name__ == "__main__":
    unittest.main()
