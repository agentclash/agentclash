import { NextRequest, NextResponse } from "next/server";
import { getIronSession } from "iron-session";
import { getWorkOSClient } from "@/lib/auth/workos";
import { getWorkOSConfig } from "@/lib/auth/config";
import { getSessionOptions } from "@/lib/auth/session";
import type { SessionData } from "@/lib/auth/session";

/**
 * GET /auth/callback
 *
 * Handles the WorkOS OAuth callback. Exchanges the authorization code
 * for access + refresh tokens, stores them in an encrypted session cookie,
 * and redirects to the dashboard.
 *
 * Uses getIronSession with response.cookies because Route Handlers that
 * return NextResponse.redirect() cannot use cookies() from next/headers.
 */
export async function GET(request: NextRequest) {
  const code = request.nextUrl.searchParams.get("code");
  if (!code) {
    return NextResponse.redirect(
      new URL("/auth/login?error=callback_failed", request.url),
    );
  }

  try {
    const workos = getWorkOSClient();
    const { clientId } = getWorkOSConfig();

    const authResponse = await workos.userManagement.authenticateWithCode({
      clientId,
      code,
    });

    const response = NextResponse.redirect(new URL("/dashboard", request.url));

    // Set session cookie on the response directly (not via cookies() API).
    const session = await getIronSession<{ data?: SessionData }>(
      response.cookies,
      getSessionOptions(),
    );
    session.data = {
      mode: "workos",
      accessToken: authResponse.accessToken,
      refreshToken: authResponse.refreshToken,
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    };
    await session.save();

    return response;
  } catch {
    return NextResponse.redirect(
      new URL("/auth/login?error=callback_failed", request.url),
    );
  }
}
