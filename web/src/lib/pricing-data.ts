// Single source of truth for AgentClash pricing.
//
// This module is intentionally framework-free (no React, no "use client") so it
// can be consumed by BOTH the client-side <PricingBlock /> marketing section and
// server-side surfaces — the /pricing page and the machine-readable pricing
// JSON-LD (pricingSchema in json-ld.tsx). Keep prices here and nowhere else so
// the human page and the agent-readable Offers can never drift apart.

export type Period = "monthly" | "yearly";

export type Price = {
  value: string;
  suffix: string;
  note?: string;
};

export type Cta = {
  label: string;
  href: string;
  external?: boolean;
  primary?: boolean;
  sublabel?: string;
};

export type ShaderPalette = {
  colorA: string;
  colorB: string;
};

export type Tier = {
  name: string;
  prices: { monthly: Price; yearly: Price };
  blurb: string;
  cta: Cta;
  features: string[];
  shader: ShaderPalette;
};

export const PRICING_TIERS: Tier[] = [
  {
    name: "Free",
    prices: {
      monthly: { value: "$0", suffix: "/ month" },
      yearly: { value: "$0", suffix: "/ month" },
    },
    blurb:
      "Run real evals first. Upgrade only when you need more runs, retention, or team controls.",
    cta: { label: "Start free", href: "/auth/login?mode=signup" },
    features: [
      "1 workspace",
      "25 eval runs / month",
      "Up to 4 models per run",
      "7-day replay retention",
      "BYOK LLM keys",
      "BYOK sandbox (E2B token)",
      "Community support",
    ],
    shader: { colorA: "#64748b", colorB: "#94a3b8" },
  },
  {
    name: "Pro",
    prices: {
      monthly: {
        value: "$49",
        suffix: "/ month",
        note: "Billed monthly",
      },
      yearly: {
        value: "$39",
        suffix: "/ month",
        note: "Billed annually · $468 / yr",
      },
    },
    blurb:
      "For teams moving from evaluation to repeated release checks.",
    cta: {
      label: "Upgrade to Pro",
      href: "/auth/login?mode=signup&returnTo=/dashboard%3Fplan%3Dpro",
      primary: true,
      sublabel: "Start on Free, pay when you need more",
    },
    features: [
      "Everything in Free, plus:",
      "500 eval runs / workspace / month",
      "Up to 8 models per run",
      "30-day replay retention",
      "Hosted sandbox with included credit",
      "Private challenge packs",
      "CI integration (GitHub Actions, webhooks)",
      "3 concurrent eval runs",
      "Email support, < 1 business day",
    ],
    shader: { colorA: "#a78bfa", colorB: "#ec4899" },
  },
  {
    name: "Team",
    prices: {
      monthly: {
        value: "$100",
        suffix: "/ month",
        note: "Billed monthly",
      },
      yearly: {
        value: "$80",
        suffix: "/ month",
        note: "Billed annually · $960 / yr",
      },
    },
    blurb:
      "For teams running evals across multiple products and surfaces.",
    cta: {
      label: "Upgrade to Team",
      href: "/auth/login?mode=signup&returnTo=/dashboard%3Fplan%3Dteam",
      sublabel: "For higher run volume and governance",
    },
    features: [
      "Everything in Pro, plus:",
      "2,000 eval runs / workspace / month",
      "Up to 12 models per run",
      "90-day replay retention",
      "10 concurrent eval runs",
      "Multiple workspaces",
      "Workspace-level audit log",
      "Slack notifications",
      "Priority email support, < 4 business hours",
    ],
    shader: { colorA: "#22d3ee", colorB: "#34d399" },
  },
  {
    name: "Enterprise",
    prices: {
      monthly: { value: "Custom", suffix: "" },
      yearly: { value: "Custom", suffix: "" },
    },
    blurb:
      "Compliance, SSO, dedicated support, and paid rollout help.",
    cta: { label: "Talk to us", href: "mailto:hello@agentclash.dev" },
    features: [
      "Everything in Team, plus:",
      "SSO / SAML",
      "Org-wide audit logs",
      "Unlimited replay retention",
      "99.9% uptime SLA",
      "Dedicated support channel",
      "Custom MSA / billing terms",
    ],
    shader: { colorA: "#fbbf24", colorB: "#f97316" },
  },
];
