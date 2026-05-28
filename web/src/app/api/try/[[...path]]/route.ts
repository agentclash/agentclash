import { NextRequest, NextResponse } from "next/server";

const TRY_CLI_SERVICE =
  process.env.TRY_CLI_API_URL ?? process.env.NEXT_PUBLIC_TRY_CLI_API_URL ?? "http://localhost:3001";

async function proxy(req: NextRequest, pathSegments: string[]): Promise<NextResponse> {
  const path = pathSegments.join("/");
  const url = new URL(req.url);
  const target = `${TRY_CLI_SERVICE.replace(/\/$/, "")}/api/${path}${url.search}`;

  const headers = new Headers();
  const contentType = req.headers.get("content-type");
  if (contentType) headers.set("content-type", contentType);
  const forwarded = req.headers.get("x-forwarded-for");
  if (forwarded) headers.set("x-forwarded-for", forwarded);

  const init: RequestInit = {
    method: req.method,
    headers,
    cache: "no-store",
  };
  if (req.method !== "GET" && req.method !== "HEAD") {
    init.body = await req.text();
  }

  const res = await fetch(target, init);
  const body = await res.arrayBuffer();
  return new NextResponse(body, {
    status: res.status,
    headers: {
      "Content-Type": res.headers.get("content-type") ?? "application/json",
    },
  });
}

export async function GET(
  req: NextRequest,
  { params }: { params: Promise<{ path?: string[] }> },
) {
  const { path = [] } = await params;
  return proxy(req, path);
}

export async function POST(
  req: NextRequest,
  { params }: { params: Promise<{ path?: string[] }> },
) {
  const { path = [] } = await params;
  return proxy(req, path);
}

export async function DELETE(
  req: NextRequest,
  { params }: { params: Promise<{ path?: string[] }> },
) {
  const { path = [] } = await params;
  return proxy(req, path);
}
