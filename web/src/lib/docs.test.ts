import { describe, expect, it } from "vitest";

import {
  buildLlmsFull,
  buildLlmsIndex,
  DOCS_NAV,
  getAllDocMarkdownPaths,
  getDocBySlug,
  getDocsSearchIndex,
} from "./docs";

const skillSlugs = [
  "agent-build-skills/agentclash-agent-build-author",
  "agent-build-skills/agentclash-agent-deployment-setup",
  "agent-build-skills/agentclash-runtime-resources-setup",
  "agentclash-agent-harness-setup",
  "agentclash-ci-release-gate",
  "agentclash-cli-setup",
  "agentclash-compare-and-triage",
  "agentclash-dataset-workflows",
  "agentclash-eval-runner",
  "agentclash-hub",
  "agentclash-multi-turn-operator",
  "agentclash-prompt-eval-playground",
  "agentclash-quickstart",
  "agentclash-regression-flywheel",
  "agentclash-scorecard-reader",
  "agentclash-security-evaluation",
  "agentclash-workspace-admin",
  "eval-pack-skills/agentclash-eval-pack-artifacts",
  "eval-pack-skills/agentclash-eval-pack-input-sets",
  "eval-pack-skills/agentclash-eval-pack-llm-judges",
  "eval-pack-skills/agentclash-eval-pack-planner",
  "eval-pack-skills/agentclash-eval-pack-scoring-validators",
  "eval-pack-skills/agentclash-eval-pack-tools-sandbox",
  "eval-pack-skills/agentclash-eval-pack-validation-publish",
  "eval-pack-skills/agentclash-eval-pack-yaml-author",
];

