"""Local maintained-image graph. Downloads are public, pinned and verified."""

import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import tarfile
import urllib.request

from common import require
from release_contract import canonical, recipe_hash, DERIVED, UPSTREAM, PLATFORM

LABEL = "io.agentclash.platform-build-sha256"


def local_base(run, identity):
    # BuildKit parses a bare sha256 ID as a Docker Hub tag in FROM. Give the
    # already inspected bytes a local, content-named tag and verify the binding.
    require(
        re.fullmatch(r"sha256:[a-f0-9]{64}", identity), "Exact local base ID required"
    )
    reference = "agentclash-reviewed-base:" + identity.removeprefix("sha256:")
    run(["docker", "tag", identity, reference])
    require(inspect(run, reference)["Id"] == identity, "Local base binding mismatch")
    return reference


def fetch(entry, destination):
    require(
        entry["url"].startswith("https://")
        and re.fullmatch(r"[a-f0-9]{64}", entry["sha256"]),
        "Platform input must have an HTTPS URL and immutable SHA256",
    )
    with urllib.request.urlopen(entry["url"], timeout=60) as response:
        require(response.url.startswith("https://"), "Insecure input redirect")
        content = response.read()
    require(
        hashlib.sha256(content).hexdigest() == entry["sha256"], "Input hash mismatch"
    )
    destination.write_bytes(content)
    os.utime(destination, (0, 0))


def inspect(run, ref, expected_hash=None):
    value = json.loads(run(["docker", "image", "inspect", ref]))[0]
    require(
        value["Os"] == "linux" and value["Architecture"] == "amd64",
        "Platform image architecture mismatch",
    )
    require(re.fullmatch(r"sha256:[a-f0-9]{64}", value["Id"]), "Invalid image ID")
    if expected_hash:
        require(
            value["Config"].get("Labels", {}).get(LABEL) == expected_hash,
            "Platform image recipe mismatch",
        )
    return value


