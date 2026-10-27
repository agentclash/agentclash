"""Quiet, scoped AWS transport with short-lived credentials and no public metadata."""

import base64
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time
import urllib.parse
import urllib.request

from common import require, digest, verify_blob
from release_contract import canonical, sha256


def validate_config(config, kind):
    require(
        config.get("enabled") is True and config["kind"] == kind,
        "Delivery not activated for this job",
    )
    require(
        re.fullmatch(r"[0-9]{12}", config["account_id"]), "Missing destination account"
    )
    require(re.fullmatch(r"[a-z]{2}-[a-z]+-\d", config["region"]), "Invalid region")
    require(
        re.fullmatch(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]", config["artifact_bucket"]),
        "Invalid artifact bucket",
    )
    registry = config["account_id"] + ".dkr.ecr." + config["region"] + ".amazonaws.com/"
    require(
        config["repository"].startswith(registry)
        and re.fullmatch(r"[a-z0-9/_-]+", config["repository"][len(registry) :]),
        "Repository destination mismatch",
    )
    if kind != "build":
        require(kind in ("staging", "production"), "Invalid deployment environment")
        if kind == "staging":
            require(
                time.time() < config.get("expires_at", 0) <= time.time() + 86400,
                "Temporary staging approval must expire within one day",
            )
        require(
            re.fullmatch(r"i-[a-f0-9]{8,17}", config["instance_id"]),
            "Invalid target instance",
        )
        for operation in ("deploy", "promote"):
            doc = config["documents"][operation]
            require(
                re.fullmatch(r"[a-zA-Z0-9_.-]{3,128}", doc["name"])
                and re.fullmatch(r"[1-9][0-9]*", doc["version"]),
                "Document must have an exact numeric version",
            )
            digest(doc["sha256"])


def get_json(url, token=None):
    headers = {
        "Accept": "application/vnd.github+json",
        "User-Agent": "agentclash-delivery",
        "X-GitHub-Api-Version": "2026-03-10",
    }
    if token:
        headers["Authorization"] = "Bearer " + token
    with urllib.request.urlopen(
        urllib.request.Request(url, headers=headers), timeout=30
    ) as response:
        return json.load(response)


def protection_snapshot(environment, branches, rules):
    require(
        any(
            rule["type"] == "required_reviewers" and rule.get("reviewers")
            for rule in environment.get("protection_rules", [])
        ),
        "Environment has no required reviewers",
    )
    require(
        environment.get("deployment_branch_policy")
        == {"protected_branches": False, "custom_branch_policies": True},
        "Environment must restrict explicit branches",
    )
    require(
        branches["total_count"] == 1
        and [(p["name"], p["type"]) for p in branches["branch_policies"]]
        == [("main", "branch")],
        "Only main may deploy",
    )
    require(
        any(rule["type"] == "pull_request" for rule in rules),
        "Main lacks required pull requests",
    )
    require(
        any(
            rule["type"] == "required_status_checks"
            and any(
                check["context"] == "Release checks"
                for check in rule["parameters"]["required_status_checks"]
            )
            for rule in rules
        ),
        "Main lacks the aggregate required status",
    )
    # No private repo/actor identifiers leave this process. A reviewed digest is
    # stored in the environment secret; any visible policy drift blocks OIDC.
    return sha256(
        canonical(
            {
                "environment": {
                    key: environment.get(key)
                    for key in (
                        "name",
                        "protection_rules",
                        "deployment_branch_policy",
                        "can_admins_bypass",
                    )
                },
                "branches": branches["branch_policies"],
                "rules": rules,
            }
        )
    )


