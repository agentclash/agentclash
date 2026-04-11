import { NextRequest, NextResponse } from "next/server";
import { sealData } from "iron-session";
import { getWorkOSClient } from "@/lib/auth/workos";
import { getWorkOSConfig, getSessionSecret } from "@/lib/auth/config";

const COOKIE_NAME = "agentclash_session";
const SESSION_TTL = 60 * 60 * 8;

/**
 * GET /auth/callback
 *
 * Handles the WorkOS OAuth callback. Exchanges the authorization code
 * for tokens, then returns a 200 HTML page that sets the cookie and
 * redirects to /dashboard via meta refresh.
 *
 * We cannot use a 307 redirect here because the browser arrives at
 * this URL from a cross-site redirect (WorkOS). SameSite=Lax cookies
 * on a 307 in a cross-site chain are silently dropped by browsers.
 * Returning a 200 page makes this a "top-level navigation" so the
 * browser commits the Set-Cookie before navigating to /dashboard.
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

    const password = getSessionSecret();
    const sealed = await sealData(
      {
        data: {
          mode: "workos",
          accessToken: authResponse.accessToken,
          refreshToken: authResponse.refreshToken,
          expiresAt: Math.floor(Date.now() / 1000) + 3600,
        },
      },
      { password, ttl: SESSION_TTL },
    );

    // Return a 200 HTML page with Set-Cookie. The browser stores the
    // cookie on this same-origin 200 response, then the meta refresh
    // navigates to /dashboard as a fresh top-level request with the cookie.
    const html = `<!DOCTYPE html>
<html><head>
<meta http-equiv="refresh" content="0;url=/dashboard">
</head><body></body></html>`;

    return new Response(html, {
      status: 200,
      headers: {
        "Content-Type": "text/html",
        "Set-Cookie": `${COOKIE_NAME}=${sealed}; Path=/; Max-Age=${SESSION_TTL}; HttpOnly; SameSite=Lax`,
      },
    });
  } catch {
    return NextResponse.redirect(
      new URL("/auth/login?error=callback_failed", request.url),
    );
  }
}
