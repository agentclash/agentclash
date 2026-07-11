import os
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path


ACTION_DIR = Path(__file__).resolve().parent


class RunScriptCLIResolutionTests(unittest.TestCase):
    def test_uses_installed_cli_when_ci_commands_exist(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            calls = root / "calls.log"
            bin_dir = root / "bin"
            bin_dir.mkdir()
            self.write_executable(
                bin_dir / "agentclash",
                f"""#!/usr/bin/env bash
printf 'agentclash %s\\n' "$*" >>"{calls}"
if [[ "$*" == "ci should-run --help" || "$*" == "ci run --help" ]]; then
  printf 'Usage:\\n  agentclash %s [flags]\\n\\nFlags:\\n  --help\\n' "${{*:1:2}}"
  exit 0
fi
if [[ "$1 $2" == "ci validate" ]]; then
  [[ -f "$3" ]] || exit 12
  exit 0
fi
if [[ "$1 $2" == "ci should-run" ]]; then
  printf '%s\\n' '{{"should_run":false,"reason":"no matched files"}}'
  exit 0
fi
exit 9
""",
            )

            result = self.run_action(root, bin_dir)

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("Using installed agentclash CLI", result.stdout)
            call_log = calls.read_text()
            self.assertIn("agentclash ci validate", call_log)
            self.assertIn("agentclash ci should-run", call_log)

    def test_falls_back_to_source_cli_when_installed_cli_is_stale(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            calls = root / "calls.log"
            bin_dir = root / "bin"
            bin_dir.mkdir()
            self.write_executable(
                bin_dir / "agentclash",
                f"""#!/usr/bin/env bash
printf 'agentclash %s\\n' "$*" >>"{calls}"
exit 1
""",
            )
            self.write_fake_go(bin_dir / "go", calls)

            result = self.run_action(root, bin_dir)

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("Using AgentClash CLI source fallback", result.stdout)
            call_log = calls.read_text()
            self.assertIn("agentclash ci should-run --help", call_log)
            self.assertIn("go -C", call_log)
            self.assertIn("build -o", call_log)
            self.assertIn("ci validate", call_log)
            self.assertIn("ci should-run", call_log)

    def test_falls_back_when_installed_cli_is_absent(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            calls = root / "calls.log"
            bin_dir = root / "bin"
            bin_dir.mkdir()
            self.write_fake_go(bin_dir / "go", calls)

            result = self.run_action(root, bin_dir)

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("Using AgentClash CLI source fallback", result.stdout)
            self.assertIn("go -C", calls.read_text())

    def test_fails_when_fallback_is_disabled_and_installed_cli_is_stale(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            calls = root / "calls.log"
            bin_dir = root / "bin"
            bin_dir.mkdir()
            self.write_executable(
                bin_dir / "agentclash",
                f"""#!/usr/bin/env bash
printf 'agentclash %s\\n' "$*" >>"{calls}"
exit 1
""",
            )
            self.write_fake_go(bin_dir / "go", calls)

            result = self.run_action(root, bin_dir, source_fallback="false")

            self.assertNotEqual(result.returncode, 0)
            self.assertIn("AgentClash CI requires an agentclash CLI", result.stdout)
            self.assertNotIn("go -C", calls.read_text())

    def run_action(
        self,
        root: Path,
        bin_dir: Path,
        *,
        source_fallback: str = "true",
    ) -> subprocess.CompletedProcess[str]:
        manifest = root / ".agentclash" / "ci.yaml"
        manifest.parent.mkdir(parents=True)
        manifest.write_text(
            textwrap.dedent(
                """
                version: 1
                trigger:
                  paths:
                    - "agents/**"
                candidate:
                  build:
                    agent_build_id: build-1
                    spec_file: agents/agent.json
                  deployment:
                    runtime_profile_id: runtime-1
                evaluation:
                  challenge_pack_version_id: pack-1
                baseline:
                  run_id: run-1
                gate:
                  fail_on: regression
                regressions:
                  promote_failures: proposed
                """
            ).strip()
            + "\n",
        )
        env = {
            **os.environ,
            "PATH": f"{bin_dir}:{os.environ['PATH']}",
            "ACTION_PATH": str(ACTION_DIR),
            "INPUT_INSTALL_CLI": "false",
            "INPUT_REMOTE_VALIDATE": "false",
            "INPUT_SOURCE_FALLBACK": source_fallback,
            "INPUT_SKIP_IF_UNMATCHED": "true",
            "INPUT_PR_COMMENT": "false",
            "INPUT_MANIFEST": ".agentclash/ci.yaml",
            "INPUT_CHANGED_FILES": "docs/readme.md",
            "RUNNER_TEMP": str(root),
        }
        return subprocess.run(
            ["bash", str(ACTION_DIR / "run.sh")],
            cwd=root,
            env=env,
            text=True,
            capture_output=True,
            check=False,
        )

    @staticmethod
    def write_executable(path: Path, content: str):
        path.write_text(textwrap.dedent(content))
        path.chmod(0o755)

    def write_fake_go(self, path: Path, calls: Path):
        self.write_executable(
            path,
            f"""#!/usr/bin/env bash
printf 'go %s\\n' "$*" >>"{calls}"
if [[ "$1" == "-C" && "$3" == "build" && "$4" == "-o" && "$6" == "." ]]; then
  cat >"$5" <<'BIN'
#!/usr/bin/env bash
printf 'source-agentclash %s\n' "$*" >>"__CALLS__"
if [[ "$*" == "ci should-run --help" || "$*" == "ci run --help" ]]; then
  printf 'Usage:\n  agentclash %s [flags]\n\nFlags:\n  --help\n' "${{*:1:2}}"
  exit 0
fi
if [[ "$1 $2" == "ci validate" ]]; then
  [[ -f "$3" ]] || exit 12
  exit 0
fi
if [[ "$1 $2" == "ci should-run" ]]; then
  printf '%s\n' '{{"should_run":false,"reason":"no matched files"}}'
  exit 0
fi
exit 9
BIN
  sed -i.bak "s#__CALLS__#{calls}#g" "$5"
  rm -f "$5.bak"
  chmod +x "$5"
  exit 0
fi
exit 9
""",
        )


if __name__ == "__main__":
    unittest.main()
