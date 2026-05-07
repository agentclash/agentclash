import { describe, expect, it } from "vitest";
import { getAllPosts, getPostBySlug } from "./blog";

describe("blog content", () => {
  it("indexes the AI agent evaluation regression testing post", () => {
    const post = getPostBySlug("ai-agent-evaluation-regression-testing");
    const indexedPost = getAllPosts().find(
      (candidate) => candidate.slug === "ai-agent-evaluation-regression-testing",
    );

    expect(post?.title).toBe(
      "AI Agent Evaluation Needs Regression Testing, Not Just Benchmarks",
    );
    expect(post?.description).toContain("AI agent evaluation");
    expect(post?.description).toContain("CI regression gates");
    expect(post?.content).toContain(
      "[AI agent evaluation platform](/platform/agent-evaluation)",
    );
    expect(post?.content).toContain(
      "[AI agent regression testing](/platform/agent-regression-testing)",
    );
    expect(post?.content).toContain(
      "[writing a challenge pack](/docs/guides/write-a-challenge-pack)",
    );
    expect(post?.content).toContain(
      "[CI/CD agent gates](/docs/guides/ci-cd-agent-gates)",
    );
    expect(indexedPost).toMatchObject({
      slug: "ai-agent-evaluation-regression-testing",
      date: "2026-05-08",
      author: "Atharva",
    });
  });
});
