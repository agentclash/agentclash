---
name: agentclash-scorecard-reader
description: Use when interpreting AgentClash rankings, scorecards, replay timelines, artifacts, LLM judge results, or failure-review evidence into source-backed findings and next actions.
metadata:
  agentclash.role: reviewing
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Scorecard Reader

## Purpose
Turn completed or inspectable AgentClash run evidence into an engineering readout: who won, why, which claims are backed by scorecard/replay/artifact evidence, and what the next command or fix should be.

## Use When
- A user asks why an AgentClash run passed, failed, regressed, drifted, or picked a winner.
- You have a run ID and need to inspect rankings, run agents, scorecards, replay steps, failure-review items, or artifacts.
- A reviewer needs evidence-first findings instead of raw JSON dumps.
- A follow-up skill needs a grounded summary before promoting regressions or changing a challenge pack.

## Do Not Use When
- The user needs to start a new eval run; use `agentclash-eval-runner`.
- The user needs to author, validate, or publish the challenge pack first; use the challenge-pack skills.
- The user is ready to promote failures into regression suites; use `agentclash-regression-flywheel` after this skill identifies the useful failures.

## Inputs Needed
- Workspace ID or configured workspace context.
- Run ID.
- Optional run agent ID or agent label for a specific scorecard or replay.
- Optional baseline expectation, expected winner, or release-gate decision to compare against.
- Optional artifact IDs from failure evidence, scorecards, or workspace artifact list.

