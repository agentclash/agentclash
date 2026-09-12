"""Build once, scan privately, then publish the same immutable image bytes."""

import json
import os
from pathlib import Path
import re
import subprocess
import tarfile

from common import require, image_ref
from release_contract import canonical, sha256, tree_hash, APPLICATIONS
from tools import install
from package import bundle

ROOT = Path(__file__).resolve().parents[1]
LOCAL_IMAGES = {
    name: "agentclash-step8-test:" + ("migrator" if name == "app-schema" else name)
    for name in APPLICATIONS
}


def build_images(run, revision=None):
    for name, target in (
        ("api", "api-server"),
        ("worker", "worker"),
        ("app-schema", "db-migrate"),
        ("terminal", None),
    ):
        command = [
            "docker",
            "build",
            "--platform",
            "linux/amd64",
            "-f",
            "deploy/aws/Dockerfile." + ("backend" if target else "terminal"),
            "-t",
            LOCAL_IMAGES[name],
        ]
        if target:
            command += ["--build-arg", "TARGET=" + target]
        if revision:
            command += ["--label", "org.opencontainers.image.revision=" + revision]
        run(command + ["."], timeout=2400)


def prepare(directory, revision, private_values=()):
    directory = Path(directory)
    require(re.fullmatch(r"[a-f0-9]{40}", revision), "Invalid source revision")
    require(
        subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT).decode().strip()
        == revision,
        "Build checkout mismatch",
    )
    require(
        not subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT).strip(),
        "Publication requires a clean reviewed checkout",
    )
    # Git's tracked source is the only build context. Ignored local files can
    # never enter images, even if a future Docker allowlist is accidentally broad.
    source = directory / "source"
    source.mkdir(mode=0o700)
    source_archive = directory / "source.tar"
    with source_archive.open("wb") as output:
        result = subprocess.run(
            ["git", "archive", "--format=tar", revision],
            cwd=ROOT,
            stdout=output,
            stderr=subprocess.DEVNULL,
        )
    require(result.returncode == 0, "Source export failed")
    with tarfile.open(source_archive) as archive:
        require(
            all(
                (m.isfile() or m.isdir())
                and not m.name.startswith("/")
                and ".." not in m.name.split("/")
                for m in archive.getmembers()
            ),
            "Source archive contains unsupported paths or links",
        )
        archive.extractall(source, filter="data")
    source_archive.unlink()
    needles = [value.encode() for value in private_values if value]
    for path in source.rglob("*"):
        if path.is_file():
            content = path.read_bytes()
            require(
                not any(value in content for value in needles),
                "Private deployment identifier found in tracked source",
            )
    env = {
        k: v
        for k, v in os.environ.items()
        if k in ("PATH", "HOME", "USER", "TMPDIR", "DOCKER_HOST", "DOCKER_CONTEXT")
    }
    env.update(
        {
            "TRIVY_CONFIG": os.devnull,
            "TRIVY_IGNORE_FILE": os.devnull,
            "TRIVY_CACHE_DIR": str(directory / "trivy-cache"),
        }
    )

    def run(command, timeout=900):
        result = subprocess.run(
            command, cwd=source, env=env, capture_output=True, timeout=timeout
        )
        require(
            result.returncode == 0,
            "Build or scanner gate failed; nothing has been published",
        )
        return result.stdout

    for name in ("compose", "gitleaks", "trivy"):
        install(name, directory / name, "linux_amd64" if name == "compose" else None)
    run(
        [
            str(directory / "gitleaks"),
            "dir",
            str(source),
            "--redact",
            "--no-banner",
            "--report-format",
            "json",
            "--report-path",
            str(directory / "source-secrets.json"),
        ]
    )
    build_images(run, revision)
    image_ids = {}
    for name, reference in LOCAL_IMAGES.items():
        detail = json.loads(run(["docker", "image", "inspect", reference]))[0]
        require(
            detail["Os"] == "linux"
            and detail["Architecture"] == "amd64"
            and detail["Config"]["Labels"]["org.opencontainers.image.revision"]
            == revision,
            "Built image source/architecture mismatch",
        )
        require(
            re.fullmatch(r"sha256:[a-f0-9]{64}", detail["Id"]),
            "Invalid built image identity",
        )
        image_ids[name] = detail["Id"]
    archive = directory / "bootstrap.tar.gz"
    metadata = bundle(source, directory / "compose", archive)
    reports = [directory / "source-secrets.json"]
    lock = json.loads((source / "deploy/aws/images.lock.json").read_text())
    # Every selected runtime/admin/base image is checked, not only the apps.
    images = {
        **image_ids,
        **{"platform-" + k: v["image"] for k, v in lock["images"].items()},
    }
    for name, ref in images.items():
        image_source = "remote" if name.startswith("platform-") else "docker"
        for scanner, extra in (
            ("secret", []),
            ("vuln", ["--severity", "HIGH,CRITICAL"]),
        ):
            report = directory / (name + "-" + scanner + ".json")
            run(
                [
                    str(directory / "trivy"),
                    "image",
                    "--platform",
                    "linux/amd64",
                    "--image-src",
                    image_source,
                    "--quiet",
                    "--ignorefile",
                    os.devnull,
                    "--scanners",
                    scanner,
                    "--exit-code",
                    "1",
                    "--format",
                    "json",
                    "--output",
                    str(report),
                    *extra,
                    ref,
                ],
                timeout=1200,
            )
            reports.append(report)
        sbom = directory / (name + "-sbom.json")
        run(
            [
                str(directory / "trivy"),
                "image",
                "--platform",
                "linux/amd64",
                "--image-src",
                image_source,
                "--quiet",
                "--format",
                "cyclonedx",
                "--output",
                str(sbom),
                ref,
            ],
            timeout=900,
        )
        reports.append(sbom)
    migrations = sorted(
        str(path.relative_to(source))
        for path in (source / "backend/db/migrations").glob("*.sql")
    )
    return {
        "source_revision": revision,
        "bootstrap": metadata,
        "migration_set_sha256": tree_hash(source, migrations),
        "image_ids": image_ids,
        "reports": reports,
        "platform_lock_sha256": sha256(
            (source / "deploy/aws/images.lock.json").read_bytes()
        ),
        "tools_lock_sha256": sha256((source / "delivery/tools.lock.json").read_bytes()),
    }


