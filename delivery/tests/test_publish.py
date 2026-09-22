import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
from build import publish
from common import Refused
from maintained import LABEL
from release_contract import APPLICATIONS, DERIVED, UPSTREAM, canonical, sha256


class PublicationContractTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.commands, self.objects, self.tags = [], {}, {}
        self.bad_identity = False
        self.config = {
            "account_id": str(1) * 12,
            "region": "test-region",
            "repository": "registry.example.com/app",
        }
        self.prepared = {
            "source_revision": "a" * 40,
            "platform_recipe_sha256": "b" * 64,
            "platform_lock_sha256": "c" * 64,
            "tools_lock_sha256": "d" * 64,
            "migration_set_sha256": "e" * 64,
            "bootstrap": {"sha256": "f" * 64, "platform_source_sha256": "a" * 64},
            "image_ids": {n: "sha256:" + sha256(n.encode()) for n in APPLICATIONS},
            "platform_images": {
                **{n: "sha256:" + sha256(n.encode()) for n in DERIVED},
                **{
                    n: "public.example.com/" + n + "@sha256:" + "a" * 64
                    for n in UPSTREAM
                },
            },
            "reports": [],
        }
        (self.directory / "bootstrap.tar.gz").write_bytes(b"synthetic bootstrap")

    def call(self, *args):
        self.assertEqual(args, ("ecr", "get-login-password"))
        return b"synthetic-password"

    def put(self, key, path):
        self.objects[key] = path.read_bytes()

    def docker(self, command, **kwargs):
        self.commands.append(command)
        self.assertNotIn("AWS_SECRET_ACCESS_KEY", kwargs["env"])
        if command[1] == "tag":
            self.tags[command[3]] = command[2]
        if command[1:3] == ["image", "inspect"]:
            tag = command[3]
            platform = "-platform-" in tag
            detail = {
                "Os": "linux",
                "Architecture": "amd64",
                "Id": "sha256:" + "0" * 64 if self.bad_identity else self.tags[tag],
                "Config": {
                    "Labels": {
                        LABEL if platform else "org.opencontainers.image.revision": "b"
                        * 64
                        if platform
                        else "a" * 40
                    }
                },
                "RepoDigests": [
                    self.config["repository"] + "@sha256:" + sha256(tag.encode())
                ],
            }
            return subprocess.CompletedProcess(command, 0, canonical([detail]), b"")
        return subprocess.CompletedProcess(command, 0, b"", b"")

    def test_nine_scanned_artifacts_and_all_platform_refs_bind_to_private_manifest(
        self,
    ):
        with patch("build.subprocess.run", side_effect=self.docker):
            publish(self, self.directory, self.prepared)
        self.assertEqual(len([c for c in self.commands if c[1] == "push"]), 9)
        manifest = json.loads((self.directory / "release.json").read_text())
        evidence = json.loads((self.directory / "build-evidence.json").read_text())
        self.assertEqual(manifest["format"], 2)
        self.assertEqual(set(manifest["images"]), set(APPLICATIONS))
        self.assertEqual(set(manifest["platform_images"]), set((*DERIVED, *UPSTREAM)))
        self.assertEqual(evidence["platform_images"], manifest["platform_images"])
        self.assertEqual(evidence["images"], manifest["images"])
        for name in DERIVED:
            self.assertTrue(
                manifest["platform_images"][name].startswith(
                    self.config["repository"] + "@sha256:"
                )
            )
        for name in UPSTREAM:
            self.assertEqual(
                manifest["platform_images"][name],
                self.prepared["platform_images"][name],
            )
        self.assertEqual(len(evidence["provenance"]["local_image_ids"]), 9)

    def test_wrong_local_bytes_fail_before_any_image_push(self):
        self.bad_identity = True
        with patch("build.subprocess.run", side_effect=self.docker):
            with self.assertRaises(Refused):
                publish(self, self.directory, self.prepared)
        self.assertFalse(any(c[1] == "push" for c in self.commands))
        self.assertEqual(self.objects, {})


if __name__ == "__main__":
    unittest.main()
