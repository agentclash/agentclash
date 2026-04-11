import { NextRequest, NextResponse } from "next/server";
import { sealData } from "iron-session";

const SESSION_COOKIE_NAME = "agentclash_session";
const SESSION_TTL = 60 * 60 * 8; // 8 hours

/**
 * Next.js proxy (formerly middleware) for route protection and
 * OAuth callback handling.
 *
 * The OAuth callback MUST be handled here (not in a Route Handler)
 * because middleware/proxy is the only layer in Next.js that can
 * reliably set cookies on redirect responses. Route Handlers have
 * a known issue where Set-Cookie headers are dropped during redirects.
 *
 * See: https://github.com/vercel/next.js/discussions/48434
 */
export async function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // ── OAuth callback: exchange code, set cookie, redirect ──
  if (pathname === "/auth/callback") {
    return handleOAuthCallback(request);
  }

  const hasSession = request.cookies.has(SESSION_COOKIE_NAME);

  // Already signed-in users visiting /auth/login → redirect to dashboard.
  if (pathname === "/auth/login" && hasSession) {
    return NextResponse.redirect(new URL("/dashboard", request.url));
  }

  // Protected routes: require session cookie.
  if (pathname.startsWith("/dashboard")) {
    if (!hasSession) {
      return NextResponse.redirect(new URL("/auth/login", request.url));
    }
  }

  return NextResponse.next();
}

/**
 * Handle the WorkOS OAuth callback entirely within the proxy.
 * Uses fetch directly (no SDK) because the proxy runs on Edge.
 */
async function handleOAuthCallback(request: NextRequest) {
  const code = request.nextUrl.searchParams.get("code");
  if (!code) {
    return NextResponse.redirect(
      new URL("/auth/login?error=callback_failed", request.url),
    );
  }

  const clientId = process.env.WORKOS_CLIENT_ID;
  const apiKey = process.env.WORKOS_API_KEY;

  if (!clientId || !apiKey) {
    console.error("[proxy/callback] Missing WORKOS_CLIENT_ID or WORKOS_API_KEY");
    return NextResponse.redirect(
      new URL("/auth/login?error=callback_failed", request.url),
    );
  }

  try {
    // Exchange authorization code for tokens via WorkOS API (plain fetch).
    const tokenResponse = await fetch(
      "https://api.workos.com/user_management/authenticate",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          grant_type: "authorization_code",
          client_id: clientId,
          client_secret: apiKey,
          code,
        }),
      },
    );

    if (!tokenResponse.ok) {
      const errorText = await tokenResponse.text();
      console.error("[proxy/callback] Token exchange failed:", tokenResponse.status, errorText);
      return NextResponse.redirect(
        new URL("/auth/login?error=callback_failed", request.url),
      );
    }

    const tokenData = await tokenResponse.json();
    const accessToken: string = tokenData.access_token;
    const refreshToken: string = tokenData.refresh_token;

    if (!accessToken) {
      console.error("[proxy/callback] No access_token in response");
      return NextResponse.redirect(
        new URL("/auth/login?error=callback_failed", request.url),
      );
    }

    // Seal the session data.
    const password = process.env.SESSION_SECRET;
    if (!password) {
      console.error("[proxy/callback] Missing SESSION_SECRET");
      return NextResponse.redirect(
        new URL("/auth/login?error=callback_failed", request.url),
      );
    }

    const sealed = await sealData(
      {
        data: {
          mode: "workos",
          accessToken,
          refreshToken: refreshToken || "",
          expiresAt: Math.floor(Date.now() / 1000) + 3600,
        },
      },
      { password, ttl: SESSION_TTL },
    );

    // Set cookie on the redirect response — this is reliable in proxy/middleware.
    const response = NextResponse.redirect(new URL("/dashboard", request.url));
    response.cookies.set(SESSION_COOKIE_NAME, sealed, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "lax",
      path: "/",
      maxAge: SESSION_TTL,
    });

    return response;
  } catch (err) {
    console.error("[proxy/callback] Error:", err);
    return NextResponse.redirect(
      new URL("/auth/login?error=callback_failed", request.url),
    );
  }
}

export const config = {
  matcher: [
    "/auth/login",
    "/auth/callback",
    "/dashboard/:path*",
  ],
};
