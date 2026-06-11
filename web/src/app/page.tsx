import { withAuth } from "@workos-inc/authkit-nextjs";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import {
  JsonLd,
  SITE_URL,
  faqSchema,
  organizationSchema,
  productSchema,
  softwareSourceCodeSchema,
  websiteSchema,
} from "@/components/marketing/json-ld";
import { getChangelogPeriods } from "@/lib/changelog";
import { HOME_FAQ } from "@/lib/home-faq";
import { AGENT_EVALUATION_FEATURES } from "@/lib/seo-features";
import { isReturningVisitor } from "@/lib/auth/returning";
import HomePage from "./landing";

export const metadata: Metadata = {
  alternates: {
    canonical: "/",
  },
};

// Freshness signal for crawlers / AI answer-engines: the start of the most
// recent changelog window (a real, stable past date — when the current batch
// of updates began), clamped to today so a still-open period's forward-looking
// endDate can never emit a future dateModified, which schema.org treats as
// invalid. Date-only (YYYY-MM-DD) is valid for dateModified and avoids churn.
const todayIso = new Date().toISOString().slice(0, 10);
const latestChangelogStart = getChangelogPeriods()[0]?.startDate;
const homepageLastModified =
  latestChangelogStart && latestChangelogStart < todayIso
    ? latestChangelogStart
    : todayIso;

const softwareApplicationSchema = {
  ...productSchema({
    name: "AgentClash",
    description:
      "Open-source AI agent evaluation platform for racing agents head-to-head on real tasks with sandboxed tools, replay, scorecards, and CI regression gates.",
    url: SITE_URL,
    applicationSubCategory: "AI agent evaluation platform",
    softwareVersion: "beta",
    featureList: AGENT_EVALUATION_FEATURES,
  }),
  dateModified: homepageLastModified,
};

export default async function RootPage() {
  const { user } = await withAuth();
  if (user) redirect("/dashboard");
  const returning = await isReturningVisitor();
  return (
    <>
      <JsonLd
        id="agentclash-homepage-schema"
        data={[
          softwareApplicationSchema,
          softwareSourceCodeSchema(),
          organizationSchema(),
          websiteSchema(),
          faqSchema(HOME_FAQ),
        ]}
      />
      <HomePage returning={returning} />
    </>
  );
}
