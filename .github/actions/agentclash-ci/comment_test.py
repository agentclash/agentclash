import unittest

import comment


class FakeGitHubClient:
    def __init__(self, comments=None):
        self.comments = comments or []
        self.created = []
        self.updated = []

    def list_comments(self, pr_number):
        self.pr_number = pr_number
        return self.comments

    def create_comment(self, pr_number, body):
        self.created.append((pr_number, body))
        return {"id": 99, "html_url": "https://github.test/comment/99"}

    def update_comment(self, comment_id, body):
        self.updated.append((comment_id, body))
        return {"id": comment_id, "html_url": f"https://github.test/comment/{comment_id}"}


def failing_result():
    return {
        "workspace_id": "workspace-1",
        "gate_verdict": "fail",
        "failure_reason": "threshold_fail_correctness",
        "candidate": {
            "run_id": "run-candidate",
            "run_agent_id": "agent-candidate",
            "ci_metadata": {
                "pull_request_number": 42,
                "workflow_run_url": "https://github.com/acme/agent/actions/runs/123",
            },
        },
        "baseline": {
            "run_id": "run-baseline",
            "run_agent_id": "agent-baseline",
        },
        "release_gate": {
            "evaluation_details": {
                "dimension_results": {
                    "correctness": {
                        "outcome": "fail",
                        "observed_delta": -1,
                        "fail_threshold": 0.05,
                    },
                    "latency": {
                        "outcome": "pass",
                        "observed_delta": -0.12,
                        "fail_threshold": 0.15,
                    },
                },
            },
        },
        "regression_promotions": {
            "policy": "proposed",
            "case_status": "proposed",
            "created": [
                {
                    "suite_id": "suite-1",
                    "challenge_key": "refund-abuse-triage",
                    "case_id": "case-1",
                },
            ],
        },
    }


class CommentFormattingTests(unittest.TestCase):
    def test_build_failed_comment_contains_reviewer_triage(self):
        body = comment.build_comment(
            manifest=".agentclash/ci.yaml",
            result=failing_result(),
            should_run={"should_run": True},
            exit_code=1,
        )

        self.assertIn("<!-- agentclash-ci-comment:v1:", body)
        self.assertIn("## AgentClash CI: Failed", body)
        self.assertIn("threshold_fail_correctness", body)
        self.assertIn("run-candidate", body)
        self.assertIn("run-baseline", body)
        self.assertIn("Inspect in AgentClash", body)
        self.assertIn("https://app.agentclash.dev/workspaces/workspace-1/runs/run-candidate", body)
        self.assertIn("https://app.agentclash.dev/workspaces/workspace-1/runs/run-baseline", body)
        self.assertIn("https://app.agentclash.dev/workspaces/workspace-1/compare?baseline=run-baseline&candidate=run-candidate", body)
        self.assertIn("https://app.agentclash.dev/workspaces/workspace-1/runs/run-candidate/failures", body)
        self.assertIn("https://app.agentclash.dev/workspaces/workspace-1/runs/run-candidate/agents/agent-candidate/scorecard", body)
        self.assertIn("https://app.agentclash.dev/workspaces/workspace-1/runs/run-candidate/agents/agent-candidate/replay", body)
        self.assertIn("Score Deltas", body)
        self.assertIn("`correctness` | `fail` | `-1` | `0.05`", body)
        self.assertIn("Regression Tracking", body)
        self.assertIn("refund-abuse-triage", body)
        self.assertIn("https://app.agentclash.dev/workspaces/workspace-1/regression-suites/suite-1/cases/case-1", body)
        self.assertIn("Next Actions", body)

    def test_build_failed_comment_uses_custom_app_url(self):
        body = comment.build_comment(
            manifest=".agentclash/ci.yaml",
            result=failing_result(),
            should_run={"should_run": True},
            exit_code=1,
            app_url="https://agentclash.example.com/app/",
        )

        self.assertIn("https://agentclash.example.com/app/workspaces/workspace-1/runs/run-candidate", body)

    def test_build_failed_comment_prefers_app_links_over_safe_api_run_urls(self):
        result = failing_result()
        result["candidate"]["run_url"] = "https://api.example.com/runs/candidate"
        result["baseline"]["run_url"] = "https://api.example.com/runs/baseline"

        body = comment.build_comment(
            manifest=".agentclash/ci.yaml",
            result=result,
            should_run={"should_run": True},
            exit_code=1,
        )

        self.assertIn(
            "| Candidate run | [`run-candidate`](https://app.agentclash.dev/workspaces/workspace-1/runs/run-candidate) |",
            body,
        )
        self.assertIn(
            "| Baseline run | [`run-baseline`](https://app.agentclash.dev/workspaces/workspace-1/runs/run-baseline) |",
            body,
        )
        self.assertNotIn("https://api.example.com/runs/candidate", body)
        self.assertNotIn("https://api.example.com/runs/baseline", body)

    def test_build_failed_comment_falls_back_to_safe_api_run_urls(self):
        result = failing_result()
        result["workspace_id"] = ""
        result["candidate"]["run_url"] = "https://api.example.com/runs/candidate"
        result["baseline"]["run_url"] = "https://api.example.com/runs/baseline"

        body = comment.build_comment(
            manifest=".agentclash/ci.yaml",
            result=result,
            should_run={"should_run": True},
            exit_code=1,
        )

        self.assertIn("| Candidate run | [`run-candidate`](https://api.example.com/runs/candidate) |", body)
        self.assertIn("| Baseline run | [`run-baseline`](https://api.example.com/runs/baseline) |", body)

    def test_build_failed_comment_rejects_unsafe_urls(self):
        result = failing_result()
        result["workspace_id"] = ""
        result["candidate"]["run_url"] = "javascript:alert(1)"
        result["candidate"]["ci_metadata"]["workflow_run_url"] = "javascript:alert(2)"

        body = comment.build_comment(
            manifest=".agentclash/ci.yaml",
            result=result,
            should_run={"should_run": True},
            exit_code=1,
        )

        self.assertIn("| Candidate run | `run-candidate` |", body)
        self.assertNotIn("javascript:", body)
        self.assertNotIn("Workflow run", body)

    def test_build_skipped_comment_explains_skip_reason(self):
        body = comment.build_comment(
            manifest=".agentclash/ci.yaml",
            result={},
            should_run={"should_run": False, "reason": "changed files did not match trigger.paths"},
            exit_code=0,
        )

        self.assertIn("## AgentClash CI: Skipped", body)
        self.assertIn("changed files did not match trigger.paths", body)
        self.assertNotIn("Score Deltas", body)


