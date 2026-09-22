#!/usr/bin/env python3
"""Exercise exact fixture allowances with the real pinned source-secret scanner."""

import argparse
import base64
import json
import os
from pathlib import Path
import re
import secrets
import string
import subprocess
import tempfile
import tomllib

ROOT = Path(__file__).resolve().parents[1]


def verify(gitleaks, evidence):
    evidence = Path(evidence).resolve()
    if evidence.is_relative_to(ROOT):
        raise RuntimeError("Scanner evidence must remain outside Git")
    policy = ROOT / ".gitleaks.toml"
    settings = tomllib.loads(policy.read_text())
    assert set(settings) == {"extend", "allowlists"}
    assert settings["extend"] == {"useDefault": True}
    names = set()
    for entry in settings["allowlists"]:
        assert entry["condition"] == "AND" and entry["regexTarget"] == "secret"
        assert entry["targetRules"] and entry["regexes"] and entry["paths"]
        for pattern in entry["paths"]:
            assert pattern.startswith("(^|/)") and pattern.endswith("$")
            name = re.sub(r"\\(.)", r"\1", pattern.removeprefix("(^|/)")[:-1])
            assert not Path(name).is_absolute() and ".." not in Path(name).parts
            assert "(^|/)" + re.escape(name).replace(r"\-", "-") + "$" == pattern
            names.add(name)
    with tempfile.TemporaryDirectory(prefix="source-policy-", dir=evidence) as tmp:
        fixture = Path(tmp)
        for name in names:
            target = fixture / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes((ROOT / name).read_bytes())

        def scan(label, expect_clean):
            report = evidence / (label + ".json")
            env = {
                key: value
                for key, value in os.environ.items()
                if key in ("PATH", "HOME")
            }
            with (evidence / (label + ".log")).open("wb") as log:
                result = subprocess.run(
                    [
                        str(gitleaks),
                        "dir",
                        str(fixture),
                        "--config",
                        str(policy),
                        "--gitleaks-ignore-path",
                        os.devnull,
                        "--ignore-gitleaks-allow",
                        "--redact",
                        "--no-banner",
                        "--report-format",
                        "json",
                        "--report-path",
                        str(report),
                    ],
                    env=env,
                    stdout=log,
                    stderr=subprocess.STDOUT,
                    timeout=60,
                )
            findings = json.loads(report.read_text())
            assert result.returncode == (0 if expect_clean else 1)
            assert bool(findings) is not expect_clean
            if label.startswith("changed-value-"):
                assert {"github-pat", "generic-api-key"} <= {
                    finding["RuleID"] for finding in findings
                }

        scan("reviewed-fixtures", True)
        # A changed value in every allowed path must still be rejected, even if
        # someone adds an inline allow comment. Tokens are synthetic test data only.
        alphabet = string.ascii_letters + string.digits
        for index, name in enumerate(sorted(names)):
            target = fixture / name
            original = target.read_bytes()
            token = "ghp_" + "".join(secrets.choice(alphabet) for _ in range(36))
            generic = "whsec_" + base64.b64encode(
                b"agentclash-secret-policy-probe-never-valid"
            ).decode()
            target.write_bytes(
                original
                + ('\n# GITHUB_TOKEN="' + token + '" # gitleaks:allow\n').encode()
                + ('\n# SECRET_TOKEN="' + generic + '" # gitleaks:allow\n').encode()
            )
            scan("changed-value-" + str(index), False)
            target.write_bytes(original)
        # A reviewed value in an unreviewed path must also be rejected.
        (fixture / "unreviewed.txt").write_bytes(
            (ROOT / "backend/internal/api/billing_test.go").read_bytes()
        )
        scan("unreviewed-path", False)
    print("Secret policy accepts reviewed fixtures and rejects changed values/paths")


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--gitleaks", required=True)
    parser.add_argument("--evidence-dir", required=True)
    args = parser.parse_args()
    verify(args.gitleaks, args.evidence_dir)


if __name__ == "__main__":
    main()
