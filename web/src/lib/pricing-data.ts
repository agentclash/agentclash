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
      "Hosted, no ops. Generous enough to actually evaluate the product on your task.",
    cta: { label: "Start your first race", href: "/auth/login" },
    features: [
      "1 seat, 1 workspace",
      "25 races / month",
      "Up to 4 models per race",
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
        suffix: "/ seat / month",
        note: "Billed monthly",
      },
      yearly: {
        value: "$39",
        suffix: "/ seat / month",
        note: "Billed annually · $468 / seat / yr",
      },
    },
    blurb:
      "For teams running real evals against real production tasks. Five seats minimum.",
    cta: {
      label: "Start free 45-day trial",
      href: "/auth/login?plan=pro",
      primary: true,
      sublabel: "No credit card required",
    },
    features: [
      "Everything in Free, plus:",
      "500 races / seat / month",
      "Up to 8 models per race",
      "30-day replay retention",
      "Hosted sandbox with included credit",
      "Private challenge packs",
      "CI integration (GitHub Actions, webhooks)",
      "3 concurrent races",
      "Email support, < 1 business day",
    ],
    shader: { colorA: "#a78bfa", colorB: "#ec4899" },
  },
  {
    name: "Team",
    prices: {
      monthly: {
        value: "$100",
        suffix: "/ seat / month",
        note: "Billed monthly",
      },
      yearly: {
        value: "$80",
        suffix: "/ seat / month",
        note: "Billed annually · $960 / seat / yr",
      },
    },
    blurb:
      "For teams running evals across multiple products and surfaces.",
    cta: {
      label: "Start free 45-day trial",
      href: "/auth/login?plan=team",
      sublabel: "No credit card required",
    },
    features: [
      "Everything in Pro, plus:",
      "2,000 races / seat / month",
      "Up to 12 models per race",
      "90-day replay retention",
      "10 concurrent races",
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
      "Compliance, SSO, dedicated support. 45-day pilot available — no card needed.",
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
