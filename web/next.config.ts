import type { NextConfig } from "next";

// PostHog reverse-proxy upstreams. Default to PostHog US Cloud; override via
// env if the project lives in the EU region.
const POSTHOG_INGEST = process.env.POSTHOG_CLOUD_HOST ?? "https://us.i.posthog.com";
const POSTHOG_ASSETS = process.env.POSTHOG_ASSETS_HOST ?? "https://us-assets.i.posthog.com";

const nextConfig: NextConfig = {
  async redirects() {
    return [
      {
        source: "/v2",
        destination: "/",
        permanent: true,
      },
      {
        source: "/v2/:path*",
        destination: "/",
        permanent: true,
      },
    ];
  },
  // Reverse-proxy PostHog under a first-party /ingest path so ad-blockers and
  // browser tracking-protection don't silently drop client analytics.
  // posthog-js is initialised with api_host: "/ingest" to match
  // (see web/src/lib/analytics/posthog-client.ts and docs/analytics.md).
  async rewrites() {
    return [
      // PostHog proxy first — must precede the try.agentclash.dev catch-all so
      // analytics still proxies correctly on that subdomain.
      {
        source: "/ingest/static/:path*",
        destination: `${POSTHOG_ASSETS}/static/:path*`,
      },
      {
        source: "/ingest/:path*",
        destination: `${POSTHOG_INGEST}/:path*`,
      },
      // Optional: try.agentclash.dev subdomain → /try routes
      {
        source: "/:path*",
        has: [{ type: "host", value: "try.agentclash.dev" }],
        destination: "/try/:path*",
      },
    ];
  },
  // Required for the PostHog proxy — a trailing-slash redirect would break the
  // capture endpoints.
  skipTrailingSlashRedirect: true,
};

export default nextConfig;
