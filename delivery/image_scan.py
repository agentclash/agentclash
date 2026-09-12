"""Strict consumed-image gates with retained, explicitly reconciled input findings."""

import json
import os

from common import require
from release_contract import canonical


def findings(report):
    return [
        (result, finding)
        for result in report.get("Results", [])
        for finding in result.get("Vulnerabilities", [])
        if finding.get("Severity") in ("HIGH", "CRITICAL")
    ]


def coverage(report, role):
    results = report.get("Results", [])
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

    def image(self, name, ref, role, *, source_input=False):
        image_source = "docker" if ref.startswith("sha256:") else "remote"
        common = [
            str(self.directory / "trivy"),
            "image",
            "--platform",
            "linux/amd64",
            "--image-src",
            image_source,
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
            self.run(command + [ref], timeout=1200)
            self.reports.append(report)
        value = json.loads(report.read_text())
        coverage(value, role)
        if not source_input:
            require(not findings(value), "Image vulnerability gate failed")
        sbom = self.directory / (name + "-sbom.json")
        self.run(
            common + ["--format", "cyclonedx", "--output", str(sbom), ref], timeout=1200
        )
        require(
            json.loads(sbom.read_text()).get("components"),
            "Image SBOM inventory is empty",
        )
        self.reports.append(sbom)
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
