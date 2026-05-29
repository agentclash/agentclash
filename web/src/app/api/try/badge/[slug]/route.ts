import { NextRequest, NextResponse } from "next/server";

// Server-side backend URL. Do NOT fall back to NEXT_PUBLIC_TRY_CLI_API_URL —
// that is the public proxy path (`/api/try`), which would loop back here.
const TRY_CLI_SERVICE = process.env.TRY_CLI_API_URL ?? "http://localhost:3001";

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ slug: string }> },
) {
  const { slug } = await params;
  // Badge URLs are published as `/api/try/badge/{slug}.svg`, so the dynamic
  // segment includes the `.svg` extension — strip it before building the
  // upstream path (otherwise we'd request `/badge/{slug}.svg.svg`).
  const cleanSlug = slug.replace(/\.svg$/, "");
  const res = await fetch(`${TRY_CLI_SERVICE.replace(/\/$/, "")}/badge/${cleanSlug}.svg`, {
    cache: "force-cache",
    next: { revalidate: 3600 },
  });
  const svg = await res.text();
  return new NextResponse(svg, {
    status: res.status,
    headers: {
      "Content-Type": "image/svg+xml",
      "Cache-Control": "public, max-age=3600",
    },
  });
}
