import { NextRequest, NextResponse } from "next/server";

const TRY_CLI_SERVICE =
  process.env.TRY_CLI_API_URL ?? process.env.NEXT_PUBLIC_TRY_CLI_API_URL ?? "http://localhost:3001";

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ slug: string }> },
) {
  const { slug } = await params;
  const res = await fetch(`${TRY_CLI_SERVICE.replace(/\/$/, "")}/badge/${slug}.svg`, {
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