def build(root, directory, run, gate):
    """Only scanned patched bases are returned to application builds.

    gate(name, input_ref, output_id) must retain original findings and verify
    their remediation, as well as enforce the strict output-image gate.
    """
    root, directory = Path(root), Path(directory)
    recipe = root / "deploy/aws/platform"
    lock = json.loads((recipe / "inputs.lock.json").read_text())
    public = json.loads((root / "deploy/aws/images.lock.json").read_text())["images"]
    require(set(public) == set(PLATFORM), "Incomplete platform input inventory")
    require(
        all(public[name]["usage"] == "remediation-source-only" for name in DERIVED)
        and all(public[name]["usage"] == "immutable-runtime" for name in UPSTREAM),
        "Platform source/runtime classification mismatch",
    )
    build_hash = recipe_hash(root)
    output, identity = {}, {}
    cache = directory / "platform-inputs"
    cache.mkdir(mode=0o700)
    for name, entry in lock["packages"].items():
        path = cache / name.replace("/", "-")
        fetch(entry, path)
    gosu = cache / "gosu.tar.gz"
    fetch(lock["gosu"], gosu)

    def docker_build(context, filename, name, arguments):
        tag = "agentclash-platform-build:" + name
        archive = directory / (name + ".image.tar")
        command = [
            "docker",
            "build",
            "--platform",
            "linux/amd64",
            "--output",
            "type=docker,rewrite-timestamp=true,dest=" + str(archive),
            "--provenance=false",
            "--build-arg",
            "SOURCE_DATE_EPOCH=" + str(lock["source_date_epoch"]),
            "--label",
            LABEL + "=" + build_hash,
            "-f",
            str(recipe / filename),
            "-t",
            tag,
        ]
        for key, value in arguments.items():
            if key in ("BUILDER", "UPSTREAM") and value.startswith("sha256:"):
                value = local_base(run, value)
            command += ["--build-arg", key + "=" + value]
        run(command + [str(context)], timeout=2400)
        run(["docker", "load", "--input", str(archive)], timeout=600)
        archive.unlink()
        return inspect(run, tag, build_hash)

    packages = {
        "alpine": (
            "v3.24",
            (
                "libcrypto3",
                "libssl3",
                "ca-certificates-bundle",
                "ca-certificates",
                "tzdata",
            ),
        ),
        "go": ("v3.24", ("libcrypto3", "libssl3")),
        "bun": ("v3.22", ("libcrypto3", "libssl3")),
        "caddy": ("v3.23", ("libcrypto3", "libssl3", "curl", "libcurl", "c-ares")),
        "postgres": ("v3.24", ("libcrypto3", "libssl3", "libuuid")),
    }
    for name in DERIVED:
        context = directory / ("platform-context-" + name)
        (context / "packages").mkdir(mode=0o700, parents=True)
        branch, names = packages[name]
        for package in names:
            entry = lock["packages"][branch + "/" + package]
            destination = (
                context / "packages" / (package + "-" + entry["version"] + ".apk")
            )
            shutil.copyfile(cache / (branch + "-" + package), destination)
            os.utime(destination, (0, 0))
        detail = docker_build(
            context,
            "Dockerfile.packages",
            name + "-packages",
            {"UPSTREAM": public[name]["image"]},
        )
        if name in ("caddy", "postgres"):
            if name == "caddy":
                shutil.copytree(recipe / "caddy", context / "source")
            else:
                (context / "source").mkdir(mode=0o700)
                with tarfile.open(gosu) as archive:
                    members = archive.getmembers()
                    require(
                        all(
                            not m.name.startswith("/")
                            and ".." not in m.name.split("/")
                            and Path(m.name).parts[0]
                            == "gosu-" + lock["gosu"]["version"]
                            for m in members
                        ),
                        "Unsafe upstream source archive",
                    )
                    seen = set()
                    for member in members:
                        # gosu is a single-package module; preserve its source and notices.
                        # Other entries (including upstream's .dockerignore symlink)
                        # are never extracted or traversed.
                        parts = Path(member.name).parts
                        if len(parts) == 2 and (
                            parts[1].endswith(".go")
                            or parts[1] in ("go.mod", "go.sum", "LICENSE", "NOTICE")
                        ):
                            require(
                                member.isfile() and parts[1] not in seen,
                                "Invalid or duplicate source member",
                            )
                            seen.add(parts[1])
                            (context / "source" / parts[1]).write_bytes(
                                archive.extractfile(member).read()
                            )
            shutil.copyfile(recipe / "licenses.sh", context / "licenses.sh")
            for path in context.rglob("*"):
                if path.is_file():
                    os.utime(path, (0, 0))
            binary = "caddy" if name == "caddy" else "gosu"
            detail = docker_build(
                context,
                "Dockerfile.binary",
                name,
                {
                    "BUILDER": output["go"],
                    "UPSTREAM": detail["Id"],
                    "BINARY": binary,
                    "BINARY_PATH": "/usr/bin/caddy"
                    if name == "caddy"
                    else "/usr/local/bin/gosu",
                },
            )
        gate(name, public[name]["image"], detail["Id"])
        output[name], identity[name] = detail["Id"], detail
    for name in UPSTREAM:
        # These unchanged, immutable upstream runtimes already satisfy the same gate.
        output[name] = public[name]["image"]
        gate(name, output[name], output[name])
    (directory / "platform-identities.json").write_bytes(canonical(identity))
    (directory / "platform-inputs.json").write_bytes(
        canonical(
            {
                "recipe_sha256": build_hash,
                "inputs": lock,
                "upstream_images": public,
                "outputs": output,
            }
        )
    )
    return output, build_hash


def selection(path):
    """Private local rehearsal inventory: exact IDs, never mutable Docker tags."""
    value = json.loads(Path(path).read_text())
    require(
        set(value["platform_images"]) == set(PLATFORM), "Incomplete rehearsal images"
    )
    images = {**value["platform_images"], **value["images"]}
    require(
        set(value["images"]) == {"api", "worker", "terminal", "app-schema"},
        "Incomplete application rehearsal images",
    )
    for ref in images.values():
        require(
            re.fullmatch(r"(?:[a-z0-9./_-]+@)?sha256:[a-f0-9]{64}", ref),
            "Rehearsals require exact image identities",
        )
    return images
