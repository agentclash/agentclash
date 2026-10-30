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
// The browser bar matches the Vibe canvas (--vibe-bg in project-workspace.css), not the
// site-wide #060606. VibeAppearance updates it when the Appearance setting overrides the system.
export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#faf8f5" },
    { media: "(prefers-color-scheme: dark)", color: "#12110f" },
  ],
};
export default function VibePage() {
  return (
    <Suspense fallback={<main className="vibe-route-fallback" />}>
      <VibeClient />
    </Suspense>
  );
}