describe("agent skill docs", () => {
  it("generates an agent skills index page", () => {
    const doc = getDocBySlug(["agent-skills"]);

    expect(doc?.title).toBe("Agent Skills");
    expect(doc?.content).toContain("web/content/agent-skills/.../SKILL.md");
    expect(doc?.content).toContain("agentclash-cli-setup");
    expect(doc?.content).toContain("agentclash-hub");
    expect(doc?.content).toContain("agentclash-quickstart");
    expect(doc?.content).toContain("Eval Pack Skills");
    expect(doc?.content).toContain("name: agentclash-skill-catalog");
    expect(doc?.content).toContain("## Generated Docs Contract");
  });

  it("generates category pages", () => {
    const doc = getDocBySlug(["agent-skills", "eval-pack-skills"]);

    expect(doc?.title).toBe("Eval Pack Skills");
    expect(doc?.content).toContain("agentclash-eval-pack-yaml-author");
    expect(doc?.content).toContain(
      "web/content/agent-skills/eval-pack-skills/<skill>/SKILL.md",
    );
  });

  it("generates top-level skill pages from canonical SKILL.md files", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-cli-setup"]);

    expect(doc?.title).toBe("CLI Setup Skill");
    expect(doc?.content).toContain("## Full SKILL.md");
    expect(doc?.content).toContain("name: agentclash-cli-setup");
    expect(doc?.content).toContain('AGENTCLASH_API_URL="https://api.agentclash.dev"');
    expect(doc?.content).toContain(
      "Workspace resolution:",
    );
    expect(doc?.content).toContain(
      "AGENTCLASH_TOKEN > stored CLI credentials",
    );
    expect(doc?.content).toContain("agentclash init --workspace-id");
    expect(doc?.content).toContain("doctor --json");
  });

  it("generates the hub skill with workflow map", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-hub"]);

    expect(doc?.title).toBe("Hub Skill");
    expect(doc?.content).toContain("name: agentclash-hub");
    expect(doc?.content).toContain("agentclash-quickstart");
    expect(doc?.content).toContain("agentclash-compare-and-triage");
    expect(doc?.content).toContain("agentclash-dataset-workflows");
    expect(doc?.content).toContain("https://agentclash.dev/docs/agent-skills");
  });

  it("generates the agent harness setup skill", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-agent-harness-setup"]);

    expect(doc?.title).toBe("Agent Harness Setup Skill");
    expect(doc?.content).toContain("agentclash agent-harness create");
    expect(doc?.content).toContain("agentclash agent-harness suite run");
    expect(doc?.content).toContain("codex_e2b");
  });

  it("generates the multi-turn operator skill", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-multi-turn-operator"]);

    expect(doc?.title).toBe("Multi Turn Operator Skill");
    expect(doc?.content).toContain("agentclash run turn status");
    expect(doc?.content).toContain("agentclash run turn submit");
    expect(doc?.content).toContain("awaiting_human");
  });

  it("generates the dataset workflows skill", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-dataset-workflows"]);

    expect(doc?.title).toBe("Dataset Workflows Skill");
    expect(doc?.content).toContain("agentclash dataset test");
    expect(doc?.content).toContain("agentclash dataset sync-regression-suite");
    expect(doc?.content).toContain("agentclash dataset import-traces");
  });

  it("generates the prompt eval playground skill", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-prompt-eval-playground"]);

    expect(doc?.title).toBe("Prompt Eval Playground Skill");
    expect(doc?.content).toContain("agentclash prompt-eval validate");
    expect(doc?.content).toContain("agentclash prompt-eval run");
    expect(doc?.content).toContain("agentclash playground list");
  });

  it("generates the security evaluation skill", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-security-evaluation"]);

    expect(doc?.title).toBe("Security Evaluation Skill");
    expect(doc?.content).toContain("agentclash security stress-run");
    expect(doc?.content).toContain("agentclash security agent-vault-stress");
    expect(doc?.content).toContain("agentclash security runtime-stress");
    expect(doc?.content).toContain("agentclash security avmock-upstream");
    expect(doc?.content).toContain("/docs-md/guides/security-evaluation");
  });

  it("generates the workspace admin skill", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-workspace-admin"]);

    expect(doc?.title).toBe("Workspace Admin Skill");
    expect(doc?.content).toContain("agentclash org list");
    expect(doc?.content).toContain("agentclash workspace create");
    expect(doc?.content).toContain("agentclash workspace members invite");
    expect(doc?.content).toContain("agentclash link");
  });

  it("generates the quickstart skill with readiness checks", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-quickstart"]);

    expect(doc?.title).toBe("Quickstart Skill");
    expect(doc?.content).toContain("agentclash quickstart");
    expect(doc?.content).toContain("next_command");
    expect(doc?.content).toContain("eval_packs");
  });

  it("generates the compare and triage skill with gate commands", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-compare-and-triage"]);

    expect(doc?.title).toBe("Compare And Triage Skill");
    expect(doc?.content).toContain("agentclash compare latest");
    expect(doc?.content).toContain("agentclash compare gate");
    expect(doc?.content).toContain("agentclash replay triage");
    expect(doc?.content).toContain("agentclash baseline set");
  });

  it("generates the eval runner skill with source-backed details", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-eval-runner"]);

    expect(doc?.title).toBe("Eval Runner Skill");
    expect(doc?.content).toContain("agentclash eval start");
    expect(doc?.content).toContain("agentclash eval session list");
    expect(doc?.content).toContain("agentclash run series report");
    expect(doc?.content).toContain("--pack <PACK_ID_OR_SLUG_OR_EXACT_NAME>");
    expect(doc?.content).toContain("--deployment <DEPLOYMENT_ID_OR_EXACT_NAME>");
    expect(doc?.content).toContain("--deployments <AGENT_DEPLOYMENT_ID>");
    expect(doc?.content).toContain("run create` does not resolve pack slugs");
    expect(doc?.content).toContain("\"agent_deployment_ids\": [\"<AGENT_DEPLOYMENT_ID>\"]");
    expect(doc?.content).toContain("\"race_context_min_step_gap\": 3");
    expect(doc?.content).toContain("\"links\":");
    expect(doc?.content).toContain("--scope suite_only");
    expect(doc?.content).toContain("--repetitions >= 2");
    expect(doc?.content).toContain("posts to `/v1/eval-sessions`");
    expect(doc?.content).toContain("`--follow` is not supported with `--repetitions >= 2`");
    expect(doc?.content).toContain("agentclash run events <RUN_ID>");
    expect(doc?.content).toContain("do not stream events");
    expect(doc?.content).toContain("one NDJSON event payload per line");
    expect(doc?.content).toContain("agentclash eval scorecard <RUN_ID> --agent");
    expect(doc?.content).toContain("`eval scorecard --json` returns an envelope");
    expect(doc?.content).toContain("missing_challenge_input_set_id");
    expect(doc?.content).toContain("invalid_race_context");
  });

  it("generates the scorecard reader skill with source-backed details", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-scorecard-reader"]);

    expect(doc?.title).toBe("Scorecard Reader Skill");
    expect(doc?.content).toContain("agentclash run ranking <RUN_ID> --json");
    expect(doc?.content).toContain("agentclash run scorecard <RUN_AGENT_ID> --json");
    expect(doc?.content).toContain("agentclash replay get <RUN_AGENT_ID> --limit 50 --json");
    expect(doc?.content).toContain("artifact list` is workspace-wide");
    expect(doc?.content).toContain("It does not have a `--run` filter today");
    expect(doc?.content).toContain("\"ranking\":");
    expect(doc?.content).toContain("\"evidence_quality\"");
    expect(doc?.content).toContain("\"llm_judge_results\"");
    expect(doc?.content).toContain("\"validator_details\"");
    expect(doc?.content).toContain("\"failure_cluster_key\"");
    expect(doc?.content).toContain("HTTP 202");
    expect(doc?.content).toContain("HTTP 409");
    expect(doc?.content).toContain("incorrect_final_output");
    expect(doc?.content).toContain("retrieval_grounding_failure");
    expect(doc?.content).toContain("timeout_or_budget_exhaustion");
    expect(doc?.content).toContain("dependency_resolution_failure");
    expect(doc?.content).toContain("flaky_non_deterministic");
    expect(doc?.content).toContain("agentclash-regression-flywheel");
  });

  it("generates the regression flywheel skill with source-backed details", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-regression-flywheel"]);

    expect(doc?.title).toBe("Regression Flywheel Skill");
    expect(doc?.content).toContain("agentclash run failures <RUN_ID> --json");
    expect(doc?.content).toContain("agentclash regression-suite create");
    expect(doc?.content).toContain("--source-eval-pack-id <EVAL_PACK_ID>");
    expect(doc?.content).toContain("agentclash run promote-failure <RUN_ID> <CHALLENGE_IDENTITY_ID>");
    expect(doc?.content).toContain("not `failure_fingerprint` or `failure_cluster_key`");
    expect(doc?.content).toContain("\"source_eval_pack_id\"");
    expect(doc?.content).toContain("\"promotion_mode\": \"full_executable\"");
    expect(doc?.content).toContain("\"judge_threshold_overrides\"");
    expect(doc?.content).toContain("regression-suite case update <CASE_ID>");
    expect(doc?.content).toContain("There is no CLI command today to create a regression case directly");
    expect(doc?.content).toContain("Backend duplicate protection is intentionally narrow");
    expect(doc?.content).toContain("--scope suite_only");
    expect(doc?.content).toContain("regression_coverage");
    expect(doc?.content).toContain("failure_review_item_ambiguous");
    expect(doc?.content).toContain("agentclash-ci-release-gate");
  });

  it("generates the CI release gate skill with source-backed details", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-ci-release-gate"]);

    expect(doc?.title).toBe("CI Release Gate Skill");
    expect(doc?.content).toContain("agentclash ci init .agentclash/ci.yaml");
    expect(doc?.content).toContain(
      "agentclash ci validate .agentclash/ci.yaml --remote --json",
    );
    expect(doc?.content).toContain(
      "agentclash ci should-run --manifest .agentclash/ci.yaml --base origin/main --head HEAD --json",
    );
    expect(doc?.content).toContain(
      "agentclash ci run --manifest .agentclash/ci.yaml --json --artifact-dir agentclash-artifacts",
    );
    expect(doc?.content).toContain("candidate.build.agent_build_id");
    expect(doc?.content).toContain("candidate.deployment.runtime_profile_id");
    expect(doc?.content).toContain("evaluation.eval_pack_version_id");
    expect(doc?.content).toContain("baseline.run_id");
    expect(doc?.content).toContain("gate.fail_on");
    expect(doc?.content).toContain("gate.policy_file");
    expect(doc?.content).toContain("does not pass `gate.fail_on`");
    expect(doc?.content).toContain("regressions.promote_failures");
    expect(doc?.content).toContain("`0`: pass");
    expect(doc?.content).toContain("`1`: release gate failed");
    expect(doc?.content).toContain("`2`: release gate warning");
    expect(doc?.content).toContain("`3`: insufficient gate evidence");
    expect(doc?.content).toContain("`10`: invalid manifest");
    expect(doc?.content).toContain("`20`: API/auth failure");
    expect(doc?.content).toContain("`30`: candidate run timed out");
    expect(doc?.content).toContain("`31`: candidate run failed before gate evaluation");
    expect(doc?.content).toContain("agentclash-artifacts/run.json");
    expect(doc?.content).toContain("agentclash-artifacts/scorecard.json");
    expect(doc?.content).toContain("agentclash-artifacts/comparison.json");
    expect(doc?.content).toContain("agentclash-artifacts/gate.json");
    expect(doc?.content).toContain("agentclash-artifacts/result.json");
    expect(doc?.content).toContain("schema_version: \"2026-05-04\"");
    expect(doc?.content).toContain("Action inputs are exactly");
    expect(doc?.content).toContain("Action outputs are exactly");
    expect(doc?.content).toContain("agentclash-regression-flywheel");
  });

  it("generates nested eval pack and agent build skill pages", () => {
    const evalPackDoc = getDocBySlug([
      "agent-skills",
      "eval-pack-skills",
      "agentclash-eval-pack-yaml-author",
    ]);
    const deploymentDoc = getDocBySlug([
      "agent-skills",
      "agent-build-skills",
      "agentclash-agent-deployment-setup",
    ]);

    expect(evalPackDoc?.content).toContain(
      "name: agentclash-eval-pack-yaml-author",
    );
    expect(deploymentDoc?.content).toContain(
      "name: agentclash-agent-deployment-setup",
    );
  });

  it("generates the runtime resources setup skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "agent-build-skills",
      "agentclash-runtime-resources-setup",
    ]);

    expect(doc?.title).toBe("Runtime Resources Setup Skill");
    expect(doc?.content).toContain("workspace-secret://OPENAI_API_KEY");
    expect(doc?.content).toContain("agentclash infra provider-account models");
    expect(doc?.content).toContain("agentclash infra runtime-profile create --from-file");
    expect(doc?.content).toContain("workspace tools are `agentclash infra tool ...` resources");
    expect(doc?.content).toContain("\"capability_key\": \"inventory.lookup\"");
    expect(doc?.content).toContain("`x-ai` becomes `PROVIDER_X_AI_API_KEY`");
  });

  it("generates the agent build author skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "agent-build-skills",
      "agentclash-agent-build-author",
    ]);

    expect(doc?.title).toBe("Agent Build Author Skill");
    expect(doc?.content).toContain("agentclash build version create <BUILD_ID> --spec-file");
    expect(doc?.content).toContain("agentclash build version validate <VERSION_ID> --json");
    expect(doc?.content).toContain("\"agent_kind\": \"llm_agent\"");
    expect(doc?.content).toContain("\"policy_spec\"");
    expect(doc?.content).toContain("\"instructions\"");
    expect(doc?.content).toContain("`version_status`");
    expect(doc?.content).toContain("Version is valid");
    expect(doc?.content).toContain("ready versions are immutable");
  });

  it("generates the agent deployment setup skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "agent-build-skills",
      "agentclash-agent-deployment-setup",
    ]);

    expect(doc?.title).toBe("Agent Deployment Setup Skill");
    expect(doc?.content).toContain("agentclash deployment create --from-file deployment.json");
    expect(doc?.content).toContain("\"agent_build_id\": \"<AGENT_BUILD_ID>\"");
    expect(doc?.content).toContain("\"runtime_profile_id\": \"<RUNTIME_PROFILE_ID>\"");
    expect(doc?.content).toContain("\"model\": \"gpt-4.1\"");
    expect(doc?.content).toContain("only ready versions can be deployed");
    expect(doc?.content).toContain("agent_deployment_ids");
    expect(doc?.content).toContain("active deployments with snapshots");
  });

  it("generates the eval pack planner skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "eval-pack-skills",
      "agentclash-eval-pack-planner",
    ]);

    expect(doc?.title).toBe("Eval Pack Planner Skill");
    expect(doc?.content).toContain(
      "`pack`, `version`, optional top-level `tools`, `challenges`, and `input_sets`",
    );
    expect(doc?.content).toContain("`prompt_eval` for prompt-style tasks");
    expect(doc?.content).toContain(
      "`native` when the agent must use files, tools, sandbox policy",
    );
    expect(doc?.content).toContain(
      "Allowed tool kinds in `version.tool_policy.allowed_tool_kinds`",
    );
    expect(doc?.content).toContain(
      "Source: validators | metric | reliability | latency | cost | behavioral | llm_judge",
    );
    expect(doc?.content).toContain(
      "Next skill: <agentclash-eval-pack-yaml-author | other>",
    );
  });

  it("generates the eval pack yaml author skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "eval-pack-skills",
      "agentclash-eval-pack-yaml-author",
    ]);

    expect(doc?.title).toBe("Eval Pack YAML Author Skill");
    expect(doc?.content).toContain(
      "agentclash eval-pack init support-eval.yaml --template prompt_eval",
    );
    expect(doc?.content).toContain(
      "agentclash eval-pack validate support-eval.yaml --json",
    );
    expect(doc?.content).toContain("`pack`, `version`, `challenges`, `input_sets`");
    expect(doc?.content).toContain("case_key");
    expect(doc?.content).toContain("source: input:prompt");
    expect(doc?.content).toContain(
      "Supported `version.tool_policy.allowed_tool_kinds` values are exactly `browser`, `build`, `data`, `file`, and `network`",
    );
    expect(doc?.content).toContain("Do not use `shell` as an allowed tool kind");
    expect(doc?.content).toContain(
      "`prompt_eval` cannot use:\n- top-level `tools`\n- `version.tool_policy`\n- `version.sandbox`",
    );
    expect(doc?.content).toContain("`version.evaluation_spec.validators`");
    expect(doc?.content).toContain("judge_mode: hybrid");
  });

  it("generates the eval pack input sets skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "eval-pack-skills",
      "agentclash-eval-pack-input-sets",
    ]);

    expect(doc?.title).toBe("Eval Pack Input Sets Skill");
    expect(doc?.content).toContain("agentclash eval-pack validate path/to/pack.yaml --json");
    expect(doc?.content).toContain("input_sets:");
    expect(doc?.content).toContain("cases:");
    expect(doc?.content).toContain("challenge_key: refund-question");
    expect(doc?.content).toContain("case_key: refund-window-basic");
    expect(doc?.content).toContain("payload:");
    expect(doc?.content).toContain("inputs:");
    expect(doc?.content).toContain("expectations:");
    expect(doc?.content).toContain("source: input:prompt");
    expect(doc?.content).toContain("All cases inside the same input set must reference the same `challenge_key`");
    expect(doc?.content).toContain("smoke, CI, full, regression, and edge");
    expect(doc?.content).toContain("`--scope suite_only` is for regression suite/case selection");
  });

  it("generates the eval pack artifacts skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "eval-pack-skills",
      "agentclash-eval-pack-artifacts",
    ]);

    expect(doc?.title).toBe("Eval Pack Artifacts Skill");
    expect(doc?.content).toContain("agentclash eval-pack validate path/to/pack.yaml --json");
    expect(doc?.content).toContain("version.assets");
    expect(doc?.content).toContain("artifact_refs");
    expect(doc?.content).toContain("artifacts");
    expect(doc?.content).toContain("artifact_key");
    expect(doc?.content).toContain("path");
    expect(doc?.content).toContain("media_type");
    expect(doc?.content).toContain("artifact_id");
    expect(doc?.content).toContain("post_execution_checks");
    expect(doc?.content).toContain("file_capture");
    expect(doc?.content).toContain("file_json_schema");
    expect(doc?.content).toContain("artifact.<path>");
    expect(doc?.content).toContain("artifact.<artifact_key>[.<field>]");
    expect(doc?.content).toContain("file:<post_execution_check_key>");
    expect(doc?.content).toContain("agentclash artifact list");
    expect(doc?.content).toContain("It does not have a `--run` filter today");
  });

  it("generates the eval pack scoring validators skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "eval-pack-skills",
      "agentclash-eval-pack-scoring-validators",
    ]);

    expect(doc?.title).toBe("Eval Pack Scoring Validators Skill");
    expect(doc?.content).toContain("agentclash eval-pack validate path/to/pack.yaml --json");
    expect(doc?.content).toContain("version.evaluation_spec.validators");
    expect(doc?.content).toContain("exact_match");
    expect(doc?.content).toContain("contains");
    expect(doc?.content).toContain("regex_match");
    expect(doc?.content).toContain("json_schema");
    expect(doc?.content).toContain("json_path_match");
    expect(doc?.content).toContain("file_json_schema");
    expect(doc?.content).toContain("directory_structure");
    expect(doc?.content).toContain("code_execution");
    expect(doc?.content).toContain("target");
    expect(doc?.content).toContain("expected_from");
    expect(doc?.content).toContain("literal:");
    expect(doc?.content).toContain("file:<post_execution_check_key>");
    expect(doc?.content).toContain("scorecard.dimensions");
    expect(doc?.content).toContain("source: validators");
    expect(doc?.content).toContain("There is no validator-level `failure_message`");
    expect(doc?.content).toContain("Unknown validator type such as `has_json`");
  });

  it("generates the eval pack llm judges skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "eval-pack-skills",
      "agentclash-eval-pack-llm-judges",
    ]);

    expect(doc?.title).toBe("Eval Pack LLM Judges Skill");
    expect(doc?.content).toContain("agentclash eval-pack validate path/to/pack.yaml --json");
    expect(doc?.content).toContain("llm_judges");
    expect(doc?.content).toContain("judge_mode: hybrid");
    expect(doc?.content).toContain("rubric");
    expect(doc?.content).toContain("assertion");
    expect(doc?.content).toContain("reference");
    expect(doc?.content).toContain("n_wise");
    expect(doc?.content).toContain("context_from");
    expect(doc?.content).toContain("reference_from");
    expect(doc?.content).toContain("score_scale");
    expect(doc?.content).toContain("consensus");
    expect(doc?.content).toContain("models");
    expect(doc?.content).toContain("samples");
    expect(doc?.content).toContain("judge_limits");
    expect(doc?.content).toContain("max_samples_per_judge");
    expect(doc?.content).toContain("source: llm_judge");
    expect(doc?.content).toContain("judge_key");
    expect(doc?.content).toContain("anti_gaming_clauses");
    expect(doc?.content).toContain("There is no `abstention_rule`");
    expect(doc?.content).toContain("Validation rejects `${secrets.*}` references");
  });

  it("generates the eval pack validation publish skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "eval-pack-skills",
      "agentclash-eval-pack-validation-publish",
    ]);

    expect(doc?.title).toBe("Eval Pack Validation Publish Skill");
    expect(doc?.content).toContain("agentclash eval-pack validate path/to/pack.yaml --json");
    expect(doc?.content).toContain("agentclash eval-pack publish path/to/pack.yaml --json");
    expect(doc?.content).toContain("POST /v1/workspaces/<workspace-id>/eval-packs/validate");
    expect(doc?.content).toContain("POST /v1/workspaces/<workspace-id>/eval-packs");
    expect(doc?.content).toContain("\"valid\": true");
    expect(doc?.content).toContain("\"field\": \"version.evaluation_spec.validators[0].type\"");
    expect(doc?.content).toContain("HTTP 400");
    expect(doc?.content).toContain("eval_pack_id");
    expect(doc?.content).toContain("eval_pack_version_id");
    expect(doc?.content).toContain("evaluation_spec_id");
    expect(doc?.content).toContain("input_set_ids");
    expect(doc?.content).toContain("bundle_artifact_id");
    expect(doc?.content).toContain("eval_pack_version_exists");
    expect(doc?.content).toContain("eval_pack_metadata_conflict");
    expect(doc?.content).toContain("The API request body is capped at 1 MiB");
    expect(doc?.content).toContain("`publish` does not upload local files referenced by `path`");
    expect(doc?.content).toContain("agentclash eval start");
    expect(doc?.content).toContain("agentclash run create");
  });

  it("generates the eval pack tools sandbox skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "eval-pack-skills",
      "agentclash-eval-pack-tools-sandbox",
    ]);

    expect(doc?.title).toBe("Eval Pack Tools Sandbox Skill");
    expect(doc?.content).toContain("agentclash eval-pack validate path/to/pack.yaml --json");
    expect(doc?.content).toContain("tools:");
    expect(doc?.content).toContain("custom:");
    expect(doc?.content).toContain("implementation:");
    expect(doc?.content).toContain("primitive: http_request");
    expect(doc?.content).toContain("args:");
    expect(doc?.content).toContain("version.tool_policy.allowed_tool_kinds");
    expect(doc?.content).toContain("`browser`, `build`, `data`, `file`, and `network`");
    expect(doc?.content).toContain("Do not use `shell`");
    expect(doc?.content).toContain("`${secrets.INVENTORY_API_KEY}`");
    expect(doc?.content).toContain("`prompt_eval` packs cannot use eval-pack tools or sandbox settings");
    expect(doc?.content).toContain("network_allowlist");
    expect(doc?.content).toContain("additional_packages");
    expect(doc?.content).toContain("sandbox_template_id");
    expect(doc?.content).toContain("`version.filesystem` exists as a raw map");
  });

  it("includes the index and every skill in markdown paths", () => {
    const paths = getAllDocMarkdownPaths();

    expect(paths).toContain("/docs-md/agent-skills");
    expect(paths).toContain("/docs-md/agent-skills/eval-pack-skills");
    expect(paths).toContain("/docs-md/agent-skills/agent-build-skills");
    for (const slug of skillSlugs) {
      expect(paths).toContain(`/docs-md/agent-skills/${slug}`);
    }
  });

  it("resolves every docs navigation item", () => {
    for (const section of DOCS_NAV) {
      for (const item of section.items) {
        const doc = getDocBySlug(item.slug);

        expect(doc, item.href).not.toBeNull();
        expect(doc?.href).toBe(item.href);
      }
    }
  });

  it("includes platform pages, blog posts, and agent skills in llms.txt", () => {
    const index = buildLlmsIndex("https://example.test");

    expect(index).toContain("https://example.test/platform/agent-evaluation");
    expect(index).toContain(
      "https://example.test/platform/agent-regression-testing",
    );
    expect(index).toContain(
      "https://example.test/blog/ai-agent-evaluation-regression-testing",
    );
    expect(index).toContain("https://example.test/changelog");
    expect(index.match(/https:\/\/example\.test\/changelog/g)?.length).toBe(1);
    expect(index).toContain(
      "AI Agent Evaluation Needs Regression Testing, Not Just Benchmarks",
    );
    expect(index).toContain("https://example.test/docs-md/agent-skills");
    expect(index).toContain(
      "https://example.test/docs-md/agent-skills/agentclash-cli-setup",
    );
    expect(index).toContain(
      "https://example.test/docs-md/agent-skills/agentclash-hub",
    );
    expect(index).toContain(
      "https://example.test/docs-md/agent-skills/agentclash-quickstart",
    );
    expect(index).toContain(
      "https://example.test/docs-md/agent-skills/agentclash-compare-and-triage",
    );
    expect(index).toContain(
      "https://example.test/docs-md/agent-skills/eval-pack-skills/agentclash-eval-pack-yaml-author",
    );
  });

  it("includes platform pages in docs search data", () => {
    const index = getDocsSearchIndex();
    const evaluation = index.find(
      (item) => item.href === "/platform/agent-evaluation",
    );
    const regression = index.find(
      (item) => item.href === "/platform/agent-regression-testing",
    );

    expect(index.slice(0, 2).map((item) => item.href)).toEqual([
      "/platform/agent-evaluation",
      "/platform/agent-regression-testing",
    ]);
    const changelog = index.find((item) => item.href === "/changelog");
    expect(changelog?.title).toBe("Product Changelog");
    expect(changelog?.searchText).toContain("release notes");
    expect(evaluation?.title).toBe("AI Agent Evaluation Platform");
    expect(evaluation?.searchText).toContain("platform/agent-evaluation");
    expect(evaluation?.searchText).toContain("ai agent evaluation");
    expect(evaluation?.searchText).toContain("eval packs");
    expect(evaluation?.searchText).toContain("ci regression gates");
    expect(regression?.title).toBe("AI Agent Regression Testing");
    expect(regression?.searchText).toContain("platform/agent-regression-testing");
    expect(regression?.searchText).toContain("ai agent regression testing");
    expect(regression?.searchText).toContain("pull request gates");
    expect(regression?.searchText).toContain("scorecards");
  });

  it("resolves revamp doc pages from navigation", () => {
    for (const href of [
      "/docs/guides/datasets-overview",
      "/docs/eval-packs/multi-turn",
      "/docs/guides/security-evaluation",
    ]) {
      const doc = getDocBySlug(href.replace("/docs/", "").split("/"));
      expect(doc?.href).toBe(href);
    }
  });

  it("includes platform pages, blog posts, skill catalog, and skill bodies in llms-full.txt", () => {
    const bundle = buildLlmsFull("https://example.test");

    expect(bundle).toContain("https://example.test/platform/agent-evaluation");
    expect(bundle).toContain(
      "https://example.test/platform/agent-regression-testing",
    );
    expect(bundle).toContain(
      "# AI Agent Evaluation Needs Regression Testing, Not Just Benchmarks",
    );
    expect(bundle).toContain(
      "Source: https://example.test/blog/ai-agent-evaluation-regression-testing",
    );
    expect(bundle).toContain("# AgentClash Changelog");
    expect(bundle).toContain("Source: https://example.test/changelog");
    expect(bundle).toContain(
      "[AI agent evaluation platform](https://example.test/platform/agent-evaluation)",
    );
    expect(bundle).toContain(
      "[AI agent regression testing](https://example.test/platform/agent-regression-testing)",
    );
    expect(bundle).toContain(
      "[CI/CD agent gates](https://example.test/docs-md/guides/ci-cd-agent-gates)",
    );
    expect(bundle).toContain(
      "https://example.test/docs-md/guides/datasets-overview",
    );
    expect(bundle).toContain(
      "https://example.test/docs-md/eval-packs/multi-turn",
    );
    expect(bundle).toContain(
      "https://example.test/docs-md/guides/security-evaluation",
    );
    expect(bundle).toContain("# Agent Skills");
    expect(bundle).toContain("name: agentclash-skill-catalog");
    expect(bundle).toContain("## Generated Docs Contract");
    expect(bundle).toContain("# CLI Setup Skill");
    expect(bundle).toContain("# Eval Pack YAML Author Skill");
    expect(bundle).toContain("name: agentclash-cli-setup");
    expect(bundle).toContain("Commands unexpectedly hit `http://localhost:8080`");
    expect(bundle).toContain("name: agentclash-runtime-resources-setup");
    expect(bundle).toContain("credential_reference: \"workspace-secret://KEY\"");
  });

  it("includes a Highlights block and the comparison page in llms.txt", () => {
    const index = buildLlmsIndex("https://example.test");

    expect(index).toContain("## Highlights");
    expect(index).toContain("Agent evaluation, not prompt evaluation");
    expect(index).toContain("https://example.test/compare");
  });

  it("lists the comparison hub as a public product page in search data", () => {
    const index = getDocsSearchIndex();
    const compare = index.find((item) => item.href === "/compare");

    expect(compare?.title).toBe("AgentClash vs prompt-eval tools");
    expect(compare?.searchText).toContain("alternative");
  });
});
