import { describe, expect, it, vi } from "vitest";
import { benchmarkRssAlternate, ogImageUrl } from "@/lib/seo";
import { metadata as benchmarksMetadata } from "./page";
import { generateMetadata } from "./[slug]/page";

const getReportBySlugMock = vi.hoisted(() => vi.fn());

vi.mock("@workos-inc/authkit-nextjs", () => ({
  withAuth: vi.fn(),
}));

vi.mock("next-mdx-remote/rsc", () => ({
  MDXRemote: () => null,
}));

vi.mock("@/lib/benchmarks", () => ({
  getAllReports: vi.fn(() => []),
  getAllSlugs: vi.fn(() => []),
  getReportBySlug: getReportBySlugMock,
}));

describe("benchmarks RSS autodiscovery metadata", () => {
  it("links the benchmarks RSS feed from the index metadata", () => {
    expect(benchmarksMetadata.alternates).toMatchObject({
      canonical: "/benchmarks",
      types: benchmarkRssAlternate,
    });
  });

  it("links the benchmarks RSS feed from report metadata", async () => {
    getReportBySlugMock.mockReturnValue({
      slug: "fixture-report",
      title: "Fixture Report",
      date: "2026-06-06",
      description: "Fixture description.",
      author: "AgentClash",
      featuredModel: "Claude Opus 4.8",
      verdict: "Opus won.",
      content: "",
    });

    const metadata = await generateMetadata({
      params: Promise.resolve({ slug: "fixture-report" }),
    });

    expect(metadata.alternates).toMatchObject({
      canonical: "/benchmarks/fixture-report",
      types: benchmarkRssAlternate,
    });
  });
});

describe("benchmarks index social metadata", () => {
  it("adds Open Graph and Twitter card metadata", () => {
    expect(benchmarksMetadata).toMatchObject({
      openGraph: {
        url: "/benchmarks",
        type: "website",
        siteName: "AgentClash",
      },
      twitter: {
        card: "summary_large_image",
      },
    });
  });
});

describe("benchmarks report social metadata", () => {
  it("uses a Benchmark-kind OG image keyed off the verdict", async () => {
    getReportBySlugMock.mockReturnValue({
      slug: "fixture-report",
      title: "Fixture Report",
      date: "2026-06-06",
      description: "Fixture description.",
      author: "AgentClash",
      featuredModel: "Claude Opus 4.8",
      verdict: "Opus won.",
      content: "",
    });

    const metadata = await generateMetadata({
      params: Promise.resolve({ slug: "fixture-report" }),
    });

    const expectedImage = ogImageUrl({
      title: "Fixture Report",
      subtitle: "Opus won.",
      kind: "Benchmark",
    });

    expect(metadata).toMatchObject({
      title: "Fixture Report — AgentClash",
      description: "Fixture description.",
      openGraph: {
        type: "article",
        url: "/benchmarks/fixture-report",
        publishedTime: "2026-06-06",
        authors: ["AgentClash"],
        images: [{ url: expectedImage, width: 1200, height: 630 }],
      },
      twitter: {
        card: "summary_large_image",
        images: [{ url: expectedImage }],
      },
    });
  });

  it("returns empty metadata for an unknown slug", async () => {
    getReportBySlugMock.mockReturnValue(null);
    const metadata = await generateMetadata({
      params: Promise.resolve({ slug: "missing" }),
    });
    expect(metadata).toEqual({});
  });

  it("noindexes a sample report so engines never index illustrative numbers", async () => {
    getReportBySlugMock.mockReturnValue({
      slug: "sample-report",
      title: "Sample Report",
      date: "2026-06-06",
      description: "Illustrative.",
      author: "AgentClash",
      featuredModel: "Claude Opus 4.8",
      verdict: "Representative only.",
      sample: true,
      content: "",
    });

    const metadata = await generateMetadata({
      params: Promise.resolve({ slug: "sample-report" }),
    });

    expect(metadata.robots).toEqual({ index: false, follow: true });
  });

  it("leaves a real (non-sample) report indexable", async () => {
    getReportBySlugMock.mockReturnValue({
      slug: "real-report",
      title: "Real Report",
      date: "2026-06-06",
      description: "Measured.",
      author: "AgentClash",
      featuredModel: "Claude Opus 4.8",
      verdict: "Opus won.",
      sample: false,
      content: "",
    });

    const metadata = await generateMetadata({
      params: Promise.resolve({ slug: "real-report" }),
    });

    expect(metadata.robots).toBeUndefined();
  });
});
