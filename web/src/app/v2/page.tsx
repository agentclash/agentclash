import type { Metadata } from "next";
import HomePage from "./landing";
import { JsonLd } from "@/components/marketing/json-ld";

const SITE_URL = "https://agentclash.dev";

const softwareApplicationSchema = {
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  name: "AgentClash",
  alternateName: "Agent Clash",
  applicationCategory: "DeveloperApplication",
  applicationSubCategory: "AI agent evaluation platform",
  operatingSystem: "Web, macOS, Linux, Windows",
  description:
    "Open-source AI agent evaluation platform. Race models head-to-head on real tasks with the same tools, same constraints, and live scoring. Wire into CI to catch regressions before you ship.",
  url: `${SITE_URL}/v2`,
  softwareVersion: "beta",
  offers: {
    "@type": "Offer",
    price: "0",
    priceCurrency: "USD",
  },
};

const organizationSchema = {
  "@context": "https://schema.org",
  "@type": "Organization",
  name: "AgentClash",
  alternateName: "Agent Clash",
  url: SITE_URL,
  logo: `${SITE_URL}/icon.svg`,
  sameAs: [
    "https://github.com/agentclash/agentclash",
    "https://www.npmjs.com/package/agentclash",
  ],
};

export const metadata: Metadata = {
  title: "AgentClash — Head-to-head AI agent evaluation & regression testing",
  description:
    "Open-source AI agent evaluation platform. Race models head-to-head on real tasks — same tools, same constraints, live scoring. Wire into CI to catch regressions before you ship.",
  openGraph: {
    title: "AgentClash — AI agent evaluation platform",
    description:
      "Race AI agents head-to-head on real tasks. Open source. CI-native. Regressions never ship again.",
    url: `${SITE_URL}/v2`,
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "AgentClash — AI agent evaluation platform",
    description:
      "Race AI agents head-to-head on real tasks. Open source. CI-native. Regressions never ship again.",
  },
  alternates: {
    canonical: "/v2",
  },
};

export default function V2RootPage() {
  return (
    <>
      <JsonLd id="ld-software-application" data={softwareApplicationSchema} />
      <JsonLd id="ld-organization" data={organizationSchema} />
      <HomePage />
    </>
  );
}
