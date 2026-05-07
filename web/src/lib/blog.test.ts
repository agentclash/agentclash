import { describe, expect, it } from "vitest";
import { getAllPosts, getPostBySlug, parseBlogPost } from "./blog";

const SLUG = "ai-agent-evaluation-regression-testing";

describe("blog content", () => {
  it("loads the AI agent evaluation regression testing metadata", () => {
    const post = getPostBySlug(SLUG);

    expect(post).not.toBeNull();
    expect(post?.title).toBe(
      "AI Agent Evaluation Needs Regression Testing, Not Just Benchmarks",
    );
    expect(post?.description).toContain("AI agent evaluation");
    expect(post?.description).toContain("CI regression gates");
  });

  it("loads internal links from the AI agent evaluation post", () => {
    const post = getPostBySlug(SLUG);

    expect(post).not.toBeNull();
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
  });

  it("indexes the AI agent evaluation post with publish metadata", () => {
    const indexedPost = getAllPosts().find((candidate) => candidate.slug === SLUG);

    expect(indexedPost).toMatchObject({
      slug: SLUG,
      date: "2026-05-07",
      author: "Atharva",
    });
  });

  it("ignores malformed frontmatter when parsing a post", () => {
    expect(parseBlogPost("broken", "---\ntitle: [\n---\ncontent")).toBeNull();
  });

  it("ignores posts missing required metadata", () => {
    expect(
      parseBlogPost(
        "missing-author",
        [
          "---",
          "title: Missing Author",
          "date: 2026-05-07",
          "description: Missing required author field.",
          "---",
          "content",
        ].join("\n"),
      ),
    ).toBeNull();
  });
});
