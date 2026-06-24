#!/usr/bin/env python3
"""Post a sticky AgentClash CI pull request comment.

The action's shell wrapper owns the gate exit code. This helper is deliberately
best-effort: missing permissions, fork tokens, missing PR context, or GitHub API
errors should never turn a release-gate result into a different CI failure.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Any

MARKER_PREFIX = "agentclash-ci-comment:v1"
PROMPT_EVAL_MARKER = "<!-- agentclash:prompt-eval -->"
DIMENSION_ORDER = ("correctness", "reliability", "latency", "cost")
DEFAULT_APP_URL = "https://agentclash.dev"


@dataclass
class CommentOutcome:
    action: str
    message: str
    url: str = ""


class GitHubClient:
    def __init__(self, api_url: str, repo: str, token: str) -> None:
        self.api_url = api_url.rstrip("/") or "https://api.github.com"
        self.repo = repo.strip()
        self.token = token.strip()

    def list_comments(self, pr_number: int) -> list[dict[str, Any]]:
        return self._request("GET", f"/repos/{self._repo_path()}/issues/{pr_number}/comments?per_page=100")

    def create_comment(self, pr_number: int, body: str) -> dict[str, Any]:
        return self._request("POST", f"/repos/{self._repo_path()}/issues/{pr_number}/comments", {"body": body})

    def update_comment(self, comment_id: int, body: str) -> dict[str, Any]:
        return self._request("PATCH", f"/repos/{self._repo_path()}/issues/comments/{comment_id}", {"body": body})

    def _repo_path(self) -> str:
        owner, repo = self.repo.split("/", 1)
        return f"{urllib.parse.quote(owner)}/{urllib.parse.quote(repo)}"

    def _request(self, method: str, path: str, payload: dict[str, Any] | None = None) -> Any:
        data = None
        if payload is not None:
            data = json.dumps(payload).encode("utf-8")
        request = urllib.request.Request(
            self.api_url + path,
            data=data,
            method=method,
            headers={
                "Accept": "application/vnd.github+json",
                "Authorization": f"Bearer {self.token}",
                "Content-Type": "application/json",
                "User-Agent": "agentclash-ci-action",
                "X-GitHub-Api-Version": "2022-11-28",
            },
        )
        with urllib.request.urlopen(request, timeout=15) as response:
            raw = response.read().decode("utf-8")
        return json.loads(raw) if raw else {}


def load_json_file(path: str) -> dict[str, Any]:
    if not path:
        return {}
    target = Path(path)
    if not target.exists() or target.stat().st_size == 0:
        return {}
    with target.open("r", encoding="utf-8") as handle:
        value = json.load(handle)
    return value if isinstance(value, dict) else {}


def marker_for_manifest(manifest: str) -> str:
    digest = hashlib.sha256(manifest.encode("utf-8")).hexdigest()[:12]
    return f"<!-- {MARKER_PREFIX}:{digest} -->"


def marker_for_mode(mode: str, manifest: str) -> str:
    return PROMPT_EVAL_MARKER if mode == "prompt-eval" else marker_for_manifest(manifest)


def build_comment(
    *,
    manifest: str,
    result: dict[str, Any],
    should_run: dict[str, Any],
    exit_code: int,
    app_url: str = DEFAULT_APP_URL,
    mode: str = "ci",
) -> str:
    if mode == "prompt-eval":
        return build_prompt_eval_comment(config=manifest, result=result, should_run=should_run, exit_code=exit_code, app_url=app_url)
    marker = marker_for_manifest(manifest)
    if should_run and should_run.get("should_run") is False:
        reason = str(should_run.get("reason") or "manifest trigger did not match this change set")
        return "\n".join(
            [
                marker,
                "## AgentClash CI: Skipped",
                "",
                f"AgentClash did not run for `{manifest}`.",
                "",
                f"**Reason:** {escape_cell(reason)}",
                "",
                "_Updated automatically by AgentClash CI._",
            ],
        )

    status = status_label(result, exit_code)
    links = agentclash_links(result, app_url)
    candidate_run_url = first_safe_url(links.get("candidate_run"), nested(result, "candidate.run_url"))
    baseline_run_url = first_safe_url(links.get("baseline_run"), nested(result, "baseline.run_url"))
    lines = [
        marker,
        f"## AgentClash CI: {status}",
        "",
        "| Field | Value |",
        "| --- | --- |",
        f"| Manifest | `{escape_cell(manifest)}` |",
        f"| Verdict | `{escape_cell(str(result.get('gate_verdict') or 'n/a'))}` |",
        f"| Failure reason | `{escape_cell(str(result.get('failure_reason') or 'n/a'))}` |",
        f"| Exit code | `{exit_code}` |",
        f"| Candidate run | {format_link_or_code(nested(result, 'candidate.run_id'), candidate_run_url)} |",
        f"| Baseline run | {format_link_or_code(nested(result, 'baseline.run_id'), baseline_run_url)} |",
    ]

    workflow_url = nested(result, "candidate.ci_metadata.workflow_run_url")
    if is_safe_http_url(workflow_url):
        lines.append(f"| Workflow run | [open]({workflow_url}) |")

    inspect_lines = inspect_link_lines(links)
    if inspect_lines:
        lines.extend(["", "### Inspect in AgentClash", ""])
        lines.extend(inspect_lines)

    dimensions = dimension_rows(result)
    if dimensions:
        lines.extend(["", "### Score Deltas", "", "| Dimension | Outcome | Observed delta | Fail threshold |", "| --- | --- | ---: | ---: |"])
        lines.extend(dimensions)

    regression_summary = regression_lines(result, links)
    if regression_summary:
        lines.extend(["", "### Regression Tracking", ""])
        lines.extend(regression_summary)

    errors = result.get("errors")
    if isinstance(errors, list) and errors:
        lines.extend(["", "### Errors", ""])
        lines.extend(f"- `{escape_cell(str(error))}`" for error in errors[:5])

    lines.extend(["", "### Next Actions", ""])
    lines.extend(next_action_lines(result))
    lines.extend(["", "_Updated automatically by AgentClash CI._"])
    return "\n".join(lines)


def build_prompt_eval_comment(
    *,
    config: str,
    result: dict[str, Any],
    should_run: dict[str, Any],
    exit_code: int,
    app_url: str = DEFAULT_APP_URL,
) -> str:
    marker = PROMPT_EVAL_MARKER
    if should_run and should_run.get("should_run") is False:
        reason = str(should_run.get("reason") or "prompt eval paths did not match this change set")
        return "\n".join(
            [
                marker,
                "## AgentClash Prompt Eval: Skipped",
                "",
                f"AgentClash did not run for `{config}`.",
                "",
                f"**Reason:** {escape_cell(reason)}",
                "",
                "_Updated automatically by AgentClash CI._",
            ],
        )

    status = prompt_eval_status_label(result, exit_code)
    summary = result.get("summary") if isinstance(result.get("summary"), dict) else {}
    lines = [
        marker,
        f"## AgentClash Prompt Eval: {status}",
        "",
        "| Field | Value |",
        "| --- | --- |",
        f"| Config | `{escape_cell(config)}` |",
        f"| Verdict | `{escape_cell(str(result.get('gate_verdict') or 'n/a'))}` |",
        f"| Exit code | `{exit_code}` |",
        f"| Cases | `{format_number(summary.get('completed_cases'))}/{format_number(summary.get('total_cases'))}` |",
        f"| Assertions | `{format_number(summary.get('assertions_passed'))} passed / {format_number(summary.get('assertions_failed'))} failed` |",
        f"| Assertion pass rate | `{format_number(summary.get('assertion_pass_rate'))}` |",
        f"| Execution errors | `{format_number(summary.get('execution_errors'))}` |",
    ]

    links = prompt_eval_links(result, app_url)
    if links:
        lines.extend(["", "### Inspect in AgentClash", ""])
        lines.extend(f"- [{label}]({url})" for label, url in links)

    failed_rows = prompt_eval_failed_rows(result)
    if failed_rows:
        lines.extend(["", "### Failed Assertions", "", "| Case | Assertion | Expected | Actual / Error |", "| --- | --- | --- | --- |"])
        for row in failed_rows[:5]:
            lines.append(
                f"| `{escape_cell(str(row.get('case_key') or 'n/a'))}` | "
                f"`{escape_cell(str(row.get('assertion') or row.get('assertion_key') or 'n/a'))}` | "
                f"`{redact_snippet(row.get('expected'))}` | "
                f"`{redact_snippet(row.get('error') or row.get('actual'))}` |",
            )
        if len(failed_rows) > 5:
            lines.append(f"| ... | ... | ... | `{len(failed_rows) - 5} more failed assertion(s)` |")

    errors = result.get("errors")
    if isinstance(errors, list) and errors:
        lines.extend(["", "### Errors", ""])
        lines.extend(f"- `{redact_snippet(error)}`" for error in errors[:5])

    lines.extend(["", "### Reproduce Locally", ""])
    lines.append(f"`agentclash prompt-eval run {escape_cell(config)} --ci --follow --json`")
    lines.extend(["", "### Next Actions", ""])
    lines.extend(prompt_eval_next_actions(result, exit_code))
    lines.extend(["", "_Updated automatically by AgentClash CI._"])
    return "\n".join(lines)


def status_label(result: dict[str, Any], exit_code: int) -> str:
    if exit_code == 0:
        return "Passed"
    verdict = str(result.get("gate_verdict") or "").lower()
    if verdict == "fail":
        return "Failed"
    if verdict in {"warn", "warning"}:
        return "Warning"
    return "Errored"


def prompt_eval_status_label(result: dict[str, Any], exit_code: int) -> str:
    if exit_code == 0:
        return "Passed"
    if exit_code == 3 or str(result.get("gate_verdict") or "").lower() == "fail":
        return "Failed"
    return "Errored"


def prompt_eval_failed_rows(result: dict[str, Any]) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for envelope in result.get("results") or []:
        if not isinstance(envelope, dict):
            continue
        for row in envelope.get("rows") or []:
            if isinstance(row, dict) and str(row.get("result") or "").upper() in {"FAIL", "ERROR"}:
                rows.append(row)
    return rows


def prompt_eval_links(result: dict[str, Any], app_url: str) -> list[tuple[str, str]]:
    links: list[tuple[str, str]] = []
    seen: set[str] = set()

    def add(label: str, url: Any) -> None:
        safe = normalize_safe_url(url)
        if safe and safe not in seen:
            links.append((label, safe))
            seen.add(safe)

    for playground in result.get("playgrounds") or []:
        if not isinstance(playground, dict):
            continue
        add(f"Playground {playground.get('name') or playground.get('playground_id') or ''}".strip(), playground.get("playground_url"))
        for experiment in playground.get("experiments") or []:
            if isinstance(experiment, dict):
                add(f"Experiment {experiment.get('experiment_id') or ''}".strip(), experiment.get("experiment_url"))

    workspace_id = clean_id(result.get("workspace_id"))
    if workspace_id:
        base = normalize_safe_url(app_url)
        for playground in result.get("playgrounds") or []:
            if not isinstance(playground, dict):
                continue
            playground_id = clean_id(playground.get("playground_id"))
            if playground_id:
                add("Playground", app_link(base, "workspaces", workspace_id, "playgrounds", playground_id))
            for experiment in playground.get("experiments") or []:
                if isinstance(experiment, dict):
                    experiment_id = clean_id(experiment.get("experiment_id"))
                    if experiment_id:
                        add("Experiment", app_link(base, "workspaces", workspace_id, "playground-experiments", experiment_id))
    return links[:8]


def prompt_eval_next_actions(result: dict[str, Any], exit_code: int) -> list[str]:
    verdict = str(result.get("gate_verdict") or "").lower()
    if exit_code == 3 or verdict == "fail":
        return [
            "- Inspect the failed assertions and AgentClash experiment links above.",
            "- Fix the prompt, model, provider account, or test expectation and push again.",
            "- If the behavior change is intentional, update the prompt eval config deliberately in a separate review.",
        ]
    if exit_code == 0:
        return ["- No action needed. The prompt eval gate passed."]
    return [
        "- Open the GitHub Actions log and AgentClash JSON output.",
        "- Fix config, auth, provider, or runtime errors before trusting the gate result.",
    ]


def redact_snippet(value: Any, limit: int = 160) -> str:
    text = escape_cell(str(value or ""))
    text = re.sub(r"(?i)(api[_-]?key|token|secret|password)\s*[:=]\s*['\"]?[^\s,'\"]+", r"\1=[redacted]", text)
    text = re.sub(r"sk-[A-Za-z0-9_-]{12,}", "sk-[redacted]", text)
    text = re.sub(r"github_pat_[A-Za-z0-9_]{12,}", "github_pat_[redacted]", text)
    text = re.sub(r"gh[pousr]_[A-Za-z0-9_]{12,}", "gh[redacted]", text)
    text = re.sub(r"-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----", "[private-key-redacted]", text)
    text = re.sub(r"(?i)bearer\s+[A-Za-z0-9._-]{12,}", "Bearer [redacted]", text)
    if len(text) > limit:
        return text[: limit - 1] + "..."
    return text


def dimension_rows(result: dict[str, Any]) -> list[str]:
    raw = nested(result, "release_gate.evaluation_details.dimension_results")
    if not isinstance(raw, dict):
        return []
    keys = [key for key in DIMENSION_ORDER if key in raw]
    keys.extend(sorted(key for key in raw if key not in set(keys)))
    rows = []
    for key in keys:
        value = raw.get(key)
        if not isinstance(value, dict):
            continue
        rows.append(
            f"| `{escape_cell(key)}` | `{escape_cell(str(value.get('outcome') or 'n/a'))}` | "
            f"`{format_number(value.get('observed_delta'))}` | `{format_number(value.get('fail_threshold'))}` |",
        )
    return rows


def regression_lines(result: dict[str, Any], links: dict[str, str] | None = None) -> list[str]:
    promotions = result.get("regression_promotions")
    if not isinstance(promotions, dict):
        return []
    links = links or {}
    lines = []
    policy = promotions.get("policy")
    case_status = promotions.get("case_status")
    if policy or case_status:
        lines.append(f"- Policy: `{escape_cell(str(policy or 'n/a'))}`; case status: `{escape_cell(str(case_status or 'n/a'))}`")
    for key, label in (
        ("created", "Created"),
        ("existing", "Already tracked"),
        ("blocked", "Blocked"),
        ("skipped", "Skipped"),
    ):
        items = promotions.get(key)
        if isinstance(items, list) and items:
            lines.append(f"- {label}: {summarize_regression_items(items, links)}")
    errors = promotions.get("errors")
    if isinstance(errors, list) and errors:
        lines.append(f"- Promotion errors: {', '.join(f'`{escape_cell(str(error))}`' for error in errors[:3])}")
    return lines


def summarize_regression_items(items: list[Any], links: dict[str, str] | None = None) -> str:
    links = links or {}
    labels = []
    for item in items[:5]:
        if isinstance(item, dict):
            label = escape_cell(str(item.get("challenge_key") or item.get("case_id") or item.get("reason") or "unknown"))
            case_url = regression_case_url(item, links)
            if case_url:
                labels.append(f"[`{label}`]({case_url})")
            else:
                labels.append(f"`{label}`")
        else:
            labels.append(f"`{escape_cell(str(item))}`")
    if len(items) > 5:
        labels.append(f"and {len(items) - 5} more")
    return ", ".join(labels)


def next_action_lines(result: dict[str, Any]) -> list[str]:
    verdict = str(result.get("gate_verdict") or "").lower()
    if verdict == "fail":
        return [
            "- If this regression is unexpected, fix the agent, prompt, model, tools, or eval-pack change and push again.",
            "- If the behavior change is intentional, inspect the AgentClash run/comparison artifacts and promote or accept the regression deliberately.",
            "- Keep the locked baseline unchanged until the new behavior has been reviewed.",
        ]
    if verdict in {"pass", "passed"}:
        return ["- No action needed. The candidate stayed within the release-gate policy."]
    return [
        "- Open the failed GitHub Actions log and uploaded AgentClash JSON artifacts.",
        "- Fix manifest/API/auth/setup errors first; rerun the PR after the gate can create a candidate run.",
    ]


def format_link_or_code(value: Any, url: Any = None) -> str:
    if not value:
        return "`n/a`"
    label = escape_cell(str(value))
    if is_safe_http_url(url):
        return f"[`{label}`]({url})"
    return f"`{label}`"


def inspect_link_lines(links: dict[str, str]) -> list[str]:
    ordered = (
        ("candidate_run", "Candidate run"),
        ("baseline_run", "Baseline run"),
        ("comparison", "Compare baseline vs candidate"),
        ("candidate_failures", "Candidate failures"),
        ("candidate_scorecard", "Candidate scorecard"),
        ("candidate_replay", "Candidate replay"),
    )
    lines = []
    for key, label in ordered:
        url = links.get(key)
        if url:
            lines.append(f"- [{label}]({url})")
    return lines


def agentclash_links(result: dict[str, Any], app_url: str) -> dict[str, str]:
    links: dict[str, str] = {}
    workspace_id = clean_id(result.get("workspace_id"))
    candidate_run_id = clean_id(nested(result, "candidate.run_id"))
    candidate_run_agent_id = clean_id(nested(result, "candidate.run_agent_id"))
    baseline_run_id = clean_id(nested(result, "baseline.run_id"))
    baseline_run_agent_id = clean_id(nested(result, "baseline.run_agent_id"))

    def add(key: str, url: Any) -> None:
        safe = normalize_safe_url(url)
        if safe:
            links[key] = safe

    if workspace_id:
        links["_workspace_id"] = workspace_id
    if normalize_safe_url(app_url):
        links["_app_url"] = normalize_safe_url(app_url)

    if workspace_id:
        if candidate_run_id:
            add("candidate_run", app_link(app_url, "workspaces", workspace_id, "runs", candidate_run_id))
            add("candidate_failures", app_link(app_url, "workspaces", workspace_id, "runs", candidate_run_id, "failures"))
            if candidate_run_agent_id:
                add(
                    "candidate_scorecard",
                    app_link(app_url, "workspaces", workspace_id, "runs", candidate_run_id, "agents", candidate_run_agent_id, "scorecard"),
                )
                add(
                    "candidate_replay",
                    app_link(app_url, "workspaces", workspace_id, "runs", candidate_run_id, "agents", candidate_run_agent_id, "replay"),
                )
        if baseline_run_id:
            add("baseline_run", app_link(app_url, "workspaces", workspace_id, "runs", baseline_run_id))
            if baseline_run_agent_id:
                add(
                    "baseline_scorecard",
                    app_link(app_url, "workspaces", workspace_id, "runs", baseline_run_id, "agents", baseline_run_agent_id, "scorecard"),
                )
                add(
                    "baseline_replay",
                    app_link(app_url, "workspaces", workspace_id, "runs", baseline_run_id, "agents", baseline_run_agent_id, "replay"),
                )
        if baseline_run_id and candidate_run_id:
            query = urllib.parse.urlencode({"baseline": baseline_run_id, "candidate": candidate_run_id})
            add("comparison", app_link(app_url, "workspaces", workspace_id, "compare", query=query))

    add("candidate_run", links.get("candidate_run") or nested(result, "candidate.run_url"))
    add("baseline_run", links.get("baseline_run") or nested(result, "baseline.run_url"))
    add("workflow_run", nested(result, "candidate.ci_metadata.workflow_run_url"))
    return links


def regression_case_url(item: dict[str, Any], links: dict[str, str]) -> str:
    workspace_id = clean_id(links.get("_workspace_id"))
    if not workspace_id:
        return ""
    suite_id = clean_id(item.get("suite_id"))
    case_id = clean_id(item.get("case_id"))
    app_url = links.get("_app_url", "")
    if not suite_id or not case_id:
        return ""
    return app_link(app_url, "workspaces", workspace_id, "regression-suites", suite_id, "cases", case_id)


def app_link(app_url: str, *parts: str, query: str = "") -> str:
    base = normalize_safe_url(app_url)
    if not base:
        return ""
    path = "/".join(urllib.parse.quote(str(part).strip(), safe="") for part in parts if str(part).strip())
    if not path:
        return base.rstrip("/")
    url = f"{base.rstrip('/')}/{path}"
    if query:
        url = f"{url}?{query}"
    return url


def clean_id(value: Any) -> str:
    if value is None:
        return ""
    return str(value).strip()


def first_safe_url(*values: Any) -> str:
    for value in values:
        safe = normalize_safe_url(value)
        if safe:
            return safe
    return ""


def normalize_safe_url(value: Any) -> str:
    if not isinstance(value, str):
        return ""
    url = value.strip()
    if not url:
        return ""
    parsed = urllib.parse.urlsplit(url)
    if parsed.scheme not in {"http", "https"} or not parsed.netloc:
        return ""
    return url


def is_safe_http_url(value: Any) -> bool:
    return normalize_safe_url(value) != ""


def nested(value: Any, path: str) -> Any:
    current = value
    for part in path.split("."):
        if not isinstance(current, dict):
            return None
        current = current.get(part)
    return current


def format_number(value: Any) -> str:
    if value is None:
        return "n/a"
    if isinstance(value, (int, float)):
        return f"{value:.4g}"
    return str(value)


def escape_cell(value: str) -> str:
    return value.replace("|", "\\|").replace("\n", " ")


def load_event_payload(path: str) -> dict[str, Any]:
    try:
        return load_json_file(path)
    except (OSError, json.JSONDecodeError):
        return {}


def find_pr_number(result: dict[str, Any], env: dict[str, str], event_payload: dict[str, Any]) -> int | None:
    for candidate in (
        nested(result, "candidate.ci_metadata.pull_request_number"),
        nested(event_payload, "pull_request.number"),
        event_payload.get("number") if isinstance(event_payload, dict) else None,
    ):
        number = parse_positive_int(candidate)
        if number is not None:
            return number
    ref = env.get("GITHUB_REF", "")
    match = re.match(r"^refs/pull/(\d+)/", ref.strip())
    if match:
        return int(match.group(1))
    return None


def parse_positive_int(value: Any) -> int | None:
    try:
        number = int(value)
    except (TypeError, ValueError):
        return None
    return number if number > 0 else None


def upsert_comment(client: Any, pr_number: int, marker: str, body: str) -> CommentOutcome:
    comments = client.list_comments(pr_number)
    for comment in comments:
        if marker in str(comment.get("body") or ""):
            updated = client.update_comment(int(comment["id"]), body)
            return CommentOutcome("updated", "updated AgentClash CI PR comment", str(updated.get("html_url") or ""))
    created = client.create_comment(pr_number, body)
    return CommentOutcome("created", "created AgentClash CI PR comment", str(created.get("html_url") or ""))


def post_comment(
    *,
    manifest: str,
    mode: str = "ci",
    result: dict[str, Any],
    should_run: dict[str, Any],
    exit_code: int,
    repo: str,
    token: str,
    api_url: str,
    env: dict[str, str],
    event_payload: dict[str, Any],
    app_url: str = DEFAULT_APP_URL,
    client: Any | None = None,
) -> CommentOutcome:
    if not token:
        return CommentOutcome("skipped", "missing GitHub token")
    if not repo or "/" not in repo:
        return CommentOutcome("skipped", "missing GitHub repository context")
    pr_number = find_pr_number(result, env, event_payload)
    if pr_number is None:
        return CommentOutcome("skipped", "missing pull request context")

    body = build_comment(manifest=manifest, result=result, should_run=should_run, exit_code=exit_code, app_url=app_url, mode=mode)
    marker = marker_for_mode(mode, manifest)
    github = client or GitHubClient(api_url, repo, token)
    try:
        return upsert_comment(github, pr_number, marker, body)
    except urllib.error.HTTPError as error:
        return CommentOutcome("skipped", f"GitHub API returned HTTP {error.code}")
    except Exception as error:  # noqa: BLE001 - best-effort action helper.
        return CommentOutcome("skipped", f"could not post GitHub comment: {error}")


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Post a sticky AgentClash CI pull request comment.")
    parser.add_argument("--manifest", required=True)
    parser.add_argument("--mode", default="ci", choices=["ci", "prompt-eval"])
    parser.add_argument("--result-file", default="")
    parser.add_argument("--should-run-file", default="")
    parser.add_argument("--exit-code", type=int, default=0)
    parser.add_argument("--enabled", default="true")
    parser.add_argument("--repo", default=os.environ.get("GITHUB_REPOSITORY", ""))
    parser.add_argument("--event-path", default=os.environ.get("GITHUB_EVENT_PATH", ""))
    parser.add_argument("--api-url", default=os.environ.get("GITHUB_API_URL", "https://api.github.com"))
    parser.add_argument("--app-url", default=os.environ.get("AGENTCLASH_APP_URL", DEFAULT_APP_URL))
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    if args.enabled.strip().lower() not in {"1", "true", "yes", "y", "on"}:
        print("AgentClash CI PR comment skipped: disabled")
        return 0

    try:
        result = load_json_file(args.result_file)
    except (OSError, json.JSONDecodeError) as error:
        result = {"errors": [f"could not read AgentClash result file: {error}"]}
    try:
        should_run = load_json_file(args.should_run_file)
    except (OSError, json.JSONDecodeError):
        should_run = {}

    outcome = post_comment(
        manifest=args.manifest,
        mode=args.mode,
        result=result,
        should_run=should_run,
        exit_code=args.exit_code,
        repo=args.repo,
        token=os.environ.get("INPUT_GITHUB_TOKEN") or os.environ.get("GITHUB_TOKEN") or "",
        api_url=args.api_url,
        app_url=args.app_url,
        env=dict(os.environ),
        event_payload=load_event_payload(args.event_path),
    )
    suffix = f": {outcome.url}" if outcome.url else f": {outcome.message}"
    print(f"AgentClash CI PR comment {outcome.action}{suffix}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