def publish(cloud, directory, prepared):
    directory = Path(directory)
    config, revision = cloud.config, prepared["source_revision"]
    docker_env = {
        k: v
        for k, v in os.environ.items()
        if not k.startswith("AWS_")
        and k
        not in (
            "AWS_DELIVERY_CONFIG",
            "GITHUB_TOKEN",
            "ACTIONS_ID_TOKEN_REQUEST_TOKEN",
            "ACTIONS_ID_TOKEN_REQUEST_URL",
        )
    }
    docker_env["DOCKER_CONFIG"] = str(directory / "docker-auth")

    def docker(*args, data=None):
        result = subprocess.run(
            ["docker", *args],
            input=data,
            env=docker_env,
            capture_output=True,
            timeout=1200,
        )
        require(
            result.returncode == 0,
            "Image publication failed; inspect the private registry",
        )
        return result.stdout

    password = cloud.call("ecr", "get-login-password")
    registry = config["repository"].split("/")[0]
    docker("login", "--username", "AWS", "--password-stdin", registry, data=password)
    images = {}
    for name, local in prepared["image_ids"].items():
        tag = config["repository"] + ":" + revision + "-" + name
        docker("tag", local, tag)
        docker("push", tag)
        details = json.loads(docker("image", "inspect", tag))[0]
        require(
            details["Os"] == "linux"
            and details["Architecture"] == "amd64"
            and details["Config"]["Labels"]["org.opencontainers.image.revision"]
            == revision,
            "Published image does not match reviewed source/architecture",
        )
        refs = [
            value
            for value in details["RepoDigests"]
            if value.startswith(config["repository"] + "@")
        ]
        require(len(refs) == 1, "Ambiguous published image digest")
        images[name] = image_ref(refs[0])
    evidence = {
        "format": 1,
        "source_revision": revision,
        "images": images,
        "tools_lock_sha256": prepared["tools_lock_sha256"],
        "platform_lock_sha256": prepared["platform_lock_sha256"],
        "scanners_passed": True,
        "provenance": {
            "type": "private-build-record",
            "repository": config.get("github", {}).get("repository"),
            "revision": revision,
            "workflow_ref": os.environ.get("GITHUB_WORKFLOW_REF"),
            "run_id": os.environ.get("GITHUB_RUN_ID"),
            "local_image_ids": prepared["image_ids"],
        },
        "reports": {},
    }
    for report in prepared["reports"]:
        sha = sha256(report.read_bytes())
        cloud.put("build-evidence/objects/" + sha + ".json", report)
        evidence["reports"][report.name] = sha
    evidence_file = directory / "build-evidence.json"
    evidence_file.write_bytes(canonical(evidence))
    evidence_sha = sha256(evidence_file.read_bytes())
    cloud.put("build-evidence/" + evidence_sha + ".json", evidence_file)
    cloud.put(
        "bundles/" + prepared["bootstrap"]["sha256"] + ".tar.gz",
        directory / "bootstrap.tar.gz",
    )
    manifest = {
        "format": 1,
        "account_id": config["account_id"],
        "region": config["region"],
        "platform": "linux/amd64",
        "source_revision": revision,
        "platform_lock_sha256": prepared["platform_lock_sha256"],
        "platform_source_sha256": prepared["bootstrap"]["platform_source_sha256"],
        "bootstrap_sha256": prepared["bootstrap"]["sha256"],
        "migration_set_sha256": prepared["migration_set_sha256"],
        "build_evidence_sha256": evidence_sha,
        "images": images,
    }
    path = directory / "release.json"
    path.write_bytes(canonical(manifest))
    release_sha = sha256(path.read_bytes())
    cloud.put("releases/" + release_sha + ".json", path)
    path = directory / "source-index.json"
    path.write_bytes(
        canonical({"source_revision": revision, "release_sha256": release_sha})
    )
    cloud.put("releases/by-source/" + revision + ".json", path)
    # No GitHub output/artifact contains the private manifest or its identifier.
