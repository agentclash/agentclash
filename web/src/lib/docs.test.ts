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
