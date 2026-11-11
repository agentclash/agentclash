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
    def build_stages(self, check_gate=None):
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
                    "package_mtimes": {
                        p.name: p.stat().st_mtime_ns
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
            if check_gate is not None:
                check_gate(name, stages)
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

        return stages, gates, output

    def test_temporal_patches_zlib_in_runtime_stages_before_the_final_gate(self):
        public = json.loads((ROOT / "deploy/aws/images.lock.json").read_text())["images"]
        stages, gates, output = self.build_stages()

        def image_id(name):
            return "sha256:" + hashlib.sha256(name.encode()).hexdigest()

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
                stages[name]["arguments"]["UPSTREAM"], public[name]["image"]
            )
            self.assertEqual(stages[name]["dockerfile"], "Dockerfile.temporal")
            self.assertEqual(stages[name]["target"], target)
            apks = ("zlib-1.3.2-r1.apk", "musl-1.2.6-r3.apk", "musl-utils-1.2.6-r3.apk")
            self.assertEqual(
                stages[name]["packages"],
                {
                    apk: "https://dl-cdn.alpinelinux.org/alpine/v3.24/main/x86_64/" + apk
                    for apk in apks
                },
            )
            self.assertEqual(
                stages[name]["package_mtimes"], {apk: 0 for apk in apks}
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

    def test_postgres_library_archives_reach_both_stages_before_the_final_gate(self):
        lock = json.loads((ROOT / "deploy/aws/platform/inputs.lock.json").read_text())
        fixes = {
            "krb5-libs-1.22.3-r0.apk": (
                "krb5-libs", "1.22.3-r0",
                "fad5b46a953e6a37a1bb61a84538fa08ecb6a07816f439eb06f2735aebfcbfbf",
            ),
            "libxslt-1.1.43-r4.apk": (
                "libxslt", "1.1.43-r4",
                "40e6cc7d09ce1e69421c2b37e1da4e9afef486843f21c3a79339cd6d93c1c617",
            ),
        }
        urls = {
            filename: "https://dl-cdn.alpinelinux.org/alpine/v3.24/main/x86_64/" + filename
            for filename in fixes
        }
        checked = []

        def check_gate(name, stages):
            if name != "postgres":
                return
            for stage_name in ("postgres-packages", "postgres"):
                stage = stages[stage_name]
                for filename in fixes:
                    self.assertIn(filename, stage["packages"])
                    self.assertEqual(stage["packages"][filename], urls[filename])
                    self.assertEqual(stage["package_mtimes"][filename], 0)
            for filename, (package, version, sha256) in fixes.items():
                self.assertEqual(lock["packages"]["v3.24/" + package], {
                    "name": package, "version": version,
                    "url": urls[filename], "sha256": sha256,
                })
            patched_id = "sha256:" + hashlib.sha256(b"postgres-packages").hexdigest()
            self.assertEqual(stages["postgres"]["arguments"]["UPSTREAM"], patched_id)
            self.assertEqual(stages["postgres"]["dockerfile"], "Dockerfile.binary")
            self.assertEqual(stages["postgres"]["arguments"]["BINARY"], "gosu")
            checked.append(name)

        stages, _, _ = self.build_stages(check_gate)
        self.assertEqual(checked, ["postgres"])
        # Complete inventories catch accidental additions to unrelated image contexts.
        unchanged = (
            ("alpine-packages", "v3.24", "libcrypto3 libssl3 ca-certificates-bundle ca-certificates tzdata zlib musl musl-utils"),
            ("go-packages", "v3.24", "libcrypto3 libssl3 zlib"),
            ("bun-packages", "v3.22", "libcrypto3 libssl3 zlib musl musl-utils"),
            ("caddy-packages", "v3.23", "libcrypto3 libssl3 curl libcurl c-ares brotli-libs zlib musl musl-utils"),
            ("caddy", "v3.23", "libcrypto3 libssl3 curl libcurl c-ares brotli-libs zlib musl musl-utils"),
            ("temporal", "v3.24", "zlib musl musl-utils"),
            ("temporal-admin", "v3.24", "zlib musl musl-utils"),
        )
        for stage_name, branch, names in unchanged:
            with self.subTest(stage=stage_name):
                expected = {}
                for package in names.split():
                    entry = lock["packages"][branch + "/" + package]
                    expected[package + "-" + entry["version"] + ".apk"] = entry["url"]
                self.assertEqual(stages[stage_name]["packages"], expected)

    def test_temporal_runtime_stages_patch_packages_and_preserve_upstream_users(self):
        dockerfile = (ROOT / "deploy/aws/platform/Dockerfile.temporal").read_text()
        patch_commands = [
            "COPY packages /opt/platform-inputs",
            "RUN apk add --no-network --no-cache --upgrade /opt/platform-inputs/*.apk "
            "&& rm -rf /opt/platform-inputs && rm -f /var/log/apk.log",
        ]
        for target in ("server", "admin"):
            with self.subTest(target=target):
                stage = dockerfile.split("FROM ${UPSTREAM} AS " + target + "\n")[1]
                stage = stage.split("\nFROM ", 1)[0].replace("\\\n", "")
                commands = [
                    " ".join(line.split()) for line in stage.splitlines()
                    if line.strip() and not line.lstrip().startswith("#")
                ]
                # Both upstream images run as temporal (amd64 config User).
                expected = ["USER root", *patch_commands, "USER temporal"]
                self.assertEqual(commands[:len(expected)], expected)
                self.assertTrue(all(
                    command.startswith("COPY --from=")
                    for command in commands[len(expected):]
                ))


if __name__ == "__main__":
    unittest.main()
