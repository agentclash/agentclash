import type { Metadata } from "next";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { AgentPromoBanner } from "@/components/marketing/agent-promo-banner";
import { BenchmarksHubContent } from "@/components/marketing/benchmarks/benchmarks-hub-content";
import { JsonLd, benchmarkHubSchema } from "@/components/marketing/json-ld";
import { getAllReports } from "@/lib/benchmarks";
import { BENCHMARKS_HUB_FAQ } from "@/lib/benchmarks-hub";
import { benchmarkRssAlternate, ogImageUrl } from "@/lib/seo";

const PAGE_TITLE = "AI Agent Benchmarks Hub | Head-to-Head Races | AgentClash";
const PAGE_DESCRIPTION =
  "Public AI agent benchmarks hub: frozen challenge packs, head-to-head races, replay evidence, scorecards, and monthly reports. Run the same benchmark on your agents.";
const SOCIAL_IMAGE = ogImageUrl({
  title: "AI Agent Benchmarks",
  subtitle: "Frozen packs, head-to-head races, replay evidence",
  kind: "Benchmark",
});

export function generateMetadata(): Metadata {
  return {
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    alternates: {
      canonical: "/benchmarks",
      types: benchmarkRssAlternate,
    },
    openGraph: {
      title: PAGE_TITLE,
      description: PAGE_DESCRIPTION,
      url: "/benchmarks",
      type: "website",
      locale: "en_US",
      siteName: "AgentClash",
      images: [{ url: SOCIAL_IMAGE, width: 1200, height: 630, alt: PAGE_TITLE }],
    },
    twitter: {
      card: "summary_large_image",
      title: PAGE_TITLE,
      description: PAGE_DESCRIPTION,
      images: [{ url: SOCIAL_IMAGE, alt: PAGE_TITLE }],
    },
  };
}

export default function BenchmarksPage() {
  const reports = getAllReports();
  const published = reports.filter((report) => !report.sample);

  return (
    <MarketingShell banner={<AgentPromoBanner page="benchmarks" />}>
      <JsonLd
        id="agentclash-benchmarks-hub-schema"
        data={benchmarkHubSchema(
          published.map((report) => ({
            slug: report.slug,
            title: report.title,
            description: report.description,
          })),
          [...BENCHMARKS_HUB_FAQ],
        )}
      />
      <BenchmarksHubContent published={published} />
    </MarketingShell>
  );
}
