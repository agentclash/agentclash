#!/usr/bin/env python3
"""Verify protected delivery without starting application workloads."""

import base64
import json
import os
from pathlib import Path
import re
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "deploy/aws/scripts"))
from common import require, digest
from cloud import Cloud, get_json, validate_config


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise RuntimeError("Receipt redirects are forbidden")


def validate(config, env=None, now=None):
    env = os.environ if env is None else env
    now = time.time() if now is None else now
    kind = config["kind"]
    require(kind in ("build", "production"), "Unsupported verification target")
    require(config["phase"] in ("claims", "permissions"), "Invalid phase")
    require(now < config["expires_at"] <= now + 3600, "Activation window expired")
    require(re.fullmatch(r"[a-f0-9]{32}", config["nonce"]), "Invalid receipt nonce")
    require(re.fullmatch(r"[a-f0-9]{40}", config["source_revision"]), "Invalid source")
    workflow = (
        config["repository"] + "/.github/workflows/aws-verify.yml@refs/heads/main"
    )
    for key, value in {
        "GITHUB_ACTIONS": "true",
        "GITHUB_EVENT_NAME": "workflow_dispatch",
        "GITHUB_REF": "refs/heads/main",
        "GITHUB_REPOSITORY": config["repository"],
        "GITHUB_WORKFLOW_REF": workflow,
        "GITHUB_SHA": config["source_revision"],
        "ACTIVATION_KIND": kind,
    }.items():
        require(env.get(key) == value, "Unexpected verification context")
    receipt = config["receipt"]
    url = urllib.parse.urlsplit(receipt["url"])
    require(
        re.fullmatch(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]", receipt["bucket"]),
        "Invalid bucket",
    )
    require(re.fullmatch(r"[a-z]{2}-[a-z]+-\d", receipt["region"]), "Invalid region")
    expected_key = (
        "build-evidence/activation/"
        + config["nonce"]
        + "/"
        + kind
        + "/"
        + config["phase"]
        + ".json"
    )
    require(
        url.scheme == "https"
        and url.hostname
        == receipt["bucket"] + ".s3." + receipt["region"] + ".amazonaws.com"
        and not url.username
        and not url.password
        and url.port is None
        and not url.fragment
        and url.path == "/" + expected_key,
        "Unexpected private receipt destination",
    )
    require(
        "X-Amz-Signature" in urllib.parse.parse_qs(url.query), "Unsigned receipt URL"
    )
    if config["phase"] == "permissions":
        delivery = config["delivery"]
        validate_config(delivery, kind)
        require(
            delivery["github"]["repository"] == config["repository"],
            "Repository mismatch",
        )
        require(
            delivery["github"]["environment"] == "aws-" + kind, "Environment mismatch"
        )
        require(
            delivery["artifact_bucket"] == receipt["bucket"]
            and delivery["region"] == receipt["region"],
            "Destination mismatch",
        )
        require(config["closed_host_verified"] is True, "Missing closed-host check")
        probe = config["probe"]
        require(
            re.fullmatch(r"i-[a-f0-9]{8,17}", probe["instance_id"]),
            "Invalid probe host",
        )
        if kind == "production":
            require(
                probe["instance_id"] == delivery["instance_id"], "Probe host mismatch"
            )
            require(
                probe["documents"] == delivery["documents"], "Probe document mismatch"
            )
        for operation in ("deploy", "promote"):
            doc = probe["documents"][operation]
            require(
                re.fullmatch(r"[a-zA-Z0-9_.-]{3,128}", doc["name"]),
                "Invalid probe document",
            )
            require(
                re.fullmatch(r"[1-9][0-9]*", doc["version"]),
                "Probe document is not pinned",
            )
            digest(doc["sha256"])
        require(
            probe["other_role_arn"] != delivery["role_arn"], "Peer role must differ"
        )
    return config


def token(audience="sts.amazonaws.com"):
    url = urllib.parse.urlsplit(os.environ["ACTIONS_ID_TOKEN_REQUEST_URL"])
    require(
        url.scheme == "https"
        and url.hostname.endswith(".actions.githubusercontent.com"),
        "Unexpected token endpoint",
    )
    query = urllib.parse.parse_qs(url.query)
    query["audience"] = [audience]
    url = urllib.parse.urlunsplit(
        url._replace(query=urllib.parse.urlencode(query, doseq=True))
    )
    return get_json(url, os.environ["ACTIONS_ID_TOKEN_REQUEST_TOKEN"])["value"]


