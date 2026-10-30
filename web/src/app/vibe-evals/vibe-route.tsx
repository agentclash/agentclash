"use client";
import dynamic from "next/dynamic";
import "@/components/vibe/project-workspace.css";

// useSearchParams does not suspend in dev. Keep the canvas visible until the client bundle loads.
export const VibeRoute = dynamic(() => import("./vibe-client").then(module => module.VibeClient), {
  ssr: false,
  loading: () => <main className="vibe-route-fallback" />,
});
