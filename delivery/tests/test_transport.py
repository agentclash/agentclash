"""Exercise the real subprocess transport against an isolated fake AWS CLI."""

import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
from cloud import Cloud
from release_contract import canonical, sha256
from test_ci import config


class TransportTests(unittest.TestCase):
    def test_sso_dispatch_uses_exact_document_hash_version_target_and_manifest(self):
        with tempfile.TemporaryDirectory(prefix="transport-test-") as directory:
            directory = Path(directory)
            value = config()
            value.update(profile="fixture-sso", operator="fixture")
            manifest = {
                "source_revision": "a" * 40,
                "account_id": value["account_id"],
                "region": value["region"],
            }
            release_sha = sha256(canonical(manifest))
            (directory / "fixture.json").write_text(
                json.dumps(
                    {
                        "account": value["account_id"],
                        "manifest": manifest,
                        "index": {
                            "source_revision": "a" * 40,
                            "release_sha256": release_sha,
                        },
                    }
                )
            )
            binary = directory / "aws"
            binary.write_text(
                "#!"
                + sys.executable
                + "\n"
                + """import json, os, pathlib, sys
p = pathlib.Path(__file__).resolve().parent
assert 'GITHUB_TOKEN' not in os.environ
assert 'ACTIONS_ID_TOKEN_REQUEST_TOKEN' not in os.environ
d = json.loads((p/'fixture.json').read_text())
with (p/'trace.jsonl').open('a') as out: out.write(json.dumps(sys.argv[1:])+'\\n')
a = sys.argv[1:]
if 'get-caller-identity' in a: result = {'Account': d['account']}
elif 'cp' in a:
    result = d['index'] if any('/by-source/' in x for x in a) else d['manifest']
elif 'send-command' in a: result = {'Command': {'CommandId': 'fixture-command'}}
elif 'get-command-invocation' in a: result = {'Status': 'Success', 'ResponseCode': 0, 'StandardOutputContent': 'PRIVATE_FIXTURE_CANARY'}
else: sys.exit(42)
print(json.dumps(result,sort_keys=True,separators=(',',':')))
"""
            )
            binary.chmod(0o700)
            with (
                patch.dict(
                    os.environ,
                    {
                        "PATH": str(directory) + os.pathsep + os.environ["PATH"],
                        "GITHUB_TOKEN": "FIXTURE_CANARY",
                        "ACTIONS_ID_TOKEN_REQUEST_TOKEN": "FIXTURE_CANARY",
                    },
                ),
                patch("cloud.time.sleep"),
            ):
                cloud = Cloud(value, directory, mode="sso")
                sha, _ = cloud.release("a" * 40)
                cloud.send("deploy", sha)
            calls = [
                json.loads(line)
                for line in (directory / "trace.jsonl").read_text().splitlines()
            ]
            send = next(args for args in calls if "send-command" in args)
            for flag, expected in (
                ("--document-name", "fixture-deploy"),
                ("--document-version", "1"),
                ("--document-hash", "b" * 64),
                ("--instance-ids", value["instance_id"]),
                ("--profile", "fixture-sso"),
            ):
                self.assertEqual(send[send.index(flag) + 1], expected)
            self.assertEqual(
                json.loads(send[send.index("--parameters") + 1]),
                {"ReleaseSha256": [release_sha]},
            )
            self.assertNotIn("secretsmanager", json.dumps(calls))


if __name__ == "__main__":
    unittest.main()
