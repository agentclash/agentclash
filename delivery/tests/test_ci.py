import copy
from contextlib import redirect_stdout
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
from common import Refused
from changes import select, aggregate, GROUPS
from cloud import Cloud, validate_config, protection_snapshot
from release_contract import canonical, sha256


def config(kind="production"):
    account = str(1) * 12
    return {
        "enabled": True,
        "kind": kind,
        "account_id": account,
        "region": "ap-southeast-1",
        "artifact_bucket": "fixture-artifacts",
        "repository": account + ".dkr.ecr.ap-southeast-1.amazonaws.com/app",
        "instance_id": "i-" + "a" * 17,
        "documents": {
            op: {"name": "fixture-" + op, "version": "1", "sha256": "b" * 64}
            for op in ("deploy", "promote")
        },
    }


class CITests(unittest.TestCase):
    def test_image_checks_and_publication_use_the_same_export_engine(self):
        import yaml

        for workflow, job in (
            ("aws-checks.yml", "images"),
            ("aws-release.yml", "build"),
        ):
            value = yaml.safe_load((ROOT / ".github/workflows" / workflow).read_text())
            setup = next(
                step
                for step in value["jobs"][job]["steps"]
                if step.get("uses", "").startswith("docker/setup-docker-action@")
            )
            self.assertEqual(setup["with"]["version"], "v29.3.1")
            daemon = json.loads(setup["with"]["daemon-config"])
            self.assertTrue(daemon["features"]["containerd-snapshotter"])
            self.assertFalse(daemon["debug"])
            self.assertEqual(daemon["log-level"], "fatal")

    def test_check_diagnostics_never_include_command_arguments_or_private_paths(self):
        from checks import command_label, refusal_detail, failure_location

        self.assertEqual(
            command_label(["/private/fixture/trivy", "PRIVATE_CANARY"]), "trivy"
        )
        self.assertEqual(
            command_label(["/private/PRIVATE_CANARY/tool", "PRIVATE_CANARY"]),
            "validation-tool",
        )
        self.assertIsNone(failure_location(Refused("PRIVATE_CANARY")))
        self.assertIsNone(refusal_detail(Refused("PRIVATE_CANARY")))
        self.assertIsNone(refusal_detail(ValueError("Input hash mismatch")))
        self.assertEqual(
            refusal_detail(Refused("Input hash mismatch")), "Input hash mismatch"
        )

    def test_check_failure_location_excludes_private_exception_values(self):
        from checks import check, failure_location

        with tempfile.TemporaryDirectory() as directory:
            with patch(
                "checks.subprocess.run", side_effect=PermissionError("PRIVATE_CANARY")
            ):
                with redirect_stdout(io.StringIO()):
                    try:
                        check("backend", directory)
                    except PermissionError as error:
                        location = failure_location(error)
                    else:
                        self.fail("Expected the subprocess failure to propagate")
        self.assertRegex(location, r"^delivery/checks\.py:[0-9]+$")
        self.assertNotIn("PRIVATE_CANARY", location)

    def test_delivery_entrypoints_do_not_shadow_python_standard_library(self):
        # Run without site initialization, which can otherwise preload operator
        # and hide startup collisions present in Python 3.12 runner venvs.
        for script in ("checks.py", "operator_cli.py"):
            result = subprocess.run(
                [sys.executable, "-S", str(ROOT / "delivery" / script), "--help"],
                capture_output=True,
            )
            self.assertEqual(result.returncode, 0, result.stderr.decode())

    def test_terminal_workspace_lock_changes_rerun_terminal_checks(self):
        for path in ("try-cli/package.json", "try-cli/bun.lock"):
            self.assertTrue(select([path])["terminal"])

    def test_sso_approval_that_expires_during_build_blocks_publication(self):
        import ci

        value = config("build")
        value.update(profile="fixture-sso", operator="fixture")
        value["publication_approval"] = {
            "operator": "fixture",
            "source_revision": "a" * 40,
            "account_id": value["account_id"],
            "region": value["region"],
            "expires_at": 1600,
            "local_required_checks_passed": True,
        }
        with (
            patch.object(
                sys,
                "argv",
                [
                    "ci.py",
                    "build",
                    "--kind",
                    "build",
                    "--source",
                    "a" * 40,
                    "--config",
                    "/fixture/private/config.json",
                ],
            ),
            patch("ci.private_json", return_value=value),
            patch("ci.time.time", side_effect=[1000, 1000, 2000]),
            patch("build.prepare", return_value={}),
            patch("ci.Cloud") as cloud,
            patch("build.publish") as publish,
        ):
            with self.assertRaises(Refused):
                ci.main()
            cloud.assert_not_called()
            publish.assert_not_called()

    def test_staging_requires_an_unexpired_bounded_lifetime(self):
        value = config("staging")
        with patch("cloud.time.time", return_value=1000):
            for expiry in (0, 999, 1000 + 86401):
                with self.assertRaises(Refused):
                    validate_config({**value, "expires_at": expiry}, "staging")
            validate_config({**value, "expires_at": 1600}, "staging")

    def test_failed_build_gate_never_acquires_cloud_credentials(self):
        import ci

        value = config("build")
        with (
            patch.dict(
                os.environ,
                {"AWS_DELIVERY_CONFIG": json.dumps(value), "GITHUB_SHA": "a" * 40},
                clear=True,
            ),
            patch.object(
                sys, "argv", ["ci.py", "build", "--kind", "build", "--source", "a" * 40]
            ),
            patch("ci.github_guard"),
            patch("build.prepare", side_effect=Refused("scanner refused")),
            patch("ci.Cloud") as cloud,
        ):
            with self.assertRaises(Refused):
                ci.main()
            cloud.assert_not_called()

    def test_shared_runtime_rechecks_all_go_consumers(self):
        selected = select(["runtime/provider/throttle/redis.go"])
        self.assertTrue(
            all(selected[g] for g in ("backend", "runtime", "cli", "images"))
        )

    def test_secret_policy_and_toolchain_changes_are_checked(self):
        self.assertTrue(select([".gitleaks.toml"])["platform"])
        self.assertTrue(all(select([".tool-versions"]).values()))

    def test_core_demo_and_migrator_dependencies(self):
        for path in (
            "try-cli/packages/core/src/sessions.ts",
            "try-cli/demos/example.yaml",
            "services/try-cli/bun.lock",
        ):
            selected = select([path])
            self.assertTrue(selected["terminal"] and selected["images"])
        for path in (
            "backend/db/migrations/new.sql",
            "scripts/db/test-migrator.py",
            "deploy/aws/compose.yaml",
        ):
            self.assertTrue(select([path])["images"])
        self.assertTrue(all(select([".github/workflows/aws-checks.yml"]).values()))

    def test_cli_release_pr_selects_only_cli_checks(self):
        release = select([".github/.release-please-manifest.json", "cli/CHANGELOG.md"])
        self.assertEqual({g for g, required in release.items() if required}, {"cli"})
        mixed = select([".github/.release-please-manifest.json", "delivery/tools.py"])
        self.assertTrue(all(mixed.values()))

    def test_workflows_outside_the_aws_checks_select_only_platform(self):
        def required(paths, **kwargs):
            return {g for g, value in select(paths, **kwargs).items() if value}

        for path in (
            ".github/release-please-config.json",
            ".github/workflows/release-please.yml",
            ".github/workflows/frontend.yml",
            ".github/actions/agentclash-ci/run_test.py",
            ".github/dependabot.yml",
            "delivery/tests/test_ci.py",
            "delivery/README.md",
        ):
            self.assertEqual(required([path]), {"platform"}, path)
        self.assertEqual(required(["deploy/aws/README.md"]), {"platform"})
        for path in (
            ".github/workflows/aws-checks.yml",
            ".github/workflows/aws-release.yml",
            "delivery/changes.py",
            "delivery/build.py",
        ):
            self.assertTrue(all(select([path], pull_request=True).values()), path)

    def test_check_parts_split_the_group_commands(self):
        import yaml
        import checks

        def commands(group, part=None):
            calls = []

            def fake(argv, **kwargs):
                name = "python" if argv[0] == sys.executable else argv[0]
                calls.append(" ".join([name, *argv[1:3]]))
                return subprocess.CompletedProcess(argv, 0)

            with (
                tempfile.TemporaryDirectory() as directory,
                patch("checks.subprocess.run", side_effect=fake),
                redirect_stdout(io.StringIO()),
            ):
                checks.check(group, directory, part)
            return calls

        go_build, go_vet = "go build ./...", "go vet ./..."
        go_test, migrator = "go test -short", "python scripts/db/test-migrator.py"
        self.assertEqual(commands("backend", "compile"), [go_build, go_vet])
        self.assertEqual(commands("backend", "test"), [go_test])
        self.assertEqual(commands("backend", "migrator"), [migrator])
        self.assertEqual(commands("backend"), [go_build, go_vet, go_test, migrator])
        self.assertEqual(commands("runtime", "test"), [go_test])
        self.assertEqual(
            commands("frontend", "static"),
            ["npm ci", "npx tsc --noEmit", "npm run lint"],
        )
        self.assertEqual(commands("frontend", "build"), ["npm ci", "npm run build"])
        with self.assertRaises(Refused):
            commands("runtime", "migrator")
        with self.assertRaises(Refused):
            commands("cli", "test")
        workflow = yaml.safe_load(
            (ROOT / ".github/workflows/aws-checks.yml").read_text()
        )
        for group, parts in checks.PARTS.items():
            job = workflow["jobs"][group]
            self.assertEqual(tuple(job["strategy"]["matrix"]["part"]), parts)
            self.assertIn("--part ${{ matrix.part }}", job["steps"][-1]["run"])

    def test_image_rehearsals_run_together_with_separate_logs(self):
        import checks

        calls = []

        def fake(argv, **kwargs):
            calls.append((Path(argv[1]).name, Path(kwargs["stdout"].name).name))
            return subprocess.CompletedProcess(argv, 0)

        with (
            tempfile.TemporaryDirectory() as directory,
            patch("checks.install"),
            patch("build.build_graph", return_value={"platform_images": {"go": "x"}}),
            patch("build.prepare_bootstrap"),
            patch("checks.subprocess.run", side_effect=fake),
            redirect_stdout(io.StringIO()) as output,
        ):
            checks.check("images", directory)
        self.assertEqual(
            sorted(calls),
            [
                ("rehearse-compose.py", "rehearse-compose.log"),
                ("rehearse-edge.py", "rehearse-edge.log"),
                ("rehearse-terminal.py", "rehearse-terminal.log"),
                ("rehearse.py", "rehearse.log"),
            ],
        )
        self.assertIn("Required check subprocess took: rehearse ", output.getvalue())

    def test_pr_code_changes_skip_images_but_image_inputs_do_not(self):
        def required(paths, **kwargs):
            return {g for g, value in select(paths, **kwargs).items() if value}

        self.assertEqual(
            required(["backend/internal/api/routes.go"], pull_request=True),
            {"backend"},
        )
        self.assertEqual(
            required(["runtime/provider/throttle/redis.go"], pull_request=True),
            {"backend", "runtime", "cli"},
        )
        self.assertEqual(
            required(["services/try-cli/server/main.ts"], pull_request=True),
            {"terminal"},
        )
        for path in (
            "backend/go.mod",
            "backend/go.sum",
            "backend/db/migrations/00099_new.sql",
            "backend/internal/engine/assets/mock_agent_vault.py",
            "runtime/go.sum",
            "runtime/challengepack/catalog/new.yaml",
            "scripts/db/test-migrator.py",
            "services/try-cli/bun.lock",
            "services/try-cli/package.json",
            "try-cli/packages/core/package.json",
            "try-cli/demos/example.yaml",
            "deploy/aws/Dockerfile.backend",
        ):
            self.assertTrue(select([path], pull_request=True)["images"], path)
        # A push to main keeps the images group for every application change.
        for path in (
            "backend/internal/api/routes.go",
            "runtime/provider/throttle/redis.go",
            "services/try-cli/server/main.ts",
        ):
            self.assertTrue(select([path])["images"], path)

    def test_aggregate_rejects_failure_missing_or_cancelled_jobs(self):
        plan = select(["runtime/types.go"])
        good = {
            "changes": {"result": "success"},
            **{
                g: {"result": "success" if required else "skipped"}
                for g, required in plan.items()
            },
        }
        self.assertTrue(aggregate(plan, good))
        for state in ("skipped", "failure", "cancelled"):
            bad = copy.deepcopy(good)
            bad["cli"]["result"] = state
            self.assertFalse(aggregate(plan, bad))
        bad = copy.deepcopy(good)
        bad.pop("backend")
        self.assertFalse(aggregate(plan, bad))
        docs = select(["docs/guide.md"])
        self.assertTrue(
            aggregate(
                docs,
                {
                    "changes": {"result": "success"},
                    **{g: {"result": "skipped"} for g in GROUPS},
                },
            )
        )

    def test_config_rejects_disabled_or_foreign_resources(self):
        value = config()
        validate_config(value, "production")
        for change in (
            {"enabled": False},
            {"kind": "staging"},
            {"repository": "foreign.example/app"},
            {"instance_id": "*"},
        ):
            with self.assertRaises(Refused):
                validate_config({**value, **change}, "production")
        value["documents"]["deploy"]["version"] = "$LATEST"
        with self.assertRaises(Refused):
            validate_config(value, "production")

    def test_environment_must_have_actual_reviewers_branch_rules_and_checks(self):
        environment = {
            "name": "aws-production",
            "protection_rules": [
                {
                    "type": "required_reviewers",
                    "reviewers": [{"type": "User", "reviewer": {"id": 1}}],
                }
            ],
            "deployment_branch_policy": {
                "protected_branches": False,
                "custom_branch_policies": True,
            },
        }
        branches = {
            "total_count": 1,
            "branch_policies": [{"name": "main", "type": "branch"}],
        }
        rules = [
            {"type": "pull_request"},
            {
                "type": "required_status_checks",
                "parameters": {
                    "required_status_checks": [{"context": "Release checks"}]
                },
            },
        ]
        self.assertEqual(len(protection_snapshot(environment, branches, rules)), 64)
        with self.assertRaises(Refused):
            protection_snapshot(
                {**environment, "protection_rules": []}, branches, rules
            )
        with self.assertRaises(Refused):
            protection_snapshot(
                environment,
                {
                    "total_count": 1,
                    "branch_policies": [{"name": "*", "type": "branch"}],
                },
                rules,
            )
        with self.assertRaises(Refused):
            protection_snapshot(environment, branches, [])

    def test_missing_github_context_fails_before_aws(self):
        value = config()
        value["github"] = {}
        with (
            patch.dict(os.environ, {}, clear=True),
            patch("cloud.subprocess.run") as run,
        ):
            with self.assertRaises(Refused):
                Cloud(value, "/unused")
            run.assert_not_called()

    def test_private_child_failure_does_not_escape_error(self):
        cloud = Cloud.__new__(Cloud)
        cloud.env = {}
        result = subprocess.CompletedProcess(
            [], 1, stdout=b"PRIVATE_CANARY", stderr=b"PRIVATE_CANARY"
        )
        with patch("cloud.subprocess.run", return_value=result):
            with self.assertRaises(Refused) as error:
                cloud.raw(["fixture"])
            self.assertNotIn("PRIVATE_CANARY", str(error.exception))

    def test_workflow_security_and_always_running_aggregate(self):
        import yaml

        checks = yaml.safe_load((ROOT / ".github/workflows/aws-checks.yml").read_text())
        self.assertNotIn("paths", checks["on"]["pull_request"])
        self.assertEqual(checks["jobs"]["required"]["if"], "${{ always() }}")
        self.assertEqual(set(checks["jobs"]["required"]["needs"]), {"changes", *GROUPS})
        for file in (ROOT / ".github/workflows").glob("aws-*.yml"):
            workflow = yaml.safe_load(file.read_text())
            self.assertNotIn("pull_request_target", workflow["on"])
            for name, job in workflow["jobs"].items():
                if job.get("permissions", {}).get("id-token") == "write":
                    if file.name == "aws-verify.yml":
                        # Bootstrap is manually dispatched before delivery is
                        # enabled; the environment and expiring private config
                        # gate it. Leave branch rejection to GitHub so it can be
                        # observed in the negative activation check.
                        self.assertEqual(set(workflow["on"]), {"workflow_dispatch"})
                        self.assertEqual(job["environment"], "aws-${{ inputs.target }}")
                        self.assertEqual(
                            workflow["on"]["workflow_dispatch"]["inputs"]["target"][
                                "options"
                            ],
                            ["build", "production"],
                        )
                        self.assertEqual(
                            job["steps"][-1]["env"]["AWS_ACTIVATION_CONFIG"],
                            "${{ secrets.AWS_ACTIVATION_CONFIG }}",
                        )
                        self.assertEqual(
                            job["steps"][-1]["run"], "python3 delivery/activation.py"
                        )
                    else:
                        self.assertIn("AWS_DELIVERY_ENABLED", job["if"])
                        self.assertEqual(
                            job["environment"],
                            "aws-" + ("build" if name == "build" else name),
                        )
                for step in job.get("steps", []):
                    if "uses" in step:
                        self.assertRegex(step["uses"], r"@[a-f0-9]{40}$")
                    self.assertNotIn("upload-artifact", step.get("uses", ""))

    def test_roles_have_no_secret_or_infrastructure_writes(self):
        import yaml

        template = yaml.safe_load(
            (ROOT / "deploy/aws/cloudformation/delivery.yaml").read_text()
        )
        for name in ("BuildRole", "StagingDeployRole", "ProductionDeployRole"):
            role = template["Resources"][name]["Properties"]
            condition = role["AssumeRolePolicyDocument"]["Statement"][0]["Condition"]
            self.assertEqual(set(condition), {"StringEquals"})
            self.assertEqual(
                condition["StringEquals"]["token.actions.githubusercontent.com:aud"],
                "sts.amazonaws.com",
            )
            self.assertIn(
                "Ref",
                condition["StringEquals"]["token.actions.githubusercontent.com:sub"],
            )
            statements = role["Policies"][0]["PolicyDocument"]["Statement"]
            for statement in statements:
                actions = (
                    statement["Action"]
                    if isinstance(statement["Action"], list)
                    else [statement["Action"]]
                )
                for action in actions:
                    self.assertFalse(
                        action.startswith(
                            ("secretsmanager:", "iam:", "cloudformation:")
                        )
                    )
                    self.assertNotIn("Delete", action)
                if statement["Resource"] == "*":
                    self.assertTrue(
                        set(actions)
                        <= {"ecr:GetAuthorizationToken", "ssm:GetCommandInvocation"}
                    )
            if name != "BuildRole":
                self.assertNotIn("s3:PutObject", json.dumps(statements))
                self.assertIn("ssm:resourceTag/Environment", json.dumps(statements))

    def test_private_manifest_content_hash_is_verified(self):
        value = config()
        cloud = Cloud.__new__(Cloud)
        cloud.config = value
        manifest = {
            "source_revision": "a" * 40,
            "account_id": value["account_id"],
            "region": value["region"],
        }
        sha = sha256(canonical(manifest))
        index = canonical({"source_revision": "a" * 40, "release_sha256": sha})
        with patch.object(cloud, "get", side_effect=[index, canonical(manifest)]):
            self.assertEqual(cloud.release("a" * 40)[0], sha)
        with patch.object(cloud, "get", side_effect=[index, b"{}"]):
            with self.assertRaises(Refused):
                cloud.release("a" * 40)


if __name__ == "__main__":
    unittest.main()
