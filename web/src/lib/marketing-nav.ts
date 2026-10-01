export type MarketingNavLink = {
  href: string;
  label: string;
  external?: boolean;
};

export const DEFAULT_MARKETING_NAV: MarketingNavLink[] = [
  ...(process.env.NEXT_PUBLIC_VIBE_EVALS_ENABLED === "true" ? [{ href: "/vibe-evals", label: "Try Vibe Evals" }] : []),
  { href: "/#features", label: "Features" },
  { href: "/enterprise", label: "Enterprise" },
  { href: "/pricing", label: "Pricing" },
  { href: "/docs", label: "Docs" },
  { href: "/benchmarks", label: "Benchmarks" },
  { href: "/compare", label: "Compare" },
  { href: "/blog", label: "Blog" },
  { href: "/changelog", label: "Changelog" },
];
