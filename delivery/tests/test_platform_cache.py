from contextlib import redirect_stdout
import io
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
from common import Refused
import maintained

BUILDER = "sha256:" + "a" * 64


def fake_docker(calls):
    def run(argv, **kwargs):
        calls.append(argv[:2])
        if argv[:2] == ["docker", "build"]:
            output = Path(argv[argv.index("--output") + 1].split("dest=", 1)[1])
            (output / "notices").mkdir(parents=True)
            (output / "docker-compose").write_bytes(b"compose")
            (output / "notices" / "LICENSE").write_text("notice")
            return ""
        if argv[:3] == ["docker", "image", "inspect"]:
            return json.dumps([{"Os": "linux", "Architecture": "amd64", "Id": BUILDER}])
        return ""

    return run


class PlatformCacheTest(unittest.TestCase):
    def setUp(self):
        self.scratch = tempfile.TemporaryDirectory()
        self.addCleanup(self.scratch.cleanup)
        self.cache = Path(self.scratch.name).resolve() / "platform-builds"
        output = redirect_stdout(io.StringIO())
        output.__enter__()
        self.addCleanup(output.__exit__, None, None, None)

    def env(self, value):
        return patch.dict(os.environ, {maintained.PLATFORM_CACHE: value})

    def test_no_cache_without_the_variable(self):
        with patch.dict(os.environ, {}, clear=True):
            self.assertIsNone(maintained.platform_cache("1" * 64))

    def test_cache_of_another_recipe_is_removed(self):
        with self.env(str(self.cache)):
            maintained.platform_cache("1" * 64)
            (self.cache / "temporal.image.tar").write_bytes(b"old recipe")
            self.assertEqual(maintained.platform_cache("1" * 64), self.cache)
            self.assertTrue((self.cache / "temporal.image.tar").is_file())
            maintained.platform_cache("2" * 64)
            self.assertFalse((self.cache / "temporal.image.tar").exists())
            self.assertEqual((self.cache / "recipe.sha256").read_text(), "2" * 64)

    def test_cache_inside_the_checkout_is_refused(self):
        with self.env(str(ROOT / "tmp-platform-cache")):
            with self.assertRaises(Refused):
                maintained.platform_cache("1" * 64)
        self.assertFalse((ROOT / "tmp-platform-cache").exists())

    def test_compose_build_is_reused_for_the_same_recipe(self):
        first, second = [], []
        with self.env(str(self.cache)):
            run_dir = Path(self.scratch.name) / "first"
            run_dir.mkdir()
            maintained.build_compose(ROOT, run_dir, fake_docker(first), BUILDER)
            self.assertIn(["docker", "build"], first)
            run_dir = Path(self.scratch.name) / "second"
            run_dir.mkdir()
            binary, notices = maintained.build_compose(
                ROOT, run_dir, fake_docker(second), BUILDER
            )
        self.assertNotIn(["docker", "build"], second)
        self.assertEqual(binary.read_bytes(), b"compose")
        self.assertEqual((notices / "LICENSE").read_text(), "notice")

    def test_compose_build_without_cache_builds_every_time(self):
        calls = []
        with patch.dict(os.environ, {}, clear=True):
            for name in ("first", "second"):
                run_dir = Path(self.scratch.name) / name
                run_dir.mkdir()
                maintained.build_compose(ROOT, run_dir, fake_docker(calls), BUILDER)
        self.assertEqual(calls.count(["docker", "build"]), 2)

    def test_only_direct_check_runs_use_the_cache(self):
        import yaml

        checks = yaml.safe_load((ROOT / ".github/workflows/aws-checks.yml").read_text())
        self.assertEqual(checks["name"], "AWS release checks")
        steps = checks["jobs"]["images"]["steps"]
        restore = next(
            s
            for s in steps
            if s.get("uses", "").startswith("actions/cache@")
            and s["with"]["key"].startswith("platform-builds-")
        )
        self.assertEqual(
            restore["if"], "${{ github.workflow == 'AWS release checks' }}"
        )
        for name in ("delivery/maintained.py", "deploy/aws/images.lock.json"):
            self.assertIn(name, restore["with"]["key"])
        self.assertIn("deploy/aws/platform/**", restore["with"]["key"])
        run = next(
            s for s in steps if s.get("run") == "python3 delivery/checks.py images"
        )
        self.assertIn(
            "github.workflow == 'AWS release checks'",
            run["env"][maintained.PLATFORM_CACHE],
        )
        for file in (ROOT / ".github/workflows").glob("*.yml"):
            if file.name != "aws-checks.yml":
                self.assertNotIn(maintained.PLATFORM_CACHE, file.read_text(), file.name)

    def test_images_job_keeps_its_own_go_cache(self):
        import yaml

        checks = yaml.safe_load((ROOT / ".github/workflows/aws-checks.yml").read_text())
        steps = checks["jobs"]["images"]["steps"]
        go = next(s for s in steps if s.get("uses", "").startswith("actions/setup-go@"))
        self.assertIs(go["with"]["cache"], False)
        keys = [
            s["with"]["key"]
            for s in steps
            if s.get("uses", "").startswith("actions/cache@")
        ]
        self.assertTrue(any(k.startswith("go-image-rehearsals-") for k in keys))


if __name__ == "__main__":
    unittest.main()
