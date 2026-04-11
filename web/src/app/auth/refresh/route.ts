import { NextRequest, NextResponse } from "next/server";
import { getIronSession } from "iron-session";
import { getWorkOSClient } from "@/lib/auth/workos";
import { getWorkOSConfig } from "@/lib/auth/config";
import { getSessionOptions, SESSION_COOKIE_NAME } from "@/lib/auth/session";
import type { SessionData } from "@/lib/auth/session";

/**
 * GET /auth/refresh
 *
 * Refreshes an expired WorkOS access token using the refresh token
 * stored in the session cookie. Redirects back to the return URL.
 *
 * This exists as a Route Handler because Server Components cannot
 * write cookies in Next.js 16.
 */
export async function GET(request: NextRequest) {
  const returnTo = request.nextUrl.searchParams.get("returnTo") || "/dashboard";

  try {
    // Read the session from the raw cookie value. We can't use
    // getIronSession with request.cookies (type mismatch), so we
    // read via a temporary response, copy the cookie, and unseal.
    const tempResponse = new Response();
    const rawCookie = request.cookies.get(SESSION_COOKIE_NAME);
    if (rawCookie) {
      tempResponse.headers.set("cookie", `${SESSION_COOKIE_NAME}=${rawCookie.value}`);
    }

    // Use getIronSession with a NextResponse to read+write.
    const response = NextResponse.redirect(new URL(returnTo, request.url));

    // Copy the existing session cookie to the response so iron-session
    // can read it, then overwrite with the refreshed data.
    if (rawCookie) {
      response.cookies.set(SESSION_COOKIE_NAME, rawCookie.value);
    }

    const session = await getIronSession<{ data?: SessionData }>(
      response.cookies,
      getSessionOptions(),
    );

    if (!session.data || session.data.mode !== "workos" || !session.data.refreshToken) {
      return NextResponse.redirect(new URL("/auth/sign-out", request.url));
    }

    const workos = getWorkOSClient();
    const { clientId } = getWorkOSConfig();

    const refreshed = await workos.userManagement.authenticateWithRefreshToken({
      clientId,
      refreshToken: session.data.refreshToken,
    });

    session.data = {
      mode: "workos",
      accessToken: refreshed.accessToken,
      refreshToken: refreshed.refreshToken,
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    };
    await session.save();

    return response;
  } catch {
    return NextResponse.redirect(new URL("/auth/sign-out", request.url));
  }
}
