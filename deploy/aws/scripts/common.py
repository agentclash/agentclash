"""Small, quiet host primitives. Never expose child output in SSM/CI responses."""

import hashlib
import json
import os
from pathlib import Path
import re
import subprocess


class Refused(RuntimeError):
    pass


def require(ok, message):
    if not ok:
        raise Refused(message)


def private_json(path):
    path = Path(path)
    require(
        path.is_file() and not path.is_symlink(),
        "Private configuration must be a regular file",
    )
    require(
        path.stat().st_mode & 0o077 == 0,
        "Private configuration permissions must be 0600",
    )
    require(
        path.stat().st_uid == os.geteuid(),
        "Private configuration must belong to the executing operator",
    )
    return json.loads(path.read_text())


def run(argv, *, data=None, env=None, timeout=300):
    result = subprocess.run(
        argv, input=data, env=env, capture_output=True, timeout=timeout
    )
    # Some tools echo SQL, endpoints or provider errors. Do not forward either stream.
    require(
        result.returncode == 0,
        "Command failed; inspect the private host state before retrying",
    )
    return result.stdout


def aws(settings, *args):
    command = ["aws", "--no-cli-pager", "--region", settings["region"]]
    if settings.get("profile"):
        command += ["--profile", settings["profile"]]
    return run(command + list(args))


def account_guard(settings):
    require(
        re.fullmatch(r"[0-9]{12}", settings.get("account_id", "")),
        "Expected account is required",
    )
    require(settings.get("operator", "").strip(), "Named platform operator is required")
    actual = json.loads(aws(settings, "sts", "get-caller-identity", "--output", "json"))
    require(
        actual["Account"] == settings["account_id"],
        "AWS account mismatch; no changes made",
    )


def digest(value):
    require(
        isinstance(value, str) and re.fullmatch(r"[a-f0-9]{64}", value),
        "Invalid immutable digest",
    )
    return value


def verify_blob(blob, expected):
    require(
        hashlib.sha256(blob).hexdigest() == digest(expected), "Content digest mismatch"
    )


def image_ref(value):
    require(
        isinstance(value, str)
        and re.fullmatch(r"[a-zA-Z0-9./_-]+@sha256:[a-f0-9]{64}", value),
        "Image must use an immutable repository digest",
    )
    return value


def write_private(path, content, uid=None):
    path = Path(path)
    require(not path.is_symlink(), "Symlink destination refused")
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w") as stream:
        stream.write(content)
    if uid is not None:
        os.chown(path, uid, uid)
