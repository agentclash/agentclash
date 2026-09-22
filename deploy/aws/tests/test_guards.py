import hashlib
import io
import json
from pathlib import Path
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import yaml

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
from common import Refused, account_guard, verify_blob, image_ref
from secret_store import validate, materialize
from render import temporal_config, acl_config
from restore import safe_extract
from host import manifest_validate
from release_contract import (
    canonical,
    sha256,
    platform_hash,
    recipe_hash,
    DERIVED,
    UPSTREAM,
)


class GuardTests(unittest.TestCase):
    def test_production_tmpfs_mount_is_one_bounded_absolute_path(self):
        compose = yaml.safe_load((ROOT / "compose.yaml").read_text())
        for name, service in compose["services"].items():
            with self.subTest(service=name):
                mounts = service["tmpfs"]
                self.assertEqual(len(mounts), 1)
                target, options = mounts[0].split(":", 1)
                self.assertEqual(target, "/tmp")
                self.assertEqual(
                    set(options.split(",")),
                    {"rw", "noexec", "nosuid", "size=64m", "mode=1777"},
                )

    def test_wrong_account_before_any_write(self):
        # Test-only generated identifiers; never reuse a real account in fixtures.
        with patch(
            "common.aws", return_value=json.dumps({"Account": str(2) * 12}).encode()
        ) as call:
            with self.assertRaises(Refused):
                account_guard({"account_id": str(1) * 12, "operator": "test"})
            self.assertEqual(call.call_args.args[1:3], ("sts", "get-caller-identity"))

    def test_immutable_content_and_images(self):
        verify_blob(b"fixture", hashlib.sha256(b"fixture").hexdigest())
        for ref in [
            "image:latest",
            "image:1.0",
            "image@sha256:bad",
            "image;command@sha256:" + "a" * 64,
        ]:
            with self.assertRaises(Refused):
                image_ref(ref)
        with self.assertRaises(Refused):
            verify_blob(b"changed", "a" * 64)

    def test_manifest_targets_account_region_repository_and_platform(self):
        settings = {
            "account_id": str(1) * 12,
            "region": "test-region",
            "repository": "registry.example.com/app",
        }
        value = {
            "format": 2,
            "source_revision": "a" * 40,
            "migration_set_sha256": "b" * 64,
            "build_evidence_sha256": "c" * 64,
            "bootstrap_sha256": "d" * 64,
            "platform_source_sha256": platform_hash(ROOT.parents[1]),
            "platform_recipe_sha256": recipe_hash(ROOT.parents[1]),
            "account_id": settings["account_id"],
            "region": "test-region",
            "platform": "linux/amd64",
            "platform_lock_sha256": hashlib.sha256(
                (ROOT / "images.lock.json").read_bytes()
            ).hexdigest(),
            "images": {
                name: "registry.example.com/app@sha256:" + "a" * 64
                for name in ("api", "worker", "terminal", "app-schema")
            },
            "platform_images": {
                **{n: "registry.example.com/app@sha256:" + "a" * 64 for n in DERIVED},
                **{
                    n: json.loads((ROOT / "images.lock.json").read_text())["images"][n][
                        "image"
                    ]
                    for n in UPSTREAM
                },
            },
        }
        settings["platform_images_sha256"] = sha256(canonical(value["platform_images"]))
        manifest_validate(value, settings)
        for key, bad in [
            ("account_id", str(2) * 12),
            ("region", "wrong"),
            ("platform", "linux/arm64"),
            ("platform_lock_sha256", "f" * 64),
            ("platform_recipe_sha256", "f" * 64),
        ]:
            changed = dict(value)
            changed[key] = bad
            with self.assertRaises(Refused):
                manifest_validate(changed, settings)
        changed = dict(value)
        changed["images"] = dict(
            value["images"], api="foreign.example.com/app@sha256:" + "a" * 64
        )
        with self.assertRaises(Refused):
            manifest_validate(changed, settings)
        # Even a valid digest in the approved registry cannot silently change
        # Caddy during an ordinary application release.
        changed = {
            **value,
            "platform_images": {
                **value["platform_images"],
                "caddy": "registry.example.com/app@sha256:" + "b" * 64,
            },
        }
        with self.assertRaises(Refused):
            manifest_validate(changed, settings)
        approved = {
            **settings,
            "platform_images_sha256": sha256(canonical(changed["platform_images"])),
        }
        manifest_validate(changed, approved)

    def test_secret_structure_and_injection(self):
        bad = [
            ("../api", {"env": {}, "files": {}}),
            ("caddy", {"env": {}, "files": {"../key": "value"}}),
            ("caddy", {"env": {"PATH": "/bad"}, "files": {}}),
            ("caddy", {"env": {"LD_PRELOAD": "bad"}, "files": {}}),
            ("caddy", {"env": {"KEY": "x\0y"}, "files": {}}),
            ("caddy", {"env": {"KEY": 123}, "files": {}}),
        ]
        for service, payload in bad:
            with self.assertRaises(Refused):
                validate(service, payload)

    def test_secret_bytes_shell_quoting_and_permissions(self):
        value = "literal ' $(`do not execute`)\nwith newline"
        with tempfile.TemporaryDirectory() as d:
            result = materialize(
                d,
                "caddy",
                {
                    "env": {
                        "SECRET_VALUE": value,
                        "API_DOMAIN": "api.example.com",
                        "TERMINAL_DOMAIN": "terminal.example.com",
                        "ACME_EMAIL": "ops@example.com",
                    },
                    "files": {},
                },
                set_owner=False,
            )
            import subprocess

            process = subprocess.run(
                [
                    "/bin/sh",
                    "-c",
                    '. "$1/env.sh"; printf %s "$SECRET_VALUE"',
                    "fixture",
                    str(result),
                ],
                capture_output=True,
            )
            self.assertEqual(process.stdout.decode(), value)
            self.assertEqual((result / "env.sh").stat().st_mode & 0o777, 0o600)
            with self.assertRaises(FileExistsError):
                materialize(
                    d,
                    "caddy",
                    {
                        "env": {
                            "API_DOMAIN": "api.example.com",
                            "TERMINAL_DOMAIN": "terminal.example.com",
                            "ACME_EMAIL": "ops@example.com",
                        },
                        "files": {},
                    },
                    set_owner=False,
                )

    def test_insecure_application_settings_rejected(self):
        for service in ("api", "worker", "terminal", "app-schema"):
            with self.assertRaises(Refused):
                validate(service, {"env": {}, "files": {}})

    def test_render_verified_tls_and_bounded_pools(self):
        config = temporal_config(
            {
                "DB_HOST": "postgres",
                "DB_PASSWORD": "fixture",
                "VISIBILITY_PASSWORD": "fixture",
            }
        )
        self.assertEqual(config["persistence"]["numHistoryShards"], 128)
        for store in config["persistence"]["datastores"].values():
            self.assertTrue(store["sql"]["tls"]["enableHostVerification"])
            self.assertLessEqual(store["sql"]["maxConns"], 5)
        for tier in ("frontend", "internode"):
            self.assertTrue(
                config["global"]["tls"][tier]["server"]["requireClientAuth"]
            )
        self.assertEqual(
            config["global"]["membership"]["broadcastAddress"], "__CONTAINER_IP__"
        )

    def test_acl_does_not_contain_raw_passwords(self):
        passwords = {
            name + "_PASSWORD": hashlib.sha256(name.encode()).hexdigest()
            for name in ("API", "WORKER", "TERMINAL", "ADMIN")
        }
        acl = acl_config(passwords)
        for password in passwords.values():
            self.assertNotIn(password, acl)
        self.assertIn("user default off", acl)
        self.assertIn("-flushall -flushdb", acl)

    def test_restore_rejects_traversal_and_symlinks(self):
        with tempfile.TemporaryDirectory() as d:
            path = Path(d) / "backup.tar"
            for name, symlink in [
                ("../escaped", False),
                ("dump.rdb", True),
                ("/dump.rdb", False),
                ("unexpected", False),
            ]:
                with tarfile.open(path, "w") as archive:
                    entry = tarfile.TarInfo(name)
                    if symlink:
                        entry.type = tarfile.SYMTYPE
                        entry.linkname = "/etc/passwd"
                    archive.addfile(entry, io.BytesIO())
                with self.assertRaises(Refused):
                    safe_extract(path, Path(d) / "restore")

    def test_unclean_prior_container_keeps_replacement_blocked(self):
        import host

        with tempfile.TemporaryDirectory() as d:

            def compose(*args, **kwargs):
                return b"old-container" if args[0] == "ps" else b""

            state = [{"State": {"Running": False, "ExitCode": 1, "OOMKilled": False}}]
            with (
                patch.object(host, "STATE", Path(d)),
                patch.object(host, "fence"),
                patch.object(host, "compose", side_effect=compose) as command,
                patch.object(host, "run", return_value=json.dumps(state).encode()),
            ):
                with self.assertRaises(Refused):
                    host.stop_applications()
                self.assertTrue((Path(d) / "cleanup-unresolved").exists())
                self.assertIn("--all", command.call_args_list[0].args)

    def test_clean_stop_clears_only_cleanup_marker(self):
        import host

        with tempfile.TemporaryDirectory() as d:
            (Path(d) / "recovery-unverified").touch()
            state = [{"State": {"Running": False, "ExitCode": 0, "OOMKilled": False}}]
            with (
                patch.object(host, "STATE", Path(d)),
                patch.object(host, "fence"),
                patch.object(
                    host,
                    "compose",
                    side_effect=lambda *args, **kw: (
                        b"container" if args[0] == "ps" else b""
                    ),
                ),
                patch.object(host, "run", return_value=json.dumps(state).encode()),
            ):
                host.stop_applications()
                self.assertFalse((Path(d) / "cleanup-unresolved").exists())
                self.assertTrue((Path(d) / "recovery-unverified").exists())

    def test_platform_readiness_retries_then_requires_authenticated_cache(self):
        import host

        with (
            patch.object(
                host, "compose", side_effect=[Refused("warming"), b"SERVING", b"PONG"]
            ),
            patch.object(host.time, "sleep"),
        ):
            host.wait_for_platform()
        with (
            patch.object(host, "compose", side_effect=[b"SERVING", b"NOAUTH"]),
            patch.object(host.time, "sleep"),
        ):
            with self.assertRaises(Refused):
                host.wait_for_platform(timeout=0)

    def test_cloudformation_stateful_and_network_boundaries(self):
        import yaml

        class Loader(yaml.SafeLoader):
            pass

        Loader.add_multi_constructor(
            "!",
            lambda loader, tag, node: (
                {tag: loader.construct_scalar(node)}
                if isinstance(node, yaml.ScalarNode)
                else {tag: loader.construct_sequence(node)}
            ),
        )
        state = yaml.load(
            (ROOT / "cloudformation/state.yaml").read_text(), Loader=Loader
        )["Resources"]
        for name in ("Database", "CacheVolume", "PrivateArtifacts", "Images"):
            self.assertEqual(state[name]["DeletionPolicy"], "Retain")
            self.assertEqual(state[name]["UpdateReplacePolicy"], "Retain")
        self.assertTrue(state["Database"]["Properties"]["DeletionProtection"])
        self.assertFalse(state["Database"]["Properties"]["PubliclyAccessible"])
        network = (ROOT / "cloudformation/network.yaml").read_text()
        self.assertNotIn("AWS::EC2::NatGateway", network)
        self.assertNotIn("FromPort: 22", network)
        host = (ROOT / "cloudformation/host.yaml").read_text()
        self.assertIn("CPUCredits: standard", host)
        self.assertIn("HttpTokens: required", host)
        ssm = (ROOT / "cloudformation/command.yaml").read_text()
        self.assertIn("interpolationType: ENV_VAR", ssm)
        self.assertNotIn("AWS-RunShellScript", ssm)


if __name__ == "__main__":
    unittest.main()
