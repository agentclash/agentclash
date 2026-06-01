import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ChangelogPeriodDetail } from "@/components/marketing/changelog/changelog-period-detail";
import { ChangelogShell } from "@/components/marketing/changelog/changelog-shell";
import { JsonLd, breadcrumbSchema } from "@/components/marketing/json-ld";
import {
  getAllChangelogPeriodSlugs,
  getChangelogPeriodBySlug,
  getChangelogPeriodHref,
  getChangelogPeriodPullRequests,
  getChangelogPullRequestUrl,
} from "@/lib/changelog";
import { ogImageUrl } from "@/lib/seo";

type Props = {
  params: Promise<{ slug: string }>;
};

export function generateStaticParams() {
  return getAllChangelogPeriodSlugs().map((slug) => ({ slug }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const period = getChangelogPeriodBySlug(slug);
  if (!period) return {};

  const title = `${period.label} — AgentClash Changelog`;
  const description = period.summary;
  const canonical = getChangelogPeriodHref(period.id);
  const ogImage = ogImageUrl({
    title: period.headline,
    subtitle: period.label,
    kind: "Changelog",
  });

  return {
    title,
    description,
    alternates: { canonical },
    openGraph: {
      title,
      description,
      url: canonical,
      type: "article",
      locale: "en_US",
      siteName: "AgentClash",
      publishedTime: period.startDate,
      modifiedTime: period.endDate,
      images: [
        {
          url: ogImage,
          width: 1200,
          height: 630,
          alt: `${period.headline} — AgentClash changelog`,
        },
      ],
    },
    twitter: {
      card: "summary_large_image",
      title,
      description,
      images: [{ url: ogImage, alt: `${period.headline} — AgentClash changelog` }],
    },
  };
}

export default async function ChangelogPeriodPage({ params }: Props) {
  const { slug } = await params;
  const period = getChangelogPeriodBySlug(slug);
  if (!period) notFound();

  const pullRequests = getChangelogPeriodPullRequests(period.id);
  const periodUrl = getChangelogPeriodHref(period.id);

  return (
    <>
      <JsonLd
        id={`agentclash-changelog-${period.id}-schema`}
        data={[
          breadcrumbSchema([
            { name: "Home", url: "/" },
            { name: "Changelog", url: "/changelog" },
            { name: period.label, url: periodUrl },
          ]),
          {
            "@context": "https://schema.org",
            "@type": "Article",
            headline: period.headline,
            description: period.summary,
            url: `https://www.agentclash.dev${periodUrl}`,
            datePublished: period.startDate,
            dateModified: period.endDate,
            author: {
              "@type": "Organization",
              name: "AgentClash",
            },
            isPartOf: {
              "@type": "WebPage",
              name: "AgentClash Changelog",
              url: "https://www.agentclash.dev/changelog",
            },
            ...(pullRequests.length
              ? {
                  citation: pullRequests.slice(0, 12).map((pullRequest) => ({
                    "@type": "CreativeWork",
                    name: pullRequest.title,
                    url: getChangelogPullRequestUrl(pullRequest.number),
                  })),
                }
              : {}),
          },
        ]}
      />
      <ChangelogShell>
        <ChangelogPeriodDetail period={period} />
      </ChangelogShell>
    </>
  );
}
