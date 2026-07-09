import type { MetadataRoute } from "next";

// AI crawlers we explicitly welcome. Two families, both allowed on purpose:
//   - answer/search bots (OAI-SearchBot, PerplexityBot, Claude-SearchBot, …)
//     power live, cited AI answers — being crawlable here = being citable.
//   - training bots (GPTBot, ClaudeBot, Google-Extended, CCBot, …) feed future
//     models, so the next generation "knows" AgentClash and can recommend it.
// For an open-source tool, broad allow maximizes discoverability. Default
// "*: allow /" already lets these through; listing them is a deliberate,
// documented signal of intent and a single place to change posture later.
export const AI_CRAWLERS = [
  "GPTBot",
  "OAI-SearchBot",
  "ChatGPT-User",
  "ClaudeBot",
  "Claude-User",
  "Claude-SearchBot",
  "PerplexityBot",
  "Perplexity-User",
  "Google-Extended",
  "CCBot",
  "Bytespider",
  "meta-externalagent",
  "Applebot-Extended",
  "Amazonbot",
] as const;

// App-shell and auth surfaces that should not compete for crawl budget.
// Public marketing + docs stay fully crawlable.
export const APP_SHELL_DISALLOW = [
  "/dashboard",
  "/workspaces/",
  "/orgs/",
  "/auth/",
  "/invites/",
  "/github/",
  "/share/",
] as const;

export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: "*",
        allow: "/",
        disallow: [...APP_SHELL_DISALLOW],
      },
      ...AI_CRAWLERS.map((userAgent) => ({
        userAgent,
        allow: "/",
        disallow: [...APP_SHELL_DISALLOW],
      })),
    ],
    sitemap: "https://www.agentclash.dev/sitemap.xml",
  };
}
