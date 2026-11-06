import json
from pathlib import Path
import sys
import unittest

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
from common import Refused
from release_contract import APPLICATIONS, DERIVED, validate_images


class ImagePinTests(unittest.TestCase):
    def test_release_accepts_reviewed_valkey_patch_and_refuses_previous_digest(self):
        lock = json.loads((ROOT / "images.lock.json").read_text())
        repository = "registry.example.com/app"
        ref = repository + "@sha256:" + "a" * 64
        value = {
            "images": {name: ref for name in APPLICATIONS},
            "platform_images": {
                **{name: ref for name in DERIVED},
                "valkey": "valkey/valkey@sha256:ac81eb0a7b37e054000af8fdd5e6c3eb17249caa71eba79c003a145e518415e5",
            },
        }
        try:
            validate_images(value, repository, lock)
        except Refused as error:
            self.fail(
                "The reviewed Valkey 8.1.11 digest must be accepted: " + str(error)
            )
        # The former 8.1.10 pin must be rejected by the runtime-image guard.
        value["platform_images"]["valkey"] = (
            "valkey/valkey@sha256:d2e18f3410b6f616de1417f570fa55261af2898b9c5b2cfb6781ce2373ea43d1"
        )
        with self.assertRaises(Refused):
            validate_images(value, repository, lock)


if __name__ == "__main__":
    unittest.main()
