import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SITE_URL } from "@/components/marketing/json-ld";
import DocsPage, { docsSchemaId } from "./[[...slug]]/page";

vi.mock("next-mdx-remote/rsc", () => ({
  MDXRemote: ({ source }: { source: string }) => <div>{source}</div>,
}));

vi.mock("@/components/docs/docs-shell", () => ({
  DocsShell: ({ children }: { children: React.ReactNode }) => (
    <section>{children}</section>
  ),
}));

vi.mock("@/components/docs/mdx-components", () => ({
  docsMDXComponents: {},
}));

vi.mock("@/lib/docs", () => ({
  DOCS_NAV: [],
  getAllDocSlugs: vi.fn(() => [["getting-started", "quickstart"]]),
  getDocBySlug: vi.fn(() => ({
    href: "/docs/getting-started/quickstart",
    title: "Quickstart",
    description: "Run your first AgentClash eval.",
    sectionTitle: "Getting Started",
    headings: [],
    content: "Fixture docs content.",
  })),
  getDocNeighbors: vi.fn(() => ({})),
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

describe("docs page structured data", () => {
  it("normalizes trailing slashes in JSON-LD script ids", () => {
    expect(docsSchemaId("/docs/api/reference/")).toBe(
      "agentclash-docs-api-reference-schema",
    );
    expect(docsSchemaId("/docs/")).toBe("agentclash-docs-home-schema");
  });

  it("renders breadcrumb and TechArticle JSON-LD", async () => {
    const page = await DocsPage({
      params: Promise.resolve({ slug: ["getting-started", "quickstart"] }),
    });

    render(page);

    const script = container?.querySelector<HTMLScriptElement>(
      "#agentclash-docs-getting-started-quickstart-schema",
    );
    expect(script?.type).toBe("application/ld+json");

    const jsonLd = JSON.parse(script?.textContent ?? "[]") as Array<
      Record<string, unknown>
    >;

    expect(jsonLd.map((item) => item["@type"])).toEqual([
      "BreadcrumbList",
      "TechArticle",
    ]);
    expect(jsonLd[0]).toMatchObject({
      "@type": "BreadcrumbList",
      itemListElement: [
        {
          "@type": "ListItem",
          position: 1,
          name: "Home",
          item: `${SITE_URL}/`,
        },
        {
          "@type": "ListItem",
          position: 2,
          name: "Docs",
          item: `${SITE_URL}/docs`,
        },
        {
          "@type": "ListItem",
          position: 3,
          name: "Quickstart",
          item: `${SITE_URL}/docs/getting-started/quickstart`,
        },
      ],
    });
    expect(jsonLd[1]).toMatchObject({
      "@type": "TechArticle",
      headline: "Quickstart",
      url: `${SITE_URL}/docs/getting-started/quickstart`,
    });
  });
});
