import type { Metadata } from "next";
import type { ReactNode } from "react";
import { CalEmbedInit } from "@/components/marketing/cal-embed-init";

// Preview marketing tree. Kept out of the production index so it can't
// compete with / for brand queries until it's promoted.
export const metadata: Metadata = {
  title: {
    default: "AgentClash — Head-to-head AI agent evaluation & regression testing",
    template: "%s — AgentClash",
  },
  description:
    "AgentClash is the open-source AI agent evaluation platform. Race models head-to-head on real tasks with the same tools, same constraints, and live scoring. Wire into CI to catch regressions before you ship.",
  keywords: [
    "AI agent evaluation",
    "agent evaluation platform",
    "agent regression testing",
    "LLM evaluation",
    "multi-turn agent evaluation",
    "head-to-head AI benchmarks",
    "CI agent testing",
    "agent eval CI/CD",
    "self-hosted agent evaluation",
    "RAG agent evaluation",
    "coding agent benchmarking",
    "Agent Clash",
    "AgentClash",
  ],
  robots: {
    index: false,
    follow: true,
    googleBot: { index: false, follow: true },
  },
  alternates: {
    canonical: "/v2",
  },
};

export default function V2Layout({ children }: { children: ReactNode }) {
  return (
    <>
      <CalEmbedInit />
      {children}
    </>
  );
}
