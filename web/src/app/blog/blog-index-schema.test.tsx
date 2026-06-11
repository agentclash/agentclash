import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SITE_URL } from "@/components/marketing/json-ld";
import BlogPage from "./page";

vi.mock("@/components/marketing/marketing-shell", () => ({
  MarketingShell: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
}));

vi.mock("@/components/marketing/research-audience-cta", () => ({
  ResearchAudienceCTA: () => <div data-testid="research-audience-cta" />,
}));

vi.mock("@/lib/blog", () => ({
  getAllPosts: vi.fn(() => [
    {
      slug: "agent-evaluation",
      title: "Agent Evaluation",
      date: "2026-05-08",
      description: "Evaluate agents on real tasks.",
      author: "AgentClash",
    },
  ]),
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

describe("blog index structured data", () => {
  it("renders Blog and ItemList JSON-LD for the post index", () => {
    render(<BlogPage />);

    const script = container?.querySelector<HTMLScriptElement>(
      "#agentclash-blog-index-schema",
    );
    expect(script?.type).toBe("application/ld+json");

    const jsonLd = JSON.parse(script?.textContent ?? "[]") as Array<
      Record<string, unknown>
    >;

    expect(jsonLd.map((item) => item["@type"])).toEqual(["Blog", "ItemList"]);
    expect(jsonLd[0]).toMatchObject({
      "@type": "Blog",
      name: "AgentClash Blog",
      url: `${SITE_URL}/blog`,
    });
    expect(jsonLd[1]).toMatchObject({
      "@type": "ItemList",
      numberOfItems: 1,
      itemListElement: [
        {
          "@type": "ListItem",
          position: 1,
          name: "Agent Evaluation",
          description: "Evaluate agents on real tasks.",
          url: `${SITE_URL}/blog/agent-evaluation`,
          item: {
            "@id": `${SITE_URL}/blog/agent-evaluation`,
          },
        },
      ],
    });
  });
});
