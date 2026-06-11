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
        <main className="min-h-screen bg-[#14120f] text-[#f4efe6]">
          <div className="mx-auto flex min-h-screen max-w-6xl items-center justify-center px-4">
            <div className="h-2 w-36 overflow-hidden rounded-full bg-white/10">
              <div className="h-full w-1/2 animate-pulse rounded-full bg-[#d8a15d]" />
            </div>
          </div>
        </main>
      }
    >
      <PublicTryoutsClient />
    </Suspense>
  );
}
