import type { Metadata } from "next";
import {
  JsonLd,
  breadcrumbSchema,
  faqSchema,
  productSchema,
  SITE_URL,
} from "@/components/marketing/json-ld";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { ogImageUrl } from "@/lib/seo";
import { AgentOpportunityClient } from "./agent-opportunity-client";
import {
  AGENT_OPPORTUNITY_DESCRIPTION,
  AGENT_OPPORTUNITY_FAQ,
  AGENT_OPPORTUNITY_KEYWORDS,
  AGENT_OPPORTUNITY_PATH,
  AGENT_OPPORTUNITY_TITLE,
} from "./agent-opportunity-seo";

const SOCIAL_IMAGE = ogImageUrl({
  title: "AI agent ROI calculator",
  subtitle: "Build vs buy assessment from a company URL",
  kind: "Report",
});

export const metadata: Metadata = {
  title: AGENT_OPPORTUNITY_TITLE,
  description: AGENT_OPPORTUNITY_DESCRIPTION,
  keywords: [...AGENT_OPPORTUNITY_KEYWORDS],
  alternates: { canonical: AGENT_OPPORTUNITY_PATH },
  openGraph: {
    title: AGENT_OPPORTUNITY_TITLE,
    description: AGENT_OPPORTUNITY_DESCRIPTION,
    url: AGENT_OPPORTUNITY_PATH,
    type: "website",
    locale: "en_US",
    siteName: "AgentClash",
    images: [
      {
        url: SOCIAL_IMAGE,
        width: 1200,
        height: 630,
        alt: AGENT_OPPORTUNITY_TITLE,
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: AGENT_OPPORTUNITY_TITLE,
    description: AGENT_OPPORTUNITY_DESCRIPTION,
    images: [{ url: SOCIAL_IMAGE, alt: AGENT_OPPORTUNITY_TITLE }],
  },
};

export default function AgentOpportunityPage() {
  return (
    <MarketingShell>
      <JsonLd
        id="agent-opportunity-schema"
        data={[
          breadcrumbSchema([
            { name: "Home", url: "/" },
            {
              name: "AI Agent ROI Calculator",
              url: AGENT_OPPORTUNITY_PATH,
            },
          ]),
          faqSchema([...AGENT_OPPORTUNITY_FAQ]),
          productSchema({
            name: "AgentClash AI Agent Opportunity Scanner",
            description: AGENT_OPPORTUNITY_DESCRIPTION,
            url: `${SITE_URL}${AGENT_OPPORTUNITY_PATH}`,
            applicationSubCategory: "AI agent evaluation",
            featureList: [
              "AI agent ROI ranges from a company URL",
              "Build vs buy AI agent guidance",
              "Agentic AI use case scoring",
              "Risk and eval readiness scorecard",
              "AgentClash evaluation pack recommendations",
            ],
          }),
        ]}
      />
      <AgentOpportunityClient />
    </MarketingShell>
  );
}
