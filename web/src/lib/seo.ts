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

export const benchmarkRssAlternate = {
  "application/rss+xml": [
    {
      title: "AgentClash Model Benchmarks",
      url: "/benchmarks/feed.xml",
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

// Webmaster-verification metadata (Google Search Console + Bing Webmaster).
// Tokens are public (they live in the HTML <head>) and set via NEXT_PUBLIC_ env
// so they're inlined at build time. Returns a Next `verification` object, or
// undefined when neither token is set — so the <head> gets no meta tag and the
// build never crashes. See docs/frontend/seo-verification.md to activate.
export function webmasterVerification(): Metadata["verification"] | undefined {
  const google = process.env.NEXT_PUBLIC_GSC_VERIFICATION;
  const bing = process.env.NEXT_PUBLIC_BING_VERIFICATION;
  if (!google && !bing) return undefined;
  return {
    ...(google ? { google } : {}),
    ...(bing ? { other: { "msvalidate.01": bing } } : {}),
  };
}
