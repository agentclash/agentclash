import type { MetadataRoute } from "next";

// Web app manifest. Next.js serves this at /manifest.webmanifest and auto-links
// it from every page. Reuses the android-chrome icons already in public/.
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "AgentClash — Open-source AI Agent Evaluation Platform",
    short_name: "AgentClash",
    description:
      "Race AI agents head-to-head on real tasks with sandboxed tools, replay, scorecards, and CI regression gates.",
    start_url: "/",
    display: "standalone",
    background_color: "#060606",
    theme_color: "#060606",
    icons: [
      {
        src: "/android-chrome-192x192.png",
        sizes: "192x192",
        type: "image/png",
      },
      {
        src: "/android-chrome-512x512.png",
        sizes: "512x512",
        type: "image/png",
      },
    ],
  };
}
