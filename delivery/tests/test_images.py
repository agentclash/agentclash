import copy
import hashlib
import io
import json
from pathlib import Path
import sys
import tempfile
import unittest
import urllib.error
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


def caddy_report(*, vulnerable=False, version="v4.1.6"):
    report = os_report()
    package = "github.com/go-jose/go-jose/" + ("v3" if vulnerable else "v4")
    result = {
        "Type": "gobinary",
        "Target": "usr/bin/caddy",
        "Packages": [{"Name": package, "Version": "v3.0.4" if vulnerable else version}],
    }
    if vulnerable:
        result["Vulnerabilities"] = [
            {
                "VulnerabilityID": "CVE-2026-116587",
                "PkgName": package,
                "Severity": "HIGH",
                "InstalledVersion": "v3.0.4",
                "FixedVersion": "",
                "Title": "Original scanner finding retained verbatim",
            }
        ]
    report["Results"].append(result)
    return report


def platform_remediations(before, after, role="caddy"):
    with tempfile.TemporaryDirectory() as directory:
        scanner = Scanner(Path(directory), lambda *_: None)
        with patch.object(scanner, "image", side_effect=[before, after]):
            scanner.platform(role, "pinned-input", "consumed-output")
        path = Path(directory) / ("remediation-" + role + ".json")
        return json.loads(path.read_text())


