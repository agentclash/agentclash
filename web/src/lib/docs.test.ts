import { describe, expect, it } from "vitest";

import {
  buildLlmsFull,
  buildLlmsIndex,
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

  it("includes the index and every skill in markdown paths", () => {
    const paths = getAllDocMarkdownPaths();

    expect(paths).toContain("/docs-md/agent-skills");
    expect(paths).toContain("/docs-md/agent-skills/challenge-pack-skills");
    expect(paths).toContain("/docs-md/agent-skills/agent-build-skills");
    for (const slug of skillSlugs) {
      expect(paths).toContain(`/docs-md/agent-skills/${slug}`);
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
