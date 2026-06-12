import { NextResponse } from "next/server";
import {
  AgentOpportunityError,
  analyzeAgentOpportunity,
  fetchCompanySnapshot,
  normalizePublicUrl,
  type AgentOpportunityInput,
} from "@/lib/agent-opportunity";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const RATE_LIMIT_WINDOW_MS = 10 * 60 * 1000;
const RATE_LIMIT_MAX_REQUESTS = 5;
const RATE_LIMIT_MAX_BUCKETS = 10000;
const rateLimitBuckets = new Map<string, { count: number; resetAt: number }>();

function optionalString(value: unknown, maxLength = 160): string | undefined {
  if (typeof value !== "string") return undefined;
  const trimmed = value.trim();
  return trimmed ? trimmed.slice(0, maxLength) : undefined;
}

function getClientKey(request: Request) {
  const forwardedFor = request.headers.get("x-forwarded-for");
  if (forwardedFor) {
    const proxyAppendedIp = forwardedFor.split(",").pop()?.trim();
    if (proxyAppendedIp) return proxyAppendedIp;
  }
  return request.headers.get("x-real-ip") ?? "unknown";
}

function pruneRateLimitBuckets(now: number) {
  for (const [key, bucket] of rateLimitBuckets) {
    if (bucket.resetAt <= now) rateLimitBuckets.delete(key);
  }

  while (rateLimitBuckets.size >= RATE_LIMIT_MAX_BUCKETS) {
    const oldestKey = rateLimitBuckets.keys().next().value as string | undefined;
    if (!oldestKey) break;
    rateLimitBuckets.delete(oldestKey);
  }
}

function rateLimit(request: Request) {
  const now = Date.now();
  pruneRateLimitBuckets(now);

  const key = getClientKey(request);
  const existing = rateLimitBuckets.get(key);
  if (!existing || existing.resetAt <= now) {
    rateLimitBuckets.set(key, {
      count: 1,
      resetAt: now + RATE_LIMIT_WINDOW_MS,
    });
    return null;
  }

  if (existing.count >= RATE_LIMIT_MAX_REQUESTS) {
    return NextResponse.json(
      {
        ok: false,
        code: "rate_limited",
        error: "Too many report requests. Please try again later.",
      },
      {
        status: 429,
        headers: {
          "retry-after": String(Math.ceil((existing.resetAt - now) / 1000)),
        },
      },
    );
  }

  existing.count += 1;
  return null;
}

function errorResponse(error: unknown) {
  if (error instanceof AgentOpportunityError) {
    return NextResponse.json(
      { ok: false, code: error.code, error: error.message },
      { status: error.status },
    );
  }

  console.error("Unexpected agent opportunity error:", error);
  return NextResponse.json(
    {
      ok: false,
      code: "internal_error",
      error: "We could not generate the report.",
    },
    { status: 500 },
  );
}

export async function POST(request: Request) {
  try {
    const limited = rateLimit(request);
    if (limited) return limited;

    const payload = await request.json().catch(() => null);
    if (!payload || typeof payload !== "object") {
      return NextResponse.json(
        { ok: false, code: "invalid_request", error: "Invalid request body." },
        { status: 400 },
      );
    }

    const rawUrl = optionalString((payload as { url?: unknown }).url, 2048);
    if (!rawUrl) {
      return NextResponse.json(
        { ok: false, code: "invalid_url", error: "Enter a company URL." },
        { status: 400 },
      );
    }

    const input: AgentOpportunityInput = {
      url: rawUrl,
      companySize: optionalString(
        (payload as { companySize?: unknown }).companySize,
      ),
      currentPain: optionalString((payload as { currentPain?: unknown }).currentPain),
      monthlySupportVolume: optionalString(
        (payload as { monthlySupportVolume?: unknown }).monthlySupportVolume,
      ),
    };
    const normalizedUrl = await normalizePublicUrl(input.url);

    if (!process.env.OPENAI_API_KEY) {
      throw new AgentOpportunityError(
        "openai_not_configured",
        "OpenAI is not configured for this endpoint.",
        503,
      );
    }

    const snapshot = await fetchCompanySnapshot(normalizedUrl);
    const report = await analyzeAgentOpportunity({
      input: { ...input, url: normalizedUrl },
      snapshot,
    });

    return NextResponse.json({ ok: true, report });
  } catch (error) {
    return errorResponse(error);
  }
}
