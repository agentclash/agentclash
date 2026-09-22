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
from image_scan import Scanner
import maintained

ROOT = Path(__file__).resolve().parents[1]
LOCAL_IMAGES = {
    name: "agentclash-step8-test:" + ("migrator" if name == "app-schema" else name)
    for name in APPLICATIONS
}


def build_images(run, bases, revision=None):
    require(set(bases) == {"go", "bun", "alpine"}, "Scanned build bases required")
    bases = {name: maintained.local_base(run, ref) for name, ref in bases.items()}
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
            command += [
                "--build-arg",
                "TARGET=" + target,
                "--build-arg",
                "GO_IMAGE=" + bases["go"],
                "--build-arg",
                "RUNTIME_IMAGE=" + bases["alpine"],
            ]
        else:
            command += ["--build-arg", "BUN_IMAGE=" + bases["bun"]]
        if revision:
            command += ["--label", "org.opencontainers.image.revision=" + revision]
        run(command + ["."], timeout=2400)


def build_graph(source, directory, run, revision=None):
    """Build and scan bases first, then applications; no cloud credentials."""
    source, directory = Path(source).resolve(), Path(directory).resolve()
    require(
        not directory.is_relative_to(source), "Build evidence must remain outside Git"
    )
    scanner = Scanner(directory, run)
    platform_images, recipe_sha = maintained.build(
        source, directory, run, scanner.platform
    )
    build_images(
        run, {name: platform_images[name] for name in ("go", "bun", "alpine")}, revision
    )
    image_ids = {}
    for name, reference in LOCAL_IMAGES.items():
        detail = maintained.inspect(run, reference)
        if revision:
            require(
                detail["Config"]
                .get("Labels", {})
                .get("org.opencontainers.image.revision")
                == revision,
                "Built application source mismatch",
            )
        image_ids[name] = detail["Id"]
        scanner.image(name, detail["Id"], name)
    selection = directory / "image-selection.json"
    selection.write_bytes(
        canonical(
            {
                "images": image_ids,
                "platform_images": platform_images,
                "platform_recipe_sha256": recipe_sha,
            }
        )
    )
    return {
        "image_ids": image_ids,
        "platform_images": platform_images,
        "platform_recipe_sha256": recipe_sha,
        "reports": scanner.reports
        + [
            directory / "platform-inputs.json",
            directory / "platform-identities.json",
            selection,
        ],
    }


def scan_bootstrap(directory, archive, run):
    """Scan installed bootstrap bytes, including Go dependencies in Compose."""
    unpacked = directory / "bootstrap-scan"
    unpacked.mkdir(mode=0o700)
    with tarfile.open(archive) as packaged:
        require(
            all(
                member.isfile()
                and not member.name.startswith("/")
                and ".." not in member.name.split("/")
                for member in packaged.getmembers()
            ),
            "Invalid bootstrap member",
        )
        packaged.extractall(unpacked, filter="data")
    # install-host.sh installs these exact bytes as executable. Trivy's Go
    # binary analyzer skips non-executable files; filesystem mode also skips it.
    (unpacked / "tools/docker-compose").chmod(0o700)
    reports = []
    for scanner in ("secret", "vuln"):
        report = directory / ("bootstrap-" + scanner + ".json")
        command = [
            str(directory / "trivy"),
            "rootfs",
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
        ]
        if scanner == "vuln":
            command += ["--severity", "HIGH,CRITICAL"]
        run(command + [str(unpacked)], timeout=1200)
        if scanner == "vuln":
            require(
                any(
                    result.get("Target") == "tools/docker-compose"
                    and result.get("Type") == "gobinary"
                    for result in json.loads(report.read_text()).get("Results", [])
                ),
                "Compose binary was not inspected by the vulnerability scanner",
            )
        reports.append(report)
    return reports


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
        if k
        in (
            "PATH",
            "HOME",
            "USER",
            "TMPDIR",
            "DOCKER_HOST",
            "DOCKER_CONTEXT",
            "DOCKER_CONFIG",
            "DOCKER_TLS_VERIFY",
            "DOCKER_CERT_PATH",
        )
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
            "--config",
            str(source / ".gitleaks.toml"),
            "--gitleaks-ignore-path",
            os.devnull,
            "--ignore-gitleaks-allow",
            "--redact",
            "--no-banner",
            "--report-format",
            "json",
            "--report-path",
            str(directory / "source-secrets.json"),
        ]
    )
    graph = build_graph(source, directory, run, revision)
    archive = directory / "bootstrap.tar.gz"
    metadata = bundle(source, directory / "compose", archive)
    reports = [directory / "source-secrets.json", *graph["reports"]]
    reports.extend(scan_bootstrap(directory, archive, run))
    migrations = sorted(
        str(path.relative_to(source))
        for path in (source / "backend/db/migrations").glob("*.sql")
    )
    return {
        "source_revision": revision,
        "bootstrap": metadata,
        "migration_set_sha256": tree_hash(source, migrations),
        "image_ids": graph["image_ids"],
        "platform_images": graph["platform_images"],
        "platform_recipe_sha256": graph["platform_recipe_sha256"],
        "reports": reports,
        "platform_lock_sha256": sha256(
            (source / "deploy/aws/images.lock.json").read_bytes()
        ),
        "tools_lock_sha256": sha256((source / "delivery/tools.lock.json").read_bytes()),
    }