class CommentContextTests(unittest.TestCase):
    def test_find_pr_number_prefers_result_metadata(self):
        self.assertEqual(
            comment.find_pr_number(
                {"candidate": {"ci_metadata": {"pull_request_number": 42}}},
                {"GITHUB_REF": "refs/pull/77/merge"},
                {"pull_request": {"number": 88}},
            ),
            42,
        )

    def test_find_pr_number_uses_event_then_ref_fallback(self):
        self.assertEqual(comment.find_pr_number({}, {}, {"pull_request": {"number": 88}}), 88)
        self.assertEqual(comment.find_pr_number({}, {"GITHUB_REF": "refs/pull/77/merge"}, {}), 77)


class CommentUpsertTests(unittest.TestCase):
    def test_upsert_updates_existing_marked_comment(self):
        marker = comment.marker_for_manifest(".agentclash/ci.yaml")
        fake = FakeGitHubClient(comments=[{"id": 12, "body": f"{marker}\nold body"}])

        outcome = comment.upsert_comment(fake, 42, marker, f"{marker}\nnew body")

        self.assertEqual(outcome.action, "updated")
        self.assertEqual(fake.updated, [(12, f"{marker}\nnew body")])
        self.assertEqual(fake.created, [])

    def test_upsert_creates_comment_when_marker_is_missing(self):
        marker = comment.marker_for_manifest(".agentclash/ci.yaml")
        fake = FakeGitHubClient(comments=[{"id": 12, "body": "some other comment"}])

        outcome = comment.upsert_comment(fake, 42, marker, f"{marker}\nnew body")

        self.assertEqual(outcome.action, "created")
        self.assertEqual(fake.created, [(42, f"{marker}\nnew body")])
        self.assertEqual(fake.updated, [])

    def test_post_comment_gracefully_skips_missing_token_or_context(self):
        base = {
            "manifest": ".agentclash/ci.yaml",
            "result": failing_result(),
            "should_run": {"should_run": True},
            "exit_code": 1,
            "repo": "acme/agent",
            "api_url": "https://api.github.test",
            "env": {},
            "event_payload": {},
        }

        missing_token = comment.post_comment(token="", **base)
        missing_pr = comment.post_comment(token="token", result={}, **{k: v for k, v in base.items() if k != "result"})

        self.assertEqual(missing_token.action, "skipped")
        self.assertIn("token", missing_token.message)
        self.assertEqual(missing_pr.action, "skipped")
        self.assertIn("pull request", missing_pr.message)


if __name__ == "__main__":
    unittest.main()
