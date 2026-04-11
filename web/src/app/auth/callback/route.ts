import { NextRequest, NextResponse } from "next/server";
import { getWorkOSClient } from "@/lib/auth/workos";
import { getWorkOSConfig } from "@/lib/auth/config";
import { setSessionCookie } from "@/lib/auth/session";

/**
 * GET /auth/callback
 *
 * Handles the WorkOS OAuth callback. Exchanges the authorization code
 * for access + refresh tokens, stores them in an encrypted session cookie,
 * and redirects to the dashboard.
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

    // Build the redirect response, then set the session cookie on it.
    // We cannot use cookies() from next/headers in Route Handlers that
    // return NextResponse.redirect() — must set cookies on the response.
    const response = NextResponse.redirect(new URL("/dashboard", request.url));
    const expiresIn = 3600; // 1 hour (WorkOS default access token lifetime)
    await setSessionCookie(response.cookies, {
      mode: "workos",
      accessToken: authResponse.accessToken,
      refreshToken: authResponse.refreshToken,
      expiresAt: Math.floor(Date.now() / 1000) + expiresIn,
    });

    return response;
  } catch {
    return NextResponse.redirect(
      new URL("/auth/login?error=callback_failed", request.url),
    );
  }
}
