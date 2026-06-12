import type { Metadata } from "next";
import { Suspense } from "react";

import { PublicTryoutsClient } from "./tryouts-client";
import { ComingSoonDialog } from "./coming-soon-dialog";

export const metadata: Metadata = {
  title: "Free AI Agent Tryout",
  description:
    "Write what you would reject in production, run a sandboxed tryout on real work, and get a scored verdict with outputs before you deploy.",
  keywords: [
    "AI agent evaluation",
    "integrate AI into business",
    "AI workflow automation",
    "customer support AI",
    "document AI extraction",
    "contract review AI",
    "AI agent pilot",
    "enterprise AI testing",
    "AI automation ROI",
    "agent quality bar",
  ],
  alternates: {
    canonical: "/tryouts",
  },
  openGraph: {
    title: "Free AI Agent Tryout for Business Workflows | AgentClash",
    description:
      "Run a free sandboxed AI agent on support, finance, legal, and ops tasks. Set your quality bar and get a scored verdict before production.",
    url: "/tryouts",
  },
};

export default function PublicTryoutsPage() {
  return (
    <Suspense
      fallback={
        <main className="min-h-screen bg-[#131312] text-white">
          <div className="mx-auto flex min-h-screen max-w-6xl items-center justify-center px-4">
            <div className="flex items-center gap-1">
              {[0, 1, 2].map((index) => (
                <span
                  key={index}
                  className="size-1.5 rounded-full bg-white/30 animate-pulse"
                  style={{ animationDelay: `${index * 180}ms` }}
                />
              ))}
            </div>
          </div>
        </main>
      }
    >
      <PublicTryoutsClient />
      <ComingSoonDialog />
    </Suspense>
  );
}
