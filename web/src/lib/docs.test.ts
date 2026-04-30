import { describe, expect, it } from "vitest";

import {
  buildLlmsFull,
  buildLlmsIndex,
  getAllDocMarkdownPaths,
  getDocBySlug,
} from "./docs";

const skillSlugs = [
  "agentclash-challenge-pack-author",
  "agentclash-ci-release-gate",
  "agentclash-cli-setup",
  "agentclash-eval-runner",
  "agentclash-regression-flywheel",
  "agentclash-scorecard-reader",
];

describe("agent skill docs", () => {
  it("generates an agent skills index page", () => {
    const doc = getDocBySlug(["agent-skills"]);

    expect(doc?.title).toBe("Agent Skills");
    expect(doc?.content).toContain("web/content/agent-skills/<skill>/SKILL.md");
    expect(doc?.content).toContain("agentclash-cli-setup");
  });

  it("generates individual skill pages from canonical SKILL.md files", () => {
    const doc = getDocBySlug(["agent-skills", "agentclash-cli-setup"]);

    expect(doc?.title).toBe("CLI Setup Skill");
    expect(doc?.content).toContain("## Full SKILL.md");
    expect(doc?.content).toContain("name: agentclash-cli-setup");
    expect(doc?.content).toContain('AGENTCLASH_API_URL="https://staging-api.agentclash.dev"');
  });

  it("includes the index and every skill in markdown paths", () => {
    const paths = getAllDocMarkdownPaths();

    expect(paths).toContain("/docs-md/agent-skills");
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
  });

  it("includes the skill catalog and skill bodies in llms-full.txt", () => {
    const bundle = buildLlmsFull("https://example.test");

    expect(bundle).toContain("# Agent Skills");
    expect(bundle).toContain("# CLI Setup Skill");
    expect(bundle).toContain("name: agentclash-cli-setup");
  });
});
