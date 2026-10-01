"""Public release schema and deterministic platform hashes; no cloud calls."""

import hashlib
import json
from pathlib import Path
import re
import time

from common import digest, require, image_ref

APPLICATIONS = ("api", "worker", "terminal", "app-schema")
DERIVED = ("alpine", "go", "bun", "caddy", "postgres")
UPSTREAM = ("temporal", "temporal-admin", "valkey")
PLATFORM = (*DERIVED, *UPSTREAM)


def canonical(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n").encode()


def sha256(value):
    return hashlib.sha256(value).hexdigest()


def platform_files(root):
    root = Path(root)
    names = {
        "delivery/release.py",
        "delivery/health.py",
        "delivery/tools.lock.json",
        "delivery/maintained.py",
        "deploy/aws/compose.yaml",
        "deploy/aws/images.lock.json",
    }
    names.update(
        str(p.relative_to(root))
        for p in (root / "deploy/aws/platform").rglob("*")
        if p.is_file()
    )
    for directory, patterns in {
        "deploy/aws/scripts": ("*.py", "*.sh"),
        "deploy/aws/config": ("*",),
        "deploy/aws/systemd": ("*.service", "*.timer", "*.mount"),
    }.items():
        for pattern in patterns:
            names.update(
                str(p.relative_to(root))
                for p in (root / directory).glob(pattern)
                if p.is_file()
            )
    for name in names:
        path = root / name
        require(
            path.is_file()
            and not path.is_symlink()
            and path.resolve().is_relative_to(root.resolve()),
            "Platform source must be regular files inside the checkout",
        )
    return sorted(names)


def tree_hash(root, names):
    return sha256(
        canonical(
            {name: sha256((Path(root) / name).read_bytes()) for name in sorted(names)}
        )
    )


def platform_hash(root):
    return tree_hash(root, platform_files(root))


def recipe_hash(root):
    root = Path(root)
    names = ["delivery/maintained.py", "deploy/aws/images.lock.json"]
    names += [
        str(p.relative_to(root))
        for p in (root / "deploy/aws/platform").rglob("*")
        if p.is_file()
    ]
    return tree_hash(root, names)


def validate_images(value, repository, lock):
    require(set(value["images"]) == set(APPLICATIONS), "Incomplete application images")
    require(
        set(value["platform_images"]) == set(PLATFORM), "Incomplete platform images"
    )
    for image in (
        *value["images"].values(),
        *(value["platform_images"][name] for name in DERIVED),
    ):
        require(
            image_ref(image).split("@")[0] == repository,
            "Image outside approved repository",
        )
    for name in UPSTREAM:
        require(
            image_ref(value["platform_images"][name]) == lock["images"][name]["image"],
            "Unapproved upstream runtime image",
        )


def validate_release(value, root):
    require(value.get("format") == 2, "Unsupported release format")
    require(
        re.fullmatch(r"[a-f0-9]{40}", value.get("source_revision", "")),
        "Invalid source revision",
    )
    for key in ("migration_set_sha256", "build_evidence_sha256", "bootstrap_sha256"):
        digest(value[key])
    require(
        value["platform_source_sha256"] == platform_hash(root),
        "Platform source requires a separately approved bootstrap",
    )
    require(
        value["platform_recipe_sha256"] == recipe_hash(root),
        "Platform image recipe mismatch",
    )


def validate_approval(value, settings, release_sha, previous, operation, now=None):
    now = time.time() if now is None else now
    delivery = settings["delivery"]
    require(delivery.get("enabled") is True, "Delivery is not activated")
    if delivery["environment"] == "staging":
        require(
            now < delivery.get("expires_at", 0) <= now + 86400
            and value["acknowledgements"].get("isolated_rehearsal_verified") is True,
            "Temporary rehearsal is expired or isolation is unverified",
        )
    require(operation in ("deploy", "promote"), "Unknown delivery operation")
    for key in ("account_id", "region", "instance_id"):
        require(value[key] == settings[key], "Approval destination mismatch")
    require(
        value["environment"] == delivery["environment"] in ("staging", "production"),
        "Approval environment mismatch",
    )
    require(
        value["operation"] == operation
        and value["release_sha256"] == digest(release_sha),
        "Approval operation mismatch",
    )
    require(
        value["previous_release_sha256"] == previous, "Approval predecessor mismatch"
    )
    require(value["operator"] == settings["operator"], "Approval operator mismatch")
    require(
        now - 3600 <= value["issued_at"] <= now + 60
        and now < value["expires_at"] <= value["issued_at"] + 3600,
        "Approval expired or outside its one-hour window",
    )
    for key in (
        "drain_verified",
        "callbacks_reconciled",
        "no_automatic_schema_rollback",
        "destination_writes_acknowledged",
    ):
        require(
            value["acknowledgements"].get(key) is True, "Operator acceptance incomplete"
        )
    if previous is None:
        require(
            value["acknowledgements"].get("no_previous_aws_release") is True,
            "First release has no image rollback",
        )
    required = {"drain", "recovery"}
    if delivery["environment"] == "production":
        required.add("rehearsal")
    if operation == "promote":
        required.add("smoke")
    require(required <= value["evidence"].keys(), "Missing reviewed evidence")
    for name in required:
        digest(value["evidence"][name])
    return required
