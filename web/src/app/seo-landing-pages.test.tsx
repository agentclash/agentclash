import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SeoLandingPage } from "@/components/marketing/seo-landing-page";
import { createSeoPageMetadata } from "@/lib/seo-pages/factory";
import { getSeoPageByPath, SEO_PAGE_REGISTRY } from "@/lib/seo-pages";
import { FactualPage, factualMetadata } from "@/components/marketing/factual-page";
import { FACTUAL_PAGES } from "@/lib/factual-pages";
import { resolvePublicContent } from "@/lib/public-content";

vi.mock("@/components/marketing/marketing-shell", () => ({
  MarketingShell: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
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

function getJsonLd(id: string) {
  const script = container?.querySelector<HTMLScriptElement>(`#${id}`);
  expect(script?.type).toBe("application/ld+json");
  return JSON.parse(script?.textContent ?? "[]") as Array<Record<string, unknown>>;
}

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  root = null;
  container = null;
});

describe("SEO landing pages", () => {
  it("emits structured data for a tier-S page", () => {
    const config = getSeoPageByPath("/agent-evals");
    expect(config).toBeDefined();
    render(<SeoLandingPage config={config!} />);

    const jsonLd = getJsonLd("agentclash-agent-evals-schema");
    expect(jsonLd.map((item) => item["@type"])).toEqual([
      "BreadcrumbList",
      "FAQPage",
      "SoftwareApplication",
    ]);
  });

  it("renders the page-specific example, limitation and section headings", () => {
    const config = getSeoPageByPath("/agent-evals")!;
    render(<SeoLandingPage config={config} />);
    expect(container?.textContent).toContain(config.editorial.example);
    expect(container?.textContent).toContain(config.editorial.limitation);
    expect([...container!.querySelectorAll("h2")].map((node) => node.textContent)).toContain(config.proofSectionTitle);
    expect(container?.textContent).toContain("illustrative example");
  });

  it("renders each page's distinct editorial content", () => {
    for (const config of SEO_PAGE_REGISTRY) {
      render(<SeoLandingPage config={config} />);
      expect(container?.textContent, config.path).toContain(config.editorial.title);
      expect(container?.textContent, config.path).toContain(config.editorial.example);
      expect(container?.textContent, config.path).toContain(config.editorial.limitation);
      act(() => root?.unmount());
      container?.remove();
      root = null;
      container = null;
    }
  });

  it.each(Object.values(FACTUAL_PAGES))("renders $path with matching Markdown and canonical metadata", (page) => {
    render(<FactualPage page={page} />);
    const markdown = resolvePublicContent(page.path)?.renderMarkdown();
    for (const section of page.sections) {
      expect(container?.textContent).toContain(section.body);
      expect(markdown).toContain(section.body);
    }
    expect(factualMetadata(page).alternates?.canonical).toBe(page.path);
    expect(container?.querySelector('[type="application/ld+json"]')).toBeNull();
  });

  it("locks canonical metadata for a feature page", () => {
    const config = getSeoPageByPath("/features/challenge-packs");
    expect(config).toBeDefined();

    expect(createSeoPageMetadata(config!)).toMatchObject({
      alternates: {
        canonical: "/features/challenge-packs",
      },
      openGraph: {
        url: "/features/challenge-packs",
      },
    });
  });
});
