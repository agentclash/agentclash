export type PublicRouteCoverage = {
  route: string;
  adapter: "registry" | "dynamic-publication";
};

export const PUBLIC_PAGE_ROUTE_COVERAGE: PublicRouteCoverage[] = [
  { route: "/", adapter: "registry" },
  { route: "/agent-evals", adapter: "registry" },
  { route: "/agent-evaluation-framework", adapter: "registry" },
  { route: "/agent-opportunity", adapter: "registry" },
  { route: "/agent-reliability-benchmark", adapter: "registry" },
  { route: "/agent-trajectory-evaluation", adapter: "registry" },
  { route: "/agentic-self-instruct", adapter: "registry" },
  { route: "/ai-agent-benchmark", adapter: "registry" },
  { route: "/ai-agent-testing", adapter: "registry" },
  { route: "/benchmarks", adapter: "registry" },
  { route: "/benchmarks/[slug]", adapter: "registry" },
  { route: "/blog", adapter: "registry" },
  { route: "/blog/[slug]", adapter: "registry" },
  { route: "/changelog", adapter: "registry" },
  { route: "/changelog/[slug]", adapter: "registry" },
  { route: "/ci-cd-agent-evaluation", adapter: "registry" },
  { route: "/compare", adapter: "registry" },
  { route: "/compare/[competitor]", adapter: "registry" },
  { route: "/docs/[[...slug]]", adapter: "registry" },
  { route: "/enterprise", adapter: "registry" },
  { route: "/features", adapter: "registry" },
  { route: "/features/[slug]", adapter: "registry" },
  { route: "/glossary", adapter: "registry" },
  { route: "/glossary/[slug]", adapter: "registry" },
  { route: "/industries", adapter: "registry" },
  { route: "/industries/[slug]", adapter: "registry" },
  { route: "/llm-agent-evaluation", adapter: "registry" },
  { route: "/open-source-ai-agent-evaluation", adapter: "registry" },
  { route: "/platform/agent-evaluation", adapter: "registry" },
  { route: "/platform/agent-regression-testing", adapter: "registry" },
  { route: "/platform/datasmith", adapter: "registry" },
  { route: "/pricing", adapter: "registry" },
  { route: "/publications", adapter: "registry" },
  { route: "/publications/[id]", adapter: "dynamic-publication" },
  { route: "/resources/eval-checklist", adapter: "registry" },
  { route: "/services", adapter: "registry" },
  { route: "/synthetic-data-generation-agents", adapter: "registry" },
  { route: "/team", adapter: "registry" },
  { route: "/trace-to-dataset", adapter: "registry" },
  { route: "/try", adapter: "registry" },
  { route: "/try/[slug]", adapter: "registry" },
  { route: "/tryouts", adapter: "registry" },
  { route: "/use-cases", adapter: "registry" },
  { route: "/use-cases/[slug]", adapter: "registry" },
  { route: "/why", adapter: "registry" },
];

export const REVIEWED_PUBLIC_PAGE_EXCLUSIONS = [
  {
    route: "/vibe-evals",
    reason: "Noindex application with private session conversations and evaluation evidence; not a public content adapter.",
  },
  {
    route: "/resources/eval-checklist/thank-you",
    reason: "Noindex conversion confirmation that contains no unique public content.",
  },
  {
    route: "/share/[token]",
    reason: "Private capability-token resource. Publications use non-secret share record IDs.",
  },
  {
    route: "/share/[token]/embed",
    reason: "Private capability-token embed inherited from the noindex share layout.",
  },
] as const;

const PRIVATE_PAGE_PREFIXES = [
  "/auth",
  "/dashboard",
  "/github",
  "/invites",
  "/onboard",
  "/orgs",
  "/workspaces",
] as const;

function matchesRouteFamily(route: string, prefix: string): boolean {
  return route === prefix || route.startsWith(`${prefix}/`);
}

export function publicPageRouteIsCovered(route: string): boolean {
  return (
    PUBLIC_PAGE_ROUTE_COVERAGE.some((entry) => entry.route === route) ||
    REVIEWED_PUBLIC_PAGE_EXCLUSIONS.some((entry) => entry.route === route) ||
    PRIVATE_PAGE_PREFIXES.some((prefix) => matchesRouteFamily(route, prefix))
  );
}
