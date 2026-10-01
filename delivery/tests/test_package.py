import json
from pathlib import Path
import shutil
import sys
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
from common import Refused
from package import bundle
from release_contract import platform_files, sha256


class PackageTests(unittest.TestCase):
    def test_deterministic_allowlisted_bundle_excludes_private_extras(self):
        with tempfile.TemporaryDirectory(prefix="bundle-test-") as directory:
            directory = Path(directory)
            root = directory / "source"
            root.mkdir()
            for name in platform_files(ROOT):
                destination = root / name
                destination.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(ROOT / name, destination)
            binary = directory / "compose"
            binary.write_bytes(b"public binary fixture")
            lock = json.loads((root / "delivery/tools.lock.json").read_bytes())
            lock["tools"]["compose"]["platforms"]["linux_amd64"]["sha256"] = sha256(
                binary.read_bytes()
            )
            (root / "delivery/tools.lock.json").write_text(json.dumps(lock))
            (root / ".env").write_text("PRIVATE_FIXTURE_MUST_NOT_BE_PACKAGED")
            first, second = directory / "one.tar.gz", directory / "two.tar.gz"
            metadata = bundle(root, binary, first)
            self.assertEqual(metadata["sha256"], bundle(root, binary, second)["sha256"])
            with tarfile.open(first) as archive:
                self.assertEqual(
                    archive.getnames(),
                    sorted(
                        [
                            *platform_files(root),
                            "tools/docker-compose",
                            "tools/SHA256SUMS",
                            "tools/platform-source.sha256",
                        ]
                    ),
                )
                self.assertTrue(
                    all(
                        member.isfile() and member.uid == 0 and member.mtime == 0
                        for member in archive.getmembers()
                    )
                )
            self.assertEqual(first.stat().st_mode & 0o777, 0o600)
            with self.assertRaises(Refused):
                bundle(root, binary, root / "public.tar.gz")
            binary.write_bytes(b"tampered")
            with self.assertRaises(Refused):
                bundle(root, binary, directory / "bad.tar.gz")


if __name__ == "__main__":
    unittest.main()
