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
            alt: "Quickstart — Run your first AgentClash eval.",
          },
        ],
      },
      twitter: {
        card: "summary_large_image",
        title: "Quickstart — AgentClash Docs",
        description: "Run your first AgentClash eval.",
        images: [
          {
            url: "/twitter-image.png",
            alt: "Quickstart — Run your first AgentClash eval.",
          },
        ],
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

  it("falls back to title-only social image alt when description is empty", async () => {
    getDocBySlugMock.mockReturnValue({
      href: "/docs/reference/cli",
      title: "CLI",
      description: "",
    });

    const metadata = await generateMetadata({
      params: Promise.resolve({
        slug: ["reference", "cli"],
      }),
    });
    const openGraph = metadata.openGraph as {
      images?: Array<{ alt?: string }>;
    };
    const twitter = metadata.twitter as {
      images?: Array<{ alt?: string }>;
    };

    expect(openGraph.images?.[0]?.alt).toBe("CLI");
    expect(twitter.images?.[0]?.alt).toBe("CLI");
  });

  it("falls back to title-only social image alt when description is missing", async () => {
    getDocBySlugMock.mockReturnValue({
      href: "/docs/reference/config",
      title: "Config",
    });

    const metadata = await generateMetadata({
      params: Promise.resolve({
        slug: ["reference", "config"],
      }),
    });
    const openGraph = metadata.openGraph as {
      images?: Array<{ alt?: string }>;
    };
    const twitter = metadata.twitter as {
      images?: Array<{ alt?: string }>;
    };

    expect(openGraph.images?.[0]?.alt).toBe("Config");
    expect(twitter.images?.[0]?.alt).toBe("Config");
  });
});
