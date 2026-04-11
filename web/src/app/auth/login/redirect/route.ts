import { NextResponse } from "next/server";
import { getWorkOSClient } from "@/lib/auth/workos";
import { getWorkOSConfig } from "@/lib/auth/config";

/**
 * GET /auth/login/redirect
 *
 * Generates a WorkOS authorization URL and redirects the browser to it.
 * WorkOS will then redirect back to /auth/callback with an authorization code.
 *
 * Uses NextResponse.redirect (not next/navigation redirect) because the
 * target is an external URL (WorkOS).
 */
export async function GET() {
  const workos = getWorkOSClient();
  const { clientId, redirectUri } = getWorkOSConfig();

  const authorizationUrl = workos.userManagement.getAuthorizationUrl({
    clientId,
    redirectUri,
    provider: "authkit",
  });

  return NextResponse.redirect(authorizationUrl);
}
