import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SITE_URL } from "@/components/marketing/json-ld";
import DocsPage from "./[[...slug]]/page";
import { docsSchemaId } from "./docs-schema-id";

const getDocBySlugMock = vi.hoisted(() => vi.fn());

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
  getAllDocSlugs: vi.fn(() => [["getting-started", "quickstart"]]),
  getDocBySlug: getDocBySlugMock,
  getDocNeighbors: vi.fn(() => ({})),
}));

function mockDoc(overrides: Record<string, unknown> = {}) {
  getDocBySlugMock.mockReturnValue({
    href: "/docs/getting-started/quickstart",
    title: "Quickstart",
    description: "Run your first AgentClash eval.",
    sectionTitle: "Getting Started",
    headings: [],
    content: "Fixture docs content.",
    ...overrides,
  });
}

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
    mockDoc();

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
      mainEntityOfPage: {
        "@type": "WebPage",
        "@id": `${SITE_URL}/docs/getting-started/quickstart`,
        url: `${SITE_URL}/docs/getting-started/quickstart`,
      },
    });
  });

  it("renders visible docs home FAQ with matching FAQPage JSON-LD", async () => {
    mockDoc({
      href: "/docs",
      title: "AgentClash Docs",
      description: "Documentation for AgentClash.",
      content: "Docs home content.",
    });

    const page = await DocsPage({
      params: Promise.resolve({ slug: [] }),
    });

    render(page);

    expect(container?.textContent).toContain("Docs FAQ");
    expect(container?.textContent).toContain(
      "Where should I start with AgentClash?",
    );
    expect(container?.textContent).toContain(
      "Can AgentClash docs help with CI agent gates?",
    );
    expect(container?.textContent).toContain(
      "Are the docs available for coding agents?",
    );

    const script = container?.querySelector<HTMLScriptElement>(
      "#agentclash-docs-home-schema",
    );
    const jsonLd = JSON.parse(script?.textContent ?? "[]") as Array<
      Record<string, unknown>
    >;

    expect(jsonLd.map((item) => item["@type"])).toEqual([
      "BreadcrumbList",
      "FAQPage",
    ]);
    expect(jsonLd[1]).toMatchObject({
      "@type": "FAQPage",
      mainEntity: [
        {
          "@type": "Question",
          name: "Where should I start with AgentClash?",
        },
        {
          "@type": "Question",
          name: "Can AgentClash docs help with CI agent gates?",
        },
        {
          "@type": "Question",
          name: "Are the docs available for coding agents?",
        },
      ],
    });
  });
});
