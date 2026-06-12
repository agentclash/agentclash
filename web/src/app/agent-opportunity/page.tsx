import type { Metadata } from "next";
import { JsonLd, breadcrumbSchema } from "@/components/marketing/json-ld";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { ogImageUrl } from "@/lib/seo";
import { AgentOpportunityClient } from "./agent-opportunity-client";

const PAGE_PATH = "/agent-opportunity";
const PAGE_TITLE = "AI Agent Opportunity Report | AgentClash";
const PAGE_DESCRIPTION =
  "Enter a company URL and get an honest AI agent opportunity report with ROI ranges, risks, and an AgentClash evaluation pack.";

export const metadata: Metadata = {
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
  alternates: { canonical: PAGE_PATH },
  openGraph: {
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    url: PAGE_PATH,
    type: "website",
    locale: "en_US",
    siteName: "AgentClash",
    images: [
      {
        url: ogImageUrl({
          title: "Should your company have an AI agent?",
          subtitle: "ROI, risk, and eval plan from a URL",
          kind: "Report",
        }),
        width: 1200,
        height: 630,
        alt: PAGE_TITLE,
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
  },
};

export default function AgentOpportunityPage() {
  return (
    <MarketingShell>
      <JsonLd
        id="agent-opportunity-breadcrumb-schema"
        data={[
          breadcrumbSchema([
            { name: "Home", url: "/" },
            { name: "AI Agent Opportunity Report", url: PAGE_PATH },
          ]),
        ]}
      />
      <AgentOpportunityClient />
    </MarketingShell>
  );
}
