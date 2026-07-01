import type { Metadata } from "next";
import { describe, expect, it, vi } from "vitest";
import { metadata as homeMetadata } from "./page";
import { metadata as blogMetadata } from "./blog/page";
import { generateMetadata as generateBlogPostMetadata } from "./blog/[slug]/page";
import { generateMetadata as generateBenchmarksIndexMetadata } from "./benchmarks/page";
import { generateMetadata as generateBenchmarkMetadata } from "./benchmarks/[slug]/page";
import { generateMetadata as generateDocsMetadata } from "./docs/[[...slug]]/page";
import { metadata as enterpriseMetadata } from "./enterprise/page";
import { metadata as evalChecklistMetadata } from "./resources/eval-checklist/page";
import { metadata as servicesMetadata } from "./services/page";
import { metadata as tryoutsMetadata } from "./tryouts/page";
import { metadata as agentOpportunityMetadata } from "./agent-opportunity/page";
import { metadata as agentEvaluationMetadata } from "./platform/agent-evaluation/page";
import { metadata as agentRegressionTestingMetadata } from "./platform/agent-regression-testing/page";
import { metadata as datasmithMetadata } from "./platform/datasmith/page";
import { metadata as sitemapMetadata } from "./sitemap/page";
import { teamMetadata } from "./team/metadata";
import { whyMetadata } from "./why/metadata";
import { changelogMetadata } from "./changelog/metadata";

const getPostBySlugMock = vi.hoisted(() => vi.fn());
const getDocBySlugMock = vi.hoisted(() => vi.fn());

vi.mock("@workos-inc/authkit-nextjs", () => ({
  withAuth: vi.fn(),
}));

vi.mock("./landing", () => ({
  default: () => null,
}));

vi.mock("next-mdx-remote/rsc", () => ({
  MDXRemote: () => null,
}));

vi.mock("@/lib/blog", () => ({
  getAllPosts: vi.fn(() => []),
  getAllSlugs: vi.fn(() => []),
  getPostBySlug: getPostBySlugMock,
}));

const getReportBySlugMock = vi.hoisted(() => vi.fn());

vi.mock("@/lib/benchmarks", () => ({
  getAllReports: vi.fn(() => []),
  getAllSlugs: vi.fn(() => []),
  getReportBySlug: getReportBySlugMock,
  hasPublishedBenchmarks: vi.fn(() => false),
}));

vi.mock("@/lib/docs", () => ({
  DOCS_NAV: [],
  getAllDocSlugs: vi.fn(() => []),
  getDocBySlug: getDocBySlugMock,
  getDocNeighbors: vi.fn(() => ({})),
}));

function expectCanonical(metadata: Metadata, canonical: string) {
  expect(metadata.alternates).toMatchObject({ canonical });
}

describe("public canonical metadata", () => {
  it("locks static public page canonicals", () => {
    expectCanonical(homeMetadata, "/");
    expectCanonical(blogMetadata, "/blog");
    expectCanonical(generateBenchmarksIndexMetadata(), "/benchmarks");
    expectCanonical(enterpriseMetadata, "/enterprise");
    expectCanonical(evalChecklistMetadata, "/resources/eval-checklist");
    expectCanonical(servicesMetadata, "/services");
    expectCanonical(tryoutsMetadata, "/tryouts");
    expectCanonical(agentOpportunityMetadata, "/agent-opportunity");
    expectCanonical(agentEvaluationMetadata, "/platform/agent-evaluation");
    expectCanonical(
      agentRegressionTestingMetadata,
      "/platform/agent-regression-testing",
    );
    expectCanonical(datasmithMetadata, "/platform/datasmith");
    expectCanonical(whyMetadata, "/why");
    expectCanonical(teamMetadata, "/team");
    expectCanonical(changelogMetadata, "/changelog");
    expectCanonical(sitemapMetadata, "/sitemap");
  });

  it("locks blog post canonical metadata", async () => {
    getPostBySlugMock.mockReturnValue({
      slug: "fixture-post",
      title: "Fixture Post",
      date: "2026-05-08",
      description: "Fixture description.",
      author: "AgentClash",
      content: "",
    });

    const metadata = await generateBlogPostMetadata({
      params: Promise.resolve({ slug: "fixture-post" }),
    });

    expectCanonical(metadata, "/blog/fixture-post");
  });

  it("locks benchmark report canonical metadata", async () => {
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

    const metadata = await generateBenchmarkMetadata({
      params: Promise.resolve({ slug: "fixture-report" }),
    });

    expectCanonical(metadata, "/benchmarks/fixture-report");
  });

  it("locks docs canonical metadata", async () => {
    getDocBySlugMock.mockReturnValue({
      href: "/docs/getting-started/quickstart",
      title: "Quickstart",
      description: "Run your first AgentClash eval.",
    });

    const metadata = await generateDocsMetadata({
      params: Promise.resolve({ slug: ["getting-started", "quickstart"] }),
    });

    expectCanonical(metadata, "/docs/getting-started/quickstart");
  });

  it("locks docs home canonical metadata", async () => {
    getDocBySlugMock.mockReturnValue({
      href: "/docs",
      title: "AgentClash Docs",
      description: "Documentation for AgentClash.",
    });

    const metadata = await generateDocsMetadata({
      params: Promise.resolve({ slug: [] }),
    });

    expectCanonical(metadata, "/docs");
  });
});
