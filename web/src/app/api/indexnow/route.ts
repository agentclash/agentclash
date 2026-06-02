import { NextResponse } from "next/server";
import { buildUrlList, submitIndexNow } from "@/lib/indexnow";

// Pings IndexNow with the current sitemap URLs. Driven by a Vercel Cron (see
// vercel.json). When CRON_SECRET is set, Vercel sends it as a Bearer token and
// we require it, so the endpoint isn't world-triggerable.
export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET(request: Request): Promise<Response> {
  const cronSecret = process.env.CRON_SECRET;
  if (cronSecret) {
    const auth = request.headers.get("authorization");
    if (auth !== `Bearer ${cronSecret}`) {
      return NextResponse.json(
        { ok: false, error: "unauthorized" },
        { status: 401 },
      );
    }
  }

  const urlList = buildUrlList();
  if (urlList.length === 0) {
    return NextResponse.json({ ok: true, submitted: 0 });
  }

  const { status, body } = await submitIndexNow(urlList);

  // IndexNow answers 200 (accepted) or 202 (accepted, key validation pending).
  if (status === 200 || status === 202) {
    return NextResponse.json({
      ok: true,
      submitted: urlList.length,
      indexnowStatus: status,
    });
  }

  // Surface failures (e.g. 403 key/keyLocation mismatch) in Vercel function logs.
  console.error(
    `IndexNow ping failed: status=${status} body=${body.slice(0, 200)}`,
  );
  return NextResponse.json(
    {
      ok: false,
      submitted: urlList.length,
      indexnowStatus: status,
      indexnowBody: body.slice(0, 500),
    },
    { status: 502 },
  );
}