class ImageGateTests(unittest.TestCase):
    def test_official_mirror_fallback_still_requires_exact_pinned_bytes(self):
        from contextlib import redirect_stdout

        original = "https://dl-cdn.alpinelinux.org/alpine/v3.24/main/x86_64/fixture.apk"
        expected = (
            "https://mirrors.edge.kernel.org/alpine/v3.24/main/x86_64/fixture.apk"
        )

        class Response(io.BytesIO):
            url = expected

        with (
            tempfile.TemporaryDirectory() as directory,
            redirect_stdout(io.StringIO()),
            patch(
                "maintained.urllib.request.urlopen",
                side_effect=[
                    urllib.error.HTTPError(original, 404, "private-message", {}, None),
                    Response(b"pinned"),
                ],
            ) as request,
        ):
            path = Path(directory) / "input"
            fetch(
                {"url": original, "sha256": hashlib.sha256(b"pinned").hexdigest()}, path
            )
            self.assertEqual(path.read_bytes(), b"pinned")
            self.assertEqual(request.call_args_list[1].args[0].full_url, expected)

    def test_retained_input_fallback_requires_exact_pinned_bytes(self):
        from contextlib import redirect_stdout

        original = "https://example.test/input"
        for content, valid in [(b"pinned", True), (b"changed", False)]:
            with self.subTest(valid=valid), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                cache = root / "cache"
                cache.mkdir()
                digest = hashlib.sha256(b"pinned").hexdigest()
                (cache / digest).write_bytes(content)
                destination = root / "input"
                with (
                    redirect_stdout(io.StringIO()),
                    patch("maintained.PINNED_INPUTS", cache, create=True),
                    patch(
                        "maintained.urllib.request.urlopen",
                        side_effect=urllib.error.HTTPError(
                            original, 404, "unavailable", {}, None
                        ),
                    ),
                ):
                    if valid:
                        fetch({"url": original, "sha256": digest}, destination)
                        self.assertEqual(destination.read_bytes(), b"pinned")
                    else:
                        with self.assertRaises(Refused):
                            fetch({"url": original, "sha256": digest}, destination)
                        self.assertFalse(destination.exists())

    def test_retained_input_cannot_be_a_symlink(self):
        from contextlib import redirect_stdout

        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            cache = root / "cache"
            cache.mkdir()
            digest = hashlib.sha256(b"pinned").hexdigest()
            target = root / "outside"
            target.write_bytes(b"pinned")
            (cache / digest).symlink_to(target)
            with (
                redirect_stdout(io.StringIO()),
                patch("maintained.PINNED_INPUTS", cache, create=True),
                patch(
                    "maintained.urllib.request.urlopen",
                    side_effect=urllib.error.HTTPError(
                        "https://example.test/input", 404, "unavailable", {}, None
                    ),
                ),
            ):
                with self.assertRaises(Refused):
                    fetch(
                        {"url": "https://example.test/input", "sha256": digest},
                        root / "input",
                    )
                self.assertFalse((root / "input").exists())

    def test_temporal_consumed_images_require_scanned_private_derivatives(self):
        ref = "registry.example.com/app@sha256:" + "a" * 64
        lock = json.loads((ROOT / "deploy/aws/images.lock.json").read_text())
        value = {
            "images": {n: ref for n in APPLICATIONS},
            "platform_images": {n: ref for n in PLATFORM},
        }
        value["platform_images"]["valkey"] = lock["images"]["valkey"]["image"]
        validate_images(value, "registry.example.com/app", lock)
        for name in ("temporal", "temporal-admin"):
            changed = copy.deepcopy(value)
            changed["platform_images"][name] = lock["images"][name]["image"]
            with self.assertRaises(Refused):
                validate_images(changed, "registry.example.com/app", lock)

    def test_temporal_remediation_preserves_runtime_identity(self):
        for key in ("User", "Cmd", "Entrypoint", "WorkingDir"):
            with tempfile.TemporaryDirectory() as directory:
                scanner = Scanner(Path(directory), lambda *_: None)
                before, after = os_report(), os_report()
                before["Metadata"]["ImageConfig"]["config"] = {key: "original"}
                after["Metadata"]["ImageConfig"]["config"] = {key: "changed"}
                with patch.object(scanner, "image", side_effect=[before, after]):
                    with self.assertRaises(Refused):
                        scanner.platform("temporal", "input", "output")

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

    def test_caddy_major_module_fix_retains_original_and_replacement_evidence(self):
        for version in ("v4.1.6", "v4.1.7", "v4.2.0", "4.1.6"):
            with self.subTest(version=version):
                before = caddy_report(vulnerable=True)
                original = copy.deepcopy(before)
                report = platform_remediations(before, caddy_report(version=version))
                remediation = report["resolutions"][0]
                self.assertEqual(report["input"], "pinned-input")
                self.assertEqual(report["output"], "consumed-output")
                self.assertEqual(before, original)
                self.assertEqual(
                    remediation["original_finding"],
                    original["Results"][1]["Vulnerabilities"][0],
                )
                self.assertEqual(
                    remediation["package"], "github.com/go-jose/go-jose/v3"
                )
                self.assertEqual(remediation["before"], "v3.0.4")
                self.assertEqual(remediation["upstream_fixed_versions"], "")
                self.assertEqual(
                    remediation["replacement_package"], "github.com/go-jose/go-jose/v4"
                )
                self.assertEqual(remediation["replacement_versions"], [version])
                self.assertEqual(
                    remediation["advisory"],
                    "https://github.com/go-jose/go-jose/security/advisories/GHSA-mgcc-cww5-fp2q",
                )

    def test_caddy_major_module_fix_cannot_waive_other_scope_or_incomplete_inventory(
        self,
    ):
        cases = [
            "other-role",
            "other-cve",
            "other-package",
            "input-target",
            "input-type",
            "output-target",
            "output-type",
            "missing-v4",
            "missing-os",
            "v3-same-target",
            "v3-other-target",
            "v3-finding",
            "other-disappeared",
            "new-high",
            "new-critical",
        ]
        for case in cases:
            with self.subTest(case=case):
                before, after = caddy_report(vulnerable=True), caddy_report()
                original, current = before["Results"][1], after["Results"][1]
                role = "api" if case == "other-role" else "caddy"
                if case == "other-cve":
                    original["Vulnerabilities"][0]["VulnerabilityID"] = "CVE-other"
                elif case == "other-package":
                    original["Vulnerabilities"][0]["PkgName"] = "example.test/other/v3"
                elif case in (
                    "input-target",
                    "input-type",
                    "output-target",
                    "output-type",
                ):
                    result = original if case.startswith("input") else current
                    result["Target" if case.endswith("target") else "Type"] = "other"
                elif case == "missing-v4":
                    current["Packages"] = [{"Name": "stdlib", "Version": "v1.26.8"}]
                elif case == "missing-os":
                    after["Results"] = [current]
                elif case in ("v3-same-target", "v3-other-target"):
                    remnant = copy.deepcopy(original)
                    remnant.pop("Vulnerabilities")
                    if case == "v3-other-target":
                        remnant["Target"] = "other-binary"
                    after["Results"].append(remnant)
                elif case == "v3-finding":
                    after["Results"][0]["Vulnerabilities"] = [
                        {"PkgName": "github.com/go-jose/go-jose/v3", "Severity": "LOW"}
                    ]
                elif case == "other-disappeared":
                    before["Results"][0]["Vulnerabilities"] = [
                        {
                            "VulnerabilityID": "CVE-other",
                            "PkgName": "disappeared",
                            "Severity": "HIGH",
                            "InstalledVersion": "1",
                            "FixedVersion": "2",
                        }
                    ]
                elif case in ("new-high", "new-critical"):
                    current["Vulnerabilities"] = [
                        {"Severity": "HIGH" if case == "new-high" else "CRITICAL"}
                    ]
                    with self.assertRaisesRegex(Refused, "still has high/critical"):
                        platform_remediations(before, after, role)
                    continue
                with self.assertRaises(Refused):
                    platform_remediations(before, after, role)

    def test_caddy_replacement_rejects_old_unknown_and_prerelease_versions(self):
        for version in (
            "v4.1.5",
            "v4.0.9",
            "v3.9.9",
            "v5.0.0",
            "v4.1.6-rc.1",
            "v4.1.7-pre",
            "v4.1.6+unknown",
            "unknown",
            "",
            None,
        ):
            with self.subTest(version=version):
                with self.assertRaises(Refused):
                    platform_remediations(
                        caddy_report(vulnerable=True), caddy_report(version=version)
                    )
        # A fixed copy must not hide another old v4 instance in that inventory.
        after = caddy_report()
        after["Results"][1]["Packages"].append(
            {"Name": "github.com/go-jose/go-jose/v4", "Version": "v4.1.5"}
        )
        with self.assertRaises(Refused):
            platform_remediations(caddy_report(vulnerable=True), after)
        # Direct callers that omit an explicit role get no replacement exception.
        with self.assertRaises(Refused):
            reconcile(caddy_report(vulnerable=True), caddy_report())

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
