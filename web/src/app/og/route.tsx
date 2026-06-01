import { ImageResponse } from "next/og";

// Dynamic Open Graph image. Pages point openGraph/twitter images at
// /og?title=...&subtitle=...&kind=... (see lib/seo.ts `ogImageUrl`) so blog
// posts, docs, and comparison pages get tailored social cards instead of one
// shared static image. Flexbox-only layout per Satori's CSS subset.
export const runtime = "nodejs";

// ImageResponse sets the image/png Content-Type header itself; a `contentType`
// export is only valid on the opengraph-image file convention, not a Route
// Handler, so it must not be exported here.
const SIZE = { width: 1200, height: 630 } as const;

// The image is fully determined by its query params, so cache it hard — social
// unfurlers (Slack, Telegram, crawlers) request these repeatedly, and
// stale-while-revalidate still lets a template change propagate.
const CACHE_CONTROL = "public, max-age=86400, stale-while-revalidate=604800";

export function GET(request: Request): Response {
  try {
    const { searchParams } = new URL(request.url);
    const title = (searchParams.get("title") || "AgentClash").slice(0, 110);
    const subtitle = (
      searchParams.get("subtitle") ||
      "Open-source AI agent evaluation platform"
    ).slice(0, 160);
    const eyebrow = (searchParams.get("kind") || "").slice(0, 40);

    return new ImageResponse(
      (
        <div
          style={{
            width: "100%",
            height: "100%",
            display: "flex",
            flexDirection: "column",
            justifyContent: "space-between",
            padding: "72px 80px",
            backgroundColor: "#060606",
            backgroundImage:
              "linear-gradient(135deg, #101012 0%, #060606 55%)",
            color: "#ffffff",
            fontFamily: "sans-serif",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: "18px" }}>
            <div
              style={{
                width: "30px",
                height: "30px",
                borderRadius: "9999px",
                border: "3px solid rgba(255,255,255,0.85)",
                display: "flex",
              }}
            />
            <div
              style={{
                fontSize: "30px",
                fontWeight: 600,
                letterSpacing: "-0.02em",
              }}
            >
              AgentClash
            </div>
          </div>

          <div
            style={{ display: "flex", flexDirection: "column", gap: "22px" }}
          >
            {eyebrow ? (
              <div
                style={{
                  fontSize: "22px",
                  textTransform: "uppercase",
                  letterSpacing: "0.22em",
                  color: "rgba(255,255,255,0.45)",
                }}
              >
                {eyebrow}
              </div>
            ) : null}
            <div
              style={{
                fontSize: "70px",
                fontWeight: 600,
                lineHeight: 1.05,
                letterSpacing: "-0.03em",
                maxWidth: "1000px",
              }}
            >
              {title}
            </div>
            <div
              style={{
                fontSize: "30px",
                lineHeight: 1.35,
                color: "rgba(255,255,255,0.6)",
                maxWidth: "940px",
              }}
            >
              {subtitle}
            </div>
          </div>

          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              fontSize: "23px",
              color: "rgba(255,255,255,0.5)",
            }}
          >
            <div>agentclash.dev</div>
            <div>Same challenge. Same tools. Agents race head-to-head.</div>
          </div>
        </div>
      ),
      { ...SIZE, headers: { "Cache-Control": CACHE_CONTROL } },
    );
  } catch {
    return new Response("Failed to generate the image", { status: 500 });
  }
}
