import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import HomePage from "./landing";
import AgentEvaluationPage from "./platform/agent-evaluation/page";
import DataSmithPage from "./platform/datasmith/page";
import { MarketingFooter } from "@/components/marketing/marketing-footer";
import { BenchmarksHubContent } from "@/components/marketing/benchmarks/benchmarks-hub-content";
import { MDXRemote } from "next-mdx-remote/rsc";
import { getDocBySlug } from "@/lib/docs";

vi.mock("@/components/marketing/track-box", () => ({ TrackBox: () => null }));
vi.mock("@/components/marketing/luminous-grid", () => ({ LuminousGrid: ({ children }: { children: React.ReactNode }) => <div>{children}</div> }));

vi.mock("@/components/marketing/expanded-cards-block", () => ({ ExpandedCardsBlock: () => null }));

vi.mock("@lobehub/icons", () => Object.fromEntries(["Anthropic", "Gemini", "Mistral", "OpenAI", "OpenRouter", "XAI"].map((name) => [name, Object.assign(() => null, { Color: () => null })])));

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAuth: () => ({ user: null, loading: false }),
}));

function links(element: React.ReactElement) {
  const document = new DOMParser().parseFromString(renderToStaticMarkup(element), "text/html");
  return new Set(Array.from(document.querySelectorAll("a[href]"), (link) => link.getAttribute("href")));
}

describe("public navigation in initial HTML", () => {
  it("exposes collection, product and tool entry points on the homepage", () => {
    const hrefs = links(<HomePage />);
    for (const href of ["/features", "/use-cases", "/industries", "/glossary", "/platform/agent-evaluation", "/platform/datasmith", "/agent-opportunity", "/tryouts", "/publications"]) {
      expect(hrefs.has(href), href).toBe(true);
    }
  });

  it("connects related evaluation, data and publication content", () => {
    expect(links(<AgentEvaluationPage />).has("/ai-agent-testing")).toBe(true);
    expect(links(<DataSmithPage />).has("/trace-to-dataset")).toBe(true);
    expect(links(<BenchmarksHubContent published={[]} />).has("/publications")).toBe(true);
    expect(links(<MarketingFooter />).has("/publications")).toBe(true);
  });

  it("links from docs to both Vibe guides and resolves their destinations", async () => {
    const index = getDocBySlug([])!;
    const guide = getDocBySlug(["guides", "vibe-evals-existing-agent"]);
    const dataUse = getDocBySlug(["vibe-evals", "data-use"]);
    expect(guide).not.toBeNull();
    expect(dataUse).not.toBeNull();
    const indexLinks = links(await MDXRemote({ source: index.content }));
    expect(indexLinks.has("/docs/guides/vibe-evals-existing-agent")).toBe(true);
    expect(indexLinks.has("/docs/vibe-evals/data-use")).toBe(true);
    expect(links(await MDXRemote({ source: guide!.content })).has("/docs/vibe-evals/data-use")).toBe(true);
  });
});
