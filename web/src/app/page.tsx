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

// Homepage title/meta lead with the category buyers search, not just the
// brand. Brand-only titles give Google no reason to rank for "ai agent
// evals", "agent benchmark", "agent workflow testing", "langsmith
// alternative", etc. Keep the title under ~60 chars so it isn't truncated.
const HOME_TITLE = "AgentClash - Open-source AI agent evals & benchmarks";
const HOME_DESCRIPTION =
  "Build, replay, score, and regression-test AI agents with open-source evals, traces, benchmarks, and CI gates. A LangSmith alternative for agent workflow testing and prompt regression testing.";

export const metadata: Metadata = {
  title: HOME_TITLE,
  description: HOME_DESCRIPTION,
  alternates: {
    canonical: "/",
  },
  keywords: [
    "AI agent evals",
    "agent evaluation",
    "LLM agent evaluation",
    "agent benchmark",
    "agent workflow testing",
    "prompt regression testing",
    "agent regression testing",
    "agent testing CI",
    "agent drift",
    "trace replay",
    "agent observability open source",
    "evals for AI agents",
    "LangSmith alternative",
    "open-source agent evals",
    "coding agent benchmark",
  ],
  openGraph: {
    title: HOME_TITLE,
    description: HOME_DESCRIPTION,
    url: "/",
    type: "website",
    locale: "en_US",
    siteName: "AgentClash",
  },
  twitter: {
    card: "summary_large_image",
    title: HOME_TITLE,
    description: HOME_DESCRIPTION,
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
      "Open-source AI agent evaluation platform for finding agent failures on real tasks, replaying every step, promoting regressions, and gating releases on scorecards.",
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
