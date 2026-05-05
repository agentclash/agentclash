import { describe, expect, it } from "vitest";

import {
  buildLlmsFull,
  buildLlmsIndex,
  DOCS_NAV,
  getAllDocMarkdownPaths,
  getDocBySlug,
} from "./docs";

const skillSlugs = [
  "agent-build-skills/agentclash-agent-build-author",
  "agent-build-skills/agentclash-agent-deployment-setup",
  "agent-build-skills/agentclash-runtime-resources-setup",
  "agentclash-ci-release-gate",
  "agentclash-cli-setup",
  "agentclash-eval-runner",
  "agentclash-regression-flywheel",
  "agentclash-scorecard-reader",
  "challenge-pack-skills/agentclash-challenge-pack-artifacts",
  "challenge-pack-skills/agentclash-challenge-pack-input-sets",
  "challenge-pack-skills/agentclash-challenge-pack-llm-judges",
  "challenge-pack-skills/agentclash-challenge-pack-planner",
  "challenge-pack-skills/agentclash-challenge-pack-scoring-validators",
  "challenge-pack-skills/agentclash-challenge-pack-tools-sandbox",
  "challenge-pack-skills/agentclash-challenge-pack-validation-publish",
  "challenge-pack-skills/agentclash-challenge-pack-yaml-author",
];

