import type { NextConfig } from "next";

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
  async rewrites() {
    return [
      // Optional: try.agentclash.dev subdomain → /try routes
      {
        source: "/:path*",
        has: [{ type: "host", value: "try.agentclash.dev" }],
        destination: "/try/:path*",
      },
    ];
  },
};

export default nextConfig;
