import { describe, expect, it, vi } from "vitest";
import { generateMetadata } from "./[[...slug]]/page";

const getDocBySlugMock = vi.hoisted(() => vi.fn());

vi.mock("@/lib/docs", () => ({
  DOCS_NAV: [],
  getAllDocSlugs: vi.fn(() => []),
  getDocBySlug: getDocBySlugMock,
  getDocNeighbors: vi.fn(() => ({})),
}));

describe("docs metadata", () => {
  it("adds page-specific Open Graph and Twitter metadata", async () => {
    getDocBySlugMock.mockReturnValue({
      href: "/docs/getting-started/quickstart",
      title: "Quickstart",
      description: "Run your first AgentClash eval.",
    });

    const metadata = await generateMetadata({
      params: Promise.resolve({
        slug: ["getting-started", "quickstart"],
      }),
    });

    expect(metadata).toMatchObject({
      title: "Quickstart — AgentClash Docs",
      description: "Run your first AgentClash eval.",
      alternates: {
        canonical: "/docs/getting-started/quickstart",
      },
      openGraph: {
        title: "Quickstart — AgentClash Docs",
        description: "Run your first AgentClash eval.",
        url: "/docs/getting-started/quickstart",
        type: "website",
        locale: "en_US",
        siteName: "AgentClash",
        images: [
          {
            url: "/og-image.png",
            width: 1200,
            height: 630,
            alt: "AgentClash docs for AI agent evaluation workflows.",
          },
        ],
      },
      twitter: {
        card: "summary_large_image",
        title: "Quickstart — AgentClash Docs",
        description: "Run your first AgentClash eval.",
        images: ["/twitter-image.png"],
      },
    });
  });

  it("returns empty metadata when the doc is missing", async () => {
    getDocBySlugMock.mockReturnValue(null);

    await expect(
      generateMetadata({
        params: Promise.resolve({
          slug: ["missing"],
        }),
      }),
    ).resolves.toEqual({});
  });
});
