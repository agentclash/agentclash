"""Strict consumed-image gates with retained, explicitly reconciled input findings."""

import json
import os
import hashlib
import tarfile

from common import require, image_ref
from release_contract import canonical, sha256


def findings(report):
    return [
        (result, finding)
        for result in report.get("Results", [])
        for finding in result.get("Vulnerabilities", [])
        if finding.get("Severity") in ("HIGH", "CRITICAL")
    ]


def coverage(report, role):
    results = report.get("Results", [])
    config = report.get("Metadata", {}).get("ImageConfig", {})
    require(
        config.get("os") == "linux" and config.get("architecture") == "amd64",
        "Scanner inspected the wrong image platform",
    )
    require(
        report.get("Metadata", {}).get("OS", {}).get("Family") == "alpine"
        and any(r.get("Type") == "alpine" and r.get("Packages") for r in results),
        "OS package inventory was not inspected",
    )
    targets = {
        "caddy": ("usr/bin/caddy",),
        "postgres": ("usr/local/bin/gosu",),
        "go": ("usr/local/go/bin/go", "usr/local/go/bin/gofmt"),
        "api": ("app",),
        "worker": ("app",),
        "app-schema": ("app",),
    }
    for target in targets.get(role, ()):
        require(
            any(
                r.get("Target") == target
                and r.get("Type") == "gobinary"
                and r.get("Packages")
                for r in results
            ),
            "Required executable dependency inventory is missing",
        )
    if role in ("temporal", "temporal-admin"):
        require(
            any(r.get("Type") == "gobinary" and r.get("Packages") for r in results),
            "Temporal executable coverage is missing",
        )
    if role == "terminal":
        require(
            any(r.get("Type") == "node-pkg" and r.get("Packages") for r in results),
            "Terminal dependency coverage is missing",
        )


def reconcile(before, after):
    """Every original affected component must still exist, at a changed version.

    A clean full output scan is also mandatory. Deleting inventory or moving a
    vulnerable binary out of scanner coverage cannot satisfy this comparison.
    New advisories remain blockers even when all previously known ones are fixed.
    """
    require(not findings(after), "Consumed image still has high/critical findings")
    remediations = []
    for result, finding in findings(before):
        candidates = [
            package
            for current in after.get("Results", [])
            if current.get("Type") == result.get("Type")
            and (
                result.get("Class") == "os-pkgs"
                or current.get("Target") == result.get("Target")
            )
            for package in current.get("Packages", [])
            if package.get("Name") == finding["PkgName"]
        ]
        require(
            candidates, "Affected input component disappeared from scanner coverage"
        )
        versions = sorted({p["Version"] for p in candidates})
        require(
            finding["InstalledVersion"] not in versions,
            "Affected input version remains in consumed output",
        )
        require(
            finding.get("FixedVersion"), "Input finding has no reviewed upstream fix"
        )
        remediations.append(
            {
                "id": finding["VulnerabilityID"],
                "package": finding["PkgName"],
                "target": result["Target"],
                "before": finding["InstalledVersion"],
                "after": versions,
                "upstream_fixed_versions": finding["FixedVersion"],
                "resolution": "replaced component; full consumed-image scan passed",
            }
        )
    return remediations


class Scanner:
    def __init__(self, directory, run):
        self.directory, self.run = directory, run
        self.reports = []

    def export_upstream(self, name, ref, role):
        # Pure FROM preserves upstream bytes. Export only the pinned platform
        # through the builder's digest cache instead of repeatedly pulling for
        # scans or saving absent platforms from a partial local image index.
        image_ref(ref)
        context = self.directory / (name + "-export")
        context.mkdir(mode=0o700)
        (context / "Dockerfile").write_text("ARG UPSTREAM\nFROM ${UPSTREAM}\n")
        archive = self.directory / (name + "-input.tar")
        self.run(
            [
                "docker",
                "build",
                "--platform",
                "linux/amd64",
                "--provenance=false",
                "--output",
                "type=docker,dest=" + str(archive),
                "--build-arg",
                "UPSTREAM=" + ref,
                "-t",
                "agentclash-source-input:" + role,
                str(context),
            ],
            timeout=1200,
        )
        with tarfile.open(archive) as exported:
            entries = json.load(exported.extractfile("manifest.json"))
            require(len(entries) == 1, "Input archive must contain one platform")
            config = exported.extractfile(entries[0]["Config"]).read()
            expected_id = "sha256:" + sha256(config)
        with archive.open("rb") as stream:
            archive_sha = hashlib.file_digest(stream, "sha256").hexdigest()
        provenance = self.directory / (name + "-input-export.json")
        provenance.write_bytes(
            canonical(
                {
                    "upstream": ref,
                    "platform": "linux/amd64",
                    "archive_sha256": archive_sha,
                    "image_id": expected_id,
                }
            )
        )
        self.reports.append(provenance)
        return archive, expected_id

    def image(self, name, ref, role, *, source_input=False):
        archive = None
        if ref.startswith("sha256:"):
            source_options, target, expected_id = ["--image-src", "docker"], [ref], ref
        else:
            archive, expected_id = self.export_upstream(name, ref, role)
            source_options, target = ["--input", str(archive)], []
        common = [
            str(self.directory / "trivy"),
            "image",
            "--platform",
            "linux/amd64",
            *source_options,
            "--quiet",
            "--ignorefile",
            os.devnull,
        ]
        for scanner in ("secret", "vuln"):
            report = self.directory / (name + "-" + scanner + ".json")
            # Nonzero scanner failures always propagate. Only the vulnerability
            # result on an explicitly named source input is reconciled later.
            command = common + [
                "--scanners",
                scanner,
                "--exit-code",
                "0" if source_input and scanner == "vuln" else "1",
                "--format",
                "json",
                "--output",
                str(report),
            ]
            if scanner == "vuln":
                command += ["--severity", "HIGH,CRITICAL", "--list-all-pkgs"]
            self.run(command + target, timeout=1200)
            self.reports.append(report)
        value = json.loads(report.read_text())
        require(
            value.get("Metadata", {}).get("ImageID") == expected_id,
            "Scanner image identity differs from the selected bytes",
        )
        coverage(value, role)
        if not source_input:
            require(not findings(value), "Image vulnerability gate failed")
        sbom = self.directory / (name + "-sbom.json")
        self.run(
            common + ["--format", "cyclonedx", "--output", str(sbom), *target],
            timeout=1200,
        )
        require(
            json.loads(sbom.read_text()).get("components"),
            "Image SBOM inventory is empty",
        )
        self.reports.append(sbom)
        if archive:
            archive.unlink()
        return value

    def platform(self, name, input_ref, output_id):
        if input_ref == output_id:
            self.image("platform-" + name, output_id, name)
            return
        before = self.image("input-" + name, input_ref, name, source_input=True)
        after = self.image("platform-" + name, output_id, name)
        report = self.directory / ("remediation-" + name + ".json")
        report.write_bytes(
            canonical(
                {
                    "input": input_ref,
                    "output": output_id,
                    "resolutions": reconcile(before, after),
                    "strict_output_gate_passed": True,
                }
            )
        )
        self.reports.append(report)