def github_guard(config, workflow):
    github = config["github"]
    require(os.environ.get("GITHUB_ACTIONS") == "true", "GitHub-only credential path")
    require(
        os.environ.get("GITHUB_REF") == "refs/heads/main",
        "Only trusted main may deliver",
    )
    require(
        os.environ.get("GITHUB_EVENT_NAME") in ("push", "workflow_dispatch"),
        "Untrusted trigger",
    )
    require(
        os.environ.get("GITHUB_REPOSITORY") == github["repository"],
        "Repository mismatch",
    )
    require(
        os.environ.get("GITHUB_WORKFLOW_REF")
        == github["repository"] + "/.github/workflows/" + workflow + "@refs/heads/main",
        "Workflow mismatch",
    )
    require(
        github["environment"] == "aws-" + config["kind"],
        "Environment configuration mismatch",
    )
    require(
        github.get("admin_bypass_verified_disabled") is True,
        "Missing private administrator bypass verification",
    )
    # REST does not consistently expose admin bypass state. Step 11 must verify
    # it and reviewer availability through the UI and actual denial tests.
    base = "https://api.github.com/repos/" + github["repository"]
    token = os.environ["GITHUB_TOKEN"]
    env_url = (
        base + "/environments/" + urllib.parse.quote(github["environment"], safe="")
    )
    environment = get_json(env_url, token)
    require(
        environment["name"] == github["environment"]
        and environment.get("can_admins_bypass") is not True,
        "Environment protection mismatch",
    )
    snapshot = protection_snapshot(
        environment,
        get_json(env_url + "/deployment-branch-policies?per_page=100", token),
        get_json(base + "/rules/branches/main", token),
    )
    require(
        snapshot == digest(github["protection_sha256"]),
        "GitHub protections differ from verified configuration",
    )
    require(
        "*" not in github["oidc_subject"] and "?" not in github["oidc_subject"],
        "OIDC subject must be exact",
    )


