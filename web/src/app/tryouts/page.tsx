import type { Metadata } from "next";
import { Suspense } from "react";

import { PublicTryoutsClient } from "./tryouts-client";

export const metadata: Metadata = {
  title: "Try an AI agent on office work",
  description:
    "Run a public AgentClash tryout on office-work tasks like PDFs, spreadsheets, meeting notes, and inbox triage.",
  alternates: {
    canonical: "/tryouts",
  },
};

export default function PublicTryoutsPage() {
  return (
    <Suspense
      fallback={
        <main className="min-h-screen bg-black text-white">
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
    </Suspense>
  );
}
