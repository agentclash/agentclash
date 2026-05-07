import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { JsonLd } from "@/components/marketing/json-ld";
import HomePage from "./landing";

const SITE_URL = "https://www.agentclash.dev";

const softwareApplicationSchema = {
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  name: "AgentClash",
  alternateName: "Agent Clash",
  applicationCategory: "DeveloperApplication",
  applicationSubCategory: "AI agent evaluation platform",
  operatingSystem: "Web, macOS, Linux, Windows",
  description:
    "Open-source AI agent evaluation platform for racing agents head-to-head on real tasks with sandboxed tools, replay, scorecards, and CI regression gates.",
  url: SITE_URL,
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

export default async function RootPage() {
  const { user } = await withAuth();
  if (user) redirect("/dashboard");
  return (
    <>
      <JsonLd
        id="agentclash-homepage-schema"
        data={[softwareApplicationSchema, organizationSchema]}
      />
      <HomePage />
    </>
  );
}
