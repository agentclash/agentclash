from contextlib import redirect_stdout
import hashlib
import io
import json
from pathlib import Path
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
from maintained import build


class MaintainedBuildTests(unittest.TestCase):
    def test_temporal_binaries_use_zlib_patched_bases_before_the_final_gate(self):
        stages, gates = {}, []
        public = json.loads((ROOT / "deploy/aws/images.lock.json").read_text())["images"]

        def image_id(name):
            return "sha256:" + hashlib.sha256(name.encode()).hexdigest()

        def fetch(entry, destination):
            if destination.name == "gosu.tar.gz":
                with tarfile.open(destination, "w:gz") as archive:
                    content = b"module fixture\n"
                    member = tarfile.TarInfo("gosu-" + entry["version"] + "/go.mod")
                    member.size = len(content)
                    archive.addfile(member, io.BytesIO(content))
            else:
                destination.write_text(entry["url"])

        def run(command, **_):
            self.assertEqual(command[0], "docker")
            if command[1] == "build":
                name = command[command.index("-t") + 1].split(":", 1)[1]
                context = Path(command[-1])
                arguments = dict(
                    command[index + 1].split("=", 1)
                    for index, value in enumerate(command)
                    if value == "--build-arg"
                )
                self.assertNotIn(name, stages, "Each stage must be built once")
                stages[name] = {
                    "arguments": arguments,
                    "dockerfile": Path(command[command.index("-f") + 1]).name,
                    "target": command[command.index("--target") + 1]
                    if "--target" in command
                    else None,
                    "packages": {
                        p.name: p.read_text()
                        for p in (context / "packages").glob("*.apk")
                    },
                }
                output = command[command.index("--output") + 1]
                Path(output.split("dest=", 1)[1]).touch()
            else:
                self.assertEqual(command[1], "load")

        def gate(name, upstream, output):
            self.assertEqual(upstream, public[name]["image"])
            stage = name + "-packages" if name in ("alpine", "go", "bun") else name
            self.assertEqual(
                output,
                public[name]["image"] if name == "valkey" else image_id(stage),
            )
            gates.append(name)

        def inspect(_, tag, __):
            return {"Id": image_id(tag.split(":", 1)[1])}

        with (
            tempfile.TemporaryDirectory() as directory,
            redirect_stdout(io.StringIO()),
            patch("maintained.fetch", side_effect=fetch),
            patch("maintained.local_base", side_effect=lambda _, identity: identity),
            patch("maintained.inspect", side_effect=inspect),
        ):
            output, _ = build(ROOT, directory, run, gate)

        self.assertEqual(
            list(stages),
            [
                "alpine-packages",
                "go-packages",
                "bun-packages",
                "caddy-packages",
                "caddy",
                "postgres-packages",
                "postgres",
                "temporal-packages",
                "temporal-admin-packages",
                "temporal",
                "temporal-admin",
            ],
        )
        for name, branch in (
            ("alpine", "v3.24"),
            ("go", "v3.24"),
            ("bun", "v3.22"),
            ("caddy", "v3.23"),
            ("postgres", "v3.24"),
            ("temporal", "v3.24"),
            ("temporal-admin", "v3.24"),
        ):
            with self.subTest(name=name):
                stage = stages[name + "-packages"]
                self.assertEqual(stage["arguments"]["UPSTREAM"], public[name]["image"])
                self.assertEqual(stage["dockerfile"], "Dockerfile.packages")
                self.assertEqual(
                    stage["packages"]["zlib-1.3.2-r1.apk"],
                    "https://dl-cdn.alpinelinux.org/alpine/"
                    + branch
                    + "/main/x86_64/zlib-1.3.2-r1.apk",
                )
        for name, target in (("temporal", "server"), ("temporal-admin", "admin")):
            self.assertEqual(
                stages[name]["arguments"]["UPSTREAM"], image_id(name + "-packages")
            )
            self.assertEqual(stages[name]["dockerfile"], "Dockerfile.temporal")
            self.assertEqual(stages[name]["target"], target)
            self.assertEqual(
                list(stages[name + "-packages"]["packages"]), ["zlib-1.3.2-r1.apk"]
            )
            self.assertEqual(stages[name]["arguments"]["BUILDER"], output["go"])
            self.assertEqual(output[name], image_id(name))
        self.assertEqual(
            gates,
            [
                "alpine", "go", "bun", "caddy", "postgres",
                "temporal", "temporal-admin", "valkey",
            ],
        )


if __name__ == "__main__":
    unittest.main()