def claims(encoded, config):
    parts = encoded.split(".")
    require(len(parts) == 3, "Malformed token")
    data = json.loads(base64.urlsafe_b64decode(parts[1] + "=" * (-len(parts[1]) % 4)))
    expected = {
        "iss": "https://token.actions.githubusercontent.com",
        "aud": "sts.amazonaws.com",
        "repository": config["repository"],
        "ref": "refs/heads/main",
        "sha": config["source_revision"],
        "environment": "aws-" + config["kind"],
        "workflow_ref": config["repository"]
        + "/.github/workflows/aws-verify.yml@refs/heads/main",
    }
    require(
        all(data.get(k) == v for k, v in expected.items()), "Token context mismatch"
    )
    require(data.get("exp", 0) > time.time(), "Expired token")
    require(
        isinstance(data.get("sub"), str)
        and data["sub"].startswith("repo:")
        and not any(x in data["sub"] for x in "*?"),
        "Invalid observed subject",
    )
    # This is observation only. STS performs signature verification on exchange.
    return {
        key: data[key]
        for key in (
            *expected,
            "sub",
            "repository_id",
            "repository_owner_id",
            "run_id",
            "run_attempt",
        )
        if key in data
    }


def authorization_denied(result, service_code=None):
    codes = service_code or (
        "AccessDenied",
        "AccessDeniedException",
        "UnauthorizedOperation",
    )
    error = result.stderr.decode(errors="replace")
    return result.returncode != 0 and any(
        re.search(r"\(" + re.escape(code) + r"\)", error) for code in codes
    )


def upload(config, report):
    payload = json.dumps(report, sort_keys=True, separators=(",", ":")).encode()
    require(len(payload) <= 65536, "Receipt too large")
    req = urllib.request.Request(
        config["receipt"]["url"],
        data=payload,
        method="PUT",
        headers={
            "Content-Type": "application/json",
            "x-amz-server-side-encryption": "AES256",
            "If-None-Match": "*",
        },
    )
    with urllib.request.build_opener(NoRedirect).open(req, timeout=30) as response:
        require(response.status == 200, "Private receipt upload failed")


