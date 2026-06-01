import { withAuth } from "@workos-inc/authkit-nextjs";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import {
  JsonLd,
  SITE_URL,
  faqSchema,
  organizationSchema,
  productSchema,
  websiteSchema,
} from "@/components/marketing/json-ld";
import { HOME_FAQ } from "@/lib/home-faq";
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

export default async function RootPage() {
  const { user } = await withAuth();
  if (user) redirect("/dashboard");
  return (
    <>
      <JsonLd
        id="agentclash-homepage-schema"
        data={[
          softwareApplicationSchema,
          organizationSchema(),
          websiteSchema(),
          faqSchema(HOME_FAQ),
        ]}
      />
      <HomePage />
    </>
  );
}
