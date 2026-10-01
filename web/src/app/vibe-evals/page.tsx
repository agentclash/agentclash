import type { Metadata } from "next";
import { Suspense } from "react";
import { VibeClient } from "./vibe-client";
export const metadata: Metadata = {
  title: "Vibe Evals | AgentClash",
  description:
    "Describe your AI agent. Build a useful evaluation, see where it needs work, and improve it in one conversation.",
  robots: { index: false, follow: false },
};
export default function VibePage() {
  return (
    <Suspense fallback={<main className="min-h-dvh bg-background" />}>
      <VibeClient />
    </Suspense>
  );
}
