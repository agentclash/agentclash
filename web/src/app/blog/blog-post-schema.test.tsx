import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SITE_URL } from "@/components/marketing/json-ld";
import BlogPostPage from "./[slug]/page";

vi.mock("next-mdx-remote/rsc", () => ({
  MDXRemote: ({ source }: { source: string }) => <div>{source}</div>,
}));

vi.mock("@/lib/blog", () => ({
  getAllSlugs: vi.fn(() => ["fixture-post"]),
  getPostBySlug: vi.fn(() => ({
    slug: "fixture-post",
    title: "Fixture Post",
    date: "2026-05-08",
    description: "Fixture description.",
    author: "AgentClash",
    content: "Fixture content.",
  })),
}));

let root: Root | null = null;
let container: HTMLDivElement | null = null;

function render(element: React.ReactElement) {
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => {
    root?.render(element);
  });
}

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  root = null;
  container = null;
});

describe("blog post structured data", () => {
  it("renders breadcrumb and BlogPosting JSON-LD", async () => {
    const page = await BlogPostPage({
      params: Promise.resolve({ slug: "fixture-post" }),
    });

    render(page);

    const script = container?.querySelector<HTMLScriptElement>(
      "#agentclash-blog-fixture-post-schema",
    );
    expect(script?.type).toBe("application/ld+json");

    const jsonLd = JSON.parse(script?.textContent ?? "[]") as Array<
      Record<string, unknown>
    >;

    expect(jsonLd.map((item) => item["@type"])).toEqual([
      "BreadcrumbList",
      "BlogPosting",
    ]);
    expect(jsonLd[0]).toMatchObject({
      "@type": "BreadcrumbList",
      itemListElement: [
        {
          position: 1,
          name: "Home",
          item: `${SITE_URL}/`,
        },
        {
          position: 2,
          name: "Blog",
          item: `${SITE_URL}/blog`,
        },
        {
          position: 3,
          name: "Fixture Post",
          item: `${SITE_URL}/blog/fixture-post`,
        },
      ],
    });
    expect(jsonLd[1]).toMatchObject({
      "@type": "BlogPosting",
      headline: "Fixture Post",
      url: `${SITE_URL}/blog/fixture-post`,
    });
  });
});
