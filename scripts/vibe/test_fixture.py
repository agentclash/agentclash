import unittest
from unittest.mock import patch

import fixture


class OwnershipTests(unittest.TestCase):
    def test_remote_nonfixture_and_url_parameter_overrides_are_rejected(self):
        for dsn in ["postgres://user@db.example/vibe_test", "postgres://user@127.0.0.1/production", "postgres://user@127.0.0.1/vibe_test?host=remote", "postgres://user@127.0.0.1/vibe_test%2fother"]:
            with self.subTest(dsn=dsn), self.assertRaises(ValueError):
                fixture.parent_env(dsn)

    def test_changed_database_oid_cannot_authorize_deletion(self):
        with patch.object(fixture, "identity", return_value="99"), patch.object(fixture, "sql") as sql:
            with self.assertRaisesRegex(ValueError, "identity changed"):
                fixture.drop_owned({"PGDATABASE":"vibe_test"}, "vibe_test_exact", "42")
            sql.assert_not_called()

    def test_parent_and_missing_creation_proof_are_never_dropped(self):
        with patch.object(fixture, "sql") as sql:
            for name, oid in [("vibe_test", "42"), ("vibe_test_exact", ""), ("vibe_test_exact", "not-an-oid")]:
                with self.assertRaises(ValueError):
                    fixture.drop_owned({"PGDATABASE":"vibe_test"}, name, oid)
            sql.assert_not_called()

    def test_cleanup_failure_cannot_emit_success_receipt(self):
        with patch.object(fixture, "identity", return_value="42"), patch.object(fixture, "sql", side_effect=RuntimeError("drop failed")), patch.object(fixture, "record") as receipt:
            with self.assertRaisesRegex(RuntimeError, "drop failed"):
                fixture.drop_owned({"PGDATABASE":"vibe_test"}, "vibe_test_exact", "42")
            receipt.assert_not_called()

    def test_completed_concurrent_owned_drop_is_confirmed_by_absence(self):
        with patch.object(fixture, "identity", side_effect=["42", "", ""]), patch.object(fixture, "sql", side_effect=RuntimeError("drop raced")), patch.object(fixture, "record") as receipt:
            fixture.drop_owned({"PGDATABASE":"vibe_test"}, "vibe_test_exact", "42")
            receipt.assert_called_once_with("database", "vibe_test_exact", phase="absent", oid="42")

    def test_libpq_secrets_do_not_enter_command_arguments_or_diagnostics(self):
        with patch.object(fixture.subprocess, "run", return_value=fixture.subprocess.CompletedProcess([], 1, "", "password=secret")) as run:
            with self.assertRaisesRegex(RuntimeError, "^local fixture database command failed \\(exit 1\\)$"):
                fixture.sql({"PGPASSWORD":"secret"}, "SELECT 1")
            self.assertNotIn("secret", " ".join(run.call_args.args[0]))


if __name__ == "__main__":
    unittest.main()
