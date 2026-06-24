import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import { SeoLandingPage } from "@/components/marketing/seo-landing-page";
import { createSeoPageMetadata } from "@/lib/seo-pages/factory";
import { getSeoPageByPath } from "@/lib/seo-pages";

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

  it("locks canonical metadata for a feature page", () => {
    const config = getSeoPageByPath("/features/eval-packs");
    expect(config).toBeDefined();

    expect(createSeoPageMetadata(config!)).toMatchObject({
      alternates: {
        canonical: "/features/eval-packs",
      },
      openGraph: {
        url: "/features/eval-packs",
      },
    });
  });
});