def publication_environment(directory):
    """Isolate registry auth without losing the daemon used for the build."""
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

    def context(*args, data=None):
        result = subprocess.run(
            ["docker", "context", *args],
            input=data,
            env=docker_env,
            capture_output=True,
            timeout=30,
        )
        require(result.returncode == 0, "Private Docker context transfer failed")
        return result.stdout

    selected = context("show").decode().strip()
    require(bool(selected), "Docker context selection is missing")
    # Export only the selected connection, including its TLS material when used.
    # Registry credentials and credential helpers are not part of this archive.
    archive = context("export", selected, "-") if selected != "default" else None
    auth = Path(directory) / "docker-auth"
    auth.mkdir(mode=0o700)
    docker_env["DOCKER_CONFIG"] = str(auth)
    if archive is not None:
        docker_env.pop("DOCKER_CONTEXT", None)
        context("import", "agentclash-publication", "-", data=archive)
        docker_env["DOCKER_CONTEXT"] = "agentclash-publication"
    return docker_env


def publish(cloud, directory, prepared):
    directory = Path(directory)
    config, revision = cloud.config, prepared["source_revision"]
    docker_env = publication_environment(directory)

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
    platform_images = {
        name: prepared["platform_images"][name] for name in maintained.UPSTREAM
    }
    artifacts = {
        **prepared["image_ids"],
        **{
            "platform-" + name: prepared["platform_images"][name]
            for name in maintained.DERIVED
        },
    }
    for name, local in artifacts.items():
        tag = config["repository"] + ":" + revision + "-" + name
        docker("tag", local, tag)

        def verified():
            detail = json.loads(docker("image", "inspect", tag))[0]
            require(
                detail["Os"] == "linux"
                and detail["Architecture"] == "amd64"
                and detail["Id"] == local
                and detail["Config"]
                .get("Labels", {})
                .get(
                    maintained.LABEL
                    if name.startswith("platform-")
                    else "org.opencontainers.image.revision"
                )
                == (
                    prepared["platform_recipe_sha256"]
                    if name.startswith("platform-")
                    else revision
                ),
                "Publication image does not match scanned source/architecture",
            )
            return detail

        verified()
        docker("push", tag)
        details = verified()
        refs = [
            value
            for value in details["RepoDigests"]
            if value.startswith(config["repository"] + "@")
        ]
        require(len(refs) == 1, "Ambiguous published image digest")
        if name.startswith("platform-"):
            platform_images[name.removeprefix("platform-")] = image_ref(refs[0])
        else:
            images[name] = image_ref(refs[0])
    evidence = {
        "format": 2,
        "source_revision": revision,
        "images": images,
        "platform_images": platform_images,
        "platform_recipe_sha256": prepared["platform_recipe_sha256"],
        "tools_lock_sha256": prepared["tools_lock_sha256"],
        "platform_lock_sha256": prepared["platform_lock_sha256"],
        "scanners_passed": True,
        "provenance": {
            "type": "private-build-record",
            "repository": config.get("github", {}).get("repository"),
            "revision": revision,
            "workflow_ref": os.environ.get("GITHUB_WORKFLOW_REF"),
            "run_id": os.environ.get("GITHUB_RUN_ID"),
            "local_image_ids": artifacts,
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
        "format": 2,
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
        "platform_images": platform_images,
        "platform_recipe_sha256": prepared["platform_recipe_sha256"],
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