def verify_permissions(config, directory):
    delivery = config["delivery"]
    cloud = Cloud(delivery, directory, workflow="aws-verify.yml")
    probe = config["probe"]
    require(
        re.fullmatch(r"releases/[a-f0-9]{64}\.json", probe["release_key"]),
        "Invalid verification release",
    )
    require(
        probe["runtime_secret_arn"].startswith(
            "arn:aws:secretsmanager:"
            + delivery["region"]
            + ":"
            + delivery["account_id"]
            + ":secret:"
        ),
        "Invalid secret probe",
    )
    require(
        probe["other_role_arn"].startswith(
            "arn:aws:iam::" + delivery["account_id"] + ":role/"
        ),
        "Invalid peer role",
    )
    results = []

    def deny(name, *args):
        result = cloud.raw(
            ["aws", "--no-cli-pager", "--region", delivery["region"], *args], ok=True
        )
        require(authorization_denied(result), "A forbidden operation was not denied")
        results.append({"probe": name, "denied": True})

    # An invalid version prevents plaintext disclosure even if a policy is wrong.
    deny(
        "runtime-secret",
        "secretsmanager",
        "get-secret-value",
        "--secret-id",
        probe["runtime_secret_arn"],
        "--version-id",
        "0" * 32,
    )
    body = Path(directory) / "probe.json"
    body.write_text(
        json.dumps({"purpose": "activation-verification", "nonce": config["nonce"]})
    )
    deny(
        "operator-approval-write",
        "s3api",
        "put-object",
        "--bucket",
        delivery["artifact_bucket"],
        "--key",
        "approvals/activation-denial/" + config["nonce"] + ".json",
        "--body",
        str(body),
        "--server-side-encryption",
        "AES256",
        "--if-none-match",
        "*",
    )
    deny(
        "arbitrary-shell",
        "ssm",
        "send-command",
        "--document-name",
        "AWS-RunShellScript",
        "--instance-ids",
        probe["instance_id"],
        "--parameters",
        json.dumps({"commands": ["true"]}),
    )
    deny(
        "unrelated-registry-read",
        "ecr",
        "batch-get-image",
        "--repository-name",
        delivery["repository"].split("/", 1)[1]
        + "/activation-denied-"
        + config["nonce"],
        "--image-ids",
        "imageTag=activation-probe",
    )
    data = cloud.get(probe["release_key"])
    require(isinstance(json.loads(data), dict), "Existing private release unavailable")
    results.append({"probe": "private-release-read", "allowed": True})
    if config["kind"] == "build":
        key = "build-evidence/activation/" + config["nonce"] + "/publication-probe.json"
        cloud.put(key, body)
        require(cloud.get(key) == body.read_bytes(), "Publication probe mismatch")
        cloud.call("ecr", "get-authorization-token")  # Captured only in memory.
        results.append(
            {"probe": "scoped-artifact-publication-and-registry-auth", "allowed": True}
        )
        doc = probe["documents"]["deploy"]
        deny(
            "deploy-document",
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
            probe["instance_id"],
            "--parameters",
            json.dumps({"ReleaseSha256": ["0" * 64]}),
        )
    else:
        deny(
            "build-artifact-write",
            "s3api",
            "put-object",
            "--bucket",
            delivery["artifact_bucket"],
            "--key",
            "build-evidence/activation/" + config["nonce"] + "/forbidden.json",
            "--body",
            str(body),
            "--server-side-encryption",
            "AES256",
            "--if-none-match",
            "*",
        )
        deny(
            "registry-publication",
            "ecr",
            "initiate-layer-upload",
            "--repository-name",
            delivery["repository"].split("/", 1)[1],
        )
        for operation in ("deploy", "promote"):
            doc = delivery["documents"][operation]
            command = json.loads(
                cloud.call(
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
                    delivery["instance_id"],
                    "--parameters",
                    json.dumps({"ReleaseSha256": ["0" * 64]}),
                    "--output",
                    "json",
                )
            )["Command"]["CommandId"]
            for _ in range(40):
                time.sleep(3)
                invocation = cloud.raw(
                    [
                        "aws",
                        "--no-cli-pager",
                        "--region",
                        delivery["region"],
                        "ssm",
                        "get-command-invocation",
                        "--command-id",
                        command,
                        "--instance-id",
                        delivery["instance_id"],
                        "--output",
                        "json",
                    ],
                    ok=True,
                )
                if invocation.returncode:
                    require(
                        b"InvocationDoesNotExist" in invocation.stderr,
                        "Command observation failed",
                    )
                    continue
                state = json.loads(invocation.stdout)
                if state["Status"] in ("Pending", "InProgress", "Delayed"):
                    continue
                require(
                    state["Status"] == "Failed" and state["ResponseCode"] != 0,
                    "Closed-host refusal was not observed",
                )
                results.append(
                    {
                        "probe": operation + "-fixed-document-closed-host",
                        "allowed": True,
                        "host_refused": True,
                        "command_id": command,
                    }
                )
                break
            else:
                raise RuntimeError("Closed-host probe timed out")
    # Use real signed tokens; never store token or temporary credentials in receipts.
    for label, role, aud in (
        (
            "wrong-audience",
            delivery["role_arn"],
            "https://invalid.agentclash.example/activation",
        ),
        ("other-environment-role", probe["other_role_arn"], "sts.amazonaws.com"),
    ):
        with tempfile.NamedTemporaryFile(dir=directory) as path:
            path.write(token(aud).encode())
            path.flush()
            result = cloud.raw(
                [
                    "aws",
                    "--no-cli-pager",
                    "--region",
                    delivery["region"],
                    "sts",
                    "assume-role-with-web-identity",
                    "--role-arn",
                    role,
                    "--role-session-name",
                    "activation-denial",
                    "--web-identity-token",
                    "file://" + path.name,
                    "--duration-seconds",
                    "900",
                ],
                ok=True,
            )
        denied = authorization_denied(
            result,
            ("AccessDenied", "InvalidIdentityToken")
            if label == "wrong-audience"
            else ("AccessDenied",),
        )
        if not denied:
            # Only a fixed probe label and known status category may be public.
            # In particular, an unexpected allow must never print credentials.
            category = "unexpected-allow" if result.returncode == 0 else "other-error"
            for code in (
                "AccessDenied",
                "AccessDeniedException",
                "InvalidIdentityToken",
                "ExpiredToken",
                "ValidationError",
                "IDPRejectedClaim",
            ):
                if ("(" + code + ")").encode() in result.stderr:
                    category = code
                    break
            print(
                "Trust probe unexpected result: " + label + "/" + category,
                file=sys.stderr,
            )
        require(denied, "Invalid trust context was not denied")
        if b"(InvalidIdentityToken)" in result.stderr:
            require(
                b"audience" in result.stderr.lower(),
                "Trust denial was unrelated to audience",
            )
        results.append({"probe": label, "denied": True})
    return results


def main():
    os.umask(0o077)
    config = validate(json.loads(os.environ["AWS_ACTIVATION_CONFIG"]))
    observed = claims(token(), config)
    report = {
        "phase": config["phase"],
        "kind": config["kind"],
        "nonce": config["nonce"],
        "source_revision": config["source_revision"],
        "claims": observed,
    }
    if config["phase"] == "permissions":
        with tempfile.TemporaryDirectory(prefix="activation-") as directory:
            report["probes"] = verify_permissions(config, directory)
    upload(config, report)
    print("Protected delivery verification passed; evidence retained privately")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        from checks import failure_location, refusal_detail

        location = failure_location(error)
        if location:
            print("Verification source location: " + location, file=sys.stderr)
        detail = refusal_detail(error)
        if detail:
            print("Verification invariant: " + detail, file=sys.stderr)
        category = type(error).__name__
        known = {
            "Refused",
            "HTTPError",
            "URLError",
            "TimeoutExpired",
            "KeyError",
            "ValueError",
            "TypeError",
            "FileNotFoundError",
        }
        print(
            "Verification failure category: "
            + (category if category in known else "internal-error"),
            file=sys.stderr,
        )
        if isinstance(error, urllib.error.HTTPError):
            print("Verification HTTP status: " + str(error.code), file=sys.stderr)
        sys.exit("Protected verification refused or failed; inspect private state")
