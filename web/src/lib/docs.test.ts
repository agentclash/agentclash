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
  });
});
