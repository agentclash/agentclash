import type { Metadata } from "next";

type AlternateTypes = NonNullable<NonNullable<Metadata["alternates"]>["types"]>;

export const blogRssAlternate = {
  "application/rss+xml": [
    {
      title: "AgentClash Blog",
      url: "/feed.xml",
    },
  ],
} satisfies AlternateTypes;

// Root-relative URL for the dynamic Open Graph image route (app/og/route.tsx).
// metadataBase upgrades it to absolute in the rendered OG/Twitter tags, so each
// page gets a tailored social card instead of one shared static image.
export function ogImageUrl({
  title,
  subtitle,
  kind,
}: {
  title: string;
  subtitle?: string;
  kind?: string;
}): string {
  const search = new URLSearchParams({ title });
  if (subtitle) search.set("subtitle", subtitle);
  if (kind) search.set("kind", kind);
  return `/og?${search.toString()}`;
}
