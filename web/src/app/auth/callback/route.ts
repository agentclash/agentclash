import { NextRequest } from "next/server";
import { sealData } from "iron-session";
import { getWorkOSClient } from "@/lib/auth/workos";
import { getWorkOSConfig, getSessionSecret } from "@/lib/auth/config";

const COOKIE_NAME = "agentclash_session";
const SESSION_TTL = 60 * 60 * 8;

export async function GET(request: NextRequest) {
  const code = request.nextUrl.searchParams.get("code");
  if (!code) {
    return errorPage("Missing authorization code");
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
          mode: "workos" as const,
          accessToken: authResponse.accessToken,
          refreshToken: authResponse.refreshToken,
          expiresAt: Math.floor(Date.now() / 1000) + 3600,
        },
      },
      { password, ttl: SESSION_TTL },
    );

    // Return a full HTML page (not a redirect) so the browser commits
    // the Set-Cookie on this same-origin 200 response. JavaScript then
    // navigates to /dashboard as a completely new top-level request.
    //
    // Why not a redirect (307)?
    // The browser arrives here from a cross-site redirect chain (WorkOS).
    // Browsers silently ignore Set-Cookie on 3xx responses during
    // cross-site redirect chains (SameSite=Lax enforcement).
    //
    // Why not meta refresh?
    // Some browsers process meta refresh before fully committing cookies
    // from the response headers.
    return new Response(
      `<!DOCTYPE html>
<html>
<head><title>Signing in...</title></head>
<body>
<script>
document.cookie = "";
window.location.replace("/dashboard");
</script>
<noscript><a href="/dashboard">Click here to continue</a></noscript>
</body>
</html>`,
      {
        status: 200,
        headers: {
          "Content-Type": "text/html; charset=utf-8",
          "Cache-Control": "no-store",
          "Set-Cookie": [
            `${COOKIE_NAME}=${sealed}`,
            `Path=/`,
            `Max-Age=${SESSION_TTL}`,
            `HttpOnly`,
            `SameSite=Lax`,
          ].join("; "),
        },
      },
    );
  } catch (err) {
    console.error("[callback] ERROR:", err);
    return errorPage("Authentication failed");
  }
}

function errorPage(message: string) {
  return new Response(
    `<!DOCTYPE html>
<html>
<head><meta http-equiv="refresh" content="2;url=/auth/login?error=callback_failed"></head>
<body><p>${message}. Redirecting...</p></body>
</html>`,
    { status: 200, headers: { "Content-Type": "text/html; charset=utf-8" } },
  );
}
