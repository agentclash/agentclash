import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import HtmlSitemapPage, { metadata } from "./sitemap/page";

vi.mock("@/lib/blog", () => ({
  getAllPosts: vi.fn(() => [
    {
      slug: "ai-agent-evaluation-regression-testing",
      title: "AI Agent Evaluation and Regression Testing",
      description: "Evaluate AI agents on real tasks.",
      date: "2026-05-07",
      author: "AgentClash",
    },
  ]),
}));

vi.mock("@/lib/docs", () => ({
  DOCS_NAV: [
    {
      title: "Getting started",
      description: "Start running AgentClash.",
      items: [
        {
          title: "Quickstart",
          description: "Run your first AgentClash eval.",
          href: "/docs/getting-started/quickstart",
        },
      ],
    },
  ],
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

describe("HTML sitemap page", () => {
  it("has canonical metadata", () => {
    expect(metadata).toMatchObject({
      title: "Sitemap - AgentClash",
      alternates: {
        canonical: "/sitemap",
      },
      openGraph: {
        url: "/sitemap",
      },
    });
  });

  it("links core pages, docs, and blog posts for crawler discovery", () => {
    render(<HtmlSitemapPage />);

    const links = Array.from(container?.querySelectorAll("a") ?? []).map(
      (link) => link.getAttribute("href"),
    );

    expect(links).toEqual(
      expect.arrayContaining([
        "/",
        "/platform/agent-evaluation",
        "/platform/agent-regression-testing",
        "/agent-evals",
        "/features/challenge-packs",
        "/docs",
        "/blog",
        "/why",
        "/team",
        "/llms.txt",
        "/llms-full.txt",
        "/docs/getting-started/quickstart",
        "/blog/ai-agent-evaluation-regression-testing",
      ]),
    );
  });
});