describe("agent skill docs", () => {
  it("generates an agent skills index page", () => {
    const doc = getDocBySlug(["agent-skills"]);

    expect(doc?.title).toBe("Agent Skills");
    expect(doc?.content).toContain("web/content/agent-skills/.../SKILL.md");
    expect(doc?.content).toContain("agentclash-cli-setup");
    expect(doc?.content).toContain("Challenge Pack Skills");
    expect(doc?.content).toContain("name: agentclash-skill-catalog");
    expect(doc?.content).toContain("## Generated Docs Contract");
  });

  it("generates category pages", () => {
    const doc = getDocBySlug(["agent-skills", "challenge-pack-skills"]);

    expect(doc?.title).toBe("Challenge Pack Skills");
    expect(doc?.content).toContain("agentclash-challenge-pack-yaml-author");
    expect(doc?.content).toContain(
      "web/content/agent-skills/challenge-pack-skills/<skill>/SKILL.md",
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

  it("generates the eval runner skill with source-backed details", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-eval-runner"]);

    expect(doc?.title).toBe("Eval Runner Skill");
    expect(doc?.content).toContain("agentclash eval start");
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
    expect(doc?.content).toContain("--source-challenge-pack-id <CHALLENGE_PACK_ID>");
    expect(doc?.content).toContain("agentclash run promote-failure <RUN_ID> <CHALLENGE_IDENTITY_ID>");
    expect(doc?.content).toContain("not `failure_fingerprint` or `failure_cluster_key`");
    expect(doc?.content).toContain("\"source_challenge_pack_id\"");
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
    expect(doc?.content).toContain("evaluation.challenge_pack_version_id");
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

  it("generates nested challenge pack and agent build skill pages", () => {
    const challengePackDoc = getDocBySlug([
      "agent-skills",
      "challenge-pack-skills",
      "agentclash-challenge-pack-yaml-author",
    ]);
    const deploymentDoc = getDocBySlug([
      "agent-skills",
      "agent-build-skills",
      "agentclash-agent-deployment-setup",
    ]);

    expect(challengePackDoc?.content).toContain(
      "name: agentclash-challenge-pack-yaml-author",
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
    expect(doc?.content).toContain("agentclash infra model-catalog list");
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

  it("generates the challenge pack planner skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "challenge-pack-skills",
      "agentclash-challenge-pack-planner",
    ]);

    expect(doc?.title).toBe("Challenge Pack Planner Skill");
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
      "Next skill: <agentclash-challenge-pack-yaml-author | other>",
    );
  });

  it("generates the challenge pack yaml author skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "challenge-pack-skills",
      "agentclash-challenge-pack-yaml-author",
    ]);

    expect(doc?.title).toBe("Challenge Pack YAML Author Skill");
    expect(doc?.content).toContain(
      "agentclash challenge-pack init support-eval.yaml --template prompt_eval",
    );
    expect(doc?.content).toContain(
      "agentclash challenge-pack validate support-eval.yaml --json",
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

  it("generates the challenge pack input sets skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "challenge-pack-skills",
      "agentclash-challenge-pack-input-sets",
    ]);

    expect(doc?.title).toBe("Challenge Pack Input Sets Skill");
    expect(doc?.content).toContain("agentclash challenge-pack validate path/to/pack.yaml --json");
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

  it("generates the challenge pack artifacts skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "challenge-pack-skills",
      "agentclash-challenge-pack-artifacts",
    ]);

    expect(doc?.title).toBe("Challenge Pack Artifacts Skill");
    expect(doc?.content).toContain("agentclash challenge-pack validate path/to/pack.yaml --json");
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

  it("generates the challenge pack scoring validators skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "challenge-pack-skills",
      "agentclash-challenge-pack-scoring-validators",
    ]);

    expect(doc?.title).toBe("Challenge Pack Scoring Validators Skill");
    expect(doc?.content).toContain("agentclash challenge-pack validate path/to/pack.yaml --json");
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

  it("generates the challenge pack llm judges skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "challenge-pack-skills",
      "agentclash-challenge-pack-llm-judges",
    ]);

    expect(doc?.title).toBe("Challenge Pack LLM Judges Skill");
    expect(doc?.content).toContain("agentclash challenge-pack validate path/to/pack.yaml --json");
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

  it("generates the challenge pack validation publish skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "challenge-pack-skills",
      "agentclash-challenge-pack-validation-publish",
    ]);

    expect(doc?.title).toBe("Challenge Pack Validation Publish Skill");
    expect(doc?.content).toContain("agentclash challenge-pack validate path/to/pack.yaml --json");
    expect(doc?.content).toContain("agentclash challenge-pack publish path/to/pack.yaml --json");
    expect(doc?.content).toContain("POST /v1/workspaces/<workspace-id>/challenge-packs/validate");
    expect(doc?.content).toContain("POST /v1/workspaces/<workspace-id>/challenge-packs");
    expect(doc?.content).toContain("\"valid\": true");
    expect(doc?.content).toContain("\"field\": \"version.evaluation_spec.validators[0].type\"");
    expect(doc?.content).toContain("HTTP 400");
    expect(doc?.content).toContain("challenge_pack_id");
    expect(doc?.content).toContain("challenge_pack_version_id");
    expect(doc?.content).toContain("evaluation_spec_id");
    expect(doc?.content).toContain("input_set_ids");
    expect(doc?.content).toContain("bundle_artifact_id");
    expect(doc?.content).toContain("challenge_pack_version_exists");
    expect(doc?.content).toContain("challenge_pack_metadata_conflict");
    expect(doc?.content).toContain("The API request body is capped at 1 MiB");
    expect(doc?.content).toContain("`publish` does not upload local files referenced by `path`");
    expect(doc?.content).toContain("agentclash eval start");
    expect(doc?.content).toContain("agentclash run create");
  });

  it("generates the challenge pack tools sandbox skill with source-backed details", () => {
    const doc = getDocBySlug([
      "agent-skills",
      "challenge-pack-skills",
      "agentclash-challenge-pack-tools-sandbox",
    ]);

    expect(doc?.title).toBe("Challenge Pack Tools Sandbox Skill");
    expect(doc?.content).toContain("agentclash challenge-pack validate path/to/pack.yaml --json");
    expect(doc?.content).toContain("tools:");
    expect(doc?.content).toContain("custom:");
    expect(doc?.content).toContain("implementation:");
    expect(doc?.content).toContain("primitive: http_request");
    expect(doc?.content).toContain("args:");
    expect(doc?.content).toContain("version.tool_policy.allowed_tool_kinds");
    expect(doc?.content).toContain("`browser`, `build`, `data`, `file`, and `network`");
    expect(doc?.content).toContain("Do not use `shell`");
    expect(doc?.content).toContain("`${secrets.INVENTORY_API_KEY}`");
    expect(doc?.content).toContain("`prompt_eval` packs cannot use challenge-pack tools or sandbox settings");
    expect(doc?.content).toContain("network_allowlist");
    expect(doc?.content).toContain("additional_packages");
    expect(doc?.content).toContain("sandbox_template_id");
    expect(doc?.content).toContain("`version.filesystem` exists as a raw map");
  });

  it("includes the index and every skill in markdown paths", () => {
    const paths = getAllDocMarkdownPaths();

    expect(paths).toContain("/docs-md/agent-skills");
    expect(paths).toContain("/docs-md/agent-skills/challenge-pack-skills");
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

  it("includes agent skills in llms.txt", () => {
    const index = buildLlmsIndex("https://example.test");

    expect(index).toContain("https://example.test/docs-md/agent-skills");
    expect(index).toContain(
      "https://example.test/docs-md/agent-skills/agentclash-cli-setup",
    );
    expect(index).toContain(
      "https://example.test/docs-md/agent-skills/challenge-pack-skills/agentclash-challenge-pack-yaml-author",
    );
  });

  it("includes the skill catalog and skill bodies in llms-full.txt", () => {
    const bundle = buildLlmsFull("https://example.test");

    expect(bundle).toContain("# Agent Skills");
    expect(bundle).toContain("name: agentclash-skill-catalog");
    expect(bundle).toContain("## Generated Docs Contract");
    expect(bundle).toContain("# CLI Setup Skill");
    expect(bundle).toContain("# Challenge Pack YAML Author Skill");
    expect(bundle).toContain("name: agentclash-cli-setup");
    expect(bundle).toContain("Commands unexpectedly hit `http://localhost:8080`");
    expect(bundle).toContain("name: agentclash-runtime-resources-setup");
    expect(bundle).toContain("credential_reference: \"workspace-secret://KEY\"");
  });
});