class Cloud:
    def __init__(self, config, directory, *, mode="oidc", workflow="aws-deploy.yml"):
        self.config, self.directory, self.mode = config, Path(directory), mode
        self.env = {
            k: v
            for k, v in os.environ.items()
            if k
            in (
                "PATH",
                "HOME",
                "USER",
                "TMPDIR",
                "HTTPS_PROXY",
                "HTTP_PROXY",
                "NO_PROXY",
            )
        }
        self.env.update(
            {
                "AWS_EC2_METADATA_DISABLED": "true",
                "AWS_CONFIG_FILE": os.devnull,
                "AWS_SHARED_CREDENTIALS_FILE": os.devnull,
            }
        )
        self.expiry = 0
        if mode == "oidc":
            github_guard(config, workflow)
        else:
            require(
                mode == "sso" and config.get("profile") and config.get("operator"),
                "Explicit SSO operator/profile required",
            )
            # SSO uses the existing local profile/cache, never long-lived access
            # keys supplied through this tool's environment.
            self.env.pop("AWS_CONFIG_FILE")
            self.env.pop("AWS_SHARED_CREDENTIALS_FILE")
        self.refresh()
        identity = json.loads(self.call("sts", "get-caller-identity", refresh=False))
        require(identity["Account"] == config["account_id"], "AWS account mismatch")

    def raw(self, args, *, data=None, timeout=300, ok=False):
        result = subprocess.run(
            args, input=data, env=self.env, capture_output=True, timeout=timeout
        )
        require(
            ok or result.returncode == 0,
            "Cloud operation failed; details were not published",
        )
        return result

    def refresh(self):
        if self.mode != "oidc" or time.time() < self.expiry - 180:
            return
        github = self.config["github"]
        url = os.environ["ACTIONS_ID_TOKEN_REQUEST_URL"]
        parsed = urllib.parse.urlsplit(url)
        require(
            parsed.scheme == "https"
            and parsed.hostname.endswith(".actions.githubusercontent.com"),
            "Untrusted OIDC endpoint",
        )
        query = urllib.parse.parse_qs(parsed.query)
        query["audience"] = ["sts.amazonaws.com"]
        url = urllib.parse.urlunsplit(
            parsed._replace(query=urllib.parse.urlencode(query, doseq=True))
        )
        token = get_json(url, os.environ["ACTIONS_ID_TOKEN_REQUEST_TOKEN"])["value"]
        part = token.split(".")[1]
        claims = json.loads(base64.urlsafe_b64decode(part + "=" * (-len(part) % 4)))
        require(
            claims["sub"] == github["oidc_subject"]
            and claims["aud"] == "sts.amazonaws.com"
            and claims["repository"] == github["repository"]
            and claims["ref"] == "refs/heads/main",
            "OIDC claims mismatch",
        )
        require(
            self.config["role_arn"].startswith(
                "arn:aws:iam::" + self.config["account_id"] + ":role/"
            ),
            "Role account mismatch",
        )
        with tempfile.NamedTemporaryFile(dir=self.directory) as token_file:
            token_file.write(token.encode())
            token_file.flush()
            # STS verifies the signed token. The decode above is only a local
            # scope check, not signature verification or an authentication bypass.
            result = self.raw(
                [
                    "aws",
                    "--no-cli-pager",
                    "--region",
                    self.config["region"],
                    "sts",
                    "assume-role-with-web-identity",
                    "--role-arn",
                    self.config["role_arn"],
                    "--role-session-name",
                    "agentclash-delivery",
                    "--web-identity-token",
                    "file://" + token_file.name,
                    "--duration-seconds",
                    "3600",
                    "--output",
                    "json",
                ]
            )
        credentials = json.loads(result.stdout)["Credentials"]
        self.env.update(
            {
                "AWS_ACCESS_KEY_ID": credentials["AccessKeyId"],
                "AWS_SECRET_ACCESS_KEY": credentials["SecretAccessKey"],
                "AWS_SESSION_TOKEN": credentials["SessionToken"],
            }
        )
        self.expiry = time.time() + 3500

    def call(self, *args, data=None, timeout=300, refresh=True):
        if refresh:
            self.refresh()
        prefix = ["aws", "--no-cli-pager", "--region", self.config["region"]]
        if self.mode == "sso":
            prefix += ["--profile", self.config["profile"]]
        return self.raw(prefix + list(args), data=data, timeout=timeout).stdout

    def get(self, key):
        return self.call(
            "s3",
            "cp",
            "s3://" + self.config["artifact_bucket"] + "/" + key,
            "-",
            "--only-show-errors",
        )

    def put(self, key, path):
        try:
            self.call(
                "s3api",
                "put-object",
                "--bucket",
                self.config["artifact_bucket"],
                "--key",
                key,
                "--body",
                str(path),
                "--server-side-encryption",
                "AES256",
                "--if-none-match",
                "*",
                "--output",
                "json",
            )
        except Exception:
            # Reusing the exact immutable object is safe; overwriting a different
            # source index or blob is never an implicit retry operation.
            require(
                self.get(key) == Path(path).read_bytes(),
                "Immutable object already differs or publication was denied",
            )

    def release(self, revision):
        require(
            re.fullmatch(r"[a-f0-9]{40}", revision), "Invalid public source revision"
        )
        index = json.loads(self.get("releases/by-source/" + revision + ".json"))
        require(index["source_revision"] == revision, "Release index mismatch")
        sha = digest(index["release_sha256"])
        raw = self.get("releases/" + sha + ".json")
        verify_blob(raw, sha)
        manifest = json.loads(raw)
        require(
            manifest["source_revision"] == revision
            and manifest["account_id"] == self.config["account_id"]
            and manifest["region"] == self.config["region"],
            "Release destination mismatch",
        )
        return sha, manifest

    def send(self, operation, release_sha):
        doc = self.config["documents"][operation]
        result = json.loads(
            self.call(
                "ssm",
                "send-command",
                "--document-name",
                doc["name"],
                "--document-version",
                doc["version"],
                "--document-hash",
                doc["sha256"],
                "--document-hash-type",
                "Sha256",
                "--instance-ids",
                self.config["instance_id"],
                "--parameters",
                json.dumps({"ReleaseSha256": [digest(release_sha)]}),
                "--timeout-seconds",
                "600",
                "--output",
                "json",
            )
        )
        command_id = result["Command"]["CommandId"]
        deadline = time.monotonic() + 11100
        while time.monotonic() < deadline:
            time.sleep(10)
            # GetCommandInvocation is eventually consistent after dispatch.
            try:
                record = json.loads(
                    self.call(
                        "ssm",
                        "get-command-invocation",
                        "--command-id",
                        command_id,
                        "--instance-id",
                        self.config["instance_id"],
                        "--output",
                        "json",
                    )
                )
            except Exception:
                if time.monotonic() < deadline - 10980:
                    continue
                raise
            state = record["Status"]
            if state in ("Pending", "InProgress", "Delayed"):
                continue
            require(
                state == "Success" and record["ResponseCode"] == 0,
                "Host operation did not succeed; intake may remain fenced",
            )
            return
        raise RuntimeError(
            "SSM wait deadline exceeded; inspect privately before retrying"
        )