## Environment
Use hosted production by default unless the user intentionally targets local or self-hosted infrastructure:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash auth status
agentclash workspace use <WORKSPACE_ID>
```

Workspace resolution follows the CLI setup rules: `--workspace`, `AGENTCLASH_WORKSPACE`, saved config, or `.agentclash.yaml`. `run failures`, `artifact list`, and `eval scorecard` require a workspace. `artifact download` uses an artifact ID directly.

## Procedure
1. Confirm the run exists and get its agent IDs.
2. Read the ranking to identify the winner, sort mode, gaps, unavailable scores, and evidence warnings.
3. Read the relevant scorecard. Use `eval scorecard` for run-first analysis and baseline comparison; use `run scorecard` when you already have a run agent ID.
4. Read failure-review items for concrete failed checks, replay refs, artifact refs, judge refs, metric refs, severity, and promotability.
5. Pull replay steps around referenced sequences, or page through replay when no refs exist.
6. Inspect artifacts only when a scorecard, failure item, or user request points to them.
7. Report claims as evidence-first findings: claim, evidence pointer, impact, next action.

## Commands
Start with run-level shape:

```bash
agentclash run get <RUN_ID> --json
agentclash run agents <RUN_ID> --json
agentclash run ranking <RUN_ID> --json
agentclash run ranking <RUN_ID> --sort-by composite --json
agentclash run ranking <RUN_ID> --sort-by correctness --json
```

Scorecard commands:

```bash
agentclash eval scorecard <RUN_ID> --agent <RUN_AGENT_ID_OR_LABEL> --json
agentclash eval scorecard --agent <RUN_AGENT_ID_OR_LABEL> --json
agentclash run scorecard <RUN_AGENT_ID> --json
```

Replay and failure-review commands:

```bash
agentclash run failures <RUN_ID> --json
agentclash run failures <RUN_ID> --agent <RUN_AGENT_ID> --severity blocking --json
agentclash run failures <RUN_ID> --class policy_violation --evidence-tier hosted_structured --json
agentclash run failures <RUN_ID> --cluster <FAILURE_CLUSTER_KEY> --limit 50 --json
agentclash replay get <RUN_AGENT_ID> --limit 50 --json
agentclash replay get <RUN_AGENT_ID> --cursor 50 --limit 50 --json
```

Artifact commands:

```bash
agentclash artifact list --json
agentclash artifact download <ARTIFACT_ID> --output <PATH>
```

Important exact CLI shapes:

- `run scorecard` takes one argument: `<RUN_AGENT_ID>`. It does not accept `<RUN_ID> <RUN_AGENT_ID>`.
- `replay get` takes one argument: `<RUN_AGENT_ID>`. It does not accept `<RUN_ID> <RUN_AGENT_ID>`.
- `artifact list` is workspace-wide. It does not have a `--run` filter today; use `artifact_refs`, `run_id`, or `run_agent_id` fields from JSON output to choose what to download.
- `run ranking --sort-by` commonly uses `composite`, `correctness`, `reliability`, `latency`, or `cost`. The backend also accepts a custom dimension key when that key exists in the scorecard dimensions; unknown keys return `invalid_sort_by`.
- `run failures --limit` defaults to 50 when omitted and is capped at 200 by the API.

## Ranking JSON
`agentclash run ranking <RUN_ID> --json` returns a stateful response:

```json
{
  "state": "ready",
  "ranking": {
    "run_id": "<RUN_ID>",
    "evaluation_spec_id": "<EVALUATION_SPEC_ID>",
    "sort": {
      "field": "correctness_then_reliability",
      "direction": "desc",
      "default_order": true
    },
    "winner": {
      "run_agent_id": "<RUN_AGENT_ID>",
      "strategy": "<strategy>",
      "status": "<status>",
      "reason_code": "<reason_code>"
    },
    "evidence_quality": {
      "missing_fields": [],
      "warnings": []
    },
    "items": [
      {
        "rank": 1,
        "run_agent_id": "<RUN_AGENT_ID>",
        "lane_index": 0,
        "label": "<agent label>",
        "status": "completed",
        "has_scorecard": true,
        "evaluation_status": "complete",
        "sort_value": 0.92,
        "delta_from_top": 0,
        "sort_state": "available",
        "strategy": "<strategy>",
        "passed": true,
        "overall_reason": "<reason>",
        "composite_score": 0.91,
        "overall_score": 0.91,
        "correctness_score": 0.95,
        "reliability_score": 0.9,
        "latency_score": 0.8,
        "cost_score": 0.7,
        "dimensions": {
          "correctness": {
            "state": "available",
            "score": 0.95,
            "better_direction": "higher"
          }
        }
      }
    ]
  }
}
```

Read `evidence_quality.warnings` before declaring a winner as conclusive. A low `sort_value`, missing `rank`, `sort_state: "unavailable"`, `has_scorecard: false`, or missing score fields means the ranking may be partial even if the run itself completed.

## Scorecard JSON
Use `agentclash run scorecard <RUN_AGENT_ID> --json` when you already know the agent ID. The top level mirrors `/v1/scorecards/{runAgentID}`:

```json
{
  "state": "ready",
  "run_agent_status": "completed",
  "id": "<SCORECARD_ID>",
  "run_agent_id": "<RUN_AGENT_ID>",
  "run_id": "<RUN_ID>",
  "evaluation_spec_id": "<EVALUATION_SPEC_ID>",
  "overall_score": 0.91,
  "correctness_score": 0.95,
  "reliability_score": 0.9,
  "latency_score": 0.8,
  "cost_score": 0.7,
  "behavioral_score": 0.85,
  "llm_judge_results": [
    {
      "id": "<JUDGE_RESULT_ID>",
      "judge_key": "<judge_key>",
      "mode": "<mode>",
      "normalized_score": 0.8,
      "confidence": "medium",
      "variance": 0.02,
      "sample_count": 3,
      "model_count": 1,
      "payload": {},
      "created_at": "<timestamp>",
      "updated_at": "<timestamp>"
    }
  ],
  "scorecard": {
    "run_agent_id": "<RUN_AGENT_ID>",
    "evaluation_spec_id": "<EVALUATION_SPEC_ID>",
    "status": "complete",
    "strategy": "<strategy>",
    "overall_score": 0.91,
    "passed": true,
    "overall_reason": "<reason>",
    "warnings": [],
    "dimensions": {
      "correctness": {
        "state": "available",
        "score": 0.95,
        "reason": "<reason>",
        "weight": 1,
        "gate": true,
        "pass_threshold": 0.8,
        "contribution": 0.95,
        "gate_passed": true
      }
    },
    "validator_summary": {},
    "validator_details": [
      {
        "key": "<validator_key>",
        "type": "<validator_type>",
        "verdict": "pass",
        "state": "complete",
        "reason": "<reason>",
        "normalized_score": 1,
        "source": {
          "kind": "final_output",
          "sequence": 12,
          "event_type": "<event type>",
          "field_path": "<field path>"
        }
      }
    ],
    "metric_summary": {},
    "metric_details": [
      {
        "key": "<metric_key>",
        "collector": "<collector>",
        "state": "available",
        "numeric_value": 123
      }
    ]
  },
  "created_at": "<timestamp>",
  "updated_at": "<timestamp>"
}
```

Read the nested `scorecard.dimensions` first, then inspect `validator_details`, `metric_details`, and `llm_judge_results` for supporting evidence. Treat `llm_judge_results.payload` as judge-specific raw data; do not invent a stable schema inside it.

## Eval Scorecard Envelope
`agentclash eval scorecard [RUN_ID] --agent <RUN_AGENT_ID_OR_LABEL> --json` is run-first. If `RUN_ID` is omitted, the CLI selects the latest run in the workspace. If the run has multiple agents in non-interactive mode, pass `--agent`.

Structured output is an envelope:

```json
{
  "candidate": {
    "workspace_id": "<WORKSPACE_ID>",
    "run_id": "<RUN_ID>",
    "run_name": "<name>",
    "run_status": "completed",
    "run_agent_id": "<RUN_AGENT_ID>",
    "run_agent_label": "<label>",
    "official_pack_mode": "full"
  },
  "baseline": null,
  "scorecard": {},
  "comparison": null,
  "release_gate": null
}
```

When a baseline bookmark exists, the CLI also fetches `/v1/compare` and `/v1/release-gates/evaluate`, then fills `baseline`, `comparison`, and `release_gate`. Use this envelope for regression-style summaries; use `run scorecard` for the raw per-agent scorecard only.

## Replay JSON
`agentclash replay get <RUN_AGENT_ID> --json` returns replay state, optional replay metadata, steps, and pagination:

```json
{
  "state": "ready",
  "run_agent_id": "<RUN_AGENT_ID>",
  "run_id": "<RUN_ID>",
  "run_agent_status": "completed",
  "replay": {
    "id": "<REPLAY_ID>",
    "artifact_id": "<ARTIFACT_ID>",
    "summary": {},
    "latest_sequence_number": 12,
    "event_count": 42,
    "created_at": "<timestamp>",
    "updated_at": "<timestamp>"
  },
  "steps": [
    {
      "sequence_number": 12,
      "step_type": "<step type>",
      "summary": "<summary>"
    }
  ],
  "pagination": {
    "next_cursor": "50",
    "limit": 50,
    "total_steps": 120,
    "has_more": true
  }
}
```

Use replay to verify whether scorecard and judge claims match observable behavior. Prefer referenced `sequence_number` values from failure items or validator sources before paging through the whole replay.

## Failure Review JSON
`agentclash run failures <RUN_ID> --json` returns:

```json
{
  "items": [
    {
      "run_id": "<RUN_ID>",
      "run_agent_id": "<RUN_AGENT_ID>",
      "challenge_identity_id": "<CHALLENGE_ID>",
      "challenge_key": "<challenge_key>",
      "case_key": "<case_key>",
      "item_key": "<item_key>",
      "failure_fingerprint": "frf_...",
      "failure_cluster_key": "frc_...",
      "failure_state": "failed",
      "failed_dimensions": ["correctness"],
      "failed_checks": ["<validator_or_judge_key>"],
      "failure_class": "policy_violation",
      "headline": "<headline>",
      "detail": "<detail>",
      "recommended_action": "<recommended action>",
      "promotable": true,
      "promotion_mode_available": ["full_executable", "output_only"],
      "replay_step_refs": [
        {
          "sequence_number": 12,
          "event_type": "<event type>",
          "kind": "<kind>"
        }
      ],
      "artifact_refs": [
        {
          "key": "<artifact key>",
          "kind": "<kind>",
          "path": "<path>",
          "media_type": "<media type>"
        }
      ],
      "judge_refs": [
        {
          "key": "<judge key>",
          "kind": "llm_judge",
          "state": "fail",
          "normalized_score": 0.2,
          "reason": "<reason>",
          "sequence_number": 12,
          "event_type": "<event type>"
        }
      ],
      "metric_refs": [
        {
          "key": "<metric key>",
          "metric_type": "<type>",
          "state": "available",
          "numeric_value": 123
        }
      ],
      "evidence_tier": "hosted_structured",
      "severity": "blocking"
    }
  ],
  "clusters": [
    {
      "failure_cluster_key": "frc_...",
      "representative_failure_fingerprint": "frf_...",
      "count": 2,
      "promotable_count": 1,
      "severity": "blocking",
      "failure_state": "failed",
      "failure_class": "policy_violation",
      "evidence_tier": "hosted_structured",
      "challenge_keys": ["<challenge_key>"],
      "case_keys": ["<case_key>"],
      "run_agent_ids": ["<RUN_AGENT_ID>"],
      "headline": "<headline>",
      "recommended_action": "<recommended action>"
    }
  ],
  "next_cursor": "<cursor>"
}
```

Filters supported by the CLI:

- `--agent <RUN_AGENT_ID>`
- `--severity info|warning|blocking`
- `--class <failure_class>`
- `--evidence-tier none|native_structured|hosted_structured|hosted_black_box|derived_summary`
- `--cluster <FAILURE_CLUSTER_KEY>`
- `--cursor <NEXT_CURSOR>`
- `--limit <COUNT>`

Failure classes currently accepted by the API are `incorrect_final_output`, `tool_selection_error`, `tool_argument_error`, `retrieval_grounding_failure`, `policy_violation`, `timeout_or_budget_exhaustion`, `sandbox_failure`, `dependency_resolution_failure`, `malformed_output`, `flaky_non_deterministic`, `insufficient_evidence`, and `other`.

## Stateful Reads and Exit Codes
Ranking, scorecard, and replay reads can be ready, pending, or errored.

- Pending responses use HTTP 202 and include `state: "pending"` plus `message`. The CLI prints the raw payload in structured mode and exits successfully.
- Errored responses use HTTP 409 and include `state: "errored"` plus `message`. The CLI prints the raw payload in structured mode and exits with code 1.
- Common messages include `ranking is not ready yet`, `scorecard generation is pending`, `scorecard generation failed or scorecard data is unavailable`, and `replay generation is pending`.

When a read is pending, do not infer pass/fail. Re-check later or inspect `run get`, `run agents`, and `run events`. When a read is errored, report the state and switch to available run, failure, event, and artifact evidence.

## Evidence-First Interpretation
Use this ordering when writing findings:

1. Ranking: winner, sort field, score gap, evidence warnings, unavailable scores.
2. Scorecard dimensions: failed gates, low scores, unavailable/error dimensions, `overall_reason`, warnings.
3. Validator and metric details: exact failed checks, source refs, numeric values, reasons.
4. LLM judge results: judge key, mode, normalized score, confidence, variance, sample/model counts, concise rationale if present in payload.
5. Failure review: `failure_state`, `failure_class`, `severity`, `evidence_tier`, refs, `recommended_action`, promotability.
6. Replay: sequence refs and behavior observed around final output, tool calls, sandbox failures, or malformed outputs.
7. Artifacts: downloaded only when needed to verify file output or user-visible content.

Do not say "the judge proved" something. Say "the scorecard/judge reports X, and replay/artifact evidence Y supports or contradicts it."

## Expected Output
- A winner or status summary that names the run and agent IDs.
- A short list of findings with evidence pointers to scorecard fields, failure fingerprints or clusters, replay sequence numbers, and artifact IDs or paths.
- A distinction between confirmed evidence, judge rationale, unavailable data, and pending/errored reads.
- Follow-up commands a reviewer can run exactly.

## Failure Modes
- Missing workspace: run `agentclash link`, `agentclash workspace use <id>`, pass `--workspace`, or set `AGENTCLASH_WORKSPACE`.
- Multiple run agents for `eval scorecard`: pass `--agent <RUN_AGENT_ID_OR_LABEL>`.
- Ranking pending: wait for scoring or inspect `run get`, `run agents`, and `run events`.
- Scorecard pending: wait for scorecard generation; do not report pass/fail yet.
- Scorecard errored: the run agent may have failed, or scorecard data may be unavailable. Use failures, replay, events, and artifacts instead.
- Replay pending or noisy: use failure `replay_step_refs` or validator `source.sequence` before paging broadly.
- `artifact list --run` fails: the command does not exist. Use workspace-wide `artifact list --json` or artifact refs from failure/scorecard evidence.
- Unknown `--sort-by`: use a built-in sort field or a dimension key that exists in the scorecard.

## Safety Notes
- Do not paste secrets, private artifact contents, raw provider keys, customer data, or long logs into chat.
- Do not overstate LLM judge rationale as ground truth.
- Download artifacts only when needed, and prefer targeted artifact IDs from evidence refs.
- Scorecards and failures can contain model outputs and tool traces. Quote only the minimum needed to support the finding.
- Read commands are safe, but follow-up mutation commands such as failure promotion belong to `agentclash-regression-flywheel` and should be intentional.

## Report Back Format
```text
Outcome: <winner/status/pending/errored>
Run: <RUN_ID>
Agent(s): <RUN_AGENT_ID or labels>
Evidence:
- <claim> | scorecard=<field/path> | replay=<sequence or none> | artifact=<id/path or none>
Findings:
- <impact> -> <next action>
Uncertainties:
- <pending/errored/unavailable evidence, or none>
Follow-up commands:
- agentclash run ranking <RUN_ID> --json
- agentclash run failures <RUN_ID> --json
- agentclash run scorecard <RUN_AGENT_ID> --json
- agentclash replay get <RUN_AGENT_ID> --limit 50 --json
Next skill: <agentclash-regression-flywheel | challenge-pack skill | none>
```

## Related Skills
- `agentclash-cli-setup`
- `agentclash-eval-runner`
- `agentclash-scorecard-reader`
- `agentclash-regression-flywheel`
- `agentclash-ci-release-gate`

## Related Docs
- `/docs-md/concepts/replay-and-scorecards`
- `/docs-md/concepts/runs-and-evals`
- `/docs-md/concepts/artifacts`
- `/docs-md/reference/cli`
