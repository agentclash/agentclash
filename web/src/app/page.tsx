import { withAuth } from "@workos-inc/authkit-nextjs";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { JsonLd, SITE_URL, productSchema } from "@/components/marketing/json-ld";
import { AGENT_EVALUATION_FEATURES } from "@/lib/seo-features";
import HomePage from "./landing";

export const metadata: Metadata = {
  alternates: {
    canonical: "/",
  },
};

const softwareApplicationSchema = productSchema({
  name: "AgentClash",
  description:
    "Open-source AI agent evaluation platform for racing agents head-to-head on real tasks with sandboxed tools, replay, scorecards, and CI regression gates.",
  url: SITE_URL,
  applicationSubCategory: "AI agent evaluation platform",
  softwareVersion: "beta",
  featureList: AGENT_EVALUATION_FEATURES,
});

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
