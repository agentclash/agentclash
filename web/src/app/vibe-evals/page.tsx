import type { Metadata, Viewport } from "next";
import { Suspense } from "react";
import { VibeClient } from "./vibe-client";
import "@/components/vibe/project-workspace.css";
export const metadata: Metadata = {
  title: "Vibe Evals | AgentClash",
  description:
    "Describe your AI agent. Build useful tests, see where it needs work, and improve it in one conversation.",
  robots: { index: false, follow: false },
};
// The browser bar matches the dark Graphite canvas (--vibe-bg in project-workspace.css).
// Dark is the default appearance; VibeAppearance updates the bar when the Appearance
// setting chooses Light or System.
export const viewport: Viewport = { themeColor: "#0a0a0b" };
export default function VibePage() {
  return (
    <Suspense fallback={<main className="vibe-route-fallback" />}>
      <VibeClient />
    </Suspense>
  );
}
