import copy
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
from common import Refused
from image_scan import coverage, reconcile, Scanner
from maintained import fetch, local_base, selection, normalize_context
from release_contract import APPLICATIONS, DERIVED, PLATFORM, UPSTREAM, validate_images


def os_report(version="3.5.8-r0", vulnerable=False):
    result = {
        "Class": "os-pkgs",
        "Type": "alpine",
        "Target": "fixture image",
        "Packages": [{"Name": "libssl3", "Version": version}],
    }
    if vulnerable:
        result["Vulnerabilities"] = [
            {
                "VulnerabilityID": "CVE-fixture",
                "PkgName": "libssl3",
                "Severity": "HIGH",
                "InstalledVersion": version,
                "FixedVersion": "3.5.8-r0",
            }
        ]
    return {
        "Metadata": {
            "OS": {"Family": "alpine"},
            "ImageConfig": {"os": "linux", "architecture": "amd64"},
        },
        "Results": [result],
    }


class ImageGateTests(unittest.TestCase):
    def test_scanner_cannot_report_different_bytes_or_architecture_as_a_pass(self):
        for wrong in ("identity", "architecture"):
            with tempfile.TemporaryDirectory() as directory:

                def run(command, **_):
                    value = os_report()
                    value["Metadata"]["ImageID"] = (
                        "sha256:" + ("b" if wrong == "identity" else "a") * 64
                    )
                    if wrong == "architecture":
                        value["Metadata"]["ImageConfig"]["architecture"] = "arm64"
                    Path(command[command.index("--output") + 1]).write_text(
                        json.dumps(value)
                    )

                with self.assertRaises(Refused):
                    Scanner(Path(directory), run).image(
                        "alpine", "sha256:" + "a" * 64, "alpine"
                    )

    def test_upstream_export_refuses_a_mutable_reference_before_building(self):
        with tempfile.TemporaryDirectory() as directory:
            with patch("subprocess.run") as call:
                with self.assertRaises(Refused):
                    Scanner(Path(directory), call).export_upstream(
                        "input-alpine", "alpine:latest", "alpine"
                    )
                call.assert_not_called()

    def test_private_and_git_archive_modes_normalize_without_following_links(self):
        with tempfile.TemporaryDirectory() as directory:
            context = Path(directory)
            source = context / "source"
            source.mkdir(mode=0o700)
            for name, mode in (("private", 0o600), ("archive", 0o644)):
                path = source / name
                path.write_bytes(b"public fixture")
                path.chmod(mode)
            normalize_context(context)
            for path in source.iterdir():
                self.assertEqual(path.stat().st_mode & 0o777, 0o644)
                self.assertEqual(path.stat().st_mtime, 0)
            self.assertEqual(source.stat().st_mode & 0o777, 0o755)
            (source / "link").symlink_to(source / "private")
            with self.assertRaises(Refused):
                normalize_context(context)

    def test_input_findings_require_present_changed_clean_components(self):
        before, after = os_report("3.5.7-r0", True), os_report()
        self.assertEqual(reconcile(before, after)[0]["after"], ["3.5.8-r0"])
        for bad in (
            os_report("3.5.7-r0"),
            os_report("3.5.8-r0", True),
            {"Results": []},
        ):
            with self.assertRaises(Refused):
                reconcile(before, bad)

    def test_a_fix_with_no_upstream_fixed_version_is_not_an_implicit_waiver(self):
        before = os_report("old", True)
        before["Results"][0]["Vulnerabilities"][0].pop("FixedVersion")
        with self.assertRaises(Refused):
            reconcile(before, os_report())

    def test_coverage_requires_actual_os_and_expected_binary_inventory(self):
        wrong_arch = os_report()
        wrong_arch["Metadata"]["ImageConfig"]["architecture"] = "arm64"
        with self.assertRaises(Refused):
            coverage(wrong_arch, "alpine")
        for role in ("caddy", "postgres", "go", "api", "terminal", "temporal"):
            with self.assertRaises(Refused):
                coverage(os_report(), role)
        value = os_report()
        value["Results"].append(
            {
                "Type": "gobinary",
                "Target": "usr/bin/caddy",
                "Packages": [{"Name": "stdlib", "Version": "v1.26.8"}],
            }
        )
        coverage(value, "caddy")
        value["Results"][1]["Packages"] = []
        with self.assertRaises(Refused):
            coverage(value, "caddy")

    def test_input_scanner_errors_and_secret_findings_always_propagate(self):
        with tempfile.TemporaryDirectory() as directory:

            def failure(*_, **__):
                raise Refused("scanner error")

            with self.assertRaises(Refused):
                Scanner(Path(directory), failure).image(
                    "input-alpine", "sha256:" + "a" * 64, "alpine", source_input=True
                )

    def test_wrong_download_hash_never_writes_an_input(self):
        with tempfile.TemporaryDirectory() as directory:

            class Response:
                url = "https://example.test/fixture"

                def __enter__(self):
                    return self

                def __exit__(self, *_):
                    pass

                def read(self):
                    return b"tampered"

            with patch("maintained.urllib.request.urlopen", return_value=Response()):
                destination = Path(directory) / "input"
                with self.assertRaises(Refused):
                    fetch({"url": Response.url, "sha256": "a" * 64}, destination)
                self.assertFalse(destination.exists())

    def test_base_tag_substitution_is_detected(self):
        def run(command):
            if command[1] == "tag":
                return b""
            return json.dumps(
                [{"Id": "sha256:" + "b" * 64, "Os": "linux", "Architecture": "amd64"}]
            ).encode()

        with self.assertRaises(Refused):
            local_base(run, "sha256:" + "a" * 64)

    def test_rehearsals_refuse_mutable_or_incomplete_selections(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "images.json"
            value = {
                "images": {n: "sha256:" + "a" * 64 for n in APPLICATIONS},
                "platform_images": {n: "sha256:" + "b" * 64 for n in PLATFORM},
            }
            path.write_text(json.dumps(value))
            self.assertEqual(len(selection(path)), 12)
            value["platform_images"]["caddy"] = "caddy:latest"
            path.write_text(json.dumps(value))
            with self.assertRaises(Refused):
                selection(path)
            value["platform_images"].pop("caddy")
            path.write_text(json.dumps(value))
            with self.assertRaises(Refused):
                selection(path)

    def test_private_platform_refs_and_exact_public_inputs_are_required(self):
        lock = json.loads((ROOT / "deploy/aws/images.lock.json").read_text())
        repository = "registry.example.com/app"
        ref = repository + "@sha256:" + "a" * 64
        value = {
            "images": {n: ref for n in APPLICATIONS},
            "platform_images": {
                **{n: ref for n in DERIVED},
                **{n: lock["images"][n]["image"] for n in UPSTREAM},
            },
        }
        validate_images(value, repository, lock)
        for role, bad in (
            ("go", "foreign.example.com/app@sha256:" + "a" * 64),
            ("caddy", lock["images"]["caddy"]["image"]),
            ("postgres", repository + ":mutable"),
            ("valkey", ref),
        ):
            candidate = copy.deepcopy(value)
            candidate["platform_images"][role] = bad
            with self.assertRaises(Refused):
                validate_images(candidate, repository, lock)


if __name__ == "__main__":
    unittest.main()
